package agent

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	satomic "sync/atomic"
	"time"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/edit"
	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/lazyre"
	"github.com/yetone/magpie/internal/provider"
)

// Headless config generation (#7002, Geequlim): an agent's config files
// for a computer magpie isn't on (a container, a sandbox, a server), as
// the files themselves, for the caller to put there. Nothing is written
// on this computer and no agent is looked for or started.
//
// The files are the adapter's own: the agent is built at a place of its
// own (genPlace), as a WSL distro's agent is, whose home is a temporary
// folder, whose gateway is an address no other place has (genRun.base)
// and whose key is a token no other place is given (genRun.key), and is
// connected there as the Agents page's switch connects it. What it wrote
// under that home is read back, the address put back as the one asked
// for and the token as the caller's key or a reference to the variable
// holding it. What magpie keeps of an agent it connected (the stash and
// the record) is kept in memory for the run (genMem), never in
// stash.json or applied.json.

// GenRequest is what the files are made for.
type GenRequest struct {
	// Agent is the agent's id as `magpie agents` lists it (codex,
	// claude, opencode, dsh); claude-code is Claude Code too.
	Agent string `json:"agent"`
	// Version is the agent's version there, for an agent whose files
	// depend on it; "" for its current one.
	Version string `json:"version,omitempty"`
	// Gateway is magpie's address as the agent reaches it.
	Gateway string `json:"gateway"`
	// Key is the gateway key the files carry; KeyEnv, in its place, the
	// name of the variable that holds it there. One of them is given.
	Key    string `json:"key,omitempty"`
	KeyEnv string `json:"key_env,omitempty"`
	// Model is the model it starts on; "" for the one magpie picks.
	Model string `json:"model,omitempty"`
	// Models, when given, are the only models its lists hold.
	Models []string `json:"models,omitempty"`
}

// GenFile is one file: its path under the agent's HOME, with / between
// folders, and what it holds.
type GenFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	// Mode is the file's permissions as magpie makes them, in octal
	// ("0600" for one only its owner may read, as a file with the key is)
	Mode string `json:"mode"`
}

// GenResult is the files, the model the agent starts on and anything the
// caller has to do beside writing them (setting the key's variable).
type GenResult struct {
	Agent string    `json:"agent"`
	Model string    `json:"model,omitempty"`
	Files []GenFile `json:"files"`
	Notes []string  `json:"notes,omitempty"`
}

// UnsupportedError is the answer for an agent magpie can't make the files
// of here: one it wires with more than files (an app's database, a step
// in the app), or one it doesn't know.
type UnsupportedError struct {
	Agent     string   `json:"agent"`
	Why       string   `json:"why"`
	Supported []string `json:"supported"`
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("%s is unsupported: %s (supported: %s)", e.Agent, e.Why, strings.Join(e.Supported, ", "))
}

// GenInputError is a request for files that can't be made as it asks: a
// value missing or not one the files can hold.
type GenInputError struct{ Err error }

func (e *GenInputError) Error() string { return e.Err.Error() }
func (e *GenInputError) Unwrap() error { return e.Err }

func genBad(err error) error { return &GenInputError{err} }

// ErrGenNoModels is a magpie with no models to give the agent: nothing
// to write its files for yet.
var ErrGenNoModels = errors.New("this magpie has no models to give")

// genKind is an agent whose files are made here, and how its files take
// the key from a variable.
type genKind struct {
	id    string
	names []string // what it is asked by besides its id
	// env points the files under home at the variable name instead of
	// the key (sentinel), in the agent's own way of reading a key from
	// its environment; notes are what the caller is told beside them
	env func(home, sentinel, name string) ([]string, error)
	// kept puts a model the files name beside the lists (a tier's) back
	// on the one picked when it isn't among the models given
	kept func(home string, models []string) error
}

var genKinds = []genKind{
	{id: "codex", env: genCodexEnv},
	{id: "claude", names: []string{"claude-code"}, env: genClaudeEnv, kept: genClaudeKept},
	{id: "opencode", env: genOpenCodeEnv},
	{id: "dsh", env: genDshEnv},
}

