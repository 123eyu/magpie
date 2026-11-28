package gui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// Sec_211 on X (v0.1.5045, Windows 11): "0-onaiproxy" and its copy
// "0-onaiproxy 副本", 16 models each on the Providers page, were nowhere
// in Codex's model list, with no word why. The original was set to "Only
// through routing groups", which the copy took from it: both served only
// through the groups magpie found of each model the two share (auto/m0…),
// and neither was named in the list. The list names a
// provider it leaves out so (left), with why, and a copy no longer takes
// "Only through routing groups" from its original. Built through the
// editor's own save, as the report's providers were: the names (a
// leading "0-", "副本", a URL) and the copy keep nothing out.
func TestAgentModelsNameProvidersLeftForGroups(t *testing.T) {
	home := sandboxHome(t)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	providerRoutes(mux, nil)
	agentModelsAPI(mux)
	do := func(method, path, body string) []byte {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
		}
		return w.Body.Bytes()
	}
	ms := make([]string, 16)
	for i := range ms {
		ms[i] = fmt.Sprintf("m%d", i)
	}
	mj, _ := json.Marshal(ms)
	do("POST", "/api/provider/save", `{"id":"0-onaiproxy","name":"0-onaiproxy","chat":"http://127.0.0.1:1/v1","key":"sk-a","models":`+string(mj)+`,"new":true}`)
	do("POST", "/api/provider/save", `{"id":"https-s2api-top","name":"https://s2api.top/","chat":"http://127.0.0.1:2/v1","key":"sk-b","models":["only"],"new":true}`)
	// the original kept for routing groups, from its editor
	do("POST", "/api/provider/save", `{"id":"0-onaiproxy","name":"0-onaiproxy","chat":"http://127.0.0.1:1/v1","models":`+string(mj)+`,"unlisted":true}`)
	if p, err := provider.Find("0-onaiproxy"); err != nil || !p.Unlisted {
		t.Fatalf("original: %+v %v", p, err)
	}
	// Duplicate's save: the form as draftOf fills it, a new id, no key
	do("POST", "/api/provider/save", `{"id":"0-onaiproxy","name":"0-onaiproxy 副本","chat":"http://127.0.0.1:1/v1","key":"","models":`+string(mj)+`,"new":true,"copyOf":"0-onaiproxy"}`)
	c, err := provider.Find("0-onaiproxy-2")
	if err != nil {
		t.Fatal(err)
	}
	if c.Unlisted || c.Key != "sk-a" || len(c.Models) != 16 {
		t.Fatalf("the copy: unlisted %v, key %q, %d models; want listed, the original's key, 16", c.Unlisted, c.Key, len(c.Models))
	}
	type left struct {
		ID, Name string
		Models   []string
		Unlisted bool
		By, Via  []string
	}
	get := func() (out struct {
		Models []agentModelJSON
		Left   []left
	}) {
		t.Helper()
		if err := json.Unmarshal(do("GET", "/api/agent-models/codex", ""), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	by := func(ms []agentModelJSON) map[string]int {
		out := map[string]int{}
		for _, m := range ms {
			out[m.Group]++
		}
		return out
	}
	got := get()
	if g := by(got.Models); g["0-onaiproxy 副本"] != 16 || g["https://s2api.top/"] != 1 || g["0-onaiproxy"] != 0 {
		t.Fatalf("Codex's list by provider: %v", g)
	}
	// its models reach Codex through the groups magpie found of each model
	// on it and its copy, listed as the model (auto/m0…)
	if len(got.Left) != 1 || got.Left[0].ID != "0-onaiproxy" || got.Left[0].Name != "0-onaiproxy" || !got.Left[0].Unlisted ||
		len(got.Left[0].Models) != 16 || !slices.Equal(got.Left[0].Via, ms) {
		t.Fatalf("left out: %+v", got.Left)
	}
	if !slices.ContainsFunc(got.Models, func(m agentModelJSON) bool { return m.ID == "auto/m0" }) {
		t.Fatalf("the group of m0 isn't in Codex's list: %v", by(got.Models))
	}
	// in a routing group of the user's, the list names it first; another
	// agent's list says the same
	if err := provider.SaveGroup(provider.Group{ID: "onai", Name: "Onai", Members: []string{"0-onaiproxy/m5", "0-onaiproxy/m6"}}); err != nil {
		t.Fatal(err)
	}
	got = get()
	if len(got.Left) != 1 || !slices.Equal(got.Left[0].Via, append([]string{"Onai"}, ms...)) {
		t.Fatalf("left out, in a group: %+v", got.Left)
	}
	// a provider of its own models alone, in no group: no group named
	do("POST", "/api/provider/save", `{"id":"lone","name":"Lone","chat":"http://127.0.0.1:3/v1","key":"sk-c","models":["solo"],"new":true}`)
	do("POST", "/api/provider/save", `{"id":"lone","name":"Lone","chat":"http://127.0.0.1:3/v1","models":["solo"],"unlisted":true}`)
	got = get()
	if i := slices.IndexFunc(got.Left, func(l left) bool { return l.ID == "lone" }); i < 0 || got.Left[i].Via == nil || len(got.Left[i].Via) != 0 {
		t.Fatalf("left out, in no group: %+v", got.Left)
	}
	var pi struct{ Left []left }
	if err := json.Unmarshal(do("GET", "/api/agent-models/pi", ""), &pi); err != nil {
		t.Fatal(err)
	}
	if len(pi.Left) != 2 || pi.Left[0].ID != "0-onaiproxy" {
		t.Fatalf("Pi's left out: %+v", pi.Left)
	}
	// a group listing alone that keeps every model of a provider out names
	// itself as why; one that keeps some leaves the provider listed
	if err := provider.SaveGroup(provider.Group{ID: "s2", Name: "S2", Members: []string{"https-s2api-top/only"}, Unlists: true}); err != nil {
		t.Fatal(err)
	}
	got = get()
	i := slices.IndexFunc(got.Left, func(l left) bool { return l.ID == "https-s2api-top" })
	if i < 0 || got.Left[i].Unlisted || !slices.Equal(got.Left[i].By, []string{"S2"}) || !slices.Equal(got.Left[i].Via, []string{"S2"}) {
		t.Fatalf("left out by a group: %+v", got.Left)
	}
	if err := provider.SaveGroup(provider.Group{ID: "s2", Name: "S2", Members: []string{"0-onaiproxy-2/m3"}, Unlists: true}); err != nil {
		t.Fatal(err)
	}
	got = get()
	if slices.ContainsFunc(got.Left, func(l left) bool { return l.ID == "0-onaiproxy-2" || l.ID == "https-s2api-top" }) {
		t.Fatalf("a provider still listed is named left out: %+v", got.Left)
	}
	// none left out: an empty list, not null
	do("POST", "/api/provider/save", `{"id":"0-onaiproxy","name":"0-onaiproxy","chat":"http://127.0.0.1:1/v1","models":`+string(mj)+`,"unlisted":false}`)
	do("POST", "/api/provider/save", `{"id":"lone","name":"Lone","chat":"http://127.0.0.1:3/v1","models":["solo"],"unlisted":false}`)
	if err := provider.DeleteGroup("s2"); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/agent-models/codex", nil))
	if !strings.Contains(w.Body.String(), `"left":[]`) {
		t.Fatalf("nothing left out: %s", w.Body)
	}
}
