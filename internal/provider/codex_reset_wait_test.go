package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/settings"
)

// setResetWait saves how many hours before a used-up week renews no reset
// is used by itself for it, in a sandbox HOME.
func setResetWait(t *testing.T, hours int) {
	t.Helper()
	if err := SetCodexResetWait(hours); err != nil {
		t.Fatal(err)
	}
}

// Steve055s on X: a week that renews within the hours set keeps its
// reset, unless the reset runs out by then; off, the default, never
// waits. Both sides of the boundary, to the second.
func TestWaitsForWeek(t *testing.T) {
	signIn(t)
	now := time.Now()
	at := func(d time.Duration) *time.Time { u := now.Add(d); return &u }
	day := 24 * time.Hour
	for _, c := range []struct {
		name        string
		wait        int
		week, until *time.Time
		want        bool
	}{
		{"off", 0, at(time.Hour), at(20 * day), false},
		{"renews in 23h of 24", 24, at(23 * time.Hour), at(20 * day), true},
		{"renews at 24h of 24", 24, at(24 * time.Hour), at(20 * day), true},
		{"renews a second past 24h", 24, at(24*time.Hour + time.Second), at(20 * day), false},
		{"renews in 3 days of 24h", 24, at(3 * day), at(20 * day), false},
		{"the reset never runs out", 24, at(5 * time.Hour), nil, true},
		{"the reset runs out before the renewal", 24, at(5 * time.Hour), at(4 * time.Hour), false},
		{"the reset runs out at the renewal", 24, at(5 * time.Hour), at(5 * time.Hour), false},
		{"the reset runs out a second after", 24, at(5 * time.Hour), at(5*time.Hour + time.Second), true},
		{"the week's end not known", 24, nil, at(20 * day), false},
		{"the most, a week", settings.CodexResetWaitMost, at(6 * day), at(20 * day), true},
	} {
		setResetWait(t, c.wait)
		if got := waitsForWeek(c.week, c.until, now); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSetCodexResetWait(t *testing.T) {
	signIn(t)
	if s := settings.Load(); s.ResetWait() != 0 {
		t.Fatalf("unset waits %v, want none", s.ResetWait())
	}
	for _, h := range []int{-1, settings.CodexResetWaitMost + 1} {
		if err := SetCodexResetWait(h); err == nil {
			t.Errorf("%d taken", h)
		}
	}
	if s := settings.Load(); s.CodexResetWait != 0 {
		t.Fatalf("a refused wait was written: %d", s.CodexResetWait)
	}
	setResetWait(t, 24)
	if s := settings.Load(); s.ResetWait() != 24*time.Hour {
		t.Fatalf("saved %v, want 24h", s.ResetWait())
	}
	// a hand-edited file is held to the range
	if (settings.Settings{CodexResetWait: 9999}).ResetWait() != time.Duration(settings.CodexResetWaitMost)*time.Hour {
		t.Fatal("not held to a week")
	}
	if (settings.Settings{CodexResetWait: -5}).ResetWait() != 0 {
		t.Fatal("a negative wait waits")
	}
}

// fakeWeekRenewing stands in for ChatGPT with an account whose week is
// used up and renews in weekIn, holding the resets whose times to run out
// are given (0: never); it says which were spent.
func fakeWeekRenewing(t *testing.T, weekIn time.Duration, cards map[string]time.Duration) func() []string {
	now := time.Now()
	var mu sync.Mutex
	var spent []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/backend-api/wham/usage":
			json.NewEncoder(w).Encode(map[string]any{"plan_type": "pro",
				"rate_limit": map[string]any{
					"primary_window":   map[string]any{"used_percent": 10, "limit_window_seconds": 18000},
					"secondary_window": map[string]any{"used_percent": 100, "limit_window_seconds": 604800, "reset_at": now.Add(weekIn).Unix()},
				},
				"rate_limit_reset_credits": map[string]any{"available_count": len(cards)}})
		case "/backend-api/wham/rate-limit-reset-credits":
			var list []any
			for id, in := range cards {
				c := map[string]any{"id": id, "status": "available"}
				if in != 0 {
					c["expires_at"] = now.Add(in).UTC().Format(time.RFC3339)
				}
				list = append(list, c)
			}
			json.NewEncoder(w).Encode(map[string]any{"available_count": len(cards), "credits": list})
		case "/backend-api/wham/rate-limit-reset-credits/consume":
			var b struct {
				Credit string `json:"credit_id"`
			}
			json.NewDecoder(r.Body).Decode(&b)
			mu.Lock()
			spent = append(spent, b.Credit)
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"code": "reset", "windows_reset": 2})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fake.Close)
	oldBase := CodexBase
	CodexBase = fake.URL + "/backend-api/codex"
	t.Cleanup(func() { CodexBase = oldBase })
	return func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), spent...) }
}