// GenSupported is the agents whose files are made, by id.
func GenSupported() []string {
	var out []string
	for _, k := range genKinds {
		out = append(out, k.id)
	}
	return out
}

func genKindOf(name string) (genKind, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, k := range genKinds {
		if k.id == n || slices.Contains(k.names, n) {
			return k, true
		}
	}
	return genKind{}, false
}

// genMark is in the id of an agent built for a generation, and in every
// stash key and record of it; genRootMark in its folder's name, and so in
// a key an agent stashes by a file's path (dsh's, dshStashKey).
const (
	genMark     = "@gen:"
	genRootMark = "magpie-gen-"
)

func isGenKey(k string) bool {
	return strings.Contains(k, genMark) || strings.Contains(k, genRootMark)
}

// genRun is the generation going on: one at a time (genMu).
type genRun struct {
	tok   string // random, in base, key and spelt
	base  string // the gateway's address as the files are written with it
	key   string // the key as they are written with it
	root  string // the temporary folder: home/ and sys/
	home  string
	spelt string // home as the agent's own paths name it
	// models are the only models the files list; nil: the agent's catalog
	models []string
}

// genListMark is in the agent a model list is asked for (magpieModels,
// viaMagpie) by a generation's place, before the run's token: its list
// is apart from the agent's own, and holds the run's models only.
const genListMark = "\x00gen:"

// list is the agent whose model list a file at this place gets: agent
// itself, or for a place made for a generation agent with the run's mark.
func (p place) list(agent string) string {
	if r := genActive.Load(); p.gen && r != nil {
		return agent + genListMark + r.tok
	}
	return agent
}

// genBase is the agent a list is for, without a generation's mark.
func genBase(agent string) string {
	base, _, _ := strings.Cut(agent, genListMark)
	return base
}

// catalogShown is provider.CatalogFor's shown list for the agent a model
// list is asked for, and that agent: a generation's list holds the run's
// models only.
func catalogShown(agent string) (string, []provider.Entry) {
	base, tok, marked := strings.Cut(agent, genListMark)
	shown, _ := provider.CatalogFor(base)
	if !marked {
		return base, shown
	}
	if r := genActive.Load(); r != nil && r.tok == tok && r.models != nil {
		shown = slices.DeleteFunc(shown, func(e provider.Entry) bool { return !slices.Contains(r.models, e.ID) })
	}
	return base, shown
}

var (
	genMu     sync.Mutex
	genActive satomic.Pointer[genRun]
	// genOn is whether a generation's values are held (genMem): looked at
	// on every read of the stash, so a bool and no lock
	genOn  satomic.Bool
	genMem struct {
		sync.Mutex
		stash   map[string]string
		applied map[string]applied
	}
)

// genKeyAt is the key for a gateway address: the run's own, at its own
// address, "" elsewhere (keyAt).
func genKeyAt(gw string) string {
	if r := genActive.Load(); r != nil && strings.TrimRight(gw, "/") == r.base {
		return r.key
	}
	return ""
}

func genStashHeld() bool { return genOn.Load() }

func genStashInto(m map[string]string) {
	if !genOn.Load() {
		return
	}
	genMem.Lock()
	maps.Copy(m, genMem.stash)
	genMem.Unlock()
}

// genStash keeps a generation's values of kv in memory and is the rest.
func genStash(kv map[string]string) map[string]string {
	if !genOn.Load() && !slices.ContainsFunc(slices.Collect(maps.Keys(kv)), isGenKey) {
		return kv
	}
	rest := map[string]string{}
	genMem.Lock()
	defer genMem.Unlock()
	for k, v := range kv {
		switch {
		case !isGenKey(k):
			rest[k] = v
		case v == "":
			delete(genMem.stash, k)
		default:
			if genMem.stash == nil {
				genMem.stash = map[string]string{}
			}
			genMem.stash[k] = v
		}
	}
	return rest
}

func genForget(keys []string) []string {
	if !slices.ContainsFunc(keys, isGenKey) {
		return keys
	}
	var rest []string
	genMem.Lock()
	defer genMem.Unlock()
	for _, k := range keys {
		if isGenKey(k) {
			delete(genMem.stash, k)
		} else {
			rest = append(rest, k)
		}
	}
	return rest
}

