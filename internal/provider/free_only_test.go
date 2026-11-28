package provider

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
)

// Free only (niuniu_8964 on X: 将"只选免费"按钮改成和"每系最新"一样的筛选条件;
// 未来免费模型列表发生变化时，不会自动更新列表): with nothing picked, of the
// vendor's list only the models it marks free are served, every one of
// them, so a model the vendor makes free later is served and one it stops
// giving away leaves, with nothing saved. The list itself is fetched again
// every LiveFilterEvery with no schedule set, and a fetch that fails keeps
// the list as last fetched.
func TestFreeOnlyFollowsTheList(t *testing.T) {
	autolistHome(t)
	t.Cleanup(func() {
		listsTried.Lock()
		clear(listsTried.m)
		listsTried.Unlock()
	})
	r := newRelayList(t, "qwen/qwen3.8-27b:free", "openai/gpt-6", "deepseek/deepseek-v4:free", "anthropic/claude-opus-5-5")
	if err := Save(Provider{ID: "router", Name: "Router", Chat: r.srv.URL + "/v1", Key: "sk-router", FreeOnly: true}); err != nil {
		t.Fatal(err)
	}
	plain := newRelayList(t, "x-1:free", "x-2")
	if err := Save(Provider{ID: "plain", Name: "Plain", Chat: plain.srv.URL + "/v1", Key: "sk-plain"}); err != nil {
		t.Fatal(err)
	}
	p, _ := Find("router")
	if !p.FreeOnly || p.RefreshSchedule() != LiveFilterEvery || p.RefreshInterval() != 6*time.Hour {
		t.Fatalf("saved: free %v, schedule %q, every %s", p.FreeOnly, p.RefreshSchedule(), p.RefreshInterval())
	}
	if p, _ := Find("plain"); p.RefreshSchedule() != "" || p.RefreshInterval() != 0 {
		t.Fatalf("a provider with no live filter got a schedule: %q", p.RefreshSchedule())
	}

	// never fetched: fetched on the gateway's first look, the plain
	// provider not
	now := time.Now()
	if got := refreshDueLists(context.Background(), now); !slices.Equal(got, []string{"router"}) {
		t.Fatalf("asked %v, want [router]", got)
	}
	if plain.asked.Load() != 0 {
		t.Fatalf("a provider with no schedule was asked %d times", plain.asked.Load())
	}
	if got := servedIDs(t, "router"); !slices.Equal(got, []string{"qwen/qwen3.8-27b:free", "deepseek/deepseek-v4:free"}) {
		t.Fatalf("served: %v", got)
	}

	// the vendor makes a model free and stops giving one away: served
	// once the list is fetched again, after 6 hours, with nothing saved
	r.set("qwen/qwen3.8-27b", "openai/gpt-6", "deepseek/deepseek-v4:free", "moonshotai/kimi-k3:free", "anthropic/claude-opus-5-5")
	if got := refreshDueLists(context.Background(), now.Add(5*time.Hour)); len(got) != 0 {
		t.Fatalf("asked %v before 6 hours", got)
	}
	if got := refreshDueLists(context.Background(), now.Add(6*time.Hour+time.Minute)); !slices.Equal(got, []string{"router"}) {
		t.Fatalf("asked %v after 6 hours", got)
	}
	want := []string{"deepseek/deepseek-v4:free", "moonshotai/kimi-k3:free"}
	if got := servedIDs(t, "router"); !slices.Equal(got, want) {
		t.Fatalf("served after the vendor's change: %v, want %v", got, want)
	}

	// a fetch that fails keeps the list as last fetched
	r.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.asked.Add(1)
		http.Error(w, "down", http.StatusBadGateway)
	})
	n := r.asked.Load()
	if got := refreshDueLists(context.Background(), now.Add(13*time.Hour)); !slices.Equal(got, []string{"router"}) || r.asked.Load() != n+1 {
		t.Fatalf("asked %v (%d times) after 13 hours", got, r.asked.Load()-n)
	}
	if got := servedIDs(t, "router"); !slices.Equal(got, want) {
		t.Fatalf("served after a failed fetch: %v, want %v", got, want)
	}

	// set off by its user: kept off, asked only when told
	if err := SetRefreshEvery("router", "off"); err != nil {
		t.Fatal(err)
	}
	if p, _ := Find("router"); p.RefreshEvery != "off" || p.RefreshInterval() != 0 {
		t.Fatalf("off: every %q, interval %s", p.RefreshEvery, p.RefreshInterval())
	}
	if got := refreshDueLists(context.Background(), now.Add(30*time.Hour)); len(got) != 0 {
		t.Fatalf("a list set off was asked: %v", got)
	}
}

