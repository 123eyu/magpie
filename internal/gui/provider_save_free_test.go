package gui

// Free only (niuniu_8964 on X): the editor's Free only is saved as
// freeOnly with no model picked, kept by a save that doesn't name it, and
// the card serves the models the list marks free, as the editor's FREE
// badge has them (free, zero, a name that says free).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

func TestProviderSaveFreeOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	mux := http.NewServeMux()
	providerRoutes(mux, nil)
	post := func(body string) {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/provider/save", strings.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	}
	card := func() (free bool, on, marked []string) {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/providers", nil))
		var got struct {
			Providers []struct {
				ID       string `json:"id"`
				FreeOnly bool   `json:"freeOnly"`
				Models   []struct {
					ID   string `json:"id"`
					On   bool   `json:"on"`
					Free bool   `json:"free"`
					Zero bool   `json:"zero"`
				} `json:"models"`
			} `json:"providers"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		for _, p := range got.Providers {
			if p.ID != "relay" {
				continue
			}
			for _, m := range p.Models {
				if m.On {
					on = append(on, m.ID)
				}
				if m.Free || m.Zero || catalog.NamedFree(m.ID) {
					marked = append(marked, m.ID)
				}
			}
			return p.FreeOnly, on, marked
		}
		t.Fatal("relay not listed")
		return
	}

	const chat = "http://127.0.0.1:1/v1"
	post(`{"id":"relay","name":"Relay","key":"sk-r","chat":"` + chat + `","new":true}`)
	ms := []catalog.Model{{ID: "qwen/qwen3.8-27b:free"}, {ID: "openai/gpt-6"}, {ID: "plan/given", Free: true}, {ID: "anthropic/claude-opus-5-5"}}
	if err := catalog.SaveLive("relay", chat, ms); err != nil {
		t.Fatal(err)
	}
	freeWant := []string{"qwen/qwen3.8-27b:free", "plan/given"}
	if free, on, marked := card(); free || len(on) != 4 || !slices.Equal(marked, freeWant) {
		t.Fatalf("before: free %v on %v marked %v", free, on, marked)
	}

	post(`{"id":"relay","from":"relay","name":"Relay","key":"","chat":"` + chat + `","models":[],"noModels":false,"bestInFamily":false,"freeOnly":true,"headers":{}}`)
	if free, on, _ := card(); !free || !slices.Equal(on, freeWant) {
		t.Fatalf("free only: %v on %v", free, on)
	}
	if p, _ := provider.Find("relay"); p.RefreshSchedule() != provider.LiveFilterEvery {
		t.Errorf("free only: schedule %q, want %q", p.RefreshSchedule(), provider.LiveFilterEvery)
	}

	// a save that doesn't name it (another editor's, the CLI's) keeps it
	post(`{"id":"relay","from":"relay","name":"Relay","key":"","chat":"` + chat + `","headers":{}}`)
	if free, on, _ := card(); !free || !slices.Equal(on, freeWant) {
		t.Errorf("not named: free %v on %v", free, on)
	}

	// Vendor's list: the editor sends it false
	post(`{"id":"relay","from":"relay","name":"Relay","key":"","chat":"` + chat + `","models":[],"noModels":false,"bestInFamily":false,"freeOnly":false,"headers":{}}`)
	if free, on, _ := card(); free || len(on) != 4 {
		t.Errorf("vendor's list: free %v on %v", free, on)
	}

	// a pick clears it, even sent with it
	post(`{"id":"relay","from":"relay","name":"Relay","key":"","chat":"` + chat + `","models":["openai/gpt-6"],"noModels":false,"freeOnly":true,"headers":{}}`)
	if p, _ := provider.Find("relay"); p.FreeOnly {
		t.Error("a pick kept Free only")
	}
	if free, on, _ := card(); free || !slices.Equal(on, []string{"openai/gpt-6"}) {
		t.Errorf("picked: free %v on %v", free, on)
	}
}