func genUnstash(k string) string {
	genMem.Lock()
	defer genMem.Unlock()
	v := genMem.stash[k]
	delete(genMem.stash, k)
	return v
}

func genForgetValues(peeked map[string]string) map[string]string {
	if !slices.ContainsFunc(slices.Collect(maps.Keys(peeked)), isGenKey) {
		return peeked
	}
	rest := map[string]string{}
	genMem.Lock()
	defer genMem.Unlock()
	for k, v := range peeked {
		if !isGenKey(k) {
			rest[k] = v
		} else if genMem.stash[k] == v {
			delete(genMem.stash, k)
		}
	}
	return rest
}

// withoutGen is m without a generation's values, for stash.json.
func withoutGen(m map[string]string) map[string]string {
	if !slices.ContainsFunc(slices.Collect(maps.Keys(m)), isGenKey) {
		return m
	}
	m = maps.Clone(m)
	maps.DeleteFunc(m, func(k, _ string) bool { return isGenKey(k) })
	return m
}

func genAppliedHeld() bool { return genOn.Load() }

func genAppliedInto(m map[string]applied) {
	if !genOn.Load() {
		return
	}
	genMem.Lock()
	maps.Copy(m, genMem.applied)
	genMem.Unlock()
}

func genAppliedOf(id string) applied {
	genMem.Lock()
	defer genMem.Unlock()
	return genMem.applied[id]
}

func genRecord(id, key, v string) {
	genMem.Lock()
	defer genMem.Unlock()
	if genMem.applied == nil {
		genMem.applied = map[string]applied{}
	}
	a := genMem.applied[id]
	if a.Fields == nil {
		a.Fields = map[string]string{}
	} else {
		a.Fields = maps.Clone(a.Fields)
	}
	if v == "" {
		delete(a.Fields, key)
	} else {
		a.Fields[key] = v
	}
	a.At = time.Now()
	genMem.applied[id] = a
}

// genURLOK is a gateway address the files can be written with: a URL's
// characters only, nothing that ends a quoted string in TOML, JSON, YAML
// or a shell.
var genURLOK = lazyre.New(`^https?://[A-Za-z0-9.\-_~:\[\]%/@]+$`)

