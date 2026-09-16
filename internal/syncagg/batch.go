package syncagg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

type SourceInput struct {
	Tool     string
	Detected bool
	Status   string
	Quality  string
}

type BuildInput struct {
	DeviceID            string
	Timezone            string
	PriceCatalogVersion string
	ClientVersion       string
	GeneratedAt         time.Time
	Loc                 *time.Location
	UserHashKey         []byte
	AccountStableID     map[string]string
	Revision            int64
	IdempotencyKey      string
	Events              []event.UsageEvent
	Turns               []event.TurnEvent
	Sources             []SourceInput
}

type Batch struct {
	SchemaVersion       int          `json:"schema_version"`
	IdempotencyKey      string       `json:"idempotency_key,omitempty"`
	DeviceID            string       `json:"device_id"`
	GeneratedAt         string       `json:"generated_at"`
	Timezone            string       `json:"timezone"`
	PriceCatalogVersion string       `json:"price_catalog_version"`
	ClientVersion       string       `json:"client_version"`
	Sources             []Source     `json:"sources"`
	DailyModelUsage     []DailyModel `json:"daily_model_usage"`
}

type Source struct {
	Tool          string      `json:"tool"`
	Detected      bool        `json:"detected"`
	Status        string      `json:"status"`
	Quality       string      `json:"quality"`
	SourceScope   SourceScope `json:"source_scope"`
	SourceKeyHash string      `json:"source_key_hash"`
}

type DailyModel struct {
	Date          string      `json:"date"`
	Tool          string      `json:"tool"`
	SourceScope   SourceScope `json:"source_scope"`
	SourceKeyHash string      `json:"source_key_hash"`
	Vendor        string      `json:"vendor"`
	Model         string      `json:"model"`
	Miss          int64       `json:"miss"`
	CacheRead     int64       `json:"cache_read"`
	CacheCreate   int64       `json:"cache_create"`
	Output        int64       `json:"output"`
	Requests      int64       `json:"requests"`
	UserTurns     int64       `json:"user_turns"`
	Quality       string      `json:"quality"`
	Derivation    string      `json:"derivation"`
	Revision      int64       `json:"revision"`
}

type rowKey struct {
	scope SourceScope
	tool  string
	hash  string
	date  string
	vend  string
	model string
}

