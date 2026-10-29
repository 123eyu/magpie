package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// The background look at Codex's accounts (KeepOnAnAccountWithRoom) reads
// the account Codex is on, and the others only once that one is spent.
// It read every saved account at once every five minutes, a burst from one
// address no official client makes, for a user holding several ChatGPT
// accounts in magpie with no agent wired (owl on Discord).
func TestCodexLookReadsTheActiveAccountOnly(t *testing.T) {
	home := signIn(t) // me@example.com, acct-1
	rememberLogins(true)
	codexSignIn(t, home, "spare@example.com", "r-spare")
	rememberLogins(true)
	codexSignIn(t, home, "work@example.com", "r-work")
	rememberLogins(true)
	for _, u := range []string{"me@example.com", "spare@example.com"} {
		if err := SetLoginOn("codex", u, true); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	used := map[string]float64{"acct-work@example.com": 20, "acct-1": 20, "acct-spare@example.com": 20}
	hits := map[string]int{}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		id := r.Header.Get("chatgpt-account-id")
		if strings.HasSuffix(r.URL.Path, "/wham/usage") {
			hits[id]++
		}
		pct := used[id]
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"plan_type": "plus", "rate_limit": map[string]any{
			"primary_window": map[string]any{"used_percent": pct, "limit_window_seconds": 18000}}})
	}))
	defer fake.Close()
	old := CodexBase
	CodexBase = fake.URL + "/backend-api/codex"
	t.Cleanup(func() { CodexBase = old })
	look := func() (string, map[string]int) {
		t.Helper()
		loginUsageCache.Lock()
		loginUsageCache.m = nil
		loginUsageCache.Unlock()
		mu.Lock()
		hits = map[string]int{}
		mu.Unlock()
		to, err := SwitchWhenSpent(context.Background(), "codex")
		if err != nil {
			t.Fatal(err)
		}
		CodexUsedUp(context.Background())
		mu.Lock()
		defer mu.Unlock()
		got := map[string]int{}
		for k, v := range hits {
			got[k] = v
		}
		return to, got
	}

	// work@ has room: only work@ is read, by both the switch and CodexUsedUp
	// (the second from the minute's cache)
	if to, got := look(); to != "" || len(got) != 1 || got["acct-work@example.com"] != 1 {
		t.Fatalf("with room: switched to %q, read %v", to, got)
	}
	// work@ spent: now the spares are read, and the first with room taken
	mu.Lock()
	used["acct-work@example.com"] = 100
	mu.Unlock()
	to, got := look()
	if to == "" || got["acct-work@example.com"] != 1 || got["acct-1"]+got["acct-spare@example.com"] == 0 {
		t.Fatalf("spent: switched to %q, read %v", to, got)
	}
	for id, n := range got {
		if n > 1 {
			t.Fatalf("%s read %d times in one look: %v", id, n, got)
		}
	}
}

// Saved sign-ins are renewed as often as Codex renews its own, and one a
// look: renewing every idle account in the same hourly look kept them
// renewed side by side, a burst of N refreshes from one address.
func TestKeepAliveRenewsOneALook(t *testing.T) {
	home := signIn(t)
	rememberLogins(true) // me@ saved
	codexSignIn(t, home, "spare@example.com", "r-spare")
	rememberLogins(true) // spare@ saved
	codexSignIn(t, home, "work@example.com", "r-work")
	rememberLogins(true) // codex on work@
	if keepAliveEvery < 8*24*time.Hour {
		t.Fatalf("saved sign-ins renewed every %v, more often than Codex's 8 days", keepAliveEvery)
	}
	var mu sync.Mutex
	var got []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		got = append(got, body["refresh_token"])
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"access_token": fakeJWT(map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())}),
			"refresh_token": body["refresh_token"] + "-2"})
	}))
	defer fake.Close()
	old := codexTokenURL
	codexTokenURL = fake.URL
	t.Cleanup(func() { codexTokenURL = old })

	rs := renewLogins(context.Background(), 0, keepAliveAtOnce)
	renewed := 0
	for _, r := range rs {
		if r.Renewed {
			renewed++
		}
	}
	if len(rs) != 2 || renewed != 1 || len(got) != 1 {
		t.Fatalf("one look renewed %d of %+v, refreshed %v", renewed, rs, got)
	}
}
