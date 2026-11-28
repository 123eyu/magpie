package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// A Claude model no catalog lists has its maker's price in what Anthropic
// told Claude Code (additionalModelCostsCache), and that is the price of
// every call to it, not only one through magpie's anthropic provider: a
// call from Claude Code's own session has no provider of magpie's, and
// was left unpriced (hagimix on X: "95 次调用未计价", cost "—"). A Mythos
// Anthropic priced keeps that price over its Fable's.
func TestClaudeCodePriceIsTheMakers(t *testing.T) {
	home := claudeHome(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755)
	os.WriteFile(catalog.CachePath(), []byte(mythosCatalog), 0o644)
	catalog.Reset()
	t.Cleanup(catalog.Reset)
	claudeCodeState(t, filepath.Join(home, ".claude.json"), "a@example.com", nil, nil, map[string]any{
		"claude-mythos-5-3": map[string]any{"inputTokens": 12, "outputTokens": 60, "promptCacheWriteTokens": 15, "promptCacheReadTokens": 1.2},
		"claude-mythos-5-2": map[string]any{"inputTokens": 9, "outputTokens": 45, "promptCacheWriteTokens": 11.25, "promptCacheReadTokens": 0.9},
	})

	for _, c := range []struct {
		id      string
		in, out float64
	}{
		{"claude-mythos-5-3", 12, 60},
		{"claude-mythos-5-3[1m]", 12, 60},
		{"anthropic/claude-mythos-5-3", 12, 60},
		{"claude-mythos-5-2", 9, 45},
		// a Fable listed: the catalog's price
		{"claude-fable-5-2", 8, 40},
	} {
		if pr, ok := MakerPrice(c.id); !ok || pr.Input != c.in || pr.Output != c.out {
			t.Errorf("MakerPrice(%q) = %+v, %v; want $%v / $%v", c.id, pr, ok, c.in, c.out)
		}
	}
	// a call that went through no provider of magpie's (Claude Code's own
	// session, a provider since removed)
	if pr, ok := tariffOf(heldSettings(), Provider{ID: "session-unknown"}, false, "claude-mythos-5-3"); !ok || pr.Input != 12 {
		t.Errorf("tariffOf(session-unknown, claude-mythos-5-3) = %+v, %v; want $12 in", pr, ok)
	}
	if got := PricedName("claude-mythos-5-3"); got != "claude-mythos-5-3" {
		t.Errorf("PricedName(claude-mythos-5-3) = %q, want itself", got)
	}
	if got := PricedName("claude-mythos-5-2"); got != "claude-mythos-5-2" {
		t.Errorf("PricedName(claude-mythos-5-2) = %q, want itself", got)
	}
	if _, ok := MakerPrice("claude-mythos-9"); ok {
		t.Error("claude-mythos-9 priced; neither Anthropic nor a catalog prices it")
	}
}
