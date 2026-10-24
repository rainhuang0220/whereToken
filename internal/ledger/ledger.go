// Package ledger is the durable usage ledger.
//
// The scan index is a disposable cache of file offsets and parsed blobs.
// This database is the copy of confirmed, normalized usage that must survive
// a deleted source log, `wheretoken rebuild`, and an index schema wipe.
//
// Ordinary CommitUsage inserts a new identity or raises a stored component
// that the new observation already covers. It does not lower a stored count.
// A lower figure is Correct: a higher parser version, a reason, and the
// replacement event, recorded in revisions. The same correction twice is a
// no-op. Revert restores the previous payload once.
//
// Facts store counts, time, model, vendor, quality, derivation, and the
// stable identity. They do not store prompts, transcripts, workspaces, or paths.
package ledger

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"

	_ "modernc.org/sqlite"
)

// ParserVersion is the accounting version stamped on facts this binary writes.
// Correct refuses a version that is not strictly newer than the stored one.
const ParserVersion = 1

const schemaVersion = 1

// ErrCorrectionRejected means the correction had no reason, no stable
// identity, or a parser version that is not newer than the stored fact.
var ErrCorrectionRejected = errors.New("ledger correction rejected")

// Status is the machine-readable ledger summary. Facts is a real count,
// including zero when the ledger opened and is empty. Callers print
// unavailable when Open fails; they do not substitute that failure with zero.
type Status struct {
	Facts         int
	Tokens        int64
	ImportGaps    int
	ImportUnkeyed int
	Imported      bool
	Revisions     int
}

// Breakdown is the checksum view used to prove a restore. Days are UTC
// dates of the stored instant. Aggregation for the product calendar still
// uses the event timestamp in the owner's local zone.
type Breakdown struct {
	Facts    int
	Tokens   int64
	BySource map[string]int64
	ByDay    map[string]int64
}

type fact struct {
	ID, Source, SessionID, RequestKey  string
	Model, Vendor, Quality, Derivation string
	TS                                 int64
	Miss, CacheRead, CacheCreate       int64
	Output, Reasoning                  int64
	Skip                               bool
	Hash                               string
}

// Ledger is one SQLite file. The scan cache is a different file.
type Ledger struct {
	db                 *sql.DB
	path               string
	mu                 sync.Mutex
	beforeImportCommit func() error
}

// PathFor is the ledger file for home. It is not under the scan cache
// directory, so index.Wipe cannot delete it. During tests the
// WHERETOKEN_LEDGER override is ignored so a developer shell cannot point
// `go test` at the real ledger.
func PathFor(h adapter.Home) (string, error) {
	if h == nil {
		return "", errors.New("ledger home is nil")
	}
	if flag.Lookup("test.v") == nil {
		if v := strings.TrimSpace(os.Getenv("WHERETOKEN_LEDGER")); v != "" {
			return v, nil
		}
	}
	var dir string
	switch runtime.GOOS {
	case "darwin":
		dir = h.AppSupport("whereToken")
	case "windows":
		dir = h.AppData("whereToken")
	default:
		dir = h.XDGData("wheretoken")
	}
	return filepath.Join(dir, "ledger.v1.db"), nil
}

// Open creates the ledger if needed. A schema value this binary does not
// know is an error. The tables are not dropped.
func Open(path string) (*Ledger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", adapter.SQLiteURI(path, "mode=rwc"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	l := &Ledger{db: db, path: path}
	if err := l.configure(); err != nil {
		db.Close()
		return nil, err
	}
	return l, nil
}

func (l *Ledger) configure() error {
	var mode string
	if err := l.db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil {
		return err
	}
	if _, err := l.db.Exec(`PRAGMA synchronous=FULL`); err != nil {
		return err
	}
	if _, err := l.db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		return err
	}
	return l.migrate()
}

