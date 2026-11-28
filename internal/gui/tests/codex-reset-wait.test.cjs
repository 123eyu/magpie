// Run with Node's test runner and Playwright on the module path; see README.md.
// How near its renewal a used-up week is waited for instead of using a
// reset by itself (Steve055s on X: 重置前24h阻止自动使用重置卡): a pick
// from the app's menu after the days row in Codex's provider editor, for
// every account, Off by default. A pick posts settings/codex-reset-wait.
// An account's card then says the week's reset waits for the week to
// renew, not that it is used, while its used-up week renews within the
// time set and the reset lasts past then; one that runs out by the
// renewal is still auto-used. Five languages, 1100 and 420 wide, both
// engines; the API is faked here.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");
const { watchSettingsReads, settingsDrawn, provPrefsProviders, openProvPrefs, inEditor } = require("./reader.cjs");

const scrolls = (page) => page.evaluate(() => [scrollY, Math.round(document.querySelector("#codexResetWaitRow .name").getBoundingClientRect().top)].join(","));

const assets = path.resolve(__dirname, "../assets");

function server(lang, users, posts, extra) {
  const fixed = {
    theme: "light", lang, tray: "panel", quotaLeft: false, currency: "usd", dock: false, dockWindow: false, proxy: "",
    redact: false, redactPersonal: false, redactWords: [], codexWarmup: "", claudeWarmup: "", codexWarmAt: "", claudeWarmAt: "",
    trayUsage: "", trayUsageEvery: 3, vision: "", imageGen: "", version: "0.1.900", dir: "~/.config/magpie",
    gateway: "http://127.0.0.1:3425", proxyNow: "none", proxySource: "none", visionModels: [], imageGenModels: [], lanURLs: [],
    fx: { rate: 7.2, at: new Date().toISOString(), stale: false },
    codexUsers: users, codexAutoReset: users.slice(0, 1).map((u) => u.toLowerCase()), codexAutoResetOnly: {},
    ...extra,
  };
  let cur = fixed;
  return async (route) => {
    const req = route.request(), url = new URL(req.url());
    const json = (data) => route.fulfill({ json: data });
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:false};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json({ agents: [], profiles: [], settings: { lang, theme: "light" }, fx: fixed.fx });
    if (url.pathname === "/api/settings/codex-reset-wait") {
      const body = req.postDataJSON();
      posts.push(body);
      cur = { ...cur, codexResetWait: body.hours };
      return json(cur);
    }
    if (url.pathname === "/api/settings") {
      if (req.method() === "POST") cur = { ...cur, ...req.postDataJSON() };
      return json(cur);
    }
    if (url.pathname === "/api/providers") return json(provPrefsProviders(["codex"], { codex: "Codex" }));
    if (url.pathname === "/api/plugins") return json({ plugins: [] });
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname === "/api/groups") return json({ groups: [], models: [] });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  };
}

const langs = ["en", "zh", "zh-TW", "ja", "de"];
const users = ["alice@example.com", "a-much-longer-team-seat-address@some-company.example.org"];
const launch = (engine) => engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" });

