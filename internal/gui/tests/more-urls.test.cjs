// Run with Node's test runner and Playwright on the module path; see README.md.
// A custom provider's Image and video URL under More endpoints (chenxiang on
// Discord: a relay with chat at /v1 in OpenAI's format and images and
// videos at /v3 in Volcengine Ark's): the URL with its format beside it,
// Auto, OpenAI or Volcengine Ark, on the app's segmented control. A
// provider with neither opens on Auto and an empty URL; one saved with them
// opens on them, and a Save that changed neither leaves them out, which the
// server keeps; what is typed and picked is saved. The Models URL comes
// with them, from the same file (moreurls.js).
// Every language, Chromium and WebKit, wide and at a phone's width.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(process.env.MAGPIE_ASSETS || path.resolve(__dirname, "../assets"));
const relay = {
  icon: "generic", catalog: "", agents: [], fallback: [], headers: {}, keyList: [], balanceToken: { takes: false, set: false }, proxy: "", ready: true, key: { set: true, masked: "sk-…one" },
  id: "relay", name: "Relay", chat: "https://relay.example.com/v1", responses: "", anthropic: "", gemini: "",
  models: [{ id: "doubao-seedream-4-5-251128", name: "doubao-seedream-4-5-251128", on: true }], chosen: ["doubao-seedream-4-5-251128"],
};

function serve(lang, provider, posts) {
  const providers = { providers: [provider], presets: [], excluded: [], gateway: { running: true, window: true } };
  const state = { agents: [], profiles: [], settings: { lang, theme: "light" } };
  return async (route) => {
    const url = new URL(route.request().url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:true};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json(state);
    if (url.pathname === "/api/plugins") return json({ plugins: [] });
    if (url.pathname === "/api/providers") return json(providers);
    if (url.pathname === "/api/groups") return json({ groups: [] });
    if (url.pathname === "/api/provider/save") { posts.push(route.request().postDataJSON()); return json(providers); }
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    try { await route.fulfill({ body: await fs.readFile(file), contentType }); } catch { await route.fulfill({ status: 404, body: "" }); }
  };
}

const say = (page, lang, s) => page.evaluate(([l, s]) => (l === "en" ? s : I18N[l][s]), [lang, s]);

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of ["en", "zh", "zh-TW", "ja", "de"]) {
    for (const width of [1200, 420]) {
      for (const saved of [false, true]) {
        test(`${engine} ${lang} ${width}px ${saved ? "saved Ark" : "unset"}: a custom provider's Image and video URL`, async (t) => {
          const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
          t.after(() => browser.close());
          const page = await (await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: "reduce" })).newPage();
          page.setDefaultTimeout(5000);
          page.setDefaultNavigationTimeout(60000);
          const errors = [], posts = [];
          page.on("pageerror", (e) => errors.push(e.message));
          const p = saved ? { ...relay, mediaURL: "https://relay.example.com/v3", mediaAPI: "ark" } : relay;
          await page.route("**/*", serve(lang, p, posts));
          await page.goto("http://magpie.test/?view=providers");
          await page.locator(".row.provider", { hasText: "Relay" }).first().click();
          const editor = page.locator("#modal:not([hidden]) .editor");
          const words = {};
          for (const s of ["Models URL", "Image and video URL", "Image and video format", "Auto", "Volcengine Ark", "More endpoints", "Where the vendor serves images and videos, if not under the base URL, and the format it takes"]) {
            words[s] = await say(page, lang, s);
            assert.ok(words[s], `${lang} has "${s}"`);
          }
          const more = editor.locator("details.more", { has: page.locator(":scope > summary", { hasText: words["More endpoints"] }) });
          await more.locator(":scope > summary").click();
          const url = more.getByLabel(words["Image and video URL"], { exact: true });
          await url.waitFor();
          assert.equal(await url.count(), 1, "one Image and video URL field");
          const models = more.getByLabel(words["Models URL"], { exact: true });
          assert.equal(await models.count(), 1, "the Models URL is there too");
          assert.ok(await models.evaluate((m, u) => !!(m.compareDocumentPosition(u) & Node.DOCUMENT_POSITION_FOLLOWING), await url.elementHandle()), "the Models URL comes first");
          assert.equal(await editor.getByText(words["Where the vendor serves images and videos, if not under the base URL, and the format it takes"], { exact: true }).count(), 1, "its hint");
          const fmt = more.getByRole("group", { name: words["Image and video format"], exact: true });
          assert.equal(await fmt.count(), 1, "the format, named");
          assert.equal(await fmt.locator("select").count(), 0);
          const pressed = () => fmt.locator(".opt[aria-pressed=true]").textContent();
          assert.equal(await pressed(), saved ? words["Volcengine Ark"] : words.Auto, "opens on the saved format");
          assert.equal(await url.inputValue(), saved ? "https://relay.example.com/v3" : "");

          const layout = await url.locator("xpath=..").evaluate((box) => {
            const b = box.getBoundingClientRect();
            const inside = (e) => { const r = e.getBoundingClientRect(); return r.left >= b.left - 1 && r.right <= b.right + 1 && r.width > 0; };
            return { segs: inside(box.querySelector(".segs")), url: inside(box.querySelector("input")) && box.querySelector("input").getBoundingClientRect().width >= 120, page: document.documentElement.scrollWidth <= innerWidth + 1 };
          });
          assert.deepEqual(layout, { segs: true, url: true, page: true }, "the format and the URL fit in the row");
          if (process.env.ARTIFACT_DIR) {
            await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
            await url.scrollIntoViewIfNeeded();
            await page.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-${lang}-${width}-${saved ? "ark" : "unset"}-more-urls.png`) });
          }

          // unset: type the relay's /v3 and pick Ark, and a Models URL;
          // saved: Save as it is
          if (!saved) {
            await models.fill("https://relay.example.com/v1/models");
            await url.fill("  https://relay.example.com/v3  ");
            await fmt.getByRole("button", { name: words["Volcengine Ark"], exact: true }).click();
            assert.equal(await pressed(), words["Volcengine Ark"]);
          }
          await editor.locator(".bar button.primary").click();
          for (let i = 0; i < 100 && !posts.length; i++) await page.waitForTimeout(20);
          if (saved) assert.deepEqual([posts[0]?.mediaURL, posts[0]?.mediaAPI], [undefined, undefined], "left out, so kept");
          else {
            assert.equal(posts[0]?.mediaURL, "https://relay.example.com/v3", "the URL is saved");
            assert.equal(posts[0]?.mediaAPI, "ark", "the format is saved");
            assert.equal(posts[0]?.modelsURL, "https://relay.example.com/v1/models", "the Models URL is saved");
          }

          // OpenAI picked on the saved one goes as openai, Auto as empty
          if (saved) {
            await page.locator(".row.provider", { hasText: "Relay" }).first().click();
            await more.locator(":scope > summary").click();
            await url.waitFor();
            await fmt.getByRole("button", { name: "OpenAI", exact: true }).click();
            await editor.locator(".bar button.primary").click();
            for (let i = 0; i < 100 && posts.length < 2; i++) await page.waitForTimeout(20);
            assert.equal(posts[1]?.mediaAPI, "openai");
            assert.equal(posts[1]?.mediaURL, undefined, "the URL, unchanged, is left out");
          }
          assert.deepEqual(errors, []);
        });
      }
    }
  }
}
