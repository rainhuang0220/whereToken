package scan

import (
	"errors"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
)

type CursorAttributionResult struct {
	ledger.AttributionResult
	Before, After, MeasuredTokens int64
	MeteredRows                   int
	Windows                       []cursor.CoverageWindow
}

// BackfillCursorAttribution adds only measured attribution to the already
// linked account's existing baseline. It never records new API coverage or
// substitutes a new account observation for the protected history.
func BackfillCursorAttribution(home adapter.Home, ad cursor.Adapter, from, to string) (CursorAttributionResult, error) {
	var result CursorAttributionResult
	path, err := ledger.PathFor(home)
	if err != nil {
		return result, errors.New("Cursor attribution ledger unavailable")
	}
	lg, err := ledger.Open(path)
	if err != nil {
		return result, errors.New("Cursor attribution ledger unavailable")
	}
	defer lg.Close()
	bound, err := lg.Meta(cursor.AccountSubjectMeta)
	if err != nil || bound == "" {
		return result, cursor.UnboundHistoryError()
	}
	ad.AcceptSubject = func(subject string) error {
		if subject == "" || subject != bound {
			return cursor.AccountMismatchError()
		}
		return nil
	}
	before, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil {
		return result, errors.New("Cursor attribution ledger unavailable")
	}
	result.Before = cursorAttributionTokens(before)
	batch, fetchErr := ad.FetchAttribution(home, from, to)
	result.Windows = batch.Windows
	result.MeteredRows = len(batch.Events)
	result.MeasuredTokens = cursorAttributionTokens(batch.Events)
	if len(batch.Events) > 0 {
		result.AttributionResult, err = lg.BackfillAttribution("cursor", batch.Events)
		if err != nil {
			return result, errors.New("Cursor attribution reconciliation rejected")
		}
	}
	after, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil {
		return result, errors.New("Cursor attribution ledger unavailable")
	}
	result.After = cursorAttributionTokens(after)
	if result.Before != result.After {
		return result, errors.New("Cursor attribution baseline changed")
	}
	return result, fetchErr
}

func cursorAttributionTokens(events []event.UsageEvent) int64 {
	var tokens int64
	for _, e := range events {
		tokens += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}
	return tokens
}