// genEnvOK is a variable's name every shell and agent takes.
var genEnvOK = lazyre.New(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

// genKeyOK is a key the files can carry as it is, in any of their formats.
var genKeyOK = lazyre.New(`^[A-Za-z0-9._~+/=\-]{1,512}$`)

// genGateway is the address the files are written with, checked: http or
// https, a host, no query or fragment, no /v1 (the agents add their own).
func genGateway(s string) (string, error) {
	s = strings.TrimSpace(s)
	u, err := url.Parse(s)
	if s == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(s, "?") || strings.HasSuffix(s, "#") || !genURLOK.MatchString(s) {
		return "", fmt.Errorf("gateway %q is not an http(s) address of magpie (like http://magpie.lan:3425)", s)
	}
	s = strings.TrimRight(s, "/")
	s = strings.TrimSuffix(s, "/v1")
	return strings.TrimRight(s, "/"), nil
}

// Generate makes the agent's files for req. Nothing on this computer is
// written: the files are made under a temporary folder, read back and
// removed.
func Generate(req GenRequest) (GenResult, error) {
	k, ok := genKindOf(req.Agent)
	if !ok {
		why := "not an agent magpie knows"
		if a, err := Find(req.Agent); err == nil && a != nil {
			why = "magpie wires " + a.Name + " on the computer it runs on only; its files can't be made for another one yet"
		}
		return GenResult{}, &UnsupportedError{Agent: req.Agent, Why: why, Supported: GenSupported()}
	}
	gw, err := genGateway(req.Gateway)
	if err != nil {
		return GenResult{}, genBad(err)
	}
	switch {
	case req.Key != "" && req.KeyEnv != "":
		return GenResult{}, genBad(errors.New("give the key or the name of its variable, not both"))
	case req.Key == "" && req.KeyEnv == "":
		return GenResult{}, genBad(errors.New("give the gateway key (key) or the name of the variable that holds it (key_env)"))
	case req.KeyEnv != "" && !genEnvOK.MatchString(req.KeyEnv):
		return GenResult{}, genBad(fmt.Errorf("key_env %q is not a variable's name (letters, digits and _, not starting with a digit)", req.KeyEnv))
	case req.Key != "" && !genKeyOK.MatchString(req.Key):
		return GenResult{}, genBad(errors.New("the key has characters a gateway key doesn't (letters, digits and ._~+/=- only)"))
	case strings.ContainsFunc(req.Version, func(r rune) bool { return r < ' ' || r == '"' || r == '\\' }) || len(req.Version) > 64:
		return GenResult{}, genBad(fmt.Errorf("version %q is not an agent's version", req.Version))
	}
	// the models asked for, each one magpie gives the agent
	shown, _ := provider.CatalogFor(k.id)
	if len(shown) == 0 {
		return GenResult{}, fmt.Errorf("%w %s yet: add a provider or subscription to it first", ErrGenNoModels, k.id)
	}
	given := func(m string) bool {
		return slices.ContainsFunc(shown, func(e provider.Entry) bool { return e.ID == m })
	}
	var models []string
	if req.Models != nil {
		var unknown []string
		for _, m := range req.Models {
			m = strings.TrimSpace(m)
			switch {
			case m == "" || slices.Contains(models, m):
			case given(m):
				models = append(models, m)
			default:
				unknown = append(unknown, m)
			}
		}
		if len(unknown) > 0 {
			return GenResult{}, genBad(fmt.Errorf("not among magpie's models for %s: %s", k.id, strings.Join(unknown, ", ")))
		}
		if len(models) == 0 {
			return GenResult{}, genBad(errors.New("models: none given"))
		}
	}
	switch {
	case req.Model != "" && models != nil && !slices.Contains(models, req.Model):
		return GenResult{}, genBad(fmt.Errorf("model %q is not among the models given", req.Model))
	case req.Model != "" && !given(req.Model):
		return GenResult{}, genBad(fmt.Errorf("model %q is not among magpie's models for %s (magpie models %s lists them)", req.Model, k.id, k.id))
	}

	genMu.Lock()
	defer genMu.Unlock()
	b := make([]byte, 8)
	rand.Read(b)
	tok := hex.EncodeToString(b)
	root, err := os.MkdirTemp("", genRootMark)
	if err != nil {
		return GenResult{}, err
	}
	defer os.RemoveAll(root)
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	run := &genRun{
		tok:    tok,
		base:   "http://magpie-gen-" + tok + ".invalid:1",
		key:    "magpie-gen-key-" + tok,
		root:   root,
		home:   filepath.Join(root, "home"),
		spelt:  "/magpie-gen-" + tok + "-home",
		models: models,
	}
	if err := os.MkdirAll(run.home, 0o700); err != nil {
		return GenResult{}, err
	}
	genActive.Store(run)
	genOn.Store(true)
	defer func() {
		genActive.Store(nil)
		genOn.Store(false)
		genMem.Lock()
		genMem.stash, genMem.applied = nil, nil
		genMem.Unlock()
		// what the agents' code keeps of a home by its folder
		for _, m := range []*sync.Map{&dshApps, &dshDistros} {
			m.Range(func(k, _ any) bool {
				if p, _ := k.(string); strings.HasPrefix(p, root) {
					m.Delete(k)
				}
				return true
			})
		}
	}()

	a := genAgent(wslKindOf(k.id), run, req.Version)
	res := GenResult{Agent: k.id}
	c, err := a.connect()
	if err != nil {
		return GenResult{}, err
	}
	if err := genPick(a, c, req.Model, models, &res); err != nil {
		return GenResult{}, err
	}
	if models != nil && k.kept != nil {
		if err := k.kept(run.home, models); err != nil {
			return GenResult{}, err
		}
	}
	if req.KeyEnv != "" {
		notes, err := k.env(run.home, run.key, req.KeyEnv)
		if err != nil {
			return GenResult{}, err
		}
		res.Notes = append(res.Notes, notes...)
	}

	files, err := genFiles(run)
	if err != nil {
		return GenResult{}, err
	}
	// the address and the key as asked
	for i := range files {
		files[i].Content = strings.ReplaceAll(files[i].Content, run.base, gw)
		if req.Key != "" {
			files[i].Content = strings.ReplaceAll(files[i].Content, run.key, req.Key)
		}
	}
	if err := genClean(files, run, gw, req); err != nil {
		return GenResult{}, err
	}
	res.Files = files
	return res, nil
}

func init() { gateway.AgentConfig = GenerateJSON }

// GenerateJSON is Generate for a JSON request (gateway.AgentConfigPath):
// the status and the JSON answered, the result or an error in the shape
// the gateway's other refusals have, an agent whose files aren't made
// here with the agents whose are.
func GenerateJSON(body []byte) (int, []byte) {
	fail := func(status int, code, msg string, extra map[string]any) (int, []byte) {
		e := map[string]any{"message": msg, "type": "invalid_request_error", "code": code}
		maps.Copy(e, extra)
		b, _ := json.Marshal(map[string]any{"error": e})
		return status, b
	}
	var req GenRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.More() {
		msg := "the body is not a JSON request for an agent's config"
		if err != nil {
			msg += ": " + err.Error()
		}
		return fail(http.StatusBadRequest, "agent_config_bad_request", msg+` (like {"agent":"codex","gateway":"http://magpie.lan:3425","key_env":"MAGPIE_KEY"})`, nil)
	}
	res, err := Generate(req)
	var un *UnsupportedError
	var bad *GenInputError
	switch {
	case errors.As(err, &un):
		return fail(http.StatusUnprocessableEntity, "agent_config_unsupported", err.Error(), map[string]any{"agent": un.Agent, "supported": un.Supported})
	case errors.As(err, &bad):
		return fail(http.StatusBadRequest, "agent_config_bad_request", err.Error(), nil)
	case errors.Is(err, ErrGenNoModels):
		return fail(http.StatusConflict, "agent_config_no_models", err.Error(), nil)
	case err != nil:
		return fail(http.StatusInternalServerError, "agent_config_failed", err.Error(), nil)
	}
	b, err := json.Marshal(res)
	if err != nil {
		return fail(http.StatusInternalServerError, "agent_config_failed", err.Error(), nil)
	}
	return http.StatusOK, b
}

// genAgent is kind's agent at the run's place, as wslAgent builds a
// distro's.
func genAgent(k wslKind, run *genRun, version string) *Agent {
	id := k.id + genMark + run.tok
	at := place{
		home: run.home,
		id:   id,
		spell: func(p string) string {
			rel, err := filepath.Rel(run.home, p)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return p
			}
			if rel == "." {
				return run.spelt
			}
			return run.spelt + "/" + filepath.ToSlash(rel)
		},
		sys:     func(p string) string { return filepath.Join(run.root, "sys", filepath.FromSlash(p)) },
		base:    func() string { return run.base },
		version: version,
		gen:     true,
	}
	a := k.in(at)
	a.ID, a.Name, a.Aliases, a.Bin, a.UA = id, k.name, nil, "", nil
	a.Gateway = at.base
	a.LastUsed = nil
	a.detect = func() bool { return true }
	return a
}

