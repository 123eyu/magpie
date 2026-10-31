package provider

// Keeping saved accounts signed in. A ChatGPT account saved in magpie but
// not the one Codex is signed in to is refreshed only when it is used or
// its allowance looked at — and one waiting its turn (smart
// routing leaves the accounts whose week resets later for days) could sit
// unused past its refresh token's life, and be found signed out just when
// it's needed. So whichever magpie runs the gateway renews each such
// sign-in about as often as Codex renews its own: every eight days, one
// account an hour, never all of them at once. The account the agent is signed in to is the
// agent's own to refresh, and is left to it.
//
// A saved Claude account that is on goes the same way, but is never
// refreshed by magpie, nor is Anthropic asked anything with its sign-in
// (9bf6d4352): Claude Code refreshes it whenever it runs on it
// (claude_dirs.go), so the keep-alive runs Claude Code's /usage once in the
// account's own config directory, as its card's refresh does, and keeps
// what Claude Code made of the sign-in (syncSavedClaude). One switched on
// to stand behind Claude Code's own but not needed for weeks
// (switch-only) otherwise had nothing run on it, and was found signed out
// only when the switch reached it (#4532). How long Anthropic keeps an idle
// Claude refresh token isn't known, so it is run as often as a ChatGPT
// one is renewed, every eight days.

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/settings"
)

// keepAliveEvery is how long a saved sign-in goes before it is renewed:
// Codex's own TOKEN_REFRESH_INTERVAL (codex-rs login/src/auth/manager.rs,
// 8 days). Once a day renewed every idle account eight times as often as
// Codex does, each one in the same hourly look as the others: a burst of
// N refreshes from one address every day (owl on Discord).
const keepAliveEvery = 8 * 24 * time.Hour

// keepAliveAtOnce is how many due sign-ins one look renews; the rest wait
// for the next look, an hour on, so accounts saved together drift apart
// instead of being renewed side by side for good.
const keepAliveAtOnce = 1

// Renewal is what came of renewing one saved account.
type Renewal struct {
	Agent string `json:"agent"`
	User  string `json:"user"`
	// Renewed is false when it wasn't due; Err is why it failed, and Lapsed
	// whether the sign-in is gone (the vendor refused it) or may yet renew.
	Renewed bool   `json:"renewed"`
	Err     string `json:"error,omitempty"`
	Lapsed  bool   `json:"lapsed,omitempty"`
}

// RenewLogins renews the saved ChatGPT sign-ins, and the saved Claude
// sign-ins that are on, not in the agent's hands that have gone every or
// longer without it; every 0 renews all of them.
func RenewLogins(ctx context.Context, every time.Duration) []Renewal {
	return renewLogins(ctx, every, 0)
}

// renewLogins is RenewLogins renewing at most atOnce of the due sign-ins
// (0: all of them); a due one past that is left for the next look and
// reported as not renewed.
func renewLogins(ctx context.Context, every time.Duration, atOnce int) []Renewal {
	if settings.AccountDiscoveryOff() {
		return nil
	}
	rememberLogins(false)
	loginsMu.Lock()
	active := map[string]string{}
	for _, a := range loginAgents {
		if l, ok := liveLogin(a); ok {
			active[a] = l.User
		}
	}
	ls := readLogins()
	loginsMu.Unlock()
	out := []Renewal{}
	tried := 0
	for _, l := range ls {
		if l.Agent != "codex" && !claudeKeptAlive(l) || strings.EqualFold(active[l.Agent], l.User) || leftAlone(l.Agent) {
			continue
		}
		r := Renewal{Agent: l.Agent, User: l.User}
		if every > 0 && !renewalDue(l, every, time.Now()) || atOnce > 0 && tried >= atOnce {
			out = append(out, r)
			continue
		}
		tried++
		if l.Agent == "claude" {
			out = append(out, renewClaude(ctx, r))
			continue
		}
		why := ctx
		if every > 0 {
			why = refreshingFor(ctx, codexRefreshScheduled)
		}
		if _, _, err := renewSavedLogin(why, l.Agent, l.User, true); err != nil {
			r.Err = err.Error()
			var refused refreshRefused
			r.Lapsed = errors.As(err, &refused)
		} else {
			r.Renewed = true
		}
		out = append(out, r)
	}
	return out
}

// claudeKeptAlive says the saved Claude login l is one the keep-alive
// runs Claude Code on: switched on, with a sign-in Claude Code refreshes
// (a setup-token has none, and lasts a year), and not signed out already,
// which its row says.
func claudeKeptAlive(l savedLogin) bool {
	return l.Agent == "claude" && l.On && claudeSetupToken(l.Auth) == "" && claudeSignedOut(l) == ""
}

// renewClaude has Claude Code keep the saved Claude account r.User signed
// in: its /usage run once in the account's config directory, where Claude
// Code refreshes the sign-in when it is due, and what it made of it kept in
// logins.json. A run that went through is the account's renewal, refreshed
// or not (a sign-in Claude Code needn't refresh yet is a live one); one
// whose sign-in Claude Code was refused is marked as needing a new one, on
// the account's row, as a request's refusal is (NoteClaudeSignInFailure).
func renewClaude(ctx context.Context, r Renewal) Renewal {
	version := ClaudeLoginVersion(r.User)
	_, err := readSavedClaudeUsage(ctx, r.User)
	if err != nil {
		NoteClaudeSignInFailure(r.User, version, err.Error())
	}
	loginsMu.Lock()
	defer loginsMu.Unlock()
	ls := readLogins()
	for i := range ls {
		if ls[i].Agent != "claude" || !strings.EqualFold(ls[i].User, r.User) {
			continue
		}
		if why := claudeSignedOut(ls[i]); why != "" {
			r.Lapsed, r.Err = true, why
			return r
		}
		if err != nil {
			r.Err = err.Error()
			return r
		}
		ls[i].Renewed = time.Now().UTC().Truncate(time.Second)
		if werr := writeLogins(ls); werr != nil {
			r.Err = werr.Error()
			return r
		}
		r.Renewed = true
		return r
	}
	if err != nil {
		r.Err = err.Error()
	}
	return r
}

// renewalDue says whether a saved sign-in has gone every without being
// renewed — since magpie last renewed it, or since it was last seen in the
// agent's hands, which kept it fresh — or its refresh token is near its end.
func renewalDue(l savedLogin, every time.Duration, now time.Time) bool {
	last := l.Renewed
	if l.Seen.After(last) {
		last = l.Seen
	}
	var a struct {
		LastRefresh time.Time `json:"last_refresh"`
	}
	if json.Unmarshal(l.Auth, &a) == nil && a.LastRefresh.After(last) {
		last = a.LastRefresh
	}
	return now.Sub(last) >= every
}

// KeepLoginsAlive renews a saved sign-in that is due, one a look, a minute
// after it starts and every hour after that — a computer that slept through a day
// catches up within the hour of waking — until ctx ends.
func KeepLoginsAlive(ctx context.Context) {
	t := time.NewTimer(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		c, cancel := context.WithTimeout(ctx, 2*time.Minute)
		for _, r := range renewLogins(c, keepAliveEvery, keepAliveAtOnce) {
			if r.Err != "" {
				log.Printf("keeping %s account %s signed in: %s", r.Agent, r.User, r.Err)
			}
		}
		cancel()
		t.Reset(time.Hour)
	}
}
