package provider

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// #4532: a saved Claude account switched on but not needed for weeks
// (switch-only) had nothing run on it, and its refresh token sat idle
// until it was found signed out. The keep-alive has Claude Code itself
// run on it, in its own config directory, and keeps what Claude Code made
// of the sign-in; magpie asks Anthropic nothing (fakeClaudeUsage fails a
// test that does).
func TestRenewLoginsKeepsSavedClaudeSignedIn(t *testing.T) {
	claudeThree(t) // team@ and max@ saved, Claude Code on pro@
	var text atomic.Value
	text.Store("Current week (all models): 1% used · resets Nov 6 at 11am (Asia/Dubai)")
	fakeClaudeUsage(t, &text, nil)
	var mu sync.Mutex
	var dirs []string
	var fail atomic.Value
	fail.Store("")
	rotate := atomic.Bool{}
	UsageClaudeVia(func(_ context.Context, dir string) (string, error) {
		mu.Lock()
		dirs = append(dirs, dir)
		mu.Unlock()
		if msg := fail.Load().(string); msg != "" {
			return "", errors.New(msg)
		}
		if rotate.Load() {
			// as Claude Code refreshes a sign-in that is due
			writeFile(t, filepath.Join(dir, ".credentials.json"), map[string]any{"claudeAiOauth": map[string]any{
				"accessToken": "tok-max-2", "refreshToken": "r-max-2",
				"expiresAt": time.Now().Add(8 * time.Hour).UnixMilli(), "subscriptionType": "max"}})
		}
		return text.Load().(string), nil
	})
	runs := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := slices.Clone(dirs)
		dirs = nil
		return out
	}
	max, team := "max@x.com", "team@x.com · Acme"
	if err := SetLoginOn("claude", max, true); err != nil {
		t.Fatal(err)
	}
	saved := func(user string) savedLogin {
		for _, l := range readLogins() {
			if l.Agent == "claude" && l.User == user {
				return l
			}
		}
		t.Fatalf("%s isn't saved", user)
		return savedLogin{}
	}
	ctx := context.Background()

	// seen in Claude Code's hands a moment ago: not due, nothing run
	if rs := RenewLogins(ctx, keepAliveEvery); len(rs) != 1 || rs[0].User != max || rs[0].Renewed || len(runs()) != 0 {
		t.Fatalf("renewed what wasn't due: %+v", rs)
	}

	// due: Claude Code runs once, in max@'s own directory; team@, off,
	// and pro@, Claude Code's own, are left alone
	rotate.Store(true)
	rs := RenewLogins(ctx, 0)
	if len(rs) != 1 || rs[0].Agent != "claude" || rs[0].User != max || !rs[0].Renewed || rs[0].Err != "" || rs[0].Lapsed {
		t.Fatalf("renew: %+v", rs)
	}
	if got := runs(); len(got) != 1 || got[0] != claudeAccountDir(max) {
		t.Fatalf("Claude Code ran in %q, want only %q", got, claudeAccountDir(max))
	}
	l := saved(max)
	if c, _ := parseClaudeCredentials(l.Auth); c.OAuth.RefreshToken != "r-max-2" {
		t.Fatalf("the sign-in Claude Code refreshed isn't kept: %+v", c.OAuth)
	}
	if time.Since(l.Renewed) > time.Minute || renewalDue(l, keepAliveEvery, time.Now()) || !renewalDue(l, keepAliveEvery, time.Now().Add(keepAliveEvery+time.Hour)) {
		t.Fatalf("renewed at %v", l.Renewed)
	}
	if c, _ := parseClaudeCredentials(saved(team).Auth); c.OAuth.RefreshToken != "r-team@x.com" {
		t.Fatalf("team@, switched off, changed: %+v", c.OAuth)
	}
	if c, _ := parseClaudeCredentials(saved("pro@x.com").Auth); c.OAuth.RefreshToken != "r-pro@x.com" {
		t.Fatalf("Claude Code's own account changed: %+v", c.OAuth)
	}

	// a sign-in Claude Code needn't refresh yet: the run is the renewal,
	// so the account isn't run again every hour
	rotate.Store(false)
	if rs := RenewLogins(ctx, 0); !rs[0].Renewed || len(runs()) != 1 {
		t.Fatalf("a live sign-in: %+v", rs)
	}
	if rs := RenewLogins(ctx, keepAliveEvery); rs[0].Renewed || len(runs()) != 0 {
		t.Fatalf("run again within its eight days: %+v", rs)
	}

	// no answer: it may yet renew, so nothing is marked
	fail.Store("Claude Code: network is unreachable")
	if rs := RenewLogins(ctx, 0); rs[0].Err == "" || rs[0].Lapsed || rs[0].Renewed || saved(max).Lapsed != "" {
		t.Fatalf("a network failure lapsed it: %+v", rs)
	}
	runs()

	// refused: the account's row says to sign in again, as a request's
	// refusal does, and the keep-alive runs nothing on it after
	fail.Store("Claude Code: OAuth session expired and could not be refreshed. Please run /login.")
	rs = RenewLogins(ctx, 0)
	if len(rs) != 1 || !rs[0].Lapsed || rs[0].Renewed || !strings.Contains(rs[0].Err, "sign in again") {
		t.Fatalf("refused: %+v", rs)
	}
	for _, lg := range Logins("claude") {
		if (lg.User == max) != (lg.Lapsed != "") {
			t.Fatalf("lapsed: %+v", lg)
		}
	}
	runs()
	if rs := RenewLogins(ctx, 0); len(rs) != 0 || len(runs()) != 0 {
		t.Fatalf("a signed-out account was run again: %+v", rs)
	}
}

