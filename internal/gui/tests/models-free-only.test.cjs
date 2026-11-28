// Run with Node's test runner and Playwright on the module path; see README.md.
// Free only is a way a provider serves its models, beside Best in family
// (niuniu_8964 on X: 将"只选免费"按钮改成和"每系最新"一样的筛选条件; the
// button picked today's free models once, and a model made free later was
// never added). It is offered when the list has free models (the plan's
// own, m.free; priced at nothing, m.zero; or one its vendor names free,
// foo:free) and paid ones, as the button was (a Discord ask: 只选择免费模型);
// a list with none free, or every one free, doesn't offer it. With it on,
// every free model is drawn dashed, more than the vendor's list's 24 too,
// and the paid ones left out; the hint says so and that the list is
// fetched again every 6 hours. Save sends freeOnly with no picks, and the
// editor opens on it again. Picked only, Vendor's list and Best in family
// turn it off. The old one-off button is gone. In every language,
// Chromium and WebKit, at 1100 and 420 wide.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const fetched = new Date(Date.now() - 3600e3).toISOString();
const relay = (id, name, models, extra) => ({
  id, name, icon: "generic", host: id + ".example.com", chat: `https://${id}.example.com/v1`, responses: "", anthropic: "", catalog: "",
  models, chosen: [], fetched, agents: [], fallback: [], headers: {},
  key: { set: true, masked: "sk-…one" }, keyList: [], balanceToken: { takes: false, set: false }, proxy: "", ...extra,
});
const m = (id, more) => ({ id, name: id, on: false, ...more });
// free: the plan's own, one priced at nothing, and 25 the vendor names
// free, so more than the vendor's list's 24
const named = Array.from({ length: 25 }, (_, i) => `vendor/free-model-${i + 1}:free`);
const FREE = ["cline-free/deepseek-v4.1-flash", "zero/priced-nothing", ...named];
const PAID = ["paid-model-1", "paid-model-2", "paid-model-3"];
const mixedModels = () => [m(FREE[0], { free: true }), m(PAID[0]), m(FREE[1], { zero: true }), m(PAID[1]), ...named.map((id) => m(id)), m(PAID[2])];
const mixed = relay("mixed", "Mixed Relay", mixedModels());
const saved = relay("savedfree", "Saved Free", mixedModels().map((x) => ({ ...x, on: FREE.includes(x.id) })), { freeOnly: true });
const allPaid = relay("allpaid", "Paid Relay", [m("gpt-5"), m("gpt-5-mini")]);
const allFree = relay("allfree", "Free Relay", [m("free-a", { free: true }), m("free-b:free")]);

function serve(lang, saves) {
  const providers = { providers: [mixed, saved, allPaid, allFree], presets: [], excluded: [], gateway: { running: true, window: true } };
  const state = { agents: [], profiles: [], settings: { lang, theme: "light" } };
  return async (route) => {
    const req = route.request(), url = new URL(req.url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:true};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(state);
    if (url.pathname === "/api/plugins") return json({ plugins: [] });
    if (url.pathname === "/api/providers") return json(providers);
    if (url.pathname === "/api/provider/save") { saves.push(req.postDataJSON()); return json(providers); }
    if (url.pathname === "/api/groups") return json({ groups: [] });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  };
}