// genPick puts the agent on the model asked for, as the Agents page's
// picker does. With none asked it stays on the one connect picked, unless
// that one isn't among the models given: then it is the first of those.
func genPick(a *Agent, c Connection, model string, models []string, res *GenResult) error {
	key := c.Field
	if key == "" {
		for _, f := range a.Fields {
			if f.Options != nil && !f.Own {
				key = f.Key
				break
			}
		}
	}
	f := a.Field(key)
	if f == nil {
		return fmt.Errorf("%s has no model to set", a.Name)
	}
	vals := a.Values()
	opts := f.Options(vals)
	refOf := func(v string) string {
		for _, o := range opts {
			if o.Value == v {
				return o.Ref
			}
		}
		return ""
	}
	want := model
	if want == "" {
		if cur := refOf(vals[key]); cur != "" && (models == nil || slices.Contains(models, cur)) {
			res.Model = cur
			return nil
		}
		if models == nil {
			return fmt.Errorf("magpie has no model to give %s", a.Name)
		}
		want = models[0]
	}
	v, err := a.Spell(key, want)
	if err != nil {
		return err
	}
	if refOf(v) == "" {
		return genBad(fmt.Errorf("model %q is not one of magpie's models for %s", want, a.Name))
	}
	if v != vals[key] {
		if err := a.Pick(key, v); err != nil {
			return err
		}
	}
	res.Model = refOf(v)
	return nil
}

