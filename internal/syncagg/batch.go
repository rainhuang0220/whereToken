package syncagg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
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
	Tool           string      `json:"tool"`
	Detected       bool        `json:"detected"`
	Status         string      `json:"status"`
	Quality        string      `json:"quality"`
	SourceScope    SourceScope `json:"source_scope"`
	SourceKeyHash  string      `json:"source_key_hash"`
}

type DailyModel struct {
	Date           string      `json:"date"`
	Tool           string      `json:"tool"`
	SourceScope    SourceScope `json:"source_scope"`
	SourceKeyHash  string      `json:"source_key_hash"`
	Vendor         string      `json:"vendor"`
	Model          string      `json:"model"`
	Miss           int64       `json:"miss"`
	CacheRead      int64       `json:"cache_read"`
	CacheCreate    int64       `json:"cache_create"`
	Output         int64       `json:"output"`
	Requests       int64       `json:"requests"`
	UserTurns      int64       `json:"user_turns"`
	Quality        string      `json:"quality"`
	Derivation     string      `json:"derivation"`
	Revision       int64       `json:"revision"`
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

	merged := mergeByRequest(in.Events)
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

func mergeByRequest(events []event.UsageEvent) []event.UsageEvent {
	var out []event.UsageEvent
	index := map[string]int{}
	for _, e := range events {
		if e.RequestID == "" {
			out = append(out, e)
			continue
		}
		key := e.Source + "\x00" + e.RequestID
		if i, ok := index[key]; ok {
			out[i] = maxEvent(out[i], e)
			continue
		}
		index[key] = len(out)
		out = append(out, e)
	}
	return out
}

func maxEvent(a, b event.UsageEvent) event.UsageEvent {
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
	if b.SkipRequest {
		a.SkipRequest = true
	}
	if a.Derivation == "" {
		a.Derivation = b.Derivation
	}
	if a.Quality == "" {
		a.Quality = b.Quality
	}
	return a
}

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
		if _, err := ParseScope(string(b.DailyModelUsage[i].SourceScope)); err != nil {
			return Batch{}, err
		}
		if n := len([]rune(b.DailyModelUsage[i].Model)); n > 128 {
			return Batch{}, fmt.Errorf("model too long")
		}
	}
	for i := range b.Sources {
		if _, err := ParseScope(string(b.Sources[i].SourceScope)); err != nil {
			return Batch{}, err
		}
	}
	return b, nil
}