// The week's auto-use, which the gateway's autoReset, autoResetSignedIn
// and resetBeforeCredits all spend through, keeps the reset while the
// week renews within the hours set and the reset outlasts it; it spends
// the one that runs out first when that is lost by the renewal anyway.
func TestWeekResetWaitsForRenewal(t *testing.T) {
	day := 24 * time.Hour
	for _, c := range []struct {
		name   string
		wait   int
		weekIn time.Duration
		cards  map[string]time.Duration
		want   string // "" none
	}{
		{"off", 0, 5 * time.Hour, map[string]time.Duration{"far": 20 * day}, "far"},
		{"renews in 5h of 24", 24, 5 * time.Hour, map[string]time.Duration{"far": 20 * day}, ""},
		{"renews in 23h of 24", 24, 23 * time.Hour, map[string]time.Duration{"far": 20 * day}, ""},
		{"renews in 25h of 24", 24, 25 * time.Hour, map[string]time.Duration{"far": 20 * day}, "far"},
		{"one that never runs out", 24, 5 * time.Hour, map[string]time.Duration{"never": 0}, ""},
		{"one runs out before the renewal", 24, 5 * time.Hour, map[string]time.Duration{"soon": 2 * time.Hour, "far": 20 * day}, "soon"},
		{"one runs out after it", 24, 5 * time.Hour, map[string]time.Duration{"mid": 6 * time.Hour, "far": 20 * day}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			signIn(t)
			spent := fakeWeekRenewing(t, c.weekIn, c.cards)
			if err := SetCodexAutoResetCase("me@example.com", ResetCaseWeek, true); err != nil {
				t.Fatal(err)
			}
			setResetWait(t, c.wait)
			out, err := AutoUseCodexReset(t.Context(), "me@example.com")
			if err != nil {
				t.Fatal(err)
			}
			got := spent()
			if c.want == "" {
				if len(got) != 0 || out.Code != resetWeekRenews {
					t.Fatalf("spent %v (%+v), want none for the renewal", got, out)
				}
				if out.Text() == resetWeekRenews {
					t.Fatal("week_renews has no words")
				}
				return
			}
			if out.Code != "reset" || len(got) != 1 || got[0] != c.want {
				t.Fatalf("spent %v (%+v), want %s", got, out, c.want)
			}
		})
	}
}

// The wait is the week's alone: the one about to run out, and one spent
// by hand, are spent however soon the week renews.
func TestResetWaitLeavesExpiringAndByHand(t *testing.T) {
	signIn(t)
	spent := fakeWeekRenewing(t, 5*time.Hour, map[string]time.Duration{"a": 20 * time.Minute, "b": 20 * 24 * time.Hour})
	setResetWait(t, 24)
	if out, err := spendCodexReset(t.Context(), "me@example.com", "a", ResetUsedExpiring); err != nil || out.Code != "reset" {
		t.Fatalf("expiring: %+v %v", out, err)
	}
	if out, err := spendCodexReset(t.Context(), "me@example.com", "b", ResetUsedByHand); err != nil || out.Code != "reset" {
		t.Fatalf("by hand: %+v %v", out, err)
	}
	if got := spent(); len(got) != 2 {
		t.Fatalf("spent %v, want a and b", got)
	}
}

// The readings' route (noteWeekCredits: the credits going while the week
// is used up) waits the same: its week renews in 72 hours and its reset
// runs out in 5 days.
func TestWeekCreditsGoingWaitsForRenewal(t *testing.T) {
	for name, c := range map[string]struct {
		wait  int
		spent int32
	}{
		"off":               {0, 1},
		"72h, renews in":    {72, 0},
		"48h, renews after": {48, 1},
	} {
		t.Run(name, func(t *testing.T) {
			home := signIn(t)
			forgetWeekCredits(t)
			if err := SetCodexAutoResetCase("me@example.com", ResetCaseWeek, true); err != nil {
				t.Fatal(err)
			}
			setResetWait(t, c.wait)
			var balance atomic.Value
			consumed := fakeWeekOnCredits(t, &balance)
			for _, b := range []string{"500", "490"} {
				balance.Store(b)
				if q := codexSubscriptionUsage(t.Context(), filepath.Join(home, ".codex"), "me@example.com"); q.Error != "" {
					t.Fatalf("reading: %s", q.Error)
				}
				endReadsOut(t, "after a reading")
			}
			if got := consumed.Load(); got != c.spent {
				t.Fatalf("spent %d resets, want %d", got, c.spent)
			}
		})
	}
}

// The card's resets say when the used-up week renews, which the GUI
// weighs the wait by; nothing while the week isn't used up.
func TestResetsSayWhenTheWeekRenews(t *testing.T) {
	home := signIn(t)
	fakeWeekRenewing(t, 5*time.Hour, map[string]time.Duration{"far": 20 * 24 * time.Hour})
	q := codexSubscriptionUsage(t.Context(), filepath.Join(home, ".codex"), "me@example.com")
	endReadsOut(t, "after a reading")
	if q.Error != "" || q.Resets == nil || q.Resets.WeekEnds == nil {
		t.Fatalf("no week's end: %+v", q.Resets)
	}
	if d := time.Until(*q.Resets.WeekEnds); d < 4*time.Hour || d > 6*time.Hour {
		t.Fatalf("the week renews in %v, want 5h", d)
	}
}
