// Package index is a local performance cache for parsed usage events.
//
// The source of truth remains agent data → adapter → normalized events.
// This package stores file identity and already-normalized events so a later
// scan can skip bytes that have not changed. It is not a second adapter and
// does not implement token accounting.
//
// File identity (path, size, mtime, inode) is a best-effort cache key, not a
// cryptographic content hash. A same-size in-place rewrite that restores the
// old mtime can replay stale blobs until rebuild.
//
// Incremental JSONL stores the last consumed byte offset, which may be behind
// Size when the file ends on an incomplete line. The next scan resumes there.
//
// Tables:
//
//	meta        schema version
//	files       path, size, mtime, inode, offset (bytes consumed, not EOF)
//	blobs       gob-encoded normalized events and turns
//
// Wipe (wheretoken rebuild) deletes the database. The next scan reads agents
// from scratch. A schema mismatch also drops the cache.
package index

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"

	_ "modernc.org/sqlite"
)

const (
	ModeFull        = "full"
	ModeIncremental = "incremental"
	ModeUnchanged   = "unchanged"
	// schema 2: wipe caches written by the first OpenClaw parser, which stored
	// empty event blobs for unchanged session JSONL.
	schemaVersion = 2
)

type ParseFunc func(*os.File) (evs []event.UsageEvent, turns []event.TurnEvent, consumed int64, err error)

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

type fileRow struct {
	Size, Mtime, Inode, Offset int64
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", adapter.SQLiteURI(path, "mode=rwc"))
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Wipe deletes the index database so the next scan rebuilds from agent data.
func Wipe(path string) error {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func PathFor(home adapter.Home) string {
	if v := os.Getenv("WHERETOKEN_INDEX"); v != "" {
		return v
	}
	return filepath.Join(home.DotDir("cache"), "wheretoken", "index.v1.db")
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	var got string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema'`).Scan(&got)
	if err == nil && got != "" && got != fmt.Sprintf("%d", schemaVersion) {
		if _, err := s.db.Exec(`DROP TABLE IF EXISTS blobs; DROP TABLE IF EXISTS files; DROP TABLE IF EXISTS meta;`); err != nil {
			return err
		}
	}
	_, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS files (
  path TEXT PRIMARY KEY,
  size INTEGER NOT NULL,
  mtime INTEGER NOT NULL,
  inode INTEGER NOT NULL,
  offset INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS blobs (
  path TEXT PRIMARY KEY,
  events BLOB NOT NULL,
  turns BLOB NOT NULL
);
`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT OR REPLACE INTO meta(key,value) VALUES('schema', ?)`, fmt.Sprintf("%d", schemaVersion))
	return err
}

// LoadOrParse replays cached events when the file is unchanged, parses only
// newly appended bytes when the file grew, and fully reparses on truncate or
// replacement (size shrink or inode change).
func (s *Store) LoadOrParse(source, path string, parse ParseFunc) ([]event.UsageEvent, []event.TurnEvent, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(source, path, parse, true)
}

// LoadOrReplay is for parsers that cannot resume from a byte offset (cumulative
// Codex rollouts, SQLite). Unchanged files replay the cache; any change is a
// full parse. Replay stores Size as the offset because the whole file is the unit.
func (s *Store) LoadOrReplay(source, path string, parse ParseFunc) ([]event.UsageEvent, []event.TurnEvent, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(source, path, parse, false)
}

func (s *Store) load(source, path string, parse ParseFunc, incremental bool) ([]event.UsageEvent, []event.TurnEvent, string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, nil, ModeFull, fmt.Errorf("stat: %w", PathFree(err))
	}
	cur := identOf(path, st)
	prev, ok, err := s.getFile(path)
	if err != nil {
		return nil, nil, ModeFull, err
	}
	mode := decide(prev, ok, cur)
	if mode == ModeIncremental && !incremental {
		mode = ModeFull
	}
	switch mode {
	case ModeUnchanged:
		evs, turns, err := s.getBlobs(path)
		if err != nil {
			return s.full(source, path, cur, parse, !incremental)
		}
		note(source, ModeUnchanged, 0)
		return evs, turns, ModeUnchanged, nil
	case ModeIncremental:
		oldE, oldT, err := s.getBlobs(path)
		if err != nil {
			return s.full(source, path, cur, parse, false)
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, ModeFull, fmt.Errorf("open: %w", PathFree(err))
		}
		if _, err := f.Seek(prev.Offset, 0); err != nil {
			f.Close()
			return s.full(source, path, cur, parse, false)
		}
		newE, newT, consumed, err := parse(f)
		f.Close()
		if err != nil {
			note(source, ModeIncremental, 0)
			return oldE, oldT, ModeIncremental, err
		}
		allE := append(append([]event.UsageEvent(nil), oldE...), newE...)
		allT := append(append([]event.TurnEvent(nil), oldT...), newT...)
		if err := s.put(path, cur, consumed, allE, allT); err != nil {
			return allE, allT, ModeIncremental, err
		}
		note(source, ModeIncremental, len(newE)+len(newT))
		return allE, allT, ModeIncremental, nil
	default:
		return s.full(source, path, cur, parse, !incremental)
	}
}

func (s *Store) full(source, path string, cur fileRow, parse ParseFunc, replay bool) ([]event.UsageEvent, []event.TurnEvent, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, ModeFull, fmt.Errorf("open: %w", PathFree(err))
	}
	evs, turns, consumed, err := parse(f)
	f.Close()
	if err != nil {
		return evs, turns, ModeFull, err
	}
	offset := consumed
	if replay {
		offset = cur.Size
	}
	if err := s.put(path, cur, offset, evs, turns); err != nil {
		return evs, turns, ModeFull, err
	}
	note(source, ModeFull, len(evs)+len(turns))
	return evs, turns, ModeFull, nil
}

