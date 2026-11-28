// Run with Node's test runner and Playwright on the module path; see README.md.
// Settings › Text size in a browser tab (`magpie web`, WeltAn_X on X). The
// tab's size is this browser's own, kept in localStorage and never posted
// to the app's (which here says 150%, and is left alone); the root is
// zoomed with CSS zoom (webzoom.js), and what the page measures is told in
// its layout's pixels, as in a zoomed window: a menu opens at its anchor,
// the sidebar and the phone's tab bar follow the width the page has, and
// nothing runs off the side. The tab's own keys stay the browser's zoom.
// Every language for the row, English and Chinese for the rest; no backend,
// the API is faked here. ARTIFACT_DIR gets screenshots at 150%.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = process.env.MAGPIE_ASSETS || path.resolve(__dirname, "../assets");

function settingsPayload(over) {
  return {
    theme: "light", lang: "en", tray: "panel", quotaLeft: false, currency: "usd", textSize: 150,
    dock: false, dockWindow: false, proxy: "", redact: false, redactPersonal: false, redactWords: [],
    codexWarmup: "", claudeWarmup: "", codexWarmAt: "", claudeWarmAt: "", workbuddyCheckin: false, noStats: false,
    trayUsage: "", trayUsageEvery: 3, vision: "", imageGen: "",
    version: "0.1.400", dir: "~/.config/magpie", gateway: "http://127.0.0.1:3425",
    proxyNow: "none", proxySource: "none", login: false,
    visionModels: [], imageGenModels: [], workbuddyCheckins: [], lanURLs: [],
    fx: { rate: 7.2, at: new Date().toISOString(), stale: false },
    ...over,
  };
}

