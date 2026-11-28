package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/provider"
)

// genHome is a sandbox HOME with a provider of three models, and nothing
// of an agent's in it.
func genHome(t *testing.T) string {
	t.Helper()
	home, _ := codexHome(t, "", "")
	os.RemoveAll(filepath.Join(home, ".codex"))
	if err := provider.Save(provider.Provider{ID: "fake", Name: "Fake", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"m1", "m2", "m3"}}); err != nil {
		t.Fatal(err)
	}
	return home
}

// genTree is every file under dirs, by path, as its mode and content.
func genTree(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, _ := d.Info()
			b, _ := os.ReadFile(p)
			out[p] = fmt.Sprintf("%v\n%s", info.Mode(), b)
			return nil
		})
	}
	return out
}

func genSame(t *testing.T, before, after map[string]string) {
	t.Helper()
	for p, v := range after {
		if w, ok := before[p]; !ok {
			t.Errorf("written on this computer: %s", p)
		} else if w != v {
			t.Errorf("changed on this computer: %s", p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			t.Errorf("removed on this computer: %s", p)
		}
	}
}

const (
	genTestGW  = "http://magpie.lan:3425"
	genTestKey = "sk-mg-test-0123456789"
)

// TestGenerateIsTheAdapters: the files made for another computer are
// what connecting the agent on this one writes into a fresh HOME, the
// address and the key apart; and making them writes nothing here, in
// HOME or in magpie's own folder (#7002).
func TestGenerateIsTheAdapters(t *testing.T) {
	for _, id := range GenSupported() {
		t.Run(id, func(t *testing.T) {
			home := genHome(t)
			conf := filepath.Dir(provider.Path())
			before := genTree(t, home, conf)
			res, err := Generate(GenRequest{Agent: id, Gateway: genTestGW + "/v1/", Key: genTestKey})
			if err != nil {
				t.Fatal(err)
			}
			genSame(t, before, genTree(t, home, conf))
			if len(res.Files) == 0 {
				t.Fatal("no files")
			}
			keyIn := false
			for _, f := range res.Files {
				keyIn = keyIn || strings.Contains(f.Content, genTestKey)
				if strings.Contains(f.Content, genRootMark) || strings.Contains(f.Content, home) {
					t.Errorf("%s names the run's own folder or this HOME:\n%s", f.Path, f.Content)
				}
			}
			if !keyIn {
				t.Error("the key is in no file")
			}

			// the same agent connected here, in a HOME of its own
			home2 := genHome(t)
			a, err := Find(id)
			if err != nil {
				t.Fatal(err)
			}
			was := genTree(t, home2)
			c, err := a.connect()
			if err != nil {
				t.Fatal(err)
			}
			// the files as made with this computer's own key, to set
			// beside what its agent is given here
			res, err = Generate(GenRequest{Agent: id, Gateway: genTestGW, Key: gateway.Token})
			if err != nil {
				t.Fatal(err)
			}
			if err := genPick(a, c, res.Model, nil, &GenResult{}); err != nil {
				t.Fatal(err)
			}
			here := strings.TrimRight(gateway.URL(), "/")
			local := map[string]string{}
			own := filepath.Dir(provider.Path()) + string(filepath.Separator)
			for p, v := range genTree(t, home2) {
				if was[p] == v || strings.HasPrefix(p, own) {
					continue
				}
				rel, _ := filepath.Rel(home2, p)
				mode, content, _ := strings.Cut(v, "\n")
				// the address and key this computer's agent is given,
				// as the caller's; a path into HOME as the home's
				content = strings.ReplaceAll(content, here, genTestGW)
				if id == "claude" {
					// this computer's own settings.json says the main
					// model as the tier carrying it, for Claude Desktop's
					// Code tab (mainWord); another computer's keeps the id
					content = strings.Replace(content, `"model": "sonnet"`, `"model": "`+res.Model+`"`, 1)
				}
				if id == "codex" {
					// Codex's catalog: beside config.toml, written
					// relative to it for another computer's HOME
					content = strings.ReplaceAll(content, `"`+filepath.ToSlash(filepath.Join(home2, ".codex", "magpie-models.json"))+`"`, `"magpie-models.json"`)
				}
				local[filepath.ToSlash(rel)] = mode + "\n" + content
			}
			got := map[string]string{}
			for _, f := range res.Files {
				var m os.FileMode
				fmt.Sscanf(f.Mode, "%o", &m)
				got[f.Path] = fmt.Sprintf("%v\n%s", m, f.Content)
			}
			for p, v := range local {
				if strings.HasPrefix(p, ".") || strings.Contains(p, "/") {
					if g, ok := got[p]; !ok {
						t.Errorf("%s: written here, not made", p)
					} else if g != v {
						t.Errorf("%s differs:\n--- made\n%s\n--- written here\n%s", p, g, v)
					}
				}
			}
			for p := range got {
				if _, ok := local[p]; !ok {
					t.Errorf("%s: made, not written here", p)
				}
			}
		})
	}
}

// TestGenerateKeyFromVariable: with key_env each agent reads the key from
// the variable in its own way, and the files hold no key at all.
func TestGenerateKeyFromVariable(t *testing.T) {
	want := map[string][]string{
		"codex":    {`env_key = "MAGPIE_KEY"`},
		"claude":   {`"apiKeyHelper": "printf %s \"$MAGPIE_KEY\""`},
		"opencode": {`"apiKey": "{env:MAGPIE_KEY}"`},
		"dsh":      {`apiKeyEnv: MAGPIE_KEY`},
	}
	for _, id := range GenSupported() {
		t.Run(id, func(t *testing.T) {
			genHome(t)
			res, err := Generate(GenRequest{Agent: id, Gateway: genTestGW, KeyEnv: "MAGPIE_KEY"})
			if err != nil {
				t.Fatal(err)
			}
			all := ""
			for _, f := range res.Files {
				all += f.Content
				for _, bad := range []string{"experimental_bearer_token", "ANTHROPIC_AUTH_TOKEN", "magpie-gen-", gateway.Token + "-" + id} {
					if strings.Contains(f.Content, bad) {
						t.Errorf("%s holds %q:\n%s", f.Path, bad, f.Content)
					}
				}
			}
			for _, w := range want[id] {
				if !strings.Contains(all, w) {
					t.Errorf("no %s in\n%s", w, all)
				}
			}
			if !strings.Contains(all, genTestGW) {
				t.Errorf("the gateway isn't in the files:\n%s", all)
			}
			if len(res.Notes) == 0 || !strings.Contains(res.Notes[0], "MAGPIE_KEY") {
				t.Errorf("notes = %q", res.Notes)
			}
		})
	}
	// Claude Code reads ANTHROPIC_AUTH_TOKEN from its environment itself
	genHome(t)
	res, err := Generate(GenRequest{Agent: "claude-code", Gateway: genTestGW, KeyEnv: "ANTHROPIC_AUTH_TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Files {
		if strings.Contains(f.Content, "apiKeyHelper") || strings.Contains(f.Content, "ANTHROPIC_AUTH_TOKEN") {
			t.Errorf("%s:\n%s", f.Path, f.Content)
		}
	}
}

// TestGenerateModels: the lists hold the models given and no other, the
// agent starts on the one asked for, and a tier's model is one given.
func TestGenerateModels(t *testing.T) {
	for _, id := range GenSupported() {
		t.Run(id, func(t *testing.T) {
			genHome(t)
			res, err := Generate(GenRequest{Agent: id, Gateway: genTestGW, Key: genTestKey, Models: []string{"fake/m3", "fake/m2"}, Model: "fake/m2"})
			if err != nil {
				t.Fatal(err)
			}
			if res.Model != "fake/m2" {
				t.Errorf("model = %q", res.Model)
			}
			all := ""
			for _, f := range res.Files {
				all += f.Content
			}
			if strings.Contains(all, "fake/m1") || strings.Contains(all, "m1 · Fake") || strings.Contains(all, "\"m1\"") {
				t.Errorf("m1 is in the files:\n%s", all)
			}
			if !strings.Contains(all, "m3") {
				t.Errorf("m3 isn't in the files:\n%s", all)
			}
		})
	}
}

// TestGenerateRefuses: an agent whose files aren't made here is said so,
// with the ones that are, and a value the files can't hold is refused
// before anything is made.
func TestGenerateRefuses(t *testing.T) {
	genHome(t)
	for _, name := range []string{"gemini", "cursor-agent-nope"} {
		_, err := Generate(GenRequest{Agent: name, Gateway: genTestGW, Key: genTestKey})
		var un *UnsupportedError
		if !errors.As(err, &un) || !slices.Equal(un.Supported, []string{"codex", "claude", "opencode", "dsh"}) {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad := []GenRequest{
		{Agent: "codex", Gateway: "", Key: genTestKey},
		{Agent: "codex", Gateway: "ftp://magpie.lan", Key: genTestKey},
		{Agent: "codex", Gateway: "http://u:p@magpie.lan", Key: genTestKey},
		{Agent: "codex", Gateway: "http://magpie.lan/?x=1", Key: genTestKey},
		{Agent: "codex", Gateway: "http://magpie.lan\"\nmodel = \"x", Key: genTestKey},
		{Agent: "codex", Gateway: genTestGW},
		{Agent: "codex", Gateway: genTestGW, Key: genTestKey, KeyEnv: "K"},
		{Agent: "codex", Gateway: genTestGW, KeyEnv: "1BAD"},
		{Agent: "codex", Gateway: genTestGW, KeyEnv: "A;rm -rf"},
		{Agent: "codex", Gateway: genTestGW, Key: "a\"b"},
		{Agent: "codex", Gateway: genTestGW, Key: genTestKey, Models: []string{"fake/nope"}},
		{Agent: "codex", Gateway: genTestGW, Key: genTestKey, Models: []string{"fake/m1"}, Model: "fake/m2"},
		{Agent: "codex", Gateway: genTestGW, Key: genTestKey, Model: "fake/nope"},
		{Agent: "codex", Gateway: genTestGW, Key: genTestKey, Version: "1\"\n"},
	}
	for _, r := range bad {
		_, err := Generate(r)
		var in *GenInputError
		if !errors.As(err, &in) {
			t.Errorf("%+v: %v", r, err)
		}
	}
}

// TestGenerateJSON: the gateway's answer, for a good request, an agent
// not made here and a body that isn't one.
func TestGenerateJSON(t *testing.T) {
	genHome(t)
	st, b := GenerateJSON([]byte(`{"agent":"opencode","gateway":"http://magpie.lan:3425","key_env":"MAGPIE_KEY","models":["fake/m2"]}`))
	var res GenResult
	if st != http.StatusOK || json.Unmarshal(b, &res) != nil || len(res.Files) == 0 || res.Model != "fake/m2" {
		t.Fatalf("%d %s", st, b)
	}
	st, b = GenerateJSON([]byte(`{"agent":"gemini","gateway":"http://magpie.lan:3425","key_env":"MAGPIE_KEY"}`))
	if st != http.StatusUnprocessableEntity || !strings.Contains(string(b), `"supported":["codex","claude","opencode","dsh"]`) {
		t.Errorf("%d %s", st, b)
	}
	// a magpie with no models has nothing to make files for: 409, not a
	// 500 about connecting the agent
	if err := provider.Delete("fake"); err != nil {
		t.Fatal(err)
	}
	if st, b := GenerateJSON([]byte(`{"agent":"codex","gateway":"http://magpie.lan:3425","key_env":"MAGPIE_KEY"}`)); st != http.StatusConflict || !strings.Contains(string(b), "agent_config_no_models") {
		t.Errorf("no models: %d %s", st, b)
	}
	for _, body := range []string{``, `{`, `{"agent":"codex","gateway":"http://x","key_env":"K","extra":1}`, `{"agent":"codex","gateway":"http://x","key_env":"K"} {}`} {
		if st, b := GenerateJSON([]byte(body)); st != http.StatusBadRequest {
			t.Errorf("%q: %d %s", body, st, b)
		}
	}
}