func decide(prev fileRow, ok bool, cur fileRow) string {
	if !ok {
		return ModeFull
	}
	if cur.Size < prev.Size {
		return ModeFull
	}
	if prev.Inode != 0 && cur.Inode != 0 && prev.Inode != cur.Inode {
		return ModeFull
	}
	if cur.Size == prev.Size && cur.Mtime == prev.Mtime && cur.Inode == prev.Inode {
		return ModeUnchanged
	}
	// Same length + new mtime is a rewrite, even if a pending incomplete
	// tail left offset behind EOF. Incremental would keep a stale prefix.
	if cur.Size == prev.Size && cur.Mtime != prev.Mtime {
		return ModeFull
	}
	if cur.Size > prev.Offset {
		return ModeIncremental
	}
	return ModeFull
}

func (s *Store) getFile(path string) (fileRow, bool, error) {
	var r fileRow
	err := s.db.QueryRow(`SELECT size, mtime, inode, offset FROM files WHERE path = ?`, path).
		Scan(&r.Size, &r.Mtime, &r.Inode, &r.Offset)
	if err == sql.ErrNoRows {
		return r, false, nil
	}
	if err != nil {
		return r, false, err
	}
	return r, true, nil
}

func (s *Store) getBlobs(path string) ([]event.UsageEvent, []event.TurnEvent, error) {
	var evB, tnB []byte
	err := s.db.QueryRow(`SELECT events, turns FROM blobs WHERE path = ?`, path).Scan(&evB, &tnB)
	if err != nil {
		return nil, nil, err
	}
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	if len(evB) == 0 {
		return nil, nil, fmt.Errorf("empty events blob")
	}
	if err := gob.NewDecoder(bytes.NewReader(evB)).Decode(&evs); err != nil {
		return nil, nil, err
	}
	if err := gob.NewDecoder(bytes.NewReader(tnB)).Decode(&turns); err != nil && len(tnB) > 0 {
		return nil, nil, err
	}
	return evs, turns, nil
}

func (s *Store) Offset(path string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok, err := s.getFile(path)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, sql.ErrNoRows
	}
	return r.Offset, nil
}

func (s *Store) put(path string, cur fileRow, offset int64, evs []event.UsageEvent, turns []event.TurnEvent) error {
	if evs == nil {
		evs = []event.UsageEvent{}
	}
	if turns == nil {
		turns = []event.TurnEvent{}
	}
	var eb, tb bytes.Buffer
	if err := gob.NewEncoder(&eb).Encode(evs); err != nil {
		return err
	}
	if err := gob.NewEncoder(&tb).Encode(turns); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if offset < 0 || offset > cur.Size {
		offset = cur.Size
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO files(path,size,mtime,inode,offset) VALUES(?,?,?,?,?)`,
		path, cur.Size, cur.Mtime, cur.Inode, offset); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO blobs(path,events,turns) VALUES(?,?,?)`,
		path, eb.Bytes(), tb.Bytes()); err != nil {
		return err
	}
	return tx.Commit()
}

func identOf(path string, st os.FileInfo) fileRow {
	return fileRow{
		Size:   st.Size(),
		Mtime:  st.ModTime().UnixNano(),
		Inode:  inodeOf(st),
		Offset: st.Size(),
	}
}