// genFiles is what the run's agent wrote: the files under its home. One
// written elsewhere (an /etc file of the agent's system) is no file the
// caller can put in HOME, so the agent isn't one made here.
func genFiles(run *genRun) ([]GenFile, error) {
	var out []GenFile
	err := filepath.WalkDir(run.home, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(run.home, p)
		if !d.Type().IsRegular() {
			return fmt.Errorf("magpie makes %s as a link, not a file", filepath.ToSlash(rel))
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out = append(out, GenFile{Path: filepath.ToSlash(rel), Content: string(b), Mode: fmt.Sprintf("%04o", info.Mode().Perm())})
		return nil
	})
	if err != nil {
		return nil, err
	}
	var outside []string
	sys := filepath.Join(run.root, "sys")
	filepath.WalkDir(sys, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(sys, p)
			outside = append(outside, "/"+filepath.ToSlash(rel))
		}
		return nil
	})
	if len(outside) > 0 {
		return nil, fmt.Errorf("magpie writes this agent's files outside its home too (%s)", strings.Join(outside, ", "))
	}
	slices.SortFunc(out, func(a, b GenFile) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

// genClean checks nothing of this computer's is in the files: the run's
// own address, key or home left in them (a spelling of them the agent
// writes that wasn't put back), this magpie's own address or a key of its
// own (the sharing key, WSL's, the gateway's token) that isn't the
// caller's.
func genClean(files []GenFile, run *genRun, gw string, req GenRequest) error {
	var secrets []string
	for _, s := range []string{access.LANSecret(), gateway.WSLKeyMade(), gateway.Token} {
		if len(s) >= 8 && s != req.Key {
			secrets = append(secrets, s)
		}
	}
	here := strings.TrimRight(gateway.URL(), "/")
	for _, f := range files {
		if strings.Contains(f.Content, run.tok) {
			return fmt.Errorf("%s: magpie writes this agent's gateway, key or home in a way it can't make for another computer yet", f.Path)
		}
		if here != "" && here != gw && strings.Contains(strings.ReplaceAll(f.Content, gw, ""), here) {
			return fmt.Errorf("%s: names this magpie's own address", f.Path)
		}
		for _, s := range secrets {
			if strings.Contains(f.Content, s) {
				return fmt.Errorf("%s: holds a key of this magpie's own", f.Path)
			}
		}
	}
	return nil
}

// genEnvNote is what the caller is told when the files read the key from
// a variable.
func genEnvNote(name, agent string) string {
	return "set " + name + " to the gateway key where " + agent + " runs: the files read it from there"
}

// genCodexEnv: Codex reads its provider's key from the variable env_key
// names, in place of experimental_bearer_token.
func genCodexEnv(home, sentinel, name string) ([]string, error) {
	path := filepath.Join(home, ".codex", "config.toml")
	table := "model_providers." + magpieID
	if v, ok := edit.GetTOML(path, table+".experimental_bearer_token"); !ok || v != sentinel {
		return nil, errors.New("Codex's config has no key of magpie's to read from a variable")
	}
	if err := edit.DelTOMLKey(path, table, "experimental_bearer_token"); err != nil {
		return nil, err
	}
	if err := edit.SetTOMLKey(path, table, "env_key", name); err != nil {
		return nil, err
	}
	return []string{genEnvNote(name, "Codex")}, nil
}

// genClaudeEnv: Claude Code's settings take no variable in a value, so
// the key isn't in its env: ANTHROPIC_AUTH_TOKEN is read from Claude
// Code's own environment, and another variable through apiKeyHelper, a
// command whose output is the key.
func genClaudeEnv(home, sentinel, name string) ([]string, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	if v, ok := edit.GetJSON(path, "env.ANTHROPIC_AUTH_TOKEN"); !ok || v != sentinel {
		return nil, errors.New("Claude Code's settings have no key of magpie's to read from a variable")
	}
	if err := edit.DelJSON(path, "env.ANTHROPIC_AUTH_TOKEN"); err != nil {
		return nil, err
	}
	if name == "ANTHROPIC_AUTH_TOKEN" {
		return []string{genEnvNote(name, "Claude Code")}, nil
	}
	if err := edit.SetJSON(path, edit.KV{Path: "apiKeyHelper", Value: `printf %s "$` + name + `"`}); err != nil {
		return nil, err
	}
	return []string{genEnvNote(name, "Claude Code") + " (its apiKeyHelper prints it)"}, nil
}

// genClaudeKept: the haiku tier takes the Haiku beside a Claude main
// model (follow), which may be no model given; it takes the main model
// then, as it does where there is no such Haiku.
func genClaudeKept(home string, models []string) error {
	path := filepath.Join(home, ".claude", "settings.json")
	main, _ := edit.GetJSON(path, "model")
	var kvs []edit.KV
	for _, k := range claudeModelEnv {
		v, ok := edit.GetJSON(path, "env."+k)
		if !ok || v == "" {
			continue
		}
		m, _ := tierAt(v)
		if !slices.Contains(models, strings.TrimSuffix(m, "[1m]")) {
			kvs = append(kvs, edit.KV{Path: "env." + k, Value: main})
		}
	}
	if len(kvs) == 0 {
		return nil
	}
	return edit.SetJSON(path, kvs...)
}

// genOpenCodeEnv: OpenCode reads {env:NAME} in its config as the
// variable's value.
func genOpenCodeEnv(home, sentinel, name string) ([]string, error) {
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	key := "provider." + magpieID + ".options.apiKey"
	if v, ok := edit.GetJSON(path, key); !ok || v != sentinel {
		return nil, errors.New("OpenCode's config has no key of magpie's to read from a variable")
	}
	if err := edit.SetJSON(path, edit.KV{Path: key, Value: "{env:" + name + "}"}); err != nil {
		return nil, err
	}
	return []string{genEnvNote(name, "OpenCode")}, nil
}

// genDshEnv: dsh's route names the variable its key is in (apiKeyEnv),
// which dsh reads from its environment when neither its key store nor
// .env holds it: those files, which hold only the key, aren't made.
func genDshEnv(home, sentinel, name string) ([]string, error) {
	dir := filepath.Join(home, ".dsh")
	// in a home made fresh they hold magpie's key and nothing else
	env, creds := filepath.Join(dir, ".env"), filepath.Join(dir, ".credentials.yaml")
	if v, ok := edit.GetEnvFile(env, dshKeyRef); ok {
		if v != sentinel || len(genLines(env)) > 1 {
			return nil, errors.New("dsh's .env holds more than magpie's key")
		}
		if err := os.Remove(env); err != nil {
			return nil, err
		}
	}
	if v, ok := edit.GetYAML(creds, "refs."+dshKeyRef); ok {
		if v != sentinel || len(edit.GetYAMLMap(creds, "refs")) > 1 {
			return nil, errors.New("dsh's key store holds more than magpie's key")
		}
		if err := os.Remove(creds); err != nil {
			return nil, err
		}
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s := strings.ReplaceAll(string(b), "apiKeyEnv: "+dshKeyRef+"\n", "apiKeyEnv: "+name+"\n")
		if s == string(b) {
			return nil
		}
		return os.WriteFile(p, []byte(s), 0o600)
	})
	if err != nil {
		return nil, err
	}
	return []string{genEnvNote(name, "DeepSeek Harness")}, nil
}

// genLines are a file's lines that hold something: not blank, not a
// comment.
func genLines(path string) []string {
	b, _ := os.ReadFile(path)
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}