// Claude Code empties the sign-in in its directory when Anthropic refuses
// its refresh, and says no more of it: that too is the account signed out.
func TestRenewLoginsClaudeEmptiedSignIn(t *testing.T) {
	claudeThree(t)
	var text atomic.Value
	text.Store("Current week (all models): 1% used")
	fakeClaudeUsage(t, &text, nil)
	max := "max@x.com"
	UsageClaudeVia(func(_ context.Context, dir string) (string, error) {
		writeFile(t, filepath.Join(dir, ".credentials.json"), map[string]any{"claudeAiOauth": map[string]any{
			"accessToken": "", "refreshToken": "", "expiresAt": 0}})
		return "", errors.New("Claude Code: Not logged in · Please run /login")
	})
	if err := SetLoginOn("claude", max, true); err != nil {
		t.Fatal(err)
	}
	rs := RenewLogins(context.Background(), 0)
	if len(rs) != 1 || !rs[0].Lapsed || rs[0].Renewed {
		t.Fatalf("emptied: %+v", rs)
	}
	for _, lg := range Logins("claude") {
		if lg.User == max && lg.Lapsed == "" {
			t.Fatalf("not marked: %+v", lg)
		}
	}
}

// A Claude account added by setup-token has no refresh token to keep, and
// one switched off isn't run on: only the accounts on are.
func TestRenewLoginsLeavesClaudeSetupToken(t *testing.T) {
	l := savedLogin{Agent: "claude", User: "ci@x.com", On: true, Auth: []byte(`{"setupToken":"sk-ant-oat01-x"}`)}
	if claudeKeptAlive(l) {
		t.Fatal("a setup-token account is kept alive")
	}
	signedIn := []byte(`{"claudeAiOauth":{"accessToken":"a","refreshToken":"r"}}`)
	if claudeKeptAlive(savedLogin{Agent: "claude", User: "x", Auth: signedIn}) {
		t.Fatal("an account switched off is kept alive")
	}
	if !claudeKeptAlive(savedLogin{Agent: "claude", User: "x", On: true, Auth: signedIn}) {
		t.Fatal("an account switched on isn't kept alive")
	}
}
