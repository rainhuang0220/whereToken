package profile

import (
	"strings"
	"testing"
)

// (g) The phrase bank: 180–240 fragments across the required trait groups,
// no duplicates, no empty strings, no banned wording.
func TestPhraseBankShape(t *testing.T) {
	required := []string{
		"intensity_high", "intensity_light",
		"cost_high", "cost_efficient",
		"model_diversity", "vendor_diversity",
		"concentration", "cache_high",
		"steady", "bursty",
	}
	total := 0
	for _, key := range required {
		pool, ok := pools[key]
		if !ok {
			t.Fatalf("missing pool %s", key)
		}
		if len(pool) < 18 || len(pool) > 24 {
			t.Fatalf("pool %s has %d phrases, want 18–24", key, len(pool))
		}
		total += len(pool)
	}
	if len(pools) != len(required) {
		t.Fatalf("pools=%d, want exactly the %d required groups", len(pools), len(required))
	}
	if total < 180 || total > 240 {
		t.Fatalf("phrase bank has %d entries, want 180–240", total)
	}
	// Every pool needs a selection salt and every salt a pool.
	for key := range pools {
		if _, ok := salts[key]; !ok {
			t.Fatalf("pool %s has no salt", key)
		}
	}
	for key := range salts {
		if _, ok := pools[key]; !ok {
			t.Fatalf("salt %s has no pool", key)
		}
	}
}

func TestPhraseBankNoDuplicatesOrEmpty(t *testing.T) {
	seen := map[string]string{}
	for key, pool := range pools {
		for _, p := range pool {
			if strings.TrimSpace(p) == "" {
				t.Fatalf("pool %s contains an empty phrase", key)
			}
			if prev, dup := seen[p]; dup {
				t.Fatalf("phrase %q in both %s and %s", p, prev, key)
			}
			seen[p] = key
		}
	}
}

// bannedSubstrings are hard bans: insults/derogatory words, social-rank
// claims, and personality or intelligence judgments. Portraits describe
// usage behavior only.
var bannedSubstrings = []string{
	// insults / derogatory
	"菜", "废", "败家", "蠢", "傻", "笨", "懒", "垃圾", "差劲",
	// social-rank claims (超过 N% 用户, 排名, 榜…)
	"超过", "排名", "榜", "%", "用户", "第一", "最强",
	// personality / intelligence judgments
	"聪明", "天才", "勤奋", "努力",
}

func TestPhraseBankNoBannedSubstrings(t *testing.T) {
	for key, pool := range pools {
		for _, p := range pool {
			for _, b := range bannedSubstrings {
				if strings.Contains(p, b) {
					t.Fatalf("pool %s phrase %q contains banned %q", key, p, b)
				}
			}
		}
	}
}
