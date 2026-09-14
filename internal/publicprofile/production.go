package publicprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ValidateProduction(s Snapshot, allowPartial bool) error {
	if err := Validate(s); err != nil {
		return err
	}
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("publicprofile: production requires schema_version %d", SchemaVersion)
	}
	if s.Provenance.Kind != ProvenanceLocal {
		return fmt.Errorf("publicprofile: production requires local_sanitized_snapshot")
	}
	var msgs []string
	for _, row := range s.Periods.All.ByAgent {
		if !cloudSource(row.ID) {
			continue
		}
		reqOK := row.Requests.Status == StatusAvailable && row.Requests.Value != nil && *row.Requests.Value > 0
		tokMissing := row.Coverage.Tokens == StatusUnavailable || row.Totals.Total.Status == StatusUnavailable
		skipped := row.Coverage.Reason == ReasonAccountAPISkipped ||
			row.Coverage.Reason == ReasonAuthMissing ||
			row.Coverage.Reason == ReasonAPIFailed
		if reqOK && tokMissing && skipped {
			msgs = append(msgs, fmt.Sprintf("Production profile has incomplete %s token coverage.\n%s account usage was skipped.\nRegenerate without --offline.", row.Label, row.Label))
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	if allowPartial {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(msgs, "\n"))
}

func ProductionWarning(s Snapshot) string {
	var names []string
	for _, row := range s.Periods.All.ByAgent {
		if cloudSource(row.ID) && row.Coverage.Tokens == StatusUnavailable && row.Coverage.Reason == ReasonAccountAPISkipped {
			names = append(names, row.Label)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "partial coverage allowed: " + strings.Join(names, ", ")
}

func LoadFile(path string) (Snapshot, error) {
	st, err := os.Stat(path)
	if err != nil {
		return Snapshot{}, err
	}
	if st.IsDir() {
		path = filepath.Join(path, "profile.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	if err := validateSchemaJSON(raw); err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return Snapshot{}, err
	}
	if err := validateSemantics(s); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

func ValidateProductionFile(path string, allowPartial bool) error {
	s, err := LoadFile(path)
	if err != nil {
		return err
	}
	return ValidateProduction(s, allowPartial)
}
