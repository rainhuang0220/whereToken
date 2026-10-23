package index

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/event"
)

// CachedBlob is one normalized cache row. Path is the source path that was
// parsed. Events and turns are the stored normalized records, not file bytes.
type CachedBlob struct {
	Path   string
	Events []event.UsageEvent
	Turns  []event.TurnEvent
}

// AbsentUnder returns cached blobs whose path is under root. The caller
// decides which of those files are gone. No active store returns nothing:
// a missing cache must not invent usage. Corrupt blobs are skipped.
func AbsentUnder(root string) []CachedBlob {
	s := Active()
	if s == nil {
		return nil
	}
	return s.AbsentUnder(root)
}

// AbsentUnder lists blobs stored under root. The prefix match is exact, so a
// sibling directory with a shared prefix is not included. The lock is not
// held while the caller stats the filesystem.
func (s *Store) AbsentUnder(root string) []CachedBlob {
	if s == nil || s.db == nil {
		return nil
	}
	root = filepath.Clean(root)
	if root == "" || root == "." || root == string(os.PathSeparator) {
		return nil
	}
	prefix := root + string(os.PathSeparator)
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT path, events, turns FROM blobs WHERE instr(path, ?) = 1`, prefix)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []CachedBlob
	for rows.Next() {
		var path string
		var evB, tnB []byte
		if err := rows.Scan(&path, &evB, &tnB); err != nil {
			continue
		}
		evs, turns, err := decodeBlob(evB, tnB)
		if err != nil {
			continue
		}
		out = append(out, CachedBlob{Path: path, Events: evs, Turns: turns})
	}
	return out
}

func decodeBlob(evB, tnB []byte) ([]event.UsageEvent, []event.TurnEvent, error) {
	if len(evB) == 0 {
		return nil, nil, fmt.Errorf("empty events blob")
	}
	var evs []event.UsageEvent
	if err := gob.NewDecoder(bytes.NewReader(evB)).Decode(&evs); err != nil {
		return nil, nil, err
	}
	var turns []event.TurnEvent
	if len(tnB) > 0 {
		if err := gob.NewDecoder(bytes.NewReader(tnB)).Decode(&turns); err != nil {
			return nil, nil, err
		}
	}
	return evs, turns, nil
}
