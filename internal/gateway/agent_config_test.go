package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/settings"
)

// TestAgentConfigGuards holds AgentConfigPath (#7002) to a script with a
// gateway key: the answer carries the catalog and whatever key it is
// asked to write, so no request without a key (from this computer too),
// no web page (Origin null, a foreign Origin, Sec-Fetch headers), no
// rebound Host, no held key and no other machine while magpie isn't
// shared reaches the maker.
func TestAgentConfigGuards(t *testing.T) {
	fresh(t)
	t.Setenv("MAGPIE_ADDR", "")
	var got []string
	old := AgentConfig
	AgentConfig = func(body []byte) (int, []byte) {
		got = append(got, string(body))
		return 200, []byte(`{"files":[]}`)
	}
	t.Cleanup(func() { AgentConfig = old })
	keys, secrets := newCaller(t, "plain", "held")
	if _, err := access.Update("models-key", access.Change{Key: keys[1].ID, Models: []string{"a/*"}}); err != nil {
		t.Fatal(err)
	}
	plain, held := secrets[0], secrets[1]
	h := lanGuard(New().Handler())
	const loop, lan = "127.0.0.1:50000", "192.0.2.7:50000"
	call := func(from, host, key string, hdr map[string]string) *httptest.ResponseRecorder {
		return adminCall(h, "POST", AgentConfigPath, from, host, key, hdr)
	}
	refused := func(t *testing.T, w *httptest.ResponseRecorder, code int, what string) {
		t.Helper()
		if w.Code != code {
			t.Errorf("%s: %d %s, want %d", what, w.Code, w.Body, code)
		}
		if strings.Contains(w.Body.String(), "files") {
			t.Errorf("%s: answered files: %s", what, w.Body)
		}
	}

	t.Run("no key", func(t *testing.T) {
		refused(t, call(loop, "", "", nil), 401, "from this computer with no key")
		refused(t, call(loop, "", "not-a-key", nil), 401, "with a key magpie doesn't know")
		refused(t, call(lan, "", "", nil), 401, "from the network with no key")
	})
	t.Run("a web page", func(t *testing.T) {
		for _, from := range []string{loop, lan} {
			for _, hdr := range []map[string]string{
				{"Origin": "null"},
				{"Origin": "https://attacker.example"},
				{"Origin": "http://127.0.0.1:3425"},
				{"Sec-Fetch-Site": "same-origin"},
				{"Sec-Fetch-Mode": "cors"},
			} {
				refused(t, call(from, "", plain, hdr), 403, from+" with a key and "+mapString(hdr))
				if w := call(from, "", "", hdr); w.Code != 401 && w.Code != 403 {
					t.Errorf("%s without a key and %s: %d %s", from, mapString(hdr), w.Code, w.Body)
				}
			}
		}
	})
	t.Run("a rebound Host", func(t *testing.T) {
		for _, host := range []string{"attacker.example", "attacker.example:3425", "rebind.localhost:3425"} {
			refused(t, call(loop, host, plain, nil), 403, "loopback reached as "+host)
		}
	})
	t.Run("a held key", func(t *testing.T) {
		refused(t, call(loop, "", held, nil), 403, "a held key from this computer")
		refused(t, call(lan, "", held, nil), 403, "a held key from the network")
	})
	if len(got) != 0 {
		t.Fatalf("a refused request reached the maker: %q", got)
	}
	t.Run("a key", func(t *testing.T) {
		for _, from := range []string{loop, lan} {
			w := call(from, "", plain, nil)
			if w.Code != 200 || w.Body.String() != `{"files":[]}` {
				t.Errorf("a plain key from %s: %d %s", from, w.Code, w.Body)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("from %s: Cache-Control %q", from, w.Header().Get("Cache-Control"))
			}
		}
		if w := call(loop, "localhost:3425", plain, nil); w.Code != 200 {
			t.Errorf("as localhost: %d %s", w.Code, w.Body)
		}
		if len(got) != 3 || got[0] != "{}" {
			t.Errorf("the maker was given %q", got)
		}
	})
	t.Run("GET", func(t *testing.T) {
		if w := adminCall(h, "GET", AgentConfigPath, loop, "", plain, nil); w.Code == 200 {
			t.Errorf("GET answered: %s", w.Body)
		}
	})
	t.Run("too large", func(t *testing.T) {
		r := httptest.NewRequest("POST", AgentConfigPath, io.LimitReader(zeros{}, agentConfigMax+1))
		r.RemoteAddr = loop
		r.Host = "127.0.0.1:3425"
		r.Header.Set("Authorization", "Bearer "+plain)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("a body over %d: %d %s", agentConfigMax, w.Code, w.Body)
		}
	})
	t.Run("not shared", func(t *testing.T) {
		s := settings.Load()
		s.LAN = false
		if err := settings.Save(s); err != nil {
			t.Fatal(err)
		}
		n := len(got)
		refused(t, call(lan, "", plain, nil), 403, "from the network while magpie isn't shared")
		if len(got) != n {
			t.Error("a request from the network reached the maker while magpie isn't shared")
		}
		if w := call(loop, "", plain, nil); w.Code != 200 {
			t.Errorf("from this computer while magpie isn't shared: %d %s", w.Code, w.Body)
		}
	})
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = '0'
	}
	return len(p), nil
}

func mapString(m map[string]string) string {
	var b strings.Builder
	for k, v := range m {
		b.WriteString(k + ": " + v)
	}
	return b.String()
}
