package gateway

import (
	"io"
	"log"
	"net/http"

	"github.com/yetone/magpie/internal/access"
)

// AgentConfigPath makes an agent's config files for a computer magpie
// isn't on (#7002, Geequlim: agents in containers and sandboxes reach a
// magpie on a server): POST a JSON body of what they are made for (the
// agent, magpie's address as the agent reaches it, the key or the name of
// the variable holding it, the models) and get the files back, each with
// its path under the agent's HOME. Nothing is written on this computer.
const AgentConfigPath = "/v1/magpie/agent-config"

// AgentConfig makes the files for a request's body and answers the
// status and the JSON to send, set by the agent package: the gateway
// doesn't know agents' configs.
var AgentConfig func(body []byte) (int, []byte)

// agentConfigMax is the largest body a request for files is read to.
const agentConfigMax = 64 << 10

// agentConfig answers AgentConfigPath. The answer holds the catalog's
// models and whatever key is asked to be written, so it is told only to
// a script with a gateway key: never a web page (any Origin, null
// included, or Sec-Fetch header), never a request without a key (from
// this computer too), never one that reached loopback under a name that
// isn't this computer's (DNS rebinding), from another machine only while
// magpie is shared, and never to a key held to a budget, models or
// accounts, which would be told models it may not use.
func (s *Server) agentConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" || r.Header.Get("Sec-Fetch-Mode") != "" {
		log.Printf("refused %s %s: agent configs are made for no web page (Origin %q)", r.Method, r.URL.Path, r.Header.Get("Origin"))
		adminRefuse(w, http.StatusForbidden, "agent_config_browser_refused", "magpie makes agent configs for scripts, not web pages: a request that carries Origin or Sec-Fetch headers is refused, with a key or without")
		return
	}
	who := access.Caller(r.Context())
	if who.KeyID == "" {
		adminRefuse(w, http.StatusUnauthorized, "agent_config_key_required", "making agent configs needs a gateway key, even from this computer: send it as Authorization: Bearer <key> (Gateway → Gateway keys, or magpie gateway-key add)")
		return
	}
	if local(r) {
		if !ownHost(r.Host) {
			log.Printf("refused %s %s: reached loopback as %q", r.Method, r.URL.Path, r.Host)
			adminRefuse(w, http.StatusForbidden, "agent_config_host_refused", "magpie doesn't make agent configs for a request that reached this computer under the name "+r.Host+": call it as localhost or 127.0.0.1")
			return
		}
	} else if !sharedWith(r) {
		adminRefuse(w, http.StatusForbidden, "agent_config_not_shared", "magpie makes agent configs for another machine only when magpie is shared on the local network (Settings → Network and sharing → Share on local network) and the request carries its API key")
		return
	}
	if heldKey(who) {
		adminRefuse(w, http.StatusForbidden, "agent_config_key_held", "this gateway key is held to a budget, models or accounts, so it is told nothing of the models it may not use: ask with a key that isn't held")
		return
	}
	if AgentConfig == nil {
		adminRefuse(w, http.StatusNotFound, "agent_config_unavailable", "this magpie makes no agent configs")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, agentConfigMax))
	if err != nil {
		adminRefuse(w, http.StatusRequestEntityTooLarge, "agent_config_too_large", "the request's body is larger than an agent config request is")
		return
	}
	status, out := AgentConfig(body)
	log.Printf("agent config: %d for gateway key %q", status, who.KeyName)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(out)
}
