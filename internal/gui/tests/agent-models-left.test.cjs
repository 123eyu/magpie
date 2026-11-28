// Run with Node's test runner and Playwright on the module path; see README.md.
// An agent's model list names a provider it leaves out, as it serves only
// through routing groups, with why (Sec_211 on X: "0-onaiproxy" and its
// copy, 16 models each on the Providers page, were nowhere in Codex's list,
// with no word): a row in the rail, a section after the providers with the
// reason, its own "Only through routing groups" one click from its editor.
// A search finds it by name or model. In the window (with its rail, and at
// a narrow width without) and the tray panel, every language, Chromium and
// WebKit. No backend: faked here, the list's left as /api/agent-models
// gives it (agentLeftOut).
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const ms = Array.from({ length: 16 }, (_, i) => "m" + i);

function fixture(lang) {
  const list = [
    { id: "group/onai", name: "Onai", group: "Routing groups", icons: ["generic"] },
    ...ms.map((m) => ({ id: "0-onaiproxy-2/" + m, name: m, group: "0-onaiproxy 副本", icon: "generic" })),
    { id: "https-s2api-top/only", name: "only", group: "https://s2api.top/", icon: "generic" },
  ];
  const left = [
    // the user's group first, then the groups magpie found of each model
    // on it and its copy: three named at most
    { id: "0-onaiproxy", name: "0-onaiproxy", models: ms, unlisted: true, via: ["Onai", ...ms] },
    { id: "lone", name: "Lone", models: ["lone-1"], unlisted: true, via: [] },
    { id: "kept", name: "Kept", models: ["kept-a", "kept-b"], by: ["S2"], via: ["S2"] },
  ];
  const count = () => ({ shown: list.length, listed: list.length, by: [], names: [] });
  const state = () => ({
    agents: [{
      id: "zcode", name: "ZCode", path: "~/.zcode/v2/config.json", icon: "zcode", wired: true, menu: true,
      fields: [{ key: "provider", label: "provider", value: "magpie", options: [{ value: "magpie", label: "magpie", icon: "magpie", note: "every magpie model in ZCode's picker" }] }],
      models: count(),
    }],
    profiles: [], settings: { lang, theme: "light" },
  });
  const posts = [];
  async function serve(route) {
    const req = route.request(), url = new URL(req.url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:true};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(state());
    if (url.pathname === "/api/agent-models/zcode") return json({ models: list, left });
    if (url.pathname === "/api/window/main") { posts.push(url.pathname + url.search); return route.fulfill({ status: 204 }); }
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname === "/api/groups") return json({ groups: [] });
    if (url.pathname === "/api/agents/cli") return json({ agents: {}, pending: false });
    if (url.pathname === "/api/providers") return json({ providers: [{ id: "0-onaiproxy", name: "0-onaiproxy", icon: "generic", chat: "https://onaiproxy.test/v1", responses: "", anthropic: "", catalog: "", models: ms.map((m) => ({ id: m, name: m, on: true })), agents: [], fallback: [], headers: {}, keyList: [], key: { set: true, masked: "sk-…1234" }, ready: true, unlisted: true }], presets: [], excluded: [], gateway: { running: true, window: true } });
    if (url.pathname === "/api/plugins") return json({ plugins: [] });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    const body = await fs.readFile(file).catch(() => null);
    await route.fulfill(body ? { body, contentType } : { status: 404, body: "" });
  }
  return { serve, posts };
}

