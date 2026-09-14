package publicprofile

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Validate(s Snapshot) error {
	raw, err := Marshal(s)
	if err != nil {
		return err
	}
	if err := validateSchemaJSON(raw); err != nil {
		return err
	}
	return validateSemantics(s)
}

func validateSemantics(s Snapshot) error {
	if s.Schema != SchemaName {
		return fmt.Errorf("publicprofile: schema %q", s.Schema)
	}
	if s.SchemaVersion != SchemaVersion && s.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("publicprofile: schema_version %d", s.SchemaVersion)
	}
	if s.SnapshotID == "" {
		return fmt.Errorf("publicprofile: snapshot_id")
	}
	if s.SchemaVersion != SchemaVersionV1 {
		wantID := s.SnapshotID
		refreshSnapshotID(&s)
		if s.SnapshotID != wantID {
			return fmt.Errorf("publicprofile: snapshot_id mismatch")
		}
	}
	if _, err := time.Parse(time.RFC3339, s.GeneratedAt); err != nil {
		return fmt.Errorf("publicprofile: generated_at: %w", err)
	}
	if _, err := time.Parse("2006-01-02", s.AsOfDate); err != nil {
		return fmt.Errorf("publicprofile: as_of_date")
	}
	if s.Producer.Name == "" || s.Producer.TokenAccountingVersion == "" {
		return fmt.Errorf("publicprofile: producer")
	}
	validProvenance := (s.Provenance.Kind == ProvenanceLocal && s.Provenance.RefreshMode == RefreshManualPublish) ||
		(s.Provenance.Kind == ProvenanceSyntheticDemo && s.Provenance.RefreshMode == RefreshCommittedFixture)
	if !validProvenance || s.Provenance.LiveSync {
		return fmt.Errorf("publicprofile: provenance")
	}
	switch s.DataStatus {
	case StatusAvailable, StatusPartial, StatusUnavailable:
	default:
		return fmt.Errorf("publicprofile: data_status")
	}
	if s.Privacy.RawEvents {
		return fmt.Errorf("publicprofile: privacy.raw_events")
	}
	if s.Owner != nil {
		if _, err := sanitizeOwner(s.Owner); err != nil {
			return err
		}
	}
	if err := validateLinks(s.Links); err != nil {
		return err
	}
	for _, id := range []string{PeriodAll, PeriodToday, Period7d, Period30d, Period53w} {
		p, ok := s.Periods.ByID(id)
		if !ok {
			return fmt.Errorf("publicprofile: missing period %s", id)
		}
		if err := validatePeriod(id, p, s.Privacy.CostIncluded, s.Privacy.ModelsIncluded, s.SchemaVersion); err != nil {
			return err
		}
	}
	if err := validateActivity(s.Activity, s.SchemaVersion); err != nil {
		return err
	}
	if err := scanSensitive(s); err != nil {
		return err
	}
	if n, err := gzipSize(s); err == nil && n > GzipBudget {
		return fmt.Errorf("publicprofile: gzip %d exceeds %d", n, GzipBudget)
	}
	return nil
}

func validatePeriod(id string, p Period, cost, models bool, version int) error {
	if err := checkTotals(id, p.Totals); err != nil {
		return err
	}
	if cost && p.Cost == nil {
		return fmt.Errorf("publicprofile: %s missing cost", id)
	}
	if !cost && p.Cost != nil {
		return fmt.Errorf("publicprofile: %s unexpected cost", id)
	}
	if models && p.ByModel == nil {
		return fmt.Errorf("publicprofile: %s missing by_model", id)
	}
	if !models && len(p.ByModel) > 0 {
		return fmt.Errorf("publicprofile: %s unexpected by_model", id)
	}
	if err := reconcile(id, "agent", p.ByAgent, p.Totals.Total, version); err != nil {
		return err
	}
	if err := reconcile(id, "vendor", p.ByVendor, p.Totals.Total, version); err != nil {
		return err
	}
	return nil
}

func checkTotals(id string, t Totals) error {
	if t.Total.Status == StatusUnavailable {
		return nil
	}
	if t.Total.Value == nil || t.Miss.Value == nil || t.CacheRead.Value == nil || t.CacheCreate.Value == nil || t.Output.Value == nil {
		return fmt.Errorf("publicprofile: %s totals null", id)
	}
	sum := satAdd(satAdd(*t.Miss.Value, *t.CacheRead.Value), satAdd(*t.CacheCreate.Value, *t.Output.Value))
	if sum != *t.Total.Value {
		return fmt.Errorf("publicprofile: %s total invariant %d != %d", id, sum, *t.Total.Value)
	}
	return nil
}