async function open(t, engine, lang, width, extra, posts) {
  const browser = await launch(engine);
  const page = await (await browser.newContext({ viewport: { width, height: 700 }, reducedMotion: "reduce" })).newPage();
  t.after(async () => {
    if (process.env.ARTIFACT_DIR) {
      await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
      await page.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-${lang}-${width}-codex-reset-wait.png`) });
    }
    await browser.close();
  });
  page.setDefaultTimeout(5000);
  page.setDefaultNavigationTimeout(60000);
  const errors = [];
  page.on("pageerror", (e) => { if (engine === "webkit" && /due to access control checks/.test(e.message)) return; errors.push(e.message); }); // a goto in WebKit can cut off a fetch (#1350)
  await page.route("**/*", server(lang, users, posts, extra));
  await watchSettingsReads(page);
  await openProvPrefs(page, "codex");
  await settingsDrawn(page);
  await page.locator(".editor .prov-prefs #codexResetWaitRow").waitFor();
  return { page, errors };
}

const tr = (page, lang, key) => page.evaluate(([lang, key]) => I18N[lang]?.[key] || key, [lang, key]);
// what the page says a wait of h hours is, in lang
const say = (page, lang, h) => page.evaluate(([lang, h]) => {
  if (!h) return I18N[lang]?.["Off"] || "Off";
  const lead = (I18N[lang]?.["{n} h"] || "{n} h").replace("{n}", h);
  return (I18N[lang]?.["Within {lead}"] || "Within {lead}").replace("{lead}", lead);
}, [lang, h]);

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of langs) {
    for (const width of [1100, 420]) {
      test(`${engine} ${lang} ${width}: Codex's editor picks how near its renewal a used-up week is waited for`, async (t) => {
        const posts = [];
        const { page, errors } = await open(t, engine, lang, width, {}, posts);
        const row = page.locator("#codexResetWaitRow");
        assert.equal(await row.isVisible(), true);
        if (lang !== "en") {
          assert.notEqual((await row.locator(".name").textContent()).trim(), "Wait for a week about to renew", "name translated");
          assert.notEqual((await row.locator(".sub").textContent()).trim().slice(0, 20), "For every account wi", "words translated");
        }
        // after the days, in the Resets under Advanced
        assert.equal(await page.evaluate(() => !!(document.querySelector("#codexResetDaysRow").compareDocumentPosition(document.querySelector("#codexResetWaitRow")) & Node.DOCUMENT_POSITION_FOLLOWING)), true);
        assert.equal(await page.evaluate(() => !!document.querySelector("#codexResetWaitRow").closest("details.more")), true, "folded under Advanced");
        const pick = row.locator("button.reset-wait");
        assert.equal((await pick.locator(".rl-say > .on").textContent()).trim(), await say(page, lang, 0), "unset is Off");

        await inEditor(page, pick);
        const top = await scrolls(page);
        await pick.click();
        const items = page.locator(".proto-menu .pm-item");
        assert.equal(await items.count(), 7, "Off, then 3 h to 72 h");
        assert.equal(await page.locator(".proto-menu .pm-item.on .pm-name").textContent(), await say(page, lang, 0));
        await page.locator(`.proto-menu .pm-item .pm-name:text-is(${JSON.stringify(await say(page, lang, 24))})`).click();
        for (let i = 0; i < 100 && !posts.length; i++) await page.waitForTimeout(20);
        assert.deepEqual(posts, [{ hours: 24 }]);
        await page.waitForFunction((want) => document.querySelector("#codexResetWaitRow button.reset-wait .rl-say > .on")?.textContent.trim() === want, await say(page, lang, 24));
        assert.equal(await scrolls(page), top, "no click moved the page");
        // back to Off
        await pick.click();
        await page.locator(`.proto-menu .pm-item .pm-name:text-is(${JSON.stringify(await say(page, lang, 0))})`).click();
        for (let i = 0; i < 100 && posts.length < 2; i++) await page.waitForTimeout(20);
        assert.deepEqual(posts, [{ hours: 24 }, { hours: 0 }]);
        await page.waitForFunction((want) => document.querySelector("#codexResetWaitRow button.reset-wait .rl-say > .on")?.textContent.trim() === want, await say(page, lang, 0));

        // fits, no sideways scroll, no native select, no left border
        const over = await page.evaluate(() => {
          const view = document.querySelector(".editor .prov-prefs[data-list=codexResetList]").getBoundingClientRect();
          return [...document.querySelectorAll("#codexResetWaitRow, #codexResetWaitRow *")]
            .filter((e) => { const r = e.getBoundingClientRect(); return r.width && (r.right > view.right + 1 || r.left < view.left - 1); })
            .map((e) => e.className + " " + e.textContent.slice(0, 30));
        });
        assert.deepEqual(over, []);
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true, "no sideways scroll");
        assert.equal(await row.evaluate((e) => getComputedStyle(e).borderLeftWidth), "0px");
        assert.equal(await page.locator(".editor select").count(), 0, "no native select");
        assert.deepEqual(errors, []);
      });
    }
  }

  test(`${engine}: a wait set from the CLI, outside the choices, is shown and kept`, async (t) => {
    const { page, errors } = await open(t, engine, "en", 900, { codexResetWait: 36 }, []);
    const pick = page.locator("#codexResetWaitRow button.reset-wait");
    assert.equal((await pick.locator(".rl-say > .on").textContent()).trim(), "Within 36 h");
    await inEditor(page, pick);
    await pick.click();
    assert.equal(await page.locator(".proto-menu .pm-item").count(), 8);
    assert.equal(await page.locator(".proto-menu .pm-item.on .pm-name").textContent(), "Within 36 h");
    assert.deepEqual(errors, []);
  });

  // the card's words for the week's reset follow the wait, as the
  // gateway's spend does (waitsForWeek)
  test(`${engine}: an account's card says the week's reset waits for a week about to renew`, async (t) => {
    const { page, errors } = await open(t, engine, "en", 900, {}, []);
    const words = (wait, untilIn, weekIn) => page.evaluate(([wait, untilIn, weekIn]) => {
      state.settings = { ...prefs, codexAutoReset: ["alice@example.com"], codexAutoResetOnly: { "alice@example.com": "week" }, codexResetWait: wait };
      const until = new Date(Date.now() + untilIn).toISOString();
      const weekEnds = weekIn == null ? undefined : new Date(Date.now() + weekIn).toISOString();
      const q = { provider: "codex", user: "alice@example.com", resets: { count: 1, until, each: [{ until }], weekEnds } };
      const w = resetsWords(q.resets, q);
      return [w.querySelector(".resets-kept")?.textContent.trim(), w.title];
    }, [wait, untilIn, weekIn]);
    const hour = 36e5, day = 24 * hour;
    assert.equal((await words(0, 20 * day, 5 * hour))[0], "· auto-used when the week runs out", "no wait set");
    let [said, title] = await words(24, 20 * day, 5 * hour);
    assert.equal(said, "· waits for the week to renew", "renews in 5 h of 24");
    assert(title.includes("renews soon anyway"), title);
    assert.equal((await words(24, 20 * day, 23 * hour))[0], "· waits for the week to renew", "23 h of 24");
    assert.equal((await words(24, 20 * day, 25 * hour))[0], "· auto-used when the week runs out", "25 h of 24");
    assert.equal((await words(24, 3 * hour, 5 * hour))[0], "· auto-used when the week runs out", "the reset runs out before the week renews");
    assert.equal((await words(24, 20 * day, null))[0], "· auto-used when the week runs out", "the week isn't used up");
    assert.deepEqual(errors, []);
  });

  test(`${engine}: with no Codex account the wait isn't there`, async (t) => {
    const browser = await launch(engine);
    const page = await (await browser.newContext({ viewport: { width: 900, height: 600 }, reducedMotion: "reduce" })).newPage();
    t.after(() => browser.close());
    page.setDefaultTimeout(5000);
    page.setDefaultNavigationTimeout(60000);
    await page.route("**/*", server("en", [], [], {}));
    await watchSettingsReads(page);
    await openProvPrefs(page, "codex");
    await settingsDrawn(page);
    await page.locator(".editor .prov-prefs #warmSegs .opt").first().waitFor();
    assert.equal(await page.locator("#codexResetWaitRow").isVisible(), false);
  });
}