const row = '.row.agent[data-id="zcode"]';
const views = { window: { width: 1100, height: 700 }, narrow: { width: 520, height: 700 }, panel: { width: 360, height: 640 } };

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of ["en", "zh", "zh-TW", "ja", "de"]) {
    for (const where of Object.keys(views)) {
      test(`${engine} ${lang} ${where}: the model list names the providers it leaves out, and why`, async (t) => {
        const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
        const page = await browser.newPage({ viewport: views[where] });
        page.setDefaultTimeout(5000);
        page.setDefaultNavigationTimeout(60000);
        const errors = [];
        page.on("pageerror", (e) => errors.push(e.message));
        const fx = fixture(lang);
        await page.route("**/*", fx.serve);
        t.after(async () => {
          if (process.env.ARTIFACT_DIR) {
            await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
            await page.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-${lang}-${where}-agent-models-left.png`) });
          }
          await browser.close();
        });
        await page.goto("http://magpie.test/" + (where === "panel" ? "?mode=panel" : ""));
        await page.locator(row).waitFor();
        const tr = (s, v) => page.evaluate(([s, v]) => t(s, v), [s, v]);
        let menu;
        if (where !== "panel") menu = page.locator(`${row} > .ag-menu`);
        else {
          await page.locator(`${row} .ag-sum`).click();
          menu = page.locator(`${row} .ag-open .ag-menu`);
          await page.waitForFunction((sel) => document.querySelector(sel)?.getBoundingClientRect().height > 0, `${row} .ag-open .ag-menu`);
        }
        // the row's slide open done: wait on it, never a duration
        await page.waitForFunction(() => document.getAnimations().length === 0);
        await menu.click();
        const box = page.locator(".am-pop:not(.leaving)");
        await box.waitFor();
        const secs = box.locator(".am-list section.am-lo");

        // every one after the providers, with why, in the reader's language
        await secs.first().waitFor();
        assert.deepEqual(await secs.evaluateAll((es) => es.map((e) => e.dataset.id)), ["0-onaiproxy", "lone", "kept"]);
        const onai = secs.nth(0), lone = secs.nth(1), kept = secs.nth(2);
        assert.equal(await onai.locator(".gn").textContent(), "0-onaiproxy");
        assert.equal(await onai.locator(".am-gh .c").textContent(), await tr("{n} models", { n: 16 }));
        assert.equal(await onai.locator(".am-lo-d > span").textContent(), await tr("Only through routing groups: {agent} reaches its models by picking {groups}.", { agent: "ZCode", groups: "Onai, m0, m1, …" }));
        assert.equal(await lone.locator(".am-gh .c").textContent(), await tr("{n} model", { n: 1 }));
        assert.equal(await lone.locator(".am-lo-d > span").textContent(), await tr("Only through routing groups, and no group has its models, so no agent can use them."));
        assert.equal(await kept.locator(".am-lo-d > span").textContent(), await tr("Listed only as {groups}: that routing group keeps its members out of agents' lists.", { groups: "S2" }));
        // its own setting is one click from its editor; a group's isn't the provider's
        assert.equal(await onai.locator(".am-lo-d button").textContent(), await tr("Open {name} in Providers", { name: "0-onaiproxy" }));
        assert.equal(await kept.locator(".am-lo-d button").count(), 0);
        // nothing to tick: no checkbox in them
        assert.equal(await secs.locator('[role="checkbox"]').count(), 0);
        // the note wraps inside the list, never wider than it
        const over = await box.locator(".am-list").evaluate((l) => [...l.querySelectorAll(".am-lo-d")].some((d) => d.getBoundingClientRect().right > l.getBoundingClientRect().right + 0.5));
        assert.equal(over, false, "a note runs past the list");

        // the rail (a wide window only) has a row each, picked alone
        const railed = where === "window";
        assert.equal(await box.locator(".am-rail").count(), railed ? 1 : 0);
        if (railed) {
          const lo = box.locator(".am-rail .am-rr.am-lo .am-ri");
          assert.deepEqual(await lo.locator(".rn").allTextContents(), ["0-onaiproxy", "Lone", "Kept"]);
          assert.match(await lo.first().getAttribute("title"), /^0-onaiproxy\n/);
          await lo.nth(1).click();
          await page.waitForFunction(() => [...document.querySelectorAll(".am-pop:not(.leaving) .am-list section")].map((s) => s.dataset.id).join() === "lone");
          assert.equal(await lo.nth(1).getAttribute("aria-pressed"), "true");
          // a provider's own row shows none of them
          await box.locator(".am-rail .am-ri", { hasText: "https://s2api.top/" }).click();
          await page.waitForFunction(() => !document.querySelector(".am-pop:not(.leaving) .am-list section.am-lo"));
          await box.locator(".am-rail .am-ri").first().click();
          await secs.first().waitFor();
        }

        // a search finds one by a model's name, and leaves the others out
        const q = box.locator('input[type="text"]');
        await q.fill("kept-b");
        await page.waitForFunction(() => [...document.querySelectorAll(".am-pop:not(.leaving) .am-list section.am-lo")].map((s) => s.dataset.id).join() === "kept");
        await q.fill("");
        await page.waitForFunction(() => document.querySelectorAll(".am-pop:not(.leaving) .am-list section.am-lo").length === 3);

        if (process.env.ARTIFACT_DIR) await box.locator(".am-list").evaluate((l) => { l.scrollTop = l.scrollHeight; });
        if (process.env.ARTIFACT_DIR) await page.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-${lang}-${where}-agent-models-left-list.png`) });
        // Open: the list closes and the provider's editor opens, in the
        // window itself, or from the panel in the window
        await onai.locator(".am-lo-d button").click();
        await page.waitForFunction(() => !document.querySelector(".am-pop:not(.leaving)"));
        if (where === "panel") {
          for (let i = 0; i < 100 && !fx.posts.length; i++) await page.waitForTimeout(20);
          assert.deepEqual(fx.posts, ["/api/window/main?view=providers&edit=0-onaiproxy"]);
        } else {
          await page.waitForFunction(() => new URLSearchParams(location.search).get("edit") === "0-onaiproxy");
          assert.equal(new URL(page.url()).searchParams.get("view"), "providers");
        }
        assert.deepEqual(errors, []);
      });
    }
  }
}
