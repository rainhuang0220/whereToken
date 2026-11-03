package scan

import (
	"errors"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/ledger"
)

// errNoHistoryChoice means the user did not answer yes or no.
// Nothing is written.
var errNoHistoryChoice = errors.New("没有记录这次选择")

// DescribeCursorHistory renders the one-time association question.
// A saved snapshot origin is not treated as an account id.
func DescribeCursorHistory(lg *ledger.Ledger, subject string) (string, error) {
	if lg == nil {
		return "", errors.New("ledger history rejected")
	}
	summary, err := lg.CursorHistorySummary()
	if err != nil {
		return "", err
	}
	if summary.Days == 0 {
		return "", errors.New("ledger history rejected")
	}
	hint, err := lg.Meta(cursor.HistoryAccountHintMeta)
	if err != nil {
		return "", err
	}
	return cursor.FormatHistoryPrompt(cursor.HistoryPrompt{
		Tokens:   summary.Tokens,
		From:     summary.From,
		To:       summary.To,
		Days:     summary.Days,
		Evidence: cursor.HistoryEvidence(hint, subject, summary.Origins),
		Subject:  subject,
	}), nil
}

// ApplyCursorHistory records one explicit answer. yes binds the account.
// no keeps the old chain independent. Any other answer writes nothing.
func ApplyCursorHistory(lg *ledger.Ledger, subject, answer string, at time.Time) error {
	if lg == nil {
		return errors.New("ledger history rejected")
	}
	decision, ok := cursor.ParseHistoryAnswer(answer)
	if !ok {
		return errNoHistoryChoice
	}
	if decision == cursor.HistoryConfirmed {
		summary, err := lg.CursorHistorySummary()
		if err != nil {
			return err
		}
		hint, err := lg.Meta(cursor.HistoryAccountHintMeta)
		if err != nil {
			return err
		}
		if cursor.HistoryEvidence(hint, subject, summary.Origins) == cursor.HistoryEvidenceConflict {
			return cursor.HistoryConflictError()
		}
	}
	err := lg.CommitCursorLink(ledger.CursorLink{Subject: subject, Decision: decision, At: at})
	switch {
	case errors.Is(err, ledger.ErrCursorAccountMismatch):
		return cursor.AccountMismatchError()
	case errors.Is(err, ledger.ErrCursorHistoryConflict):
		return cursor.HistoryConflictError()
	case errors.Is(err, ledger.ErrCursorHistoryHeld):
		return cursor.HistoryHeldError()
	default:
		return err
	}
}