func (l *Ledger) migrate() error {
	var got string
	err := l.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema'`).Scan(&got)
	if err == nil && got != "" && got != fmt.Sprintf("%d", schemaVersion) {
		return errors.New("ledger schema mismatch")
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) && !strings.Contains(err.Error(), "no such table") {
		// First open has no meta table. Any other error is returned.
		if !strings.Contains(err.Error(), "no such") {
			return err
		}
	}
	_, err = l.db.Exec(`
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS facts (
  id TEXT PRIMARY KEY,
  source TEXT NOT NULL,
  session_id TEXT NOT NULL,
  request_key TEXT NOT NULL,
  model TEXT NOT NULL,
  vendor TEXT NOT NULL,
  ts INTEGER NOT NULL,
  miss INTEGER NOT NULL,
  cache_read INTEGER NOT NULL,
  cache_create INTEGER NOT NULL,
  output INTEGER NOT NULL,
  reasoning INTEGER NOT NULL,
  quality TEXT NOT NULL,
  derivation TEXT NOT NULL,
  skip_request INTEGER NOT NULL,
  parser_version INTEGER NOT NULL,
  payload_hash TEXT NOT NULL,
  origin TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS facts_source ON facts(source);
CREATE TABLE IF NOT EXISTS revisions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  fact_id TEXT NOT NULL,
  reason TEXT NOT NULL,
  prev_parser_version INTEGER NOT NULL,
  next_parser_version INTEGER NOT NULL,
  prev_hash TEXT NOT NULL,
  next_hash TEXT NOT NULL,
  prev_model TEXT NOT NULL,
  next_model TEXT NOT NULL,
  prev_vendor TEXT NOT NULL,
  next_vendor TEXT NOT NULL,
  prev_ts INTEGER NOT NULL,
  next_ts INTEGER NOT NULL,
  prev_miss INTEGER NOT NULL,
  prev_cache_read INTEGER NOT NULL,
  prev_cache_create INTEGER NOT NULL,
  prev_output INTEGER NOT NULL,
  prev_reasoning INTEGER NOT NULL,
  next_miss INTEGER NOT NULL,
  next_cache_read INTEGER NOT NULL,
  next_cache_create INTEGER NOT NULL,
  next_output INTEGER NOT NULL,
  next_reasoning INTEGER NOT NULL,
  prev_quality TEXT NOT NULL,
  next_quality TEXT NOT NULL,
  prev_derivation TEXT NOT NULL,
  next_derivation TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS api_observations (
  source TEXT NOT NULL,
  payload_hash TEXT NOT NULL,
  tokens INTEGER NOT NULL,
  events INTEGER NOT NULL,
  parser_version INTEGER NOT NULL,
  PRIMARY KEY (source, payload_hash)
);
CREATE TABLE IF NOT EXISTS omissions (
  id TEXT PRIMARY KEY,
  reason TEXT NOT NULL
);
`)
	if err != nil {
		return err
	}
	_, err = l.db.Exec(`INSERT OR REPLACE INTO meta(key,value) VALUES('schema', ?)`, fmt.Sprintf("%d", schemaVersion))
	return err
}

// Path is the ledger file. Doctor output must not print it.
func (l *Ledger) Path() string { return l.path }

// Close releases the database.
func (l *Ledger) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}

// CommitUsage records facts for events that have a stable identity.
// Existing identities are left in place unless every stored component is
// less than or equal to the new observation and at least one is greater.
func (l *Ledger) CommitUsage(events []event.UsageEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.commitLocked(events, "scan")
}