// a fake `magpie web` whose app is at 150%: every text size posted is kept
// in posts, and there must be none
function server(lang, theme, posts) {
  const cur = settingsPayload({ lang, theme });
  return async (route) => {
    const req = route.request(), url = new URL(req.url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"${theme}",web:true,textSize:150};` });
    if (url.pathname === "/api/state") return json({ agents: [], profiles: [], settings: { lang, theme, textSize: 150 }, fx: cur.fx });
    if (url.pathname === "/api/settings/text-size") { posts.push(req.postDataJSON()); return json(cur); }
    if (url.pathname === "/api/settings") return json(cur);
    if (url.pathname === "/api/usage") return json({ calls: 0, errors: 0, series: [], agents: [], models: [], bucket: "day" });
    if (url.pathname === "/api/library") return json({ dir: "~/.config/magpie/library", backups: "", home: "~", agents: [], instructions: { agents: [], sets: [] }, servers: [], foundServers: [], skills: [] });
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname === "/api/groups") return json({ groups: [], models: [] });
    if (url.pathname === "/api/providers") return json({ providers: [], presets: [], excluded: [], gateway: { running: true, mine: true, url: "http://127.0.0.1:3999", calls: [], groups: [] } });
    if (url.pathname === "/api/gateway/trace") {
      if (url.searchParams.get("wait")) await new Promise((r) => setTimeout(r, 20e3));
      return json({ mine: true, now: new Date().toISOString(), seq: 0, totals: { requests: 0, rerouted: 0, errors: 0 }, routes: [] });
    }
    if (url.pathname === "/api/gateway/history") return json({ cut: false, days: [], routes: [] });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    try { await route.fulfill({ body: await fs.readFile(file), contentType }); } catch { await route.fulfill({ status: 404 }); }
  };
}

const zoom = (page) => page.evaluate(() => document.documentElement.style.zoom || "1");
const kept = (page) => page.evaluate(() => localStorage.getItem("magpie.textSize"));
const sideNav = (page) => page.evaluate(() => document.body.classList.contains("side-nav"));
// nothing can be scrolled sideways: the page, the view shown, the header
const sideways = (page) => page.evaluate(() => {
  const out = [];
  const se = document.scrollingElement;
  se.scrollLeft = 1e4;
  if (se.scrollLeft > 0) out.push("page " + se.scrollLeft);
  se.scrollLeft = 0;
  const v = [...document.querySelectorAll("main.view")].find((m) => !m.hidden);
  if (v && v.scrollWidth > v.clientWidth) out.push(v.id + " " + v.scrollWidth + ">" + v.clientWidth);
  const top = document.querySelector(".top");
  if (top && top.scrollWidth > top.clientWidth) out.push("header " + top.scrollWidth + ">" + top.clientWidth);
  return out;
});

const LANGS = [["en", "Text size", "in this browser only"], ["zh", "文字大小", "仅限此浏览器"], ["zh-TW", "文字大小", "僅限此瀏覽器"], ["ja", null, "このブラウザのみ"], ["de", null, "nur in diesem Browser"]];

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  test(engine + ": the text size in a browser tab", async (t) => {
    assert(["chromium", "webkit"].includes(engine), "BROWSER must be chromium or webkit");
    const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
    const shots = [];
    t.after(async () => {
      if (process.env.ARTIFACT_DIR) {
        await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
        for (const [name, buf] of shots) await fs.writeFile(path.join(process.env.ARTIFACT_DIR, `${engine}-web-text-size-${name}.png`), buf);
      }
      await browser.close();
    });
    const open = async (lang, posts, { width = 1200, height = 800, theme = "light", size } = {}) => {
      const context = await browser.newContext({ viewport: { width, height }, reducedMotion: "reduce", colorScheme: theme });
      if (size) await context.addInitScript((n) => { if (!sessionStorage.getItem("seeded")) { sessionStorage.setItem("seeded", "1"); localStorage.setItem("magpie.textSize", String(n)); } }, size);
      const page = await context.newPage();
      page.setDefaultTimeout(5000);
      page.setDefaultNavigationTimeout(60000);
      page.errors = [];
      page.on("pageerror", (e) => page.errors.push(e.message));
      await page.route("**/*", server(lang, theme, posts));
      await page.goto("http://magpie.test/");
      return page;
    };
    const settings = async (page) => {
      await page.locator("#prefs").click();
      await page.locator("#textSizeSegs .opt").first().waitFor();
    };

    for (const [lang, name, tip] of LANGS) {
      await t.test(lang + ": the row is there, at this browser's size", async () => {
        const posts = [];
        const page = await open(lang, posts);
        await settings(page);
        assert.equal(await page.locator("#textSizeRow").isVisible(), true);
        if (name) assert.equal((await page.locator("#textSizeRow .name").textContent()).trim(), name);
        assert.deepEqual((await page.locator("#textSizeSegs .opt").allTextContents()).map((s) => s.trim()), ["100%", "110%", "115%", "120%", "125%", "150%"]);
        // the app's 150% is the app's: this browser has picked nothing
        assert.equal(await page.locator("#textSizeSegs .opt.on").textContent(), "100%");
        assert.equal(await zoom(page), "1");
        const title = await page.locator("#textSizeSegs").getAttribute("title");
        assert(title.includes(tip), title);
        assert(!/⌘|Ctrl/.test(title), "a tab's keys are its own zoom's: " + title);
        assert.deepEqual(page.errors, []);
        await page.context().close();
      });
    }

    for (const lang of ["en", "zh"]) {
      await t.test(lang + ": picked, kept in this browser, the keys left to it", async () => {
        const posts = [];
        const page = await open(lang, posts);
        await settings(page);
        await page.locator("#textSizeSegs .opt", { hasText: "125%" }).click();
        await page.locator("#textSizeSegs .opt.on", { hasText: "125%" }).waitFor();
        assert.equal(await zoom(page), "1.25");
        assert.equal(await kept(page), "125");
        // what is placed against the window's drawing stays at 1
        assert.equal(await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--zoom").trim()), "1");
        // kept: a reload is at 125% before app.js runs
        await page.reload();
        assert.equal(await page.evaluate(() => document.documentElement.style.zoom), "1.25");
        await settings(page);
        await page.locator("#textSizeSegs .opt.on", { hasText: "125%" }).waitFor();
        // Ctrl/Cmd + − 0 are the browser's zoom here: magpie takes none of them
        const taken = await page.evaluate(() => ["=", "-", "0"].map((key) => {
          const mac = /^Mac/.test(navigator.platform);
          const e = new KeyboardEvent("keydown", { key, code: key === "=" ? "Equal" : key === "-" ? "Minus" : "Digit0", metaKey: mac, ctrlKey: !mac, bubbles: true, cancelable: true });
          document.body.dispatchEvent(e);
          return e.defaultPrevented;
        }));
        assert.deepEqual(taken, [false, false, false]);
        await page.waitForTimeout(100);
        assert.equal(await kept(page), "125");
        await page.locator("#textSizeSegs .opt", { hasText: "100%" }).click();
        await page.locator("#textSizeSegs .opt.on", { hasText: "100%" }).waitFor();
        assert.equal(await zoom(page), "1");
        assert.equal(await kept(page), "100");
        // the app's own size is never written from a tab
        assert.deepEqual(posts, []);
        assert.deepEqual(page.errors, []);
        await page.context().close();
      });

      await t.test(lang + ": at 150% a menu opens at its anchor, inside the window", async () => {
        for (const width of [1200, 420]) {
          const page = await open(lang, [], { width, height: 800, size: 150 });
          assert.equal(await zoom(page), "1.5");
          await settings(page);
          const pick = page.locator("#langPick");
          await pick.scrollIntoViewIfNeeded();
          await pick.click();
          const menu = page.locator(".proto-menu");
          await menu.waitFor();
          await page.waitForTimeout(250);
          // the browser's own boxes, in the window's pixels
          const a = await pick.boundingBox(), m = await menu.boundingBox();
          assert(m.y >= a.y + a.height - 2 && m.y - (a.y + a.height) < 16 || m.y + m.height <= a.y + 2 && a.y - (m.y + m.height) < 16, `at ${width}px the menu (${JSON.stringify(m)}) is not at its anchor (${JSON.stringify(a)})`);
          if (width > 600) assert(Math.abs(m.x + m.width - (a.x + a.width)) < 16, `at ${width}px the menu's right ${m.x + m.width} is not the anchor's ${a.x + a.width}`);
          assert(m.x >= 0 && m.x + m.width <= width + 0.5, `at ${width}px the menu runs off: ${JSON.stringify(m)}`);
          // a click lands where it is seen: the item under the pointer is the one picked
          const item = menu.locator(".pm-item, [role=menuitemradio], [role=menuitem]").last();
          const b = await item.boundingBox();
          const hit = await page.evaluate(([x, y]) => document.elementFromPoint(x, y)?.closest(".pm-item, [role=menuitemradio], [role=menuitem]")?.textContent, [b.x + b.width / 2, b.y + b.height / 2].map((v, i) => v / 1.5));
          assert.equal(hit, await item.textContent());
          shots.push([`${lang}-${width}-menu`, await page.screenshot()]);
          await page.keyboard.press("Escape");
          assert.deepEqual(page.errors, []);
          await page.context().close();
        }
      });

      await t.test(lang + ": the layout follows the width the page has, as a zoomed window's", async () => {
        // the sidebar is from 1024 CSS pixels: 1200 at 125% is 960
        const page = await open(lang, [], { width: 1200, height: 800 });
        assert.equal(await sideNav(page), true);
        await settings(page);
        await page.locator("#textSizeSegs .opt", { hasText: "125%" }).click();
        await page.waitForFunction(() => !document.body.classList.contains("side-nav"));
        assert.equal(await page.evaluate(() => innerWidth), 960);
        await page.locator("#textSizeSegs .opt", { hasText: "100%" }).click();
        await page.waitForFunction(() => document.body.classList.contains("side-nav"));
        assert.equal(await page.evaluate(() => innerWidth), 1200);
        await page.context().close();
        // and from the start
        const at = await open(lang, [], { width: 1200, height: 800, size: 125 });
        assert.equal(await sideNav(at), false);
        assert.deepEqual(at.errors, []);
        await at.context().close();
      });

      for (const theme of ["light", "dark"]) {
        await t.test(lang + ", " + theme + ": nothing runs off the side at 150%", async () => {
          // the smallest window's 560 CSS pixels at 150%, and a phone's 280
          for (const [width, height] of [[840, 630], [420, 760]]) {
            const page = await open(lang, [], { width, height, theme, size: 150 });
            for (const v of ["agents", "providers", "gateway", "routing", "usage", "library", "settings"]) {
              const b = page.locator(v === "settings" ? "#prefs" : `#nav [data-view="${v}"]`);
              if (!(await b.isVisible())) continue; // past the phone's tab bar
              await b.click();
              await page.waitForTimeout(250);
              assert.deepEqual(await sideways(page), [], `${v} at ${width}px`);
              if (v === "agents" || v === "settings") shots.push([`${lang}-${theme}-${width}-${v}`, await page.screenshot()]);
            }
            assert.deepEqual(page.errors, []);
            await page.context().close();
          }
        });
      }
    }
  });
}
