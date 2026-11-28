package gateway

import (
	"encoding/json"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// Free only (niuniu_8964 on X): what an agent is given for the provider
// is the models its list marks free, as the list changes: a model the
// vendor makes free later is listed to agents, and one it stops giving
// away is not, with nothing saved; the paid ones aren't, though a request
// naming one is still served as one naming an unpicked model is.
func TestFreeOnlyAgentsModelList(t *testing.T) {
	f := &fake{t: t, reply: sse(`data: {"choices":[{"delta":{"content":"ok"}}]}`, `data: [DONE]`)}
	setup(t, provider.Chat, f)
	up := httptest.NewServer(f)
	defer up.Close()
	if err := provider.Save(provider.Provider{ID: "router", Name: "Router", Key: "k", Chat: up.URL + "/v1", FreeOnly: true}); err != nil {
		t.Fatal(err)
	}
	listed := func(ids ...string) {
		t.Helper()
		var ms []catalog.Model
		for _, id := range ids {
			ms = append(ms, catalog.Model{ID: id})
		}
		if err := catalog.SaveLive("router", up.URL+"/v1", ms); err != nil {
			t.Fatal(err)
		}
	}
	list := func() []string {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+TokenFor("opencode"))
		New().Handler().ServeHTTP(rec, req)
		var got struct {
			Data []struct{ ID string } `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("%v: %s", err, rec.Body)
		}
		var out []string
		for _, m := range got.Data {
			if len(m.ID) > 7 && m.ID[:7] == "router/" {
				out = append(out, m.ID)
			}
		}
		slices.Sort(out)
		return out
	}
	listed("qwen/qwen3.8-27b:free", "openai/gpt-6", "deepseek/deepseek-v4:free")
	if got, want := list(), []string{"router/deepseek/deepseek-v4:free", "router/qwen/qwen3.8-27b:free"}; !slices.Equal(got, want) {
		t.Fatalf("agents are given %v, want %v", got, want)
	}
	listed("qwen/qwen3.8-27b", "openai/gpt-6", "deepseek/deepseek-v4:free", "moonshotai/kimi-k3:free")
	if got, want := list(), []string{"router/deepseek/deepseek-v4:free", "router/moonshotai/kimi-k3:free"}; !slices.Equal(got, want) {
		t.Fatalf("after the vendor's change agents are given %v, want %v", got, want)
	}
}
