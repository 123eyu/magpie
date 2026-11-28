package usage

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/sessions"
)

// Calls in Claude Code's own session log on a Claude model only Anthropic
// prices, through what it told Claude Code (additionalModelCostsCache),
// are priced in the rows and the total (hagimix on X: cost "—", "95 次
// 调用未计价"). The price is taken as the rows are read, so calls recorded
// before are priced too.
func TestClaudeCodeSessionCallsAtAnthropicsPrice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755)
	os.WriteFile(catalog.CachePath(), []byte(`{"anthropic":{"id":"anthropic","models":{
	    "claude-fable-5-2":{"id":"claude-fable-5-2","cost":{"input":8,"output":40,"cache_read":0.2,"cache_write":10}}}}}`), 0o644)
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"additionalModelCostsCache":{
	    "claude-mythos-5-3":{"inputTokens":12,"outputTokens":60,"promptCacheWriteTokens":15,"promptCacheReadTokens":1.2}}}`), 0o600)
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	now := holdClock(t, time.Date(2026, 11, 28, 12, 0, 0, 0, time.Local))
	tok := sessions.Tokens{Input: 1000, Output: 2000, CacheRead: 100000}
	old := LogCalls
	LogCalls = func(time.Time) []sessions.Call {
		return []sessions.Call{
			{Time: now.Add(-time.Hour), Agent: "claude", Session: "c1", Model: "claude-mythos-5-3", Tokens: tok},
			{Time: now.Add(-2 * time.Hour), Agent: "claude", Session: "c1", Model: "claude-mythos-5-3[1m]", Tokens: tok},
			{Time: now.Add(-3 * time.Hour), Agent: "claude", Session: "c2", Model: "claude-mythos-5-2", Tokens: tok},
		}
	}
	t.Cleanup(func() { LogCalls = old })

	cost := func(in, out, cr float64) float64 { return (1000*in + 2000*out + 100000*cr) / 1e6 }
	want := map[string]float64{
		"claude-mythos-5-3": cost(12, 60, 1.2),
		// no price of its own: its Fable's (mythosFable)
		"claude-mythos-5-2": cost(8, 40, 0.2),
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	rows, total, _ := Ledger(Month, Filter{})
	if len(rows) != 3 {
		t.Fatalf("rows %d", len(rows))
	}
	var sum float64
	for _, r := range rows {
		w, ok := want[r.Model]
		if !ok {
			w = want["claude-mythos-5-3"] // the [1m] one, its window cut from the name
		}
		if !r.Priced || !near(r.Cost, w) {
			t.Errorf("%s: priced %v cost %v, want %v", r.Model, r.Priced, r.Cost, w)
		}
		sum += w
	}
	if total.Unpriced != 0 || !near(total.Cost, sum) {
		t.Errorf("total: cost %v unpriced %d, want %v and 0", total.Cost, total.Unpriced, sum)
	}
}