func reconcile(period, kind string, rows []Breakdown, total Component, version int) error {
	if total.Value == nil || total.Status == StatusUnavailable {
		return nil
	}
	var sum int64
	for _, r := range rows {
		if r.Totals.Total.Status != StatusUnavailable && r.Totals.Total.Value != nil {
			sum = satAdd(sum, *r.Totals.Total.Value)
		}
		if r.ID == "" || r.Label == "" {
			return fmt.Errorf("publicprofile: %s %s empty id/label", period, kind)
		}
		if version >= SchemaVersion {
			if err := validateCoverage(period, kind, r); err != nil {
				return err
			}
		}
	}
	if sum > *total.Value {
		return fmt.Errorf("publicprofile: %s %s breakdown %d > total %d", period, kind, sum, *total.Value)
	}
	return nil
}

func validateCoverage(period, kind string, r Breakdown) error {
	switch r.Coverage.Tokens {
	case StatusAvailable, StatusPartial, StatusUnavailable:
	default:
		return fmt.Errorf("publicprofile: %s %s %s coverage.tokens", period, kind, r.ID)
	}
	switch r.Coverage.Requests {
	case StatusAvailable, StatusPartial, StatusUnavailable:
	default:
		return fmt.Errorf("publicprofile: %s %s %s coverage.requests", period, kind, r.ID)
	}
	if r.Coverage.Tokens == StatusUnavailable {
		if r.Totals.Total.Status != StatusUnavailable || r.Totals.Total.Value != nil {
			return fmt.Errorf("publicprofile: %s %s %s unavailable tokens labelled available", period, kind, r.ID)
		}
		if r.Share != emDash {
			return fmt.Errorf("publicprofile: %s %s %s unavailable share", period, kind, r.ID)
		}
	}
	if r.Coverage.Tokens != r.Totals.Total.Status {
		return fmt.Errorf("publicprofile: %s %s %s coverage/total status mismatch", period, kind, r.ID)
	}
	return nil
}

func validateActivity(a Activity, version int) error {
	if a.WeekStart != "monday" {
		return fmt.Errorf("publicprofile: week_start")
	}
	if len(a.Dates) != WallDays {
		return fmt.Errorf("publicprofile: dates want %d got %d", WallDays, len(a.Dates))
	}
	for _, s := range a.Series {
		if version >= SchemaVersion {
			if s.Metric != MetricTokens && s.Metric != MetricRequests {
				return fmt.Errorf("publicprofile: series %s metric %q", s.ID, s.Metric)
			}
		}
		if len(s.Values) != len(a.Dates) || len(s.Levels) != len(a.Dates) || len(s.States) != len(a.Dates) {
			return fmt.Errorf("publicprofile: series %s length", s.ID)
		}
		for i, st := range s.States {
			switch st {
			case CellActive, CellEmpty, CellUnknown, CellFuture:
			default:
				return fmt.Errorf("publicprofile: series %s state %q", s.ID, st)
			}
			if st != CellActive && s.Levels[i] != 0 {
				return fmt.Errorf("publicprofile: series %s level on inactive", s.ID)
			}
		}
	}
	return nil
}

func validateLinks(l Links) error {
	for _, raw := range []string{l.Project, l.TokenAccounting, l.LivePage} {
		if err := requirePublicHTTPS(raw); err != nil {
			return err
		}
	}
	return nil
}

func requirePublicHTTPS(raw string) error {
	if strings.TrimSpace(raw) != raw || raw == "" {
		return fmt.Errorf("publicprofile: invalid url")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" {
		return fmt.Errorf("publicprofile: invalid url")
	}
	return nil
}

func gzipSize(s Snapshot) (int, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return 0, err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return 0, err
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}
	return buf.Len(), nil
}

func ValidateFile(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() {
		path = filepath.Join(path, "profile.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := validateSchemaJSON(raw); err != nil {
		return err
	}
	var s Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	return validateSemantics(s)
}

func Marshal(s Snapshot) ([]byte, error) {
	s.Notices = nonNil(s.Notices)
	s.Privacy.OmittedSections = nonNil(s.Privacy.OmittedSections)
	return json.MarshalIndent(s, "", "  ")
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func scanSensitive(s Snapshot) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if Sensitive(string(raw)) {
		return fmt.Errorf("publicprofile: denylist hit")
	}
	return nil
}
