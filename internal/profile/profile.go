// Package profile turns a metric.Summary window into a deterministic, local
// user portrait for the dashboard. No LLM, no embeddings, no network, no
// wall-clock or global randomness: metrics land in documented buckets, a
// fixed priority chain picks the dominant traits, and a seed (the anonymous
// local install identity) jitters phrasing inside same-direction phrase
// pools. Same data + same seed → byte-identical portrait; data inside the
// same buckets → identical phrasing.
package profile

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/metric"
)

// Portrait states.
const (
	StateNone         = "none"         // no records in the window at all
	StateInsufficient = "insufficient" // some records, too few to profile
	StateOK           = "ok"           // enough data for a portrait
)

// Portrait is the dashboard's bottom-right cell: a primary phrase, up to two
// tag phrases, and a tooltip detail. none/insufficient carry no phrasing.
type Portrait struct {
	State   string   `json:"state"`
	Primary string   `json:"primary"`
	Tags    []string `json:"tags,omitempty"`
	Detail  string   `json:"detail,omitempty"`
}

// Direction classes. Traits in the same class say the same thing in
// different words (or, for rhythm, contradict: steady vs bursty), so a
// portrait never combines two traits from one class.
const (
	classHeavy   = "heavy"   // cost_high, intensity_high
	classExplore = "explore" // model_diversity, vendor_diversity
	classFocus   = "focus"   // concentration
	classReuse   = "reuse"   // cache_high
	classRhythm  = "rhythm"  // steady, bursty (opposites — never paired)
	classFrugal  = "frugal"  // cost_efficient, intensity_light
)

// traitRef is one fired trait: its phrase-pool key and direction class.
type traitRef struct {
	key, class string
}

// modifierKeys may appear as the optional second tag when extreme and not
// already used as primary/secondary.
var modifierKeys = map[string]bool{"cache_high": true, "steady": true, "bursty": true}

// Evaluate profiles the window behind sum. seed is the anonymous install
// identity (see Identity); an empty seed falls back to a constant so tests
// and synthetic results stay deterministic. Pure: no I/O, no clock.
func Evaluate(sum metric.Summary, seed string) Portrait {
	tot := sum.All.Total()
	if tot <= 0 {
		return Portrait{State: StateNone, Primary: "—"}
	}
	if tot < minProfileTokens {
		return Portrait{State: StateInsufficient, Primary: "数据不足"}
	}
	if seed == "" {
		seed = FallbackSeed
	}

	v := buildVector(sum)
	primary, secondary, modifier := selectTraits(v)

	// The hash binds seed + full trait selection; each pool pick then xors
	// in its own trait salt, so two users in the same buckets get different
	// phrasings while one user's phrasing is stable per seed.
	h := fnv64(seed, StateOK, primary, secondary, modifier)
	p := Portrait{State: StateOK, Primary: pickPhrase(h, primary)}
	if secondary != "" {
		p.Tags = append(p.Tags, pickPhrase(h, secondary))
	}
	if modifier != "" {
		p.Tags = append(p.Tags, pickPhrase(h, modifier))
	}
	p.Detail = buildDetail(sum)
	return p
}

// selectTraits picks primary, secondary, and modifier trait keys. Primary
// is the first fired trait in this fixed priority order:
//
//	cost_high > intensity_high > model_diversity > vendor_diversity >
//	concentration > cache_high > steady > bursty > cost_efficient >
//	intensity_light
//
// Secondary is the next fired trait from a different direction class.
// Modifier is the first fired cache/steady/bursty trait not already used
// and from a class not already shown (so steady never pairs with bursty).
// A window with data but no extreme dimension falls back to intensity_light,
// mirroring insight.LevelLight's "everything else that has data".
func selectTraits(v vector) (primary, secondary, modifier string) {
	var cand []traitRef
	add := func(key, class string, on bool) {
		if on {
			cand = append(cand, traitRef{key, class})
		}
	}
	add("cost_high", classHeavy, v.costAvail && v.cost == 2)
	add("intensity_high", classHeavy, v.intensity == 2)
	add("model_diversity", classExplore, v.modelDiv == 2)
	add("vendor_diversity", classExplore, v.vendorDiv == 2)
	add("concentration", classFocus, v.concAvail && v.concentration == 2)
	add("cache_high", classReuse, v.cacheAvail && v.cache == 2)
	add("steady", classRhythm, v.consistency == 2 && v.activeDays >= steadyMinActiveDays)
	add("bursty", classRhythm, v.burstiness == 2)
	add("cost_efficient", classFrugal, v.costAvail && v.cost == 0)
	add("intensity_light", classFrugal, v.intensity == 0)
	if len(cand) == 0 {
		cand = append(cand, traitRef{"intensity_light", classFrugal})
	}

	primary = cand[0].key
	usedClass := map[string]bool{cand[0].class: true}
	for _, c := range cand[1:] {
		if !usedClass[c.class] {
			secondary = c.key
			usedClass[c.class] = true
			break
		}
	}
	for _, c := range cand {
		if !modifierKeys[c.key] || c.key == primary || c.key == secondary || usedClass[c.class] {
			continue
		}
		modifier = c.key
		break
	}
	return primary, secondary, modifier
}

// fnv64 hashes the seed and the trait selection into the phrase picker.
func fnv64(parts ...string) uint64 {
	h := fnv.New64a()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte("|"))
		}
		h.Write([]byte(p))
	}
	return h.Sum64()
}

// pickPhrase chooses one phrase from the trait's pool, seeded by the
// selection hash xored with the trait's salt. Deterministic per (seed,
// traits) and confined to the pool: jitter can never cross directions.
func pickPhrase(h uint64, key string) string {
	pool := pools[key]
	r := rand.New(rand.NewSource(int64(h ^ salts[key])))
	return pool[r.Intn(len(pool))]
}

// buildDetail is the tooltip: exact window stats. Unavailable measurements
// (no labeled model, no cache traffic) are omitted, never zeroed.
func buildDetail(sum metric.Summary) string {
	tot := sum.All.Total()
	lines := []string{
		fmt.Sprintf("本周期 %s tokens", metric.FormatM(tot)),
		fmt.Sprintf("活跃 %d 天", activeDays(sum)),
	}
	if models := labeledModels(sum); len(models) > 0 {
		lines = append(lines, fmt.Sprintf("模型 %d 个", len(models)))
		if tot > 0 {
			lines = append(lines, fmt.Sprintf("主力模型占比 %.0f%%", 100*float64(topModelTokens(models))/float64(tot)))
		}
	}
	if hit, ok := metric.HitRate(sum.All.Miss, sum.All.CacheRead, sum.All.CacheCreate); ok {
		lines = append(lines, fmt.Sprintf("缓存命中率 %.1f%%", hit))
	}
	return strings.Join(lines, "\n")
}