func (l *Ledger) commitLocked(events []event.UsageEvent, origin string) error {
	reduced, _ := reduce(events)
	if len(reduced) == 0 {
		return nil
	}
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, f := range reduced {
		if err := upsertFact(tx, f, origin); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Import copies already-normalized events into the ledger once.
// A second call is a no-op. gaps is the number of cache blobs that could
// not be decoded; those tokens are not invented. The insert and the imported
// mark share one transaction.
func (l *Ledger) Import(events []event.UsageEvent, gaps int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.importedLocked() {
		return nil
	}
	reduced, unkeyed := reduce(events)
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, f := range reduced {
		if err := upsertFact(tx, f, "import"); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO meta(key,value) VALUES('imported','1')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO meta(key,value) VALUES('import_gaps',?)`, fmt.Sprintf("%d", gaps)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO meta(key,value) VALUES('import_unkeyed',?)`, fmt.Sprintf("%d", unkeyed)); err != nil {
		return err
	}
	if l.beforeImportCommit != nil {
		if err := l.beforeImportCommit(); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Imported reports whether the one-time cache import has committed.
func (l *Ledger) Imported() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.importedLocked()
}

func (l *Ledger) importedLocked() bool {
	var v string
	err := l.db.QueryRow(`SELECT value FROM meta WHERE key='imported'`).Scan(&v)
	return err == nil && v == "1"
}

// NoteAPI records a distinct account-API observation. The row is not a fact
// and Unseen will not add it to a scan. The same payload twice is one row.
func (l *Ledger) NoteAPI(events []event.UsageEvent) error {
	type acc struct {
		tokens int64
		n      int
		lines  []string
	}
	by := map[string]*acc{}
	for _, e := range events {
		if e.Derivation != event.DeriveProviderAPI || e.Source == "" {
			continue
		}
		a := by[e.Source]
		if a == nil {
			a = &acc{}
			by[e.Source] = a
		}
		a.n++
		a.tokens += e.Miss + e.CacheRead + e.CacheCreate + e.Output
		a.lines = append(a.lines, fmt.Sprintf("%s|%d|%d|%d|%d|%d", e.Model, e.Timestamp.UnixNano(), e.Miss, e.CacheRead, e.CacheCreate, e.Output))
	}
	if len(by) == 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for source, a := range by {
		sort.Strings(a.lines)
		sum := sha256.New()
		for _, line := range a.lines {
			sum.Write([]byte(line))
			sum.Write([]byte{'\n'})
		}
		hash := hex.EncodeToString(sum.Sum(nil))
		if _, err := tx.Exec(`INSERT OR IGNORE INTO api_observations(source,payload_hash,tokens,events,parser_version) VALUES(?,?,?,?,?)`,
			source, hash, a.tokens, a.n, ParserVersion); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// NoteComplete records a successful reread of the sessions in events.
// A stored fact for one of those sessions that was not in the reread is
// omitted from later totals. The fact row stays. A key that this reread did
// emit is removed from the omission list, so a reset archive and its live
// file are both kept when both were parsed.
//
// Provider API rows are not a reread of the local session. A parse error
// must not call NoteComplete: a partial event list would omit the rest.
func (l *Ledger) NoteComplete(events []event.UsageEvent) error {
	type sess struct{ source, session string }
	present := map[string]struct{}{}
	sessions := map[sess]struct{}{}
	for _, e := range events {
		if e.Derivation == event.DeriveProviderAPI || e.SkipRequest || e.Source == "" || e.SessionID == "" {
			continue
		}
		sessions[sess{e.Source, e.SessionID}] = struct{}{}
		if id, ok := Key(e); ok {
			present[id] = struct{}{}
		}
	}
	if len(sessions) == 0 && len(present) == 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for id := range present {
		if _, err := tx.Exec(`DELETE FROM omissions WHERE id=?`, id); err != nil {
			return err
		}
	}
	for s := range sessions {
		rows, err := tx.Query(`SELECT id FROM facts WHERE source=? AND session_id=?`, s.source, s.session)
		if err != nil {
			return err
		}
		var missing []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			if _, ok := present[id]; !ok {
				missing = append(missing, id)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, id := range missing {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO omissions(id, reason) VALUES(?, 'reread')`, id); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// Unseen returns stored facts whose identity was not in emitted, so a deleted
// source file still counts. A source that emitted any provider_api event is
// omitted entirely: that API response is the authority for the source and
// must not be added to local facts. Facts omitted by a later complete reread
// of the same session stay out.
func (l *Ledger) Unseen(emitted []event.UsageEvent) ([]event.UsageEvent, error) {
	seen := map[string]struct{}{}
	api := map[string]struct{}{}
	for _, e := range emitted {
		if e.Derivation == event.DeriveProviderAPI && e.Source != "" {
			api[e.Source] = struct{}{}
		}
		if id, ok := Key(e); ok {
			seen[id] = struct{}{}
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	omitted := map[string]struct{}{}
	orows, err := l.db.Query(`SELECT id FROM omissions`)
	if err != nil {
		return nil, err
	}
	for orows.Next() {
		var id string
		if err := orows.Scan(&id); err != nil {
			orows.Close()
			return nil, err
		}
		omitted[id] = struct{}{}
	}
	if err := orows.Err(); err != nil {
		orows.Close()
		return nil, err
	}
	orows.Close()
	rows, err := l.db.Query(`SELECT id, source, session_id, request_key, model, vendor, ts, miss, cache_read, cache_create, output, reasoning, quality, derivation, skip_request FROM facts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []event.UsageEvent
	for rows.Next() {
		var f fact
		var skip int
		if err := rows.Scan(&f.ID, &f.Source, &f.SessionID, &f.RequestKey, &f.Model, &f.Vendor, &f.TS, &f.Miss, &f.CacheRead, &f.CacheCreate, &f.Output, &f.Reasoning, &f.Quality, &f.Derivation, &skip); err != nil {
			return nil, err
		}
		f.Skip = skip != 0
		if _, ok := omitted[f.ID]; ok {
			continue
		}
		if _, ok := api[f.Source]; ok {
			continue
		}
		if _, ok := seen[f.ID]; ok {
			continue
		}
		out = append(out, f.event())
	}
	return out, rows.Err()
}

// Correct replaces a stored fact when parserVersion is newer and the payload
// hash differs. reason is required. A second call with the same payload does
// not write another revision.
func (l *Ledger) Correct(e event.UsageEvent, reason string, parserVersion int) (bool, error) {
	reason, err := cleanReason(reason)
	if err != nil {
		return false, err
	}
	id, ok := Key(e)
	if !ok {
		return false, ErrCorrectionRejected
	}
	next := factFrom(e, id)
	l.mu.Lock()
	defer l.mu.Unlock()
	tx, err := l.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	cur, ver, ok, err := loadFact(tx, id)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrCorrectionRejected
	}
	if next.Hash == cur.Hash {
		return false, nil
	}
	if parserVersion <= ver {
		return false, ErrCorrectionRejected
	}
	if err := insertRevision(tx, cur, next, ver, parserVersion, reason); err != nil {
		return false, err
	}
	if err := writeFact(tx, next, parserVersion); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// Revert undoes the latest non-revert correction when the fact still has
// that correction's payload. A second call is a no-op.
func (l *Ledger) Revert(factID, reason string) (bool, error) {
	reason, err := cleanReason(reason)
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(reason, "revert:") {
		reason = "revert:" + reason
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tx, err := l.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	cur, ver, ok, err := loadFact(tx, factID)
	if err != nil || !ok {
		if err == nil {
			err = ErrCorrectionRejected
		}
		return false, err
	}
	var prev fact
	var prevVer, nextVer int
	var nextHash string
	err = tx.QueryRow(`SELECT prev_parser_version, next_parser_version, prev_hash, next_hash,
		prev_model, prev_vendor, prev_ts, prev_miss, prev_cache_read, prev_cache_create, prev_output, prev_reasoning, prev_quality, prev_derivation
		FROM revisions WHERE fact_id=? AND reason NOT LIKE 'revert:%' ORDER BY id DESC LIMIT 1`, factID).Scan(
		&prevVer, &nextVer, &prev.Hash, &nextHash,
		&prev.Model, &prev.Vendor, &prev.TS, &prev.Miss, &prev.CacheRead, &prev.CacheCreate, &prev.Output, &prev.Reasoning, &prev.Quality, &prev.Derivation)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrCorrectionRejected
	}
	if err != nil {
		return false, err
	}
	if cur.Hash != nextHash || ver != nextVer {
		return false, nil
	}
	prev.ID, prev.Source, prev.SessionID, prev.RequestKey = cur.ID, cur.Source, cur.SessionID, cur.RequestKey
	prev.Skip = cur.Skip
	if err := insertRevision(tx, cur, prev, ver, prevVer, reason); err != nil {
		return false, err
	}
	if err := writeFact(tx, prev, prevVer); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// Status counts facts. Tokens are Miss+Cache Read+Cache Create+Output.
// Reasoning is not included.
func (l *Ledger) Status() (Status, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var st Status
	err := l.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(miss+cache_read+cache_create+output),0) FROM facts WHERE id NOT IN (SELECT id FROM omissions)`).Scan(&st.Facts, &st.Tokens)
	if err != nil {
		return Status{}, err
	}
	if err := l.db.QueryRow(`SELECT COUNT(*) FROM revisions`).Scan(&st.Revisions); err != nil {
		return Status{}, err
	}
	st.Imported = l.importedLocked()
	st.ImportGaps = metaInt(l.db, "import_gaps")
	st.ImportUnkeyed = metaInt(l.db, "import_unkeyed")
	return st, nil
}

// Breakdown groups tokens by source and UTC day.
func (l *Ledger) Breakdown() (Breakdown, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, err := l.db.Query(`SELECT source, ts, miss, cache_read, cache_create, output FROM facts WHERE id NOT IN (SELECT id FROM omissions)`)
	if err != nil {
		return Breakdown{}, err
	}
	defer rows.Close()
	b := Breakdown{BySource: map[string]int64{}, ByDay: map[string]int64{}}
	for rows.Next() {
		var source string
		var ts, miss, cr, cc, out int64
		if err := rows.Scan(&source, &ts, &miss, &cr, &cc, &out); err != nil {
			return Breakdown{}, err
		}
		toks := miss + cr + cc + out
		b.Facts++
		b.Tokens += toks
		b.BySource[source] += toks
		day := time.Unix(0, ts).UTC().Format("2006-01-02")
		b.ByDay[day] += toks
	}
	return b, rows.Err()
}

// Digest is a checksum of identity and payload. It does not include paths.
func (l *Ledger) Digest() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, err := l.db.Query(`SELECT id, payload_hash, miss, cache_read, cache_create, output, reasoning FROM facts ORDER BY id`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	h := sha256.New()
	for rows.Next() {
		var id, hash string
		var miss, cr, cc, out, reason int64
		if err := rows.Scan(&id, &hash, &miss, &cr, &cc, &out, &reason); err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s %s %d %d %d %d %d\n", id, hash, miss, cr, cc, out, reason)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BackupTo writes a consistent snapshot. dest must not already exist.
// The snapshot is taken with VACUUM INTO, not by copying a live database file.
func (l *Ledger) BackupTo(dest string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return vacuumInto(l.db, dest)
}

// Restore replaces dest with a consistent snapshot of backup.
// The caller must not have dest open. WAL sidecars of dest are removed so
// an interrupted writer cannot replay over the restored pages.
func Restore(backup, dest string) error {
	db, err := sql.Open("sqlite", adapter.SQLiteURI(backup, "mode=ro"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return errors.New("ledger backup unreadable")
	}
	tmp := dest + ".restore"
	_ = os.Remove(tmp)
	if err := vacuumInto(db, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	for _, p := range []string{dest, dest + "-wal", dest + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return errors.New("ledger restore failed")
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		return errors.New("ledger restore failed")
	}
	return nil
}

func vacuumInto(db *sql.DB, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return errors.New("ledger backup failed")
	}
	if _, err := os.Stat(dest); err == nil {
		return errors.New("ledger backup failed")
	}
	q := strings.ReplaceAll(dest, "'", "''")
	if _, err := db.Exec(`VACUUM INTO '` + q + `'`); err != nil {
		return errors.New("ledger backup failed")
	}
	return nil
}

func reduce(events []event.UsageEvent) (map[string]fact, int) {
	out := map[string]fact{}
	unkeyed := 0
	for _, e := range events {
		if e.Miss == 0 && e.CacheRead == 0 && e.CacheCreate == 0 && e.Output == 0 && e.Reasoning == 0 {
			continue
		}
		id, ok := Key(e)
		if !ok {
			unkeyed++
			continue
		}
		f := factFrom(e, id)
		if prev, ok := out[id]; ok {
			f = maxFact(prev, f)
		}
		out[id] = f
	}
	return out, unkeyed
}

func factFrom(e event.UsageEvent, id string) fact {
	source, session, req, _ := splitKey(id)
	f := fact{
		ID: id, Source: source, SessionID: session, RequestKey: req,
		Model: e.Model, Vendor: e.Vendor,
		TS:   e.Timestamp.UnixNano(),
		Miss: e.Miss, CacheRead: e.CacheRead, CacheCreate: e.CacheCreate,
		Output: e.Output, Reasoning: e.Reasoning,
		Quality: string(e.Quality), Derivation: e.Derivation,
		Skip: e.SkipRequest,
	}
	f.Hash = hashFact(f)
	return f
}

func splitKey(id string) (source, session, req string, ok bool) {
	parts := strings.Split(id, "\x1f")
	if len(parts) != 3 || parts[0] == "" || parts[2] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func hashFact(f fact) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%s\n%d\n%d\n%d\n%d\n%d\n%d\n%s\n%s",
		f.Model, f.Vendor, f.TS, f.Miss, f.CacheRead, f.CacheCreate, f.Output, f.Reasoning, f.Quality, f.Derivation)
	return hex.EncodeToString(h.Sum(nil))
}

func maxFact(a, b fact) fact {
	if b.Miss > a.Miss {
		a.Miss = b.Miss
	}
	if b.CacheRead > a.CacheRead {
		a.CacheRead = b.CacheRead
	}
	if b.CacheCreate > a.CacheCreate {
		a.CacheCreate = b.CacheCreate
	}
	if b.Output > a.Output {
		a.Output = b.Output
	}
	if b.Reasoning > a.Reasoning {
		a.Reasoning = b.Reasoning
	}
	if b.TS > a.TS {
		a.TS = b.TS
	}
	a.Hash = hashFact(a)
	return a
}

func (f fact) event() event.UsageEvent {
	req := f.RequestKey
	if f.SessionID != "" {
		req = f.SessionID + "\x1f" + f.RequestKey
	}
	return event.UsageEvent{
		Source:      f.Source,
		Vendor:      f.Vendor,
		SessionID:   f.SessionID,
		RequestID:   req,
		Model:       f.Model,
		Timestamp:   time.Unix(0, f.TS).UTC(),
		Miss:        f.Miss,
		CacheRead:   f.CacheRead,
		CacheCreate: f.CacheCreate,
		Output:      f.Output,
		Reasoning:   f.Reasoning,
		Quality:     event.Quality(f.Quality),
		Derivation:  f.Derivation,
		SkipRequest: f.Skip,
	}
}

func upsertFact(tx *sql.Tx, f fact, origin string) error {
	res, err := tx.Exec(`INSERT OR IGNORE INTO facts(
		id, source, session_id, request_key, model, vendor, ts,
		miss, cache_read, cache_create, output, reasoning,
		quality, derivation, skip_request, parser_version, payload_hash, origin
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.ID, f.Source, f.SessionID, f.RequestKey, f.Model, f.Vendor, f.TS,
		f.Miss, f.CacheRead, f.CacheCreate, f.Output, f.Reasoning,
		f.Quality, f.Derivation, boolInt(f.Skip), ParserVersion, f.Hash, origin)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}
	cur, ver, ok, err := loadFact(tx, f.ID)
	if err != nil || !ok {
		return err
	}
	if !superset(f, cur) {
		return nil
	}
	merged := maxFact(cur, f)
	_, err = tx.Exec(`UPDATE facts SET miss=?, cache_read=?, cache_create=?, output=?, reasoning=?, ts=?, payload_hash=? WHERE id=? AND parser_version=?`,
		merged.Miss, merged.CacheRead, merged.CacheCreate, merged.Output, merged.Reasoning, merged.TS, merged.Hash, f.ID, ver)
	return err
}

func superset(next, cur fact) bool {
	if next.Miss < cur.Miss || next.CacheRead < cur.CacheRead || next.CacheCreate < cur.CacheCreate || next.Output < cur.Output || next.Reasoning < cur.Reasoning {
		return false
	}
	return next.Miss > cur.Miss || next.CacheRead > cur.CacheRead || next.CacheCreate > cur.CacheCreate || next.Output > cur.Output || next.Reasoning > cur.Reasoning
}

func loadFact(tx *sql.Tx, id string) (fact, int, bool, error) {
	var f fact
	var skip, ver int
	err := tx.QueryRow(`SELECT id, source, session_id, request_key, model, vendor, ts, miss, cache_read, cache_create, output, reasoning, quality, derivation, skip_request, parser_version, payload_hash FROM facts WHERE id=?`, id).Scan(
		&f.ID, &f.Source, &f.SessionID, &f.RequestKey, &f.Model, &f.Vendor, &f.TS,
		&f.Miss, &f.CacheRead, &f.CacheCreate, &f.Output, &f.Reasoning,
		&f.Quality, &f.Derivation, &skip, &ver, &f.Hash)
	if errors.Is(err, sql.ErrNoRows) {
		return fact{}, 0, false, nil
	}
	if err != nil {
		return fact{}, 0, false, err
	}
	f.Skip = skip != 0
	return f, ver, true, nil
}

func writeFact(tx *sql.Tx, f fact, parserVersion int) error {
	_, err := tx.Exec(`UPDATE facts SET model=?, vendor=?, ts=?, miss=?, cache_read=?, cache_create=?, output=?, reasoning=?, quality=?, derivation=?, parser_version=?, payload_hash=? WHERE id=?`,
		f.Model, f.Vendor, f.TS, f.Miss, f.CacheRead, f.CacheCreate, f.Output, f.Reasoning, f.Quality, f.Derivation, parserVersion, f.Hash, f.ID)
	return err
}

func insertRevision(tx *sql.Tx, prev, next fact, prevVer, nextVer int, reason string) error {
	_, err := tx.Exec(`INSERT INTO revisions(
		fact_id, reason, prev_parser_version, next_parser_version, prev_hash, next_hash,
		prev_model, next_model, prev_vendor, next_vendor, prev_ts, next_ts,
		prev_miss, prev_cache_read, prev_cache_create, prev_output, prev_reasoning,
		next_miss, next_cache_read, next_cache_create, next_output, next_reasoning,
		prev_quality, next_quality, prev_derivation, next_derivation, created_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		prev.ID, reason, prevVer, nextVer, prev.Hash, next.Hash,
		prev.Model, next.Model, prev.Vendor, next.Vendor, prev.TS, next.TS,
		prev.Miss, prev.CacheRead, prev.CacheCreate, prev.Output, prev.Reasoning,
		next.Miss, next.CacheRead, next.CacheCreate, next.Output, next.Reasoning,
		prev.Quality, next.Quality, prev.Derivation, next.Derivation,
		time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func cleanReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 200 || strings.ContainsAny(reason, "\r\n") {
		return "", ErrCorrectionRejected
	}
	return reason, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func metaInt(db *sql.DB, key string) int {
	var v string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v); err != nil {
		return 0
	}
	var n int
	fmt.Sscan(v, &n)
	return n
}
