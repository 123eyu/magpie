package gateway

import (
	"strconv"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// Steve055s on X: no reset is used by itself for a used-up week that
// renews within the hours set anyway, while the reset outlasts the
// renewal. The fake's week renews in 3 days and its reset runs out in 20:
// 72 hours set waits, 71 spends, on each route that spends one for the
// week (autoResetSignedIn, autoReset, resetBeforeCredits). Held back, the
// turn goes as with no reset held, refused or on the credits, and the log
// says why in one line.
func TestCodexResetWaitsForRenewal(t *testing.T) {
	const why = "reset for me@example.com not used: the week renews within the hours set to wait for it"
	for _, c := range []struct {
		wait  int
		spent bool
	}{{72, false}, {71, true}, {0, true}} {
		name := map[bool]string{true: "spends", false: "waits"}[c.spent] + " at " + strconv.Itoa(c.wait) + "h"
		t.Run("signed in "+name, func(t *testing.T) {
			codexSignedIn(t)
			b := newResetBackend(t, "acct-1")
			if err := provider.SetCodexAutoResetCase("me@example.com", provider.ResetCaseWeek, true); err != nil {
				t.Fatal(err)
			}
			if err := provider.SetCodexResetWait(c.wait); err != nil {
				t.Fatal(err)
			}
			logs := keepLog(t)
			srv := New()
			code, body := resetPost(t, srv)
			tried, spent := b.seen()
			if c.spent {
				if code != 200 || spent != "acct-1" || tried != "acct-1,acct-1" {
					t.Fatalf("%d %s tried %s spent %s", code, body, tried, spent)
				}
				return
			}
			if code != 429 || !strings.Contains(body, "usage_limit_reached") || tried != "acct-1" || spent != "" {
				t.Fatalf("%d %s tried %s spent %s", code, body, tried, spent)
			}
			if got := resetsTraced(srv); len(got) != 0 {
				t.Fatalf("traced %v", got)
			}
			if n := strings.Count(logs.String(), why); n != 1 {
				t.Fatalf("logged %d times why, want 1:\n%s", n, logs.String())
			}
		})
		t.Run("routed "+name, func(t *testing.T) {
			codexSignedIn(t, "spare@example.com")
			b := newResetBackend(t, "acct-1", "acct-2")
			b.held["acct-2"] = 0
			if err := provider.SetCodexAutoResetCase("me@example.com", provider.ResetCaseWeek, true); err != nil {
				t.Fatal(err)
			}
			if err := provider.SetCodexResetWait(c.wait); err != nil {
				t.Fatal(err)
			}
			logs := keepLog(t)
			code, body := resetPost(t, New())
			_, spent := b.seen()
			if c.spent != (spent == "acct-1") || c.spent != (code == 200) {
				t.Fatalf("%d %s spent %q", code, body, spent)
			}
			if !c.spent && !strings.Contains(logs.String(), why) {
				t.Fatalf("no why logged:\n%s", logs.String())
			}
		})
		t.Run("before credits "+name, func(t *testing.T) {
			codexSignedIn(t)
			b := newResetBackend(t, "acct-1")
			b.credits["acct-1"] = true
			creditsUsage(t, map[string]float64{"me@example.com": 100}, "me@example.com")
			if err := provider.SetCodexAutoResetCase("me@example.com", provider.ResetCaseWeek, true); err != nil {
				t.Fatal(err)
			}
			if err := provider.SetCodexResetWait(c.wait); err != nil {
				t.Fatal(err)
			}
			logs := keepLog(t)
			code, body := resetPost(t, New())
			tried, spent := b.seen()
			// answered either way: on the reset, or on its credits as with none held
			if code != 200 || !strings.Contains(body, "pong") || tried != "acct-1" || c.spent != (spent == "acct-1") {
				t.Fatalf("%d %s tried %q spent %q", code, body, tried, spent)
			}
			if !c.spent && !strings.Contains(logs.String(), why) {
				t.Fatalf("no why logged:\n%s", logs.String())
			}
		})
	}
}
