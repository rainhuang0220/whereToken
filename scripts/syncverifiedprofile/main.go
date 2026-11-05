// Command syncverifiedprofile regenerates the public GitHub Pages fallback
// from a verified hosted wall. It never reads local usage data or credentials.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

type wallEnvelope struct {
	Schema        string                     `json:"schema"`
	SchemaVersion int                        `json:"schema_version"`
	DataRevision  string                     `json:"data_revision"`
	Snapshot      json.RawMessage            `json:"snapshot"`
	Presentation  publicprofile.Presentation `json:"presentation"`
	Wall          struct {
		Phase              string `json:"phase"`
		VerifiedSnapshotID string `json:"verified_snapshot_id"`
		VerifiedAsOfDate   string `json:"verified_as_of_date"`
		VerifiedTotal      *int64 `json:"verified_total"`
		VerifiedAt         string `json:"verified_at"`
	} `json:"wall"`
}

func buildVerifiedFallback(raw []byte, current *publicprofile.Snapshot) (map[string][]byte, bool, error) {
	var env wallEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, false, err
	}
	if env.Schema != "wheretoken.public-profile-live" || env.SchemaVersion != 1 || env.Wall.Phase != "verified" ||
		env.Wall.VerifiedAt == "" || env.Wall.VerifiedSnapshotID == "" || env.Wall.VerifiedTotal == nil {
		return nil, false, errors.New("hosted wall is not fully verified")
	}
	var snap publicprofile.Snapshot
	if err := json.Unmarshal(env.Snapshot, &snap); err != nil {
		return nil, false, fmt.Errorf("snapshot decode: %w", err)
	}
	if snap.SnapshotID != env.DataRevision || snap.SnapshotID != env.Wall.VerifiedSnapshotID ||
		snap.AsOfDate != env.Wall.VerifiedAsOfDate || snap.Periods.All.Totals.Total.Value == nil ||
		*snap.Periods.All.Totals.Total.Value != *env.Wall.VerifiedTotal {
		return nil, false, errors.New("hosted snapshot does not match verified GitHub wall")
	}
	if snap.Owner != nil && snap.Owner.GitHubLogin != "" && snap.Owner.GitHubLogin != "rainhuang0220" {
		return nil, false, errors.New("wrong verified profile owner")
	}
	if err := publicprofile.ValidateProduction(snap, true); err != nil {
		return nil, false, err
	}
	if publicprofile.Sensitive(string(env.Snapshot)) {
		return nil, false, errors.New("sensitive data in public snapshot")
	}
	if current != nil {
		ctotal := current.Periods.All.Totals.Total.Value
		if snap.AsOfDate < current.AsOfDate || (ctotal != nil && *env.Wall.VerifiedTotal < *ctotal) {
			return nil, false, errors.New("verified wall regressed behind checked-in static profile")
		}
	}
	files, err := publicprofile.BundleWith(snap, env.Presentation)
	if err != nil {
		return nil, false, err
	}
	var rebuilt publicprofile.Snapshot
	if json.Unmarshal(files["profile.json"], &rebuilt) != nil || rebuilt.SnapshotID != env.Wall.VerifiedSnapshotID {
		return nil, false, errors.New("rebuilt snapshot ID differs from verified wall")
	}
	return files, current == nil || current.SnapshotID != snap.SnapshotID, nil
}
func sync(wallPath, dir string) (bool, error) {
	raw, err := os.ReadFile(wallPath)
	if err != nil {
		return false, err
	}
	var old *publicprofile.Snapshot
	if snap, err := publicprofile.LoadFile(filepath.Join(dir, "profile.json")); err == nil {
		old = &snap
	} else if !os.IsNotExist(err) {
		return false, err
	}
	files, changed, err := buildVerifiedFallback(raw, old)
	if err != nil {
		return false, err
	}
	for name, data := range files {
		previous, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil && !os.IsNotExist(e) {
			return false, e
		}
		if !bytes.Equal(previous, data) {
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	for _, name := range publicprofile.GeneratedFiles {
		data := files[name]
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return false, err
		}
		tmp, err := os.CreateTemp(filepath.Dir(path), ".wheretoken-sync-*")
		if err != nil {
			return false, err
		}
		tmpName := tmp.Name()
		if _, err = tmp.Write(data); err != nil {
			tmp.Close()
			os.Remove(tmpName)
			return false, err
		}
		if err = tmp.Close(); err != nil {
			os.Remove(tmpName)
			return false, err
		}
		if err = os.Rename(tmpName, path); err != nil {
			os.Remove(tmpName)
			return false, err
		}
	}
	return true, nil
}
func main() {
	wall := flag.String("wall", "", "downloaded public wall JSON")
	dir := flag.String("dir", "public-profile", "static bundle directory")
	flag.Parse()
	if strings.TrimSpace(*wall) == "" {
		fmt.Fprintln(os.Stderr, "--wall is required")
		os.Exit(2)
	}
	changed, err := sync(*wall, *dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verified wall fallback sync rejected:", err)
		os.Exit(1)
	}
	if changed {
		fmt.Println("VERIFIED_FALLBACK_UPDATED")
	} else {
		fmt.Println("VERIFIED_FALLBACK_UNCHANGED")
	}
}
