package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func runPricing(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	app, out, errb := testApp(args)
	code := app.Run()
	return code, out.String(), errb.String()
}

func TestPricingRendersCardWithSources(t *testing.T) {
	code, out, _ := runPricing(t, "pricing")
	if code != ExitOK {
		t.Fatalf("exit %d", code)
		for _, want := range []string{
			"whereToken pricing",
			"价目卡 2026-08-19",
			"USD / 1M tokens",
			"Anthropic（anthropic）",
			"来源 https://platform.claude.com/docs/en/about-claude/pricing · 核验 2026-08-19",
			"xAI（xai）",
			"OpenAI（openai）",
			"Moonshot（moonshot）",
			"MiniMax（minimax）",
			"Zhipu（zhipu）",
			"Google（google）",
			"DeepSeek（deepseek）",
			"模型", "输入", "缓存读", "缓存写", "输出",
			"opus-4.1",
			"$15.00",
			"$75.00",
			"不是订阅账单",
			"不是运行日期",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("pricing output missing %q:\n%s", want, out)
			}
		}
	}
}

func TestPricingUnknownIsNeverZero(t *testing.T) {
	_, out, _ := runPricing(t, "pricing")
	if strings.Contains(out, "$0.0000") {
		t.Fatalf("no four-zero bills:\n%s", out)
	}
	// The only $0.00 cells are the ones a card lists as free (Z.ai cache
	// write); every one must carry the 限免 marker. The trailing space in
	// "$0.00 " keeps "$0.005"-style rates out of the count.
	zeros := strings.Count(out, "$0.00 ")
	free := strings.Count(out, "$0.00 限免")
	if zeros == 0 || zeros != free {
		t.Fatalf("$0.00 cells: %d, with 限免: %d\n%s", zeros, free, out)
	}
	// gemini-2.0-flash-lite has no listed cache-read rate: a dash, not $0.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "gemini-2.0-flash-lite") && !strings.Contains(line, "—") {
			t.Fatalf("unpriced component must be —, not $0: %q", line)
		}
	}
}

func TestPricingVendorFilter(t *testing.T) {
	_, out, _ := runPricing(t, "pricing", "--vendor", "anthropic")
	if !strings.Contains(out, "Anthropic（anthropic）") || !strings.Contains(out, "opus-4.6") {
		t.Fatalf("vendor filter lost anthropic:\n%s", out)
	}
	for _, gone := range []string{"OpenAI（openai）", "Google（google）", "gemini-", "glm-"} {
		if strings.Contains(out, gone) {
			t.Fatalf("--vendor anthropic must hide %q:\n%s", gone, out)
		}
	}
}

func TestPricingModelFilterFuzzyAndNormalized(t *testing.T) {
	_, out, _ := runPricing(t, "pricing", "--model", "opus")
	if !strings.Contains(out, "opus-4.1") || strings.Contains(out, "gemini-") {
		t.Fatalf("fuzzy opus:\n%s", out)
	}
	// A provider-prefixed id resolves to its family row through the same
	// normalization the calculator uses.
	_, out, _ = runPricing(t, "pricing", "--model", "claude-opus-4-1")
	if !strings.Contains(out, "opus-4.1") {
		t.Fatalf("normalized id must find the opus-4.1 row:\n%s", out)
	}
	if strings.Contains(out, "opus-4.8") || strings.Contains(out, "opus-4.6") {
		t.Fatalf("sibling rows must not leak in:\n%s", out)
	}
	// A dated suffix is a different id on purpose: it stays unmatched rather
	// than inheriting the family price.
	_, out, _ = runPricing(t, "pricing", "--model", "claude-opus-4-1-20260801")
	if !strings.Contains(out, "没有匹配的价目行") {
		t.Fatalf("dated id must not inherit the family rate:\n%s", out)
	}
}

func TestPricingNoMatchIsHonest(t *testing.T) {
	for _, args := range [][]string{
		{"pricing", "--vendor", "doubao"},
		{"pricing", "--model", "no-such-model"},
	} {
		code, out, _ := runPricing(t, args...)
		if code != ExitOK {
			t.Fatalf("%v: exit %d", args, code)
		}
		if !strings.Contains(out, "没有匹配的价目行") || strings.Contains(out, "$0.00") {
			t.Fatalf("%v must say unpriced, never $0:\n%s", args, out)
		}
	}
}

func TestPricingJSONSchema(t *testing.T) {
	code, out, _ := runPricing(t, "pricing", "--json")
	if code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	var payload struct {
		Card string `json:"card"`
		Unit string `json:"unit"`
	}
	var raw struct {
		Providers []struct {
			Vendor   string           `json:"vendor"`
			Label    string           `json:"label"`
			Source   string           `json:"source"`
			Verified string           `json:"verified"`
			Models   []map[string]any `json:"models"`
		} `json:"providers"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatal(err)
	}
	if payload.Card != "2026-08-19" || payload.Unit != "usd_per_1m_tokens" {
		t.Fatalf("card/unit: %+v", payload)
	}
	if len(raw.Providers) != 8 {
		t.Fatalf("providers: %d", len(raw.Providers))
	}
	var zhipuRow, flashLite map[string]any
	for _, p := range raw.Providers {
		if p.Source == "" || p.Verified == "" {
			t.Fatalf("every provider needs source and verified: %+v", p)
		}
		for _, m := range p.Models {
			if p.Vendor == "zhipu" && m["model"] == "glm-5" {
				zhipuRow = m
			}
			if m["model"] == "gemini-2.0-flash-lite" {
				flashLite = m
			}
		}
	}
	// Free is explicit: cache_create 0 plus the free flag.
	if zhipuRow == nil || zhipuRow["cache_create_free"] != true {
		t.Fatalf("zhipu cache write must be marked free: %v", zhipuRow)
	}
	if v, ok := zhipuRow["cache_create"].(float64); !ok || v != 0 {
		t.Fatalf("free cache write is a real 0: %v", zhipuRow["cache_create"])
	}
	// Unknown is null, never 0.
	if flashLite == nil || flashLite["cache_read"] != nil {
		t.Fatalf("unlisted cache read must be null: %v", flashLite)
	}
}

func TestPricingJSONVendorFilter(t *testing.T) {
	_, out, _ := runPricing(t, "pricing", "--vendor", "google", "--json")
	var raw struct {
		Providers []struct {
			Vendor string `json:"vendor"`
		} `json:"providers"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Providers) != 1 || raw.Providers[0].Vendor != "google" {
		t.Fatalf("%+v", raw.Providers)
	}
}

func TestPricingRejectsReportFlags(t *testing.T) {
	for _, args := range [][]string{
		{"pricing", "--today"},
		{"pricing", "--since", "7d"},
		{"pricing", "--tool", "claude"},
		{"pricing", "--offline"},
	} {
		code, _, errb := runPricing(t, args...)
		if code != ExitUsage || !strings.Contains(errb, "pricing") {
			t.Fatalf("%v: code=%d stderr=%q", args, code, errb)
		}
	}
}

func TestHelpListsPricing(t *testing.T) {
	help := HelpText()
	if !strings.Contains(help, "pricing") {
		t.Fatal("help must list the pricing command")
	}
}