// Free only serves every free model, not the first 24 of them, as the
// one-off button's picks weren't cut either; the vendor's word (Free) and
// a $0 price count as its name does, as the editor's FREE badge has them.
func TestFreeOnlyServesEveryFreeModel(t *testing.T) {
	azureHome(t)
	var ms []catalog.Model
	var want []string
	for i := range 40 {
		id := fmt.Sprintf("vendor/model-%d:free", i)
		ms = append(ms, catalog.Model{ID: id}, catalog.Model{ID: fmt.Sprintf("vendor/paid-%d", i)})
		want = append(want, id)
	}
	ms = append(ms, catalog.Model{ID: "plan/given", Free: true}, catalog.Model{ID: "freedom-7b"}, catalog.Model{ID: "x/gpt-free-tier"})
	want = append(want, "plan/given", "x/gpt-free-tier")
	if err := catalog.SaveLive("many", "https://many.example.invalid/v1", ms); err != nil {
		t.Fatal(err)
	}
	if err := Save(Provider{ID: "many", Name: "Many", Key: "sk-x", Chat: "https://many.example.invalid/v1", FreeOnly: true}); err != nil {
		t.Fatal(err)
	}
	if got := servedIDs(t, "many"); !slices.Equal(got, want) {
		t.Fatalf("served %d: %v\nwant %d: %v", len(got), got, len(want), want)
	}
	// the vendor's list, as before, with Free only off
	p, _ := Find("many")
	p.FreeOnly = false
	if err := Save(*p); err != nil {
		t.Fatal(err)
	}
	if got := servedIDs(t, "many"); len(got) != manyModels {
		t.Fatalf("Free only off: %d served, want %d", len(got), manyModels)
	}
}

// A pick, Select none or Best in family clears Free only; a save of
// something else keeps it, and a file without it reads as before.
func TestFreeOnlyClearedByOtherWays(t *testing.T) {
	azureHome(t)
	if err := catalog.SaveLive("fo", "https://fo.example.invalid/v1", []catalog.Model{{ID: "a:free"}, {ID: "b"}}); err != nil {
		t.Fatal(err)
	}
	find := func() Provider {
		t.Helper()
		p, err := Find("fo")
		if err != nil {
			t.Fatal(err)
		}
		return *p
	}
	if err := Save(Provider{ID: "fo", Name: "Fo", Key: "sk-x", Chat: "https://fo.example.invalid/v1"}); err != nil {
		t.Fatal(err)
	}
	if got := servedIDs(t, "fo"); !slices.Equal(got, []string{"a:free", "b"}) {
		t.Fatalf("before: %v", got)
	}
	p := find()
	p.FreeOnly = true
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	p = find()
	p.Name = "Fo renamed"
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	if !find().FreeOnly {
		t.Fatal("a rename dropped Free only")
	}
	if err := SetModels("fo", []string{"b"}); err != nil {
		t.Fatal(err)
	}
	if got := find(); got.FreeOnly || got.RefreshSchedule() != "" {
		t.Fatalf("a pick: free %v, schedule %q", got.FreeOnly, got.RefreshSchedule())
	}
	p = find()
	p.Models, p.FreeOnly, p.NoModels = nil, true, true
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	if got := find(); got.FreeOnly || !got.ServesNone() {
		t.Fatalf("Select none: free %v", got.FreeOnly)
	}
	p = find()
	p.NoModels, p.FreeOnly, p.BestInFamily = false, true, true
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	if got := find(); got.FreeOnly || !got.BestInFamily || got.RefreshSchedule() != LiveFilterEvery {
		t.Fatalf("Best in family: free %v best %v schedule %q", got.FreeOnly, got.BestInFamily, got.RefreshSchedule())
	}
}
