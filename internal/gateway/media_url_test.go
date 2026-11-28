package gateway

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// mediaRelay stands in for chenxiang's relay (Discord): chat at /v1 in
// OpenAI's format, images and videos at /v3 in Volcengine Ark's. It
// answers an image on either images path and Ark's video task, and keeps
// every request it was sent.
type mediaRelay struct {
	*httptest.Server
	mu     sync.Mutex
	paths  []string
	bodies []string
}

func newMediaRelay(t *testing.T) *mediaRelay {
	t.Helper()
	m := &mediaRelay{}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.mu.Lock()
		m.paths = append(m.paths, r.Method+" "+r.URL.Path)
		m.bodies = append(m.bodies, string(body))
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/images/generations"):
			io.WriteString(w, `{"created":1,"data":[{"b64_json":"`+base64.StdEncoding.EncodeToString(pngBytes)+`"}]}`)
		case r.Method == "POST" && r.URL.Path == "/v3/contents/generations/tasks":
			io.WriteString(w, volcengineVideoCreateResponse)
		default:
			w.WriteHeader(404)
			io.WriteString(w, `{"error":{"code":"NotFound","message":"no such path"}}`)
		}
	}))
	t.Cleanup(m.Close)
	return m
}

func (m *mediaRelay) sent(t *testing.T, i int) (string, map[string]any) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.paths) <= i {
		t.Fatalf("sent %d requests: %v", len(m.paths), m.paths)
	}
	var b map[string]any
	if err := json.Unmarshal([]byte(m.bodies[i]), &b); err != nil {
		t.Fatalf("%v: %s", err, m.bodies[i])
	}
	return m.paths[i], b
}

func mediaProvider(t *testing.T, m *mediaRelay, mediaURL, api string) *Server {
	t.Helper()
	if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Chat: m.URL + "/v1", Key: "key", MediaURL: mediaURL, MediaAPI: api,
		Models: []string{"gpt-6", "doubao-seedream-4-5-251128", "doubao-seedance-2-0-260128"}}); err != nil {
		t.Fatal(err)
	}
	return New()
}

// A relay whose images are at /v3 in Ark's format, the user having said so
// in the provider's More endpoints, is asked there in Ark's body: watermark
// false, once per image, no n.
func TestMediaURLDrawsInArksFormat(t *testing.T) {
	fresh(t)
	m := newMediaRelay(t)
	s := mediaProvider(t, m, m.URL+"/v3/", provider.MediaArk)
	code, ans, raw := postImages(t, s, "/v1/images/generations", "application/json", `{"model":"relay/doubao-seedream-4-5-251128","prompt":"一只喜鹊","size":"16:9","n":2}`)
	if code != 200 || len(ans.Data) != 2 {
		t.Fatalf("want two images: %d %s", code, raw)
	}
	for i := range 2 {
		path, b := m.sent(t, i)
		if path != "POST /v3/images/generations" {
			t.Fatalf("asked %s, want POST /v3/images/generations", path)
		}
		if b["watermark"] != false || b["size"] != "2848x1600" || b["model"] != "doubao-seedream-4-5-251128" {
			t.Fatalf("sent %v, not Ark's body", b)
		}
		if _, ok := b["n"]; ok {
			t.Fatalf("sent n, which Ark doesn't take: %v", b)
		}
	}
}

// Its Seedance videos go to Ark's content-generation tasks under the same
// URL, in Ark's body, not to an OpenAI /videos under the chat URL.
func TestMediaURLMakesVideosAtArksTasks(t *testing.T) {
	fresh(t)
	m := newMediaRelay(t)
	s := mediaProvider(t, m, m.URL+"/v3", provider.MediaArk)
	code, _, raw, _ := request(t, s, http.MethodPost, "/v1/videos", "application/json", `{"model":"relay/doubao-seedance-2-0-260128","prompt":"一只喜鹊飞过","seconds":"5","size":"1280x720"}`)
	if code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	path, b := m.sent(t, 0)
	if path != "POST /v3/contents/generations/tasks" {
		t.Fatalf("asked %s, want POST /v3/contents/generations/tasks", path)
	}
	content, _ := b["content"].([]any)
	if b["model"] != "doubao-seedance-2-0-260128" || len(content) != 1 || content[0].(map[string]any)["text"] != "一只喜鹊飞过" {
		t.Fatalf("sent %v, not Ark's body", b)
	}
}

// With the URL and Auto, a relay's host doesn't make it Ark: the images go
// under the URL in OpenAI's format. With neither set, nothing changes:
// they go under the chat URL in OpenAI's format, as before.
func TestMediaURLAutoIsOpenAIOffArk(t *testing.T) {
	for _, c := range []struct{ url, want string }{{"/v3", "POST /v3/images/generations"}, {"", "POST /v1/images/generations"}} {
		fresh(t)
		m := newMediaRelay(t)
		url := c.url
		if url != "" {
			url = m.URL + url
		}
		s := mediaProvider(t, m, url, "")
		if code, _, raw := postImages(t, s, "/v1/images/generations", "application/json", `{"model":"relay/doubao-seedream-4-5-251128","prompt":"一只喜鹊","n":2}`); code != 200 {
			t.Fatalf("%q: %d %s", c.url, code, raw)
		}
		path, b := m.sent(t, 0)
		if path != c.want {
			t.Fatalf("%q: asked %s, want %s", c.url, path, c.want)
		}
		if _, ok := b["watermark"]; ok || b["n"] != float64(2) {
			t.Fatalf("%q: sent %v, not OpenAI's body", c.url, b)
		}
	}
}

// OpenAI picked holds on an Ark host too: its images go in OpenAI's
// format, n and all.
func TestMediaAPIOpenAIOverridesArksHost(t *testing.T) {
	fresh(t)
	m := newMediaRelay(t)
	was := isArk
	isArk = func(p provider.Provider) bool { return p.ID == "relay" }
	t.Cleanup(func() { isArk = was })
	s := mediaProvider(t, m, m.URL+"/v3", provider.MediaOpenAI)
	if code, _, raw := postImages(t, s, "/v1/images/generations", "application/json", `{"model":"relay/doubao-seedream-4-5-251128","prompt":"一只喜鹊","n":2}`); code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	path, b := m.sent(t, 0)
	if _, ok := b["watermark"]; ok || path != "POST /v3/images/generations" || b["n"] != float64(2) {
		t.Fatalf("asked %s with %v, want OpenAI's body", path, b)
	}
}

func TestMediaAPIOf(t *testing.T) {
	for in, want := range map[string]string{"": "", "auto": "", " OpenAI ": "openai", "ark": "ark", "Volcengine": "ark", "火山方舟": "ark", "dall-e": ""} {
		if got := provider.MediaAPIOf(in); got != want {
			t.Errorf("MediaAPIOf(%q) = %q, want %q", in, got, want)
		}
	}
}