// hint: the start of the editor's hint under Free only
const words = {
  en: { save: "Save", list: "Vendor's list", picked: "Picked only", best: "Best in family", free: "Free only", hint: "Free only: agents see every model it lists as free" },
  zh: { save: "保存", list: "供应商列表", picked: "仅所选", best: "每系最新", free: "仅免费", hint: "仅免费：Agent 看到它列表里所有免费的模型" },
  "zh-TW": { save: "儲存", list: "供應商列表", picked: "僅所選", best: "每系最新", free: "僅免費", hint: "僅免費：Agent 看到它列表裡所有免費的模型" },
  ja: { save: "保存", list: "ベンダーのリスト", picked: "選択したもののみ", best: "系列の最新のみ", free: "無料のみ", hint: "無料のみ：エージェントには一覧で無料のモデルがすべて見えます" },
  de: { save: "Speichern", list: "Liste des Anbieters", picked: "Nur gewählte", best: "Neuestes je Familie", free: "Nur kostenlose", hint: "Nur kostenlose: Agents sehen jedes Modell, das er als kostenlos listet" },
};

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of Object.keys(words)) {
    const w = words[lang];
    for (const width of [1100, 420]) {
      test(`${engine} ${lang} ${width}: Free only serves the free models as they change`, async (t) => {
        const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
        const page = await (await browser.newContext({ viewport: { width, height: 760 }, reducedMotion: "reduce" })).newPage();
        t.after(async () => {
          if (process.env.ARTIFACT_DIR) {
            await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
            await page.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-${lang}-${width}-models-free-only.png`) });
          }
          await browser.close();
        });
        page.setDefaultTimeout(5000);
        page.setDefaultNavigationTimeout(60000);
        const errors = [];
        page.on("pageerror", (e) => errors.push(e.message));
        const saves = [];
        await page.route("**/*", serve(lang, saves));
        await page.goto("http://magpie.test/?view=providers");
        const editor = page.locator("#modal:not([hidden]) .editor");
        const open = async (name) => {
          await page.locator(".row.provider", { hasText: name }).click();
          await editor.locator(".ehead b", { hasText: name }).waitFor();
          await editor.locator(".mchips .mchip").first().waitFor();
        };
        const close = async () => {
          await page.keyboard.press("Escape");
          await page.locator("#modal").waitFor({ state: "hidden" });
        };
        // each chip: [picked, served by the list (dashed)]
        const cls = () => editor.locator(".mchips .mchip").evaluateAll((cs) => Object.fromEntries(cs.map((c) => [c.querySelector("span").textContent, [c.classList.contains("on"), c.classList.contains("auto")]])));
        const servedOnly = async (want) => {
          const got = await cls();
          for (const id of FREE) assert.deepEqual(got[id], [false, want.includes(id)], id);
          for (const id of PAID) assert.deepEqual(got[id], [false, want.includes(id)], id);
        };
        const serves = async () => {
          const seg = editor.locator(".mserves");
          const box = await seg.boundingBox(), ed = await editor.boundingBox();
          assert.ok(box && box.x >= ed.x - 1 && box.x + box.width <= ed.x + ed.width + 1, "the control fits the editor");
          for (const o of await seg.locator(".opt").all()) assert.ok(await o.evaluate((b) => b.scrollWidth <= b.clientWidth + 1), "an option's label fits");
          return seg.locator(".opt.on").innerText();
        };
        const save = async () => {
          const n = saves.length;
          await editor.locator(".bar").getByRole("button", { name: w.save, exact: true }).click();
          for (let i = 0; i < 60 && saves.length === n; i++) await page.waitForTimeout(50);
          assert.equal(saves.length, n + 1, "saved");
          await page.locator("#modal:not([hidden]) .editor").waitFor({ state: "detached" }).catch(() => {});
          return saves[n];
        };
        const freeOpt = editor.locator(".mserves .opt[data-serves=free]");

        // none free, or every one free: not offered
        for (const name of ["Paid Relay", "Free Relay"]) {
          await open(name);
          assert.equal(await serves(), w.list);
          assert.equal(await freeOpt.count(), 0, `${name} offers Free only`);
          await close();
        }

        // free and paid: the fifth option; the old one-off button is gone
        await open("Mixed Relay");
        assert.equal(await serves(), w.list);
        assert.deepEqual(await editor.locator(".mserves .opt").evaluateAll((os) => os.map((o) => o.dataset.serves)), ["list", "picked", "best", "free", "none"]);
        assert.equal(await editor.locator(".mbulk").getByRole("button", { name: w.free, exact: true }).count(), 0, "the one-off button is still there");
        assert.equal(await editor.getByText(w.hint, { exact: false }).count(), 0);
        await freeOpt.click();
        assert.equal(await serves(), w.free);
        // every free one served, more than 24, none of the paid ones
        await servedOnly(FREE);
        assert.ok(FREE.length > 24);
        await editor.getByText(w.hint, { exact: false }).waitFor();
        assert.ok(await freeOpt.getAttribute("title"));
        if (process.env.ARTIFACT_DIR) await editor.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-${lang}-${width}-models-free-only-editor.png`) });
        let sent = await save();
        assert.equal(sent.freeOnly, true, "freeOnly sent");
        assert.equal(sent.bestInFamily, false);
        assert.equal(sent.noModels, false);
        assert.deepEqual(sent.models || [], []);

        // one saved with it opens on it; Best in family and Picked only
        // turn it off
        await open("Saved Free");
        assert.equal(await serves(), w.free);
        await servedOnly(FREE);
        await editor.getByText(w.hint, { exact: false }).waitFor();
        await editor.locator(".mserves .opt[data-serves=best]").click();
        assert.equal(await serves(), w.best);
        assert.equal(await editor.getByText(w.hint, { exact: false }).count(), 0);
        await freeOpt.click();
        await editor.locator(".mserves .opt[data-serves=picked]").click();
        assert.equal(await serves(), w.picked);
        assert.equal(await editor.locator(".mchips .mchip.on").count(), 0, "Picked only picks nothing yet");
        // Vendor's list: the first 24 dashed, and Save sends it off
        await editor.locator(".mserves .opt[data-serves=list]").click();
        assert.equal(await serves(), w.list);
        assert.equal(await editor.locator(".mchips .mchip.auto").count(), 24);
        sent = await save();
        assert.equal(sent.freeOnly, false, "Vendor's list sends it off");
        assert.equal(sent.noModels, false);
        assert.deepEqual(errors, []);
      });
    }
  }
}
