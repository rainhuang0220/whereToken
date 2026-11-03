package ledger

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Cursor history association keys. They match the cursor adapter's meta
// names. The values are an audit of a user answer, not a session.
const (
	cursorSubjectKey         = "cursor_account_subject"
	cursorDecisionKey        = "cursor_history_decision"
	cursorAuditSourceKey     = "cursor_history_audit_source"
	cursorAuditAtKey         = "cursor_history_audit_at"
	cursorBaselineKey        = "cursor_history_baseline"
	cursorDecisionSubjectKey = "cursor_history_decision_subject"
	cursorHintKey            = "cursor_history_account_hint"
)

var (
	ErrCursorAccountMismatch = errors.New("ACCOUNT_MISMATCH")
	ErrCursorHistoryConflict = errors.New("HISTORY_CONFLICT")
	ErrCursorHistoryHeld     = errors.New("HISTORY_HELD")
)

// CursorHistorySummary is the old Cursor chain a user is asked to associate.
// Baseline is tokens, the first day, the last day, the day count, and a
// hash of the sorted day components.
type CursorHistorySummary struct {
	Tokens   int64
	From, To string
	Days     int
	Baseline string
	Origins  []string
}

// CursorLink is one explicit answer. Decision is "confirmed" or "declined".
type CursorLink struct {
	Subject  string
	Decision string
	At       time.Time
}

// CursorHistorySummary reads the unbound Cursor chain. It does not create rows.
func (l *Ledger) CursorHistorySummary() (CursorHistorySummary, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cursorHistorySummaryLocked()
}

// CommitCursorLink writes the association in one transaction.
// A repeated answer for the same account does not rewrite the baseline or
// the original audit time. A decline does not set the account subject.
// A later decline does not undo a confirmation. A different account does
// not replace either decision.
func (l *Ledger) CommitCursorLink(in CursorLink) error {
	subject := strings.TrimSpace(in.Subject)
	if subject == "" || len(subject) > 200 || metaSecret(subject) {
		return errors.New("ledger history rejected")
	}
	if in.At.IsZero() {
		return errors.New("ledger history rejected")
	}
	audit := ""
	switch in.Decision {
	case "confirmed":
		audit = "user_confirm"
	case "declined":
		audit = "user_decline"
	default:
		return errors.New("ledger history rejected")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	summary, err := l.cursorHistorySummaryLocked()
	if err != nil {
		return err
	}
	if summary.Days == 0 || summary.Baseline == "" {
		return errors.New("ledger history rejected")
	}
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	prevSubject, err := metaTx(tx, cursorSubjectKey)
	if err != nil {
		return err
	}
	prevDecision, err := metaTx(tx, cursorDecisionKey)
	if err != nil {
		return err
	}
	prevWho, err := metaTx(tx, cursorDecisionSubjectKey)
	if err != nil {
		return err
	}
	prevBase, err := metaTx(tx, cursorBaselineKey)
	if err != nil {
		return err
	}
	hint, err := metaTx(tx, cursorHintKey)
	if err != nil {
		return err
	}
	if prevSubject != "" && prevSubject != subject {
		return ErrCursorAccountMismatch
	}

	writes := map[string]string{}
	switch in.Decision {
	case "confirmed":
		if hint != "" && hint != subject {
			return ErrCursorHistoryConflict
		}
		if prevDecision == "confirmed" && prevWho == subject && prevSubject == subject {
			return nil
		}
		if prevDecision == "confirmed" || (prevDecision == "declined" && prevWho != "" && prevWho != subject) {
			if prevDecision == "confirmed" {
				return ErrCursorAccountMismatch
			}
			return ErrCursorHistoryHeld
		}
		baseline := summary.Baseline
		if prevBase != "" {
			baseline = prevBase
		}
		writes[cursorSubjectKey] = subject
		writes[cursorDecisionKey] = "confirmed"
		writes[cursorAuditSourceKey] = audit
		writes[cursorAuditAtKey] = in.At.UTC().Format(time.RFC3339)
		writes[cursorBaselineKey] = baseline
		writes[cursorDecisionSubjectKey] = subject
	case "declined":
		if prevDecision == "confirmed" {
			return nil
		}
		if prevDecision == "declined" && prevWho == subject {
			return nil
		}
		if prevDecision == "declined" && prevWho != "" && prevWho != subject {
			return ErrCursorHistoryHeld
		}
		if prevSubject != "" {
			return ErrCursorAccountMismatch
		}
		baseline := summary.Baseline
		if prevBase != "" {
			baseline = prevBase
		}
		writes[cursorDecisionKey] = "declined"
		writes[cursorAuditSourceKey] = audit
		writes[cursorAuditAtKey] = in.At.UTC().Format(time.RFC3339)
		writes[cursorBaselineKey] = baseline
		writes[cursorDecisionSubjectKey] = subject
	}
	for key, value := range writes {
		if err := putMetaTx(tx, key, value); err != nil {
			return err
		}
	}
	if l.beforeMetaCommit != nil {
		if err := l.beforeMetaCommit(); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (l *Ledger) cursorHistorySummaryLocked() (CursorHistorySummary, error) {
	obs, err := l.observationsAsc("cursor", "")
	if err != nil {
		return CursorHistorySummary{}, err
	}
	days := chainDayMap(obs)
	keys := make([]string, 0, len(days))
	for day, d := range days {
		if d.Miss+d.CacheRead+d.CacheCreate+d.Output == 0 {
			continue
		}
		keys = append(keys, day)
	}
	sort.Strings(keys)
	origins, err := l.observationOriginsLocked("cursor")
	if err != nil {
		return CursorHistorySummary{}, err
	}
	if len(keys) == 0 {
		return CursorHistorySummary{Origins: origins}, nil
	}
	var tokens int64
	lines := make([]string, 0, len(keys))
	for _, day := range keys {
		d := days[day]
		tokens += d.Miss + d.CacheRead + d.CacheCreate + d.Output
		lines = append(lines, fmt.Sprintf("%s|%d|%d|%d|%d", day, d.Miss, d.CacheRead, d.CacheCreate, d.Output))
	}
	sum := sha256.New()
	for _, line := range lines {
		sum.Write([]byte(line))
		sum.Write([]byte{'\n'})
	}
	baseline := fmt.Sprintf("%d|%s|%s|%d|%s", tokens, keys[0], keys[len(keys)-1], len(keys), hex.EncodeToString(sum.Sum(nil)))
	return CursorHistorySummary{
		Tokens:   tokens,
		From:     keys[0],
		To:       keys[len(keys)-1],
		Days:     len(keys),
		Baseline: baseline,
		Origins:  origins,
	}, nil
}

func (l *Ledger) observationOriginsLocked(source string) ([]string, error) {
	rows, err := l.db.Query(`SELECT DISTINCT origin FROM api_observations WHERE source=? AND observed_at != '' ORDER BY origin`, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var origin string
		if err := rows.Scan(&origin); err != nil {
			return nil, err
		}
		out = append(out, origin)
	}
	return out, rows.Err()
}

func metaTx(tx *sql.Tx, key string) (string, error) {
	var v string
	err := tx.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func putMetaTx(tx *sql.Tx, key, value string) error {
	if key == "" || metaSecret(value) {
		return errors.New("ledger meta rejected")
	}
	_, err := tx.Exec(`INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func metaSecret(value string) bool {
	return strings.Contains(strings.ToLower(value), "eyj") || strings.ContainsAny(value, "\r\n")
}
