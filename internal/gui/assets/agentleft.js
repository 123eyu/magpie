// A provider none of whose models an agent's list has, as they serve only
// through routing groups (agentLeftOut in agent_models.go): the list names
// it, in its rail and after the providers, with why, rather than leave it
// out without a word (Sec_211 on X: a provider and its copy, 16 models
// each on the Providers page, were nowhere in Codex's list). app.js
// imports this beside the list's own read (openAgentModels), so it weighs
// nothing on the core. pick is the rail's row picked, "lo:<id>" for one
// of these; to picks one.
// at most three groups named: one magpie found for each model the same on
// another provider (a copy's sixteen) would make a long line
const amLeftNames = (gs) => gs.slice(0, 3).join(", ") + (gs.length > 3 ? ", …" : "");
const amLeftSay = (l, agent) => {
  if (!l.unlisted) return t("Listed only as {groups}: that routing group keeps its members out of agents' lists.", { groups: amLeftNames(l.by) });
  return l.via.length ? t("Only through routing groups: {agent} reaches its models by picking {groups}.", { agent, groups: amLeftNames(l.via) })
    : t("Only through routing groups, and no group has its models, so no agent can use them.");
};

window.amLeft = (left, agent, to) => ({
  // a row each in the rail: no tick, as there is nothing of it to show
  // here; the routing groups' fan where a provider's count is
  rail(rail, pick) {
    for (const l of left) {
      const row = el("div", "am-rr am-lo");
      const b = el("button", "am-ri" + (pick === "lo:" + l.id ? " on" : ""));
      b.type = "button";
      b.setAttribute("aria-pressed", String(pick === "lo:" + l.id));
      const c = el("span", "c");
      c.append(svg(FAN, 12, 1.5));
      b.append(icon(l.icon || "generic"), el("span", "rn", l.name), c);
      b.title = l.name + "\n" + amLeftSay(l, agent);
      b.onclick = () => to("lo:" + l.id);
      row.append(b);
      rail.append(row);
    }
  },
  // a section each after the providers, for All, its own row, or a search
  // that finds its name or a model's: why, and, for its own "Only through
  // routing groups", its editor, where that is unticked
  list(list, pick, words) {
    for (const l of left) {
      if (words ? ![l.name, ...l.models].some((s) => s.toLowerCase().includes(words)) : pick && pick !== "lo:" + l.id) continue;
      const sec = el("section", "am-g am-lo");
      sec.dataset.id = l.id;
      const gh = el("div", "am-gh");
      gh.append(icon(l.icon || "generic"), el("span", "gn", l.name), el("span", "c", t(l.models.length === 1 ? "{n} model" : "{n} models", { n: l.models.length })));
      const d = el("div", "am-lo-d");
      d.append(el("span", "", amLeftSay(l, agent)));
      if (l.unlisted) {
        const go = el("button", "text action", t("Open {name} in Providers", { name: l.name }));
        go.type = "button";
        go.onclick = () => {
          closeAgentModels();
          if (mode !== "window") api("window/main?view=providers&edit=" + encodeURIComponent(l.id), {}).catch(statusErr);
          else openProvider(l.id);
        };
        d.append(go);
      }
      sec.append(gh, d);
      list.append(sec);
    }
  },
});
