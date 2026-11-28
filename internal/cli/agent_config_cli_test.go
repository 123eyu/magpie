package cli

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configHomeFiles is every file under the sandbox's HOME and config dirs, by
// path, with its content: `magpie agent config` must leave them as they
// were.
func configHomeFiles(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, dir := range []string{os.Getenv("HOME"), os.Getenv("XDG_CONFIG_HOME"), os.Getenv("XDG_CACHE_HOME")} {
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			b, _ := os.ReadFile(p)
			out[p] = string(b)
			return nil
		})
	}
	return out
}

// `magpie agent config` (#7002, Geequlim on X) prints an agent's files
// for another computer and writes nothing here; --help says how, and a
// bad value is refused before anything is made.
func TestAgentConfigCommand(t *testing.T) {
	groupsHome(t)
	before := configHomeFiles(t)

	for _, args := range [][]string{{"agent", "config", "--help"}, {"agent", "--help"}, {"agents", "config", "-h"}} {
		if !helpAsked(args) || !strings.Contains(helpFor(args), "magpie agent config <agent> --gateway <url>") {
			t.Errorf("magpie %v prints:\n%s", args, helpFor(args))
		}
	}
	if !strings.Contains(usage, "magpie agent config <agent>") {
		t.Error("magpie --help doesn't list agent config")
	}

	var out bytes.Buffer
	if err := agentConfigTo(&out, strings.NewReader(""), []string{"config", "opencode", "--gateway", "http://magpie.lan:3425", "--key-env", "MAGPIE_KEY", "--models", "a/m,b/vendor/m", "--model=b/vendor/m"}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"# ~/.config/opencode/opencode.json", "http://magpie.lan:3425", "{env:MAGPIE_KEY}", "# starts on b/vendor/m"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "only-a") || strings.Contains(s, "gpt-5.5") {
		t.Errorf("a model left out of --models is in the files:\n%s", s)
	}

	out.Reset()
	if err := agentConfigTo(&out, strings.NewReader("sk-mg-from-stdin\n"), []string{"config", "codex", "--gateway", "http://magpie.lan:3425", "--key", "-", "--json"}); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Agent string
		Files []struct{ Path, Content string }
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("--json: %v\n%s", err, out.String())
	}
	if res.Agent != "codex" || len(res.Files) == 0 || !strings.Contains(out.String(), "sk-mg-from-stdin") {
		t.Errorf("--key - from stdin: %s", out.String())
	}

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{}, "usage"},
		{[]string{"make"}, "usage"},
		{[]string{"config"}, "which agent"},
		{[]string{"config", "gemini", "--gateway", "http://h:1", "--key-env", "K"}, "unsupported"},
		{[]string{"config", "nope", "--gateway", "http://h:1", "--key-env", "K"}, "codex"},
		{[]string{"config", "codex", "opencode", "--gateway", "http://h:1", "--key-env", "K"}, "one agent"},
		{[]string{"config", "codex", "--gateway"}, "--gateway needs a value"},
		{[]string{"config", "codex", "--gateway", "--key-env", "K"}, "--gateway needs a value"},
		{[]string{"config", "codex", "--gateway", "http://h:1", "--key-env", "K", "--bogus", "x"}, "unknown flag --bogus"},
		{[]string{"config", "codex", "--gateway", "http://h:1", "--key-env", "K", "--json=yes"}, "--json takes no value"},
		{[]string{"config", "codex", "--gateway", "http://h:1", "--key-env", "K", "--models", ","}, "--models: none given"},
		{[]string{"config", "codex", "--gateway", "http://h:1", "--key", "-"}, "no key on stdin"},
		{[]string{"config", "codex", "--gateway", "not a url", "--key-env", "K"}, "gateway"},
		{[]string{"config", "codex", "--gateway", "http://h:1"}, "key"},
		{[]string{"config", "codex", "--gateway", "http://h:1", "--key-env", "1 BAD"}, "1 BAD"},
		{[]string{"config", "codex", "--gateway", "http://h:1", "--key-env", "K", "--model", "a/nope"}, "a/nope"},
	} {
		out.Reset()
		err := agentConfigTo(&out, strings.NewReader(""), c.args)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want an error naming %q", c.args, err, c.want)
		}
		if out.Len() != 0 {
			t.Errorf("%v printed files on an error:\n%s", c.args, out.String())
		}
	}

	after := configHomeFiles(t)
	for p, b := range after {
		if before[p] != b {
			t.Errorf("magpie agent config wrote %s", p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			t.Errorf("magpie agent config removed %s", p)
		}
	}
}