func Build(in BuildInput) (Batch, error) {
	if in.DeviceID == "" {
		return Batch{}, fmt.Errorf("device_id required")
	}
	if in.Loc == nil {
		return Batch{}, fmt.Errorf("timezone location required")
	}
	if in.Revision <= 0 {
		return Batch{}, fmt.Errorf("revision must increase from 1")
	}
	loc := in.Loc
	out := Batch{
		SchemaVersion:       SchemaVersion,
		IdempotencyKey:      in.IdempotencyKey,
		DeviceID:            in.DeviceID,
		GeneratedAt:         in.GeneratedAt.In(loc).Format(time.RFC3339),
		Timezone:            in.Timezone,
		PriceCatalogVersion: in.PriceCatalogVersion,
		ClientVersion:       in.ClientVersion,
		Sources:             []Source{},
		DailyModelUsage:     []DailyModel{},
	}

	// Canonicalize with the same request-merge behavior the local report,
	// dashboard, and public profile use (max per token component, latest
	// timestamp wins, quality promoted by rank, negative rows dropped) so a
	// hosted view cannot silently disagree with the local ledger it was
	// synced from.
	merged := metric.CanonicalEvents(in.Events)
	rows := map[rowKey]*DailyModel{}
	turnCount := map[string]int64{}

	for _, e := range merged {
		scope := ScopeOf(e)
		hash, err := hashFor(in, e.Source, scope)
		if err != nil {
			if scope == ScopeAccountGlobal {
				continue
			}
			return Batch{}, err
		}
		if e.Timestamp.IsZero() {
			continue
		}
		date := e.Timestamp.In(loc).Format("2006-01-02")
		k := rowKey{scope, e.Source, hash, date, e.Vendor, e.Model}
		row := rows[k]
		if row == nil {
			row = &DailyModel{
				Date:          date,
				Tool:          e.Source,
				SourceScope:   scope,
				SourceKeyHash: hash,
				Vendor:        e.Vendor,
				Model:         e.Model,
				Quality:       string(e.Quality),
				Derivation:    e.Derivation,
				Revision:      in.Revision,
			}
			rows[k] = row
		}
		row.Miss += e.Miss
		row.CacheRead += e.CacheRead
		row.CacheCreate += e.CacheCreate
		row.Output += e.Output
		if !e.SkipRequest {
			row.Requests++
		}
		if e.Quality == event.QualityAuthoritative {
			row.Quality = string(e.Quality)
		}
		if e.Derivation == event.DeriveProviderAPI {
			row.Derivation = e.Derivation
		}
	}

	for _, turn := range in.Turns {
		if turn.Timestamp.IsZero() || turn.Source == "" {
			continue
		}
		date := turn.Timestamp.In(loc).Format("2006-01-02")
		hash, err := hashFor(in, turn.Source, ScopeDeviceLocal)
		if err != nil {
			return Batch{}, err
		}
		tk := string(ScopeDeviceLocal) + "\x00" + turn.Source + "\x00" + hash + "\x00" + date
		turnCount[tk]++
	}
	for k, row := range rows {
		if k.scope != ScopeDeviceLocal {
			continue
		}
		tk := string(k.scope) + "\x00" + k.tool + "\x00" + k.hash + "\x00" + k.date
		row.UserTurns = turnCount[tk]
	}

	for _, row := range rows {
		out.DailyModelUsage = append(out.DailyModelUsage, *row)
	}
	sort.Slice(out.DailyModelUsage, func(i, j int) bool {
		a, b := out.DailyModelUsage[i], out.DailyModelUsage[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if a.Tool != b.Tool {
			return a.Tool < b.Tool
		}
		if a.SourceScope != b.SourceScope {
			return a.SourceScope < b.SourceScope
		}
		if a.Vendor != b.Vendor {
			return a.Vendor < b.Vendor
		}
		return a.Model < b.Model
	})

	seen := map[string]struct{}{}
	for _, src := range in.Sources {
		scopes := sourceScopes(out.DailyModelUsage, src.Tool)
		if len(scopes) == 0 {
			scopes = []SourceScope{ScopeDeviceLocal}
		}
		for _, scope := range scopes {
			hash, err := hashFor(in, src.Tool, scope)
			if err != nil {
				if scope == ScopeAccountGlobal {
					continue
				}
				return Batch{}, err
			}
			id := src.Tool + "\x00" + string(scope) + "\x00" + hash
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out.Sources = append(out.Sources, Source{
				Tool:          src.Tool,
				Detected:      src.Detected,
				Status:        src.Status,
				Quality:       src.Quality,
				SourceScope:   scope,
				SourceKeyHash: hash,
			})
		}
	}
	return out, nil
}

func hashFor(in BuildInput, tool string, scope SourceScope) (string, error) {
	stable := in.DeviceID
	if scope == ScopeAccountGlobal {
		stable = strings.TrimSpace(in.AccountStableID[tool])
		if stable == "" {
			return "", fmt.Errorf("missing account identity for %s", tool)
		}
	}
	return SourceKeyHash(in.UserHashKey, tool, scope, stable)
}

func sourceScopes(rows []DailyModel, tool string) []SourceScope {
	found := map[SourceScope]struct{}{}
	for _, r := range rows {
		if r.Tool == tool {
			found[r.SourceScope] = struct{}{}
		}
	}
	var out []SourceScope
	if _, ok := found[ScopeAccountGlobal]; ok {
		out = append(out, ScopeAccountGlobal)
	}
	if _, ok := found[ScopeDeviceLocal]; ok {
		out = append(out, ScopeDeviceLocal)
	}
	return out
}

// Bounds below mirror the DB column widths in internal/hosted/migrate.go
// (tool/vendor VARCHAR(32), model VARCHAR(128), quality VARCHAR(32),
// derivation VARCHAR(64), source_key_hash CHAR(64)) plus resource limits
// that keep one row from making hosted dashboard reconstruction
// (internal/hosted/dashboard.go) do unbounded work. A bearer token proves
// the caller is a paired device, not that its request body is trustworthy;
// the server must independently bound and validate it.
const (
	maxToolLen       = 32
	maxVendorLen     = 32
	maxModelLen      = 128
	maxQualityLen    = 32
	maxDerivationLen = 64

	// maxDailyTokens bounds a single day's per-model token component. It is
	// generous relative to any real account (Cursor's own 53-week ledgers
	// run in the low billions) while still being finite.
	maxDailyTokens = 1_000_000_000_000 // 1e12
	// maxDailyCount bounds requests/user_turns: one literal event per unit,
	// so this also bounds how many synthetic events a hosted dashboard
	// request will materialize per row (see aggregateRows).
	maxDailyCount = 1_000_000
	maxRevision   = 1 << 62
)

var validSourceStatus = map[string]bool{
	"ok": true, "auth_missing": true, "auth_expired": true, "absent": true, "degraded": true,
}

var validQuality = map[string]bool{
	"":                                 true,
	string(event.QualityAuthoritative): true,
	string(event.QualityDegraded):      true,
	string(event.QualityEstimated):     true,
	string(event.QualityAbsent):        true,
}

var validDerivation = map[string]bool{
	"":                       true,
	event.DeriveRaw:          true,
	event.DeriveProviderAPI:  true,
	event.DeriveDerived:      true,
	event.DeriveDeduplicated: true,
	event.DeriveEstimated:    true,
}

var sourceKeyHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func DecodeBatch(raw []byte) (Batch, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var b Batch
	if err := dec.Decode(&b); err != nil {
		return Batch{}, err
	}
	if b.SchemaVersion != SchemaVersion {
		return Batch{}, fmt.Errorf("unsupported schema_version %d", b.SchemaVersion)
	}
	if b.DeviceID == "" {
		return Batch{}, fmt.Errorf("device_id required")
	}
	for i := range b.DailyModelUsage {
		if err := b.DailyModelUsage[i].validate(); err != nil {
			return Batch{}, fmt.Errorf("daily_model_usage[%d]: %w", i, err)
		}
	}
	for i := range b.Sources {
		if err := b.Sources[i].validate(); err != nil {
			return Batch{}, fmt.Errorf("sources[%d]: %w", i, err)
		}
	}
	return b, nil
}

func (r DailyModel) validate() error {
	if _, err := ParseScope(string(r.SourceScope)); err != nil {
		return err
	}
	if _, err := time.Parse("2006-01-02", r.Date); err != nil {
		return fmt.Errorf("invalid date %q", r.Date)
	}
	if err := validateBoundedField("tool", r.Tool, maxToolLen, true); err != nil {
		return err
	}
	if err := validateBoundedField("vendor", r.Vendor, maxVendorLen, false); err != nil {
		return err
	}
	if n := len([]rune(r.Model)); n > maxModelLen {
		return fmt.Errorf("model too long")
	}
	if !sourceKeyHashPattern.MatchString(r.SourceKeyHash) {
		return fmt.Errorf("source_key_hash must be 64 lowercase hex characters")
	}
	if err := validateBoundedField("quality", r.Quality, maxQualityLen, false); err != nil {
		return err
	}
	if !validQuality[r.Quality] {
		return fmt.Errorf("unknown quality %q", r.Quality)
	}
	if err := validateBoundedField("derivation", r.Derivation, maxDerivationLen, false); err != nil {
		return err
	}
	if !validDerivation[r.Derivation] {
		return fmt.Errorf("unknown derivation %q", r.Derivation)
	}
	for name, v := range map[string]int64{
		"miss": r.Miss, "cache_read": r.CacheRead, "cache_create": r.CacheCreate, "output": r.Output,
	} {
		if v < 0 || v > maxDailyTokens {
			return fmt.Errorf("%s out of range", name)
		}
	}
	for name, v := range map[string]int64{"requests": r.Requests, "user_turns": r.UserTurns} {
		if v < 0 || v > maxDailyCount {
			return fmt.Errorf("%s out of range", name)
		}
	}
	if r.Revision <= 0 || r.Revision > maxRevision {
		return fmt.Errorf("revision out of range")
	}
	return nil
}

func (s Source) validate() error {
	if _, err := ParseScope(string(s.SourceScope)); err != nil {
		return err
	}
	if err := validateBoundedField("tool", s.Tool, maxToolLen, true); err != nil {
		return err
	}
	if !validSourceStatus[s.Status] {
		return fmt.Errorf("unknown status %q", s.Status)
	}
	if err := validateBoundedField("quality", s.Quality, maxQualityLen, false); err != nil {
		return err
	}
	if !validQuality[s.Quality] {
		return fmt.Errorf("unknown quality %q", s.Quality)
	}
	if !sourceKeyHashPattern.MatchString(s.SourceKeyHash) {
		return fmt.Errorf("source_key_hash must be 64 lowercase hex characters")
	}
	return nil
}

func validateBoundedField(name, v string, max int, required bool) error {
	if required && v == "" {
		return fmt.Errorf("%s required", name)
	}
	if n := len([]rune(v)); n > max {
		return fmt.Errorf("%s too long", name)
	}
	return nil
}
