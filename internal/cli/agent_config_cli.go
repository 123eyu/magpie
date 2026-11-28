package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/gateway"
)

// agentConfigUsage is `magpie agent config --help`.
var agentConfigUsage = `usage: magpie agent config <agent> --gateway <url> (--key-env <NAME> | --key <key>|-) [--model <provider/model>] [--models <a,b,...>] [--version <v>] [--json]

  An agent's config files for a computer magpie isn't on (a container, a
  sandbox, a server), printed, each with its path under that computer's
  HOME. Nothing is written here and the agent needn't be installed.

  <agent>          ` + strings.Join(agent.GenSupported(), ", ") + ` (claude-code is claude); any other is answered "unsupported"
  --gateway        magpie's address as the agent reaches it, like http://magpie.lan:3425
  --key-env        the variable the gateway key is in where the agent runs: the files read it from there
  --key            the gateway key itself, written in the files (- reads it from stdin)
  --model          the model the agent starts on (magpie picks one otherwise)
  --models         the only models its lists hold (every model magpie gives it otherwise)
  --version        the agent's version there, for an agent whose files depend on it
  --json           {"agent","model","files":[{"path","content","mode"}],"notes"}

  Remote callers: POST ` + gateway.AgentConfigPath + ` with a gateway key and the same
  values as JSON (agent, gateway, key_env or key, model, models, version).
`

// agentConfigCmd is `magpie agent config …` (and `magpie agents config …`).
func agentConfigCmd(args []string) error { return agentConfigTo(os.Stdout, os.Stdin, args) }

func agentConfigTo(out io.Writer, in io.Reader, args []string) error {
	if len(args) == 0 || args[0] != "config" {
		return fmt.Errorf("usage: magpie agent config <agent> --gateway <url> --key-env <NAME> (magpie agent config --help)")
	}
	args = args[1:]
	var req agent.GenRequest
	asJSON, modelsSet := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, val, inline := strings.Cut(a, "=")
		if !strings.HasPrefix(a, "--") {
			if req.Agent != "" {
				return fmt.Errorf("one agent at a time: %q and %q", req.Agent, a)
			}
			req.Agent = a
			continue
		}
		if name == "--json" {
			if inline {
				return fmt.Errorf("--json takes no value")
			}
			asJSON = true
			continue
		}
		if !inline {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return fmt.Errorf("%s needs a value", name)
			}
			i++
			val = args[i]
		}
		switch name {
		case "--gateway":
			req.Gateway = val
		case "--key-env":
			req.KeyEnv = val
		case "--key":
			if val == "-" {
				line, err := bufio.NewReader(in).ReadString('\n')
				if err != nil && err != io.EOF {
					return err
				}
				val = strings.TrimSpace(line)
				if val == "" {
					return fmt.Errorf("--key -: no key on stdin")
				}
			}
			req.Key = val
		case "--model":
			req.Model = val
		case "--models":
			modelsSet = true
			for _, m := range strings.Split(val, ",") {
				if m = strings.TrimSpace(m); m != "" {
					req.Models = append(req.Models, m)
				}
			}
		case "--version":
			req.Version = val
		default:
			return fmt.Errorf("unknown flag %s (magpie agent config --help)", name)
		}
	}
	if req.Agent == "" {
		return fmt.Errorf("which agent? magpie agent config <%s> --gateway <url> --key-env <NAME>", strings.Join(agent.GenSupported(), "|"))
	}
	if modelsSet && len(req.Models) == 0 {
		return fmt.Errorf("--models: none given")
	}
	res, err := agent.Generate(req)
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	for i, f := range res.Files {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "# ~/%s (%s)\n%s", f.Path, f.Mode, f.Content)
		if !strings.HasSuffix(f.Content, "\n") {
			fmt.Fprintln(out)
		}
	}
	if res.Model != "" {
		fmt.Fprintf(out, "\n# starts on %s\n", res.Model)
	}
	for _, n := range res.Notes {
		fmt.Fprintf(out, "# %s\n", n)
	}
	return nil
}
