// A contributor's card: usemagpie.ai/c/<id>, <id> being their row's id on
// the contributors page (credits.mjs anchors). credits-cli.mjs writes each
// person's card to public/contributors/c/<id>.json at deploy (credits.mjs
// cards); this serves the page around it, in the reader's language
// (?lang=, else Accept-Language), with the card drawn by public/card.js
// and everything it credits listed under it, so the page reads the same
// with script off and to a crawler. A link preview gets the card itself
// (/c/<id>/og.jpg, below), name, number and first lines.

import { esc } from "./groups.js";

const SITE = "https://usemagpie.ai";
const ROUTE = /^\/c\/([^/]+)\/?$/;
const OG_ROUTE = /^\/c\/([^/]+)\/og\.jpg$/;

export const WORDS = {
  en: {
    html: "en", wall: "/docs/contributors",
    contributor: "Contributor", back: "Contributors",
    title: (n, no) => `${n} · magpie contributor No. ${no}`,
    desc: (k, v) => `Credited in ${k === 1 ? "1 release" : `${k} releases`} of magpie${v ? `, since ${v}` : ""}.`,
    built: "What I built for magpie", since: "Since", releases: "Releases", more: (n) => `+ ${n} more`,
    flip: "Turn over", save: "Save image", copy: "Copy link", copied: "Copied", share: "Share on X",
    post: (no, t) => `I'm magpie contributor No. ${no}: ${t}.`,
    gone: "There's no card here. Find yours on the contributors page.",
  },
  zh: {
    html: "zh-CN", wall: "/docs/zh/contributors",
    contributor: "贡献者", back: "贡献者",
    title: (n, no) => `${n} · magpie 第 ${no} 号贡献者`,
    desc: (k, v) => `在 magpie 的 ${k} 个版本中被致谢${v ? `，始于 ${v}` : ""}。`,
    built: "我为 magpie 做的", since: "始于", releases: "版本", more: (n) => `还有 ${n} 项`,
    flip: "翻面", save: "保存图片", copy: "复制链接", copied: "已复制", share: "分享到 X",
    post: (no, t) => `我是 magpie 的第 ${no} 号贡献者：${t}。`,
    gone: "这里没有贡献卡。去贡献者页面找到你自己的吧。",
  },
  ja: {
    html: "ja", wall: "/docs/ja/contributors",
    contributor: "コントリビューター", back: "コントリビューター",
    title: (n, no) => `${n} · magpie コントリビューター No. ${no}`,
    desc: (k, v) => `magpie の ${k} リリースに名前が載っています${v ? `（${v} から）` : ""}。`,
    built: "magpie で作ったもの", since: "Since", releases: "リリース", more: (n) => `ほか ${n} 件`,
    flip: "裏返す", save: "画像を保存", copy: "リンクをコピー", copied: "コピーしました", share: "X で共有",
    post: (no, t) => `magpie のコントリビューター No. ${no}、${t}です。`,
    gone: "このカードはありません。コントリビューターのページで探してください。",
  },
};

export function langOf(req) {
  const url = new URL(req.url);
  const asked = (url.searchParams.get("lang") || "").toLowerCase().slice(0, 2);
  if (WORDS[asked]) return asked;
  for (const part of (req.headers.get("Accept-Language") || "").split(",")) {
    const tag = part.split(";")[0].trim().toLowerCase().slice(0, 2);
    if (WORDS[tag]) return tag;
  }
  return "en";
}

// What the card calls its person (credits.mjs role): a joke that names the
// part of magpie they built, so it travels and still says where their work
// went. Each part has three, the one a person gets fixed by their id; a
// product (Codex, Claude, …) takes the part's name when most of the work
// was for it. Ten lines make it Chief, fifty Legendary.
const T = (en, zh, ja) => ({ en, zh, ja });
export const NAMES = {
  gateway: T(["Gateway Lion Tamer", "Gateway Bouncer", "Gateway Plumber"], ["网关驯兽师", "网关守门大爷", "网关水管工"], ["ゲートウェイの猛獣使い", "ゲートウェイの門番", "ゲートウェイの配管工"]),
  interface: T(["Pixel Masseur", "UI Beautician", "Pixel Nudger"], ["界面像素按摩师", "UI 美容师", "像素强迫症患者"], ["ピクセル整体師", "UI 美容師", "ピクセル微調整職人"]),
  providers: T(["Provider Diplomat", "Minister of Subscriptions", "Account Whisperer"], ["供应商外交官", "订阅外交部长", "账号调解员"], ["プロバイダ外交官", "サブスク外務大臣", "アカウント調停人"]),
  agents: T(["Agent Herder", "Agent Kindergarten Principal", "Agent Wrangler"], ["Agent 牧羊人", "Agent 幼儿园园长", "Agent 驯龙高手"], ["エージェントの羊飼い", "エージェント保育園の園長", "エージェント使い"]),
  library: T(["Skills Librarian", "MCP Hoarder", "Keeper of the Toolbox"], ["技能库图书管理员", "MCP 收藏家", "工具箱仓库管理员"], ["スキル図書館の司書", "MCP コレクター", "道具箱の番人"]),
  usage: T(["Token Accountant", "Usage Actuary", "Quota Scrooge"], ["Token 会计师", "用量精算师", "额度守财奴"], ["トークン会計士", "使用量アクチュアリー", "クォータの番頭"]),
  routing: T(["Model Traffic Cop", "Route Navigator", "Model Matchmaker"], ["模型交通警察", "路由导航员", "模型红娘"], ["モデル交通整理員", "ルート案内人", "モデルの仲人"]),
  sessions: T(["Session Historian", "Chat Log Archaeologist", "Context Keeper"], ["会话史官", "聊天记录考古学家", "上下文记忆大师"], ["セッション史官", "チャットログ考古学者", "コンテキストの番人"]),
  privacy: T(["Privacy Bouncer", "Redaction Artist", "Secret Keeper"], ["隐私保镖", "打码艺术家", "秘密守护者"], ["プライバシーの用心棒", "モザイク職人", "秘密の守り人"]),
  plugins: T(["Plugin Magician", "Plugin Alchemist", "Plugin Tailor"], ["插件魔术师", "插件炼金术士", "插件裁缝"], ["プラグイン手品師", "プラグイン錬金術師", "プラグイン仕立て屋"]),
  platform: T(["Cross-Platform Plumber", "Installer Courier", "OS Peacekeeper"], ["跨平台水管工", "安装包快递员", "系统兼容调解员"], ["クロスプラットフォーム配管工", "インストーラー配達員", "OS の調停役"]),
  quality: T(["Bug Exterminator", "Bug Terminator", "Test Obsessive"], ["捉虫特工", "Bug 终结者", "测试强迫症"], ["バグ駆除班", "バグ・ターミネーター", "テストの鬼"]),
  docs: T(["Docs Bard", "README Poet", "Docs Interpreter"], ["文档吟游诗人", "README 诗人", "文档翻译官"], ["ドキュメント吟遊詩人", "README 詩人", "ドキュメント通訳"]),
  polymath: T(["Swiss Army Knife of magpie", "Full-Stack Magpie", "magpie All-Rounder"], ["magpie 瑞士军刀", "全栈喜鹊", "magpie 六边形战士"], ["magpie の十徳ナイフ", "フルスタックなカササギ", "magpie の六角形戦士"]),
  builder: T(["Nest Builder", "Magpie Nest Architect", "Bricklayer of magpie"], ["筑巢工匠", "喜鹊窝建筑师", "magpie 添砖人"], ["巣づくり職人", "カササギの巣の建築家", "magpie のレンガ職人"]),
};
const OF = { agents: T(["{p} Herder", "{p} Kindergarten Principal", "{p} Wrangler"], ["{p} 牧羊人", "{p} 幼儿园园长", "{p} 驯龙高手"], ["{p} の羊飼い", "{p} 保育園の園長", "{p} 使い"]), providers: T(["{p} Diplomat", "{p} Ambassador", "{p} Whisperer"], ["{p} 外交官", "{p} 驻地大使", "{p} 调解员"], ["{p} 外交官", "{p} 大使", "{p} 調停人"]) };
const RANKS = T(["Chief ", "Legendary "], ["首席", "传奇"], ["首席", "伝説の"]);
export function titleOf(c, lang) {
  const set = (c.focus && OF[c.role]) || NAMES[c.role];
  if (!set) return "";
  const list = set[lang] || set.en;
  let h = 0;
  for (const ch of c.id) h = (Math.imul(h, 31) + ch.codePointAt(0)) >>> 0;
  let t = list[h % list.length].replace("{p}", c.focus || "");
  const k = c.lines.length >= 50 ? 1 : c.lines.length >= 10 ? 0 : -1;
  if (k >= 0) {
    const r = (RANKS[lang] || RANKS.en)[k];
    t = r + (lang !== "en" && /^[A-Za-z]/.test(t) ? " " : "") + t;
  }
  return t;
}

// No. 0042: four digits at least, as a card's number
export const number = (no) => String(no).padStart(4, "0");

const PICS = "https://avatars.githubusercontent.com https://pbs.twimg.com";

function headers(nonce, cache) {
  return {
    "Content-Type": "text/html; charset=utf-8",
    "Cache-Control": cache,
    Vary: "Accept-Language",
    "Content-Security-Policy": `default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self' 'nonce-${nonce}'; img-src 'self' data: blob: ${PICS}; base-uri 'none'; form-action 'none'; frame-ancestors 'none'`,
    "X-Content-Type-Options": "nosniff",
    "Referrer-Policy": "strict-origin-when-cross-origin",
  };
}

const THEME = `(()=>{let m="system";try{m=localStorage.getItem("theme")||"system"}catch{}const q=matchMedia("(prefers-color-scheme: dark)");const a=()=>{const t=m==="system"?(q.matches?"dark":"light"):m;document.documentElement.dataset.theme=t;document.querySelector('meta[name="theme-color"]').content=t==="dark"?"#0b0b0c":"#fafafa"};q.addEventListener("change",()=>m==="system"&&a());a()})();`;

const nonceOf = () => btoa(String.fromCharCode(...crypto.getRandomValues(new Uint8Array(16))));

// a card's fields, each checked: the file is ours, but a page puts nothing
// in it unchecked
export function clean(c) {
  if (!c || typeof c !== "object" || typeof c.id !== "string" || typeof c.name !== "string" || !Number.isInteger(c.no) || c.no < 1) return null;
  const str = (s) => (typeof s === "string" ? s : "");
  const pic = /^https:\/\/(avatars\.githubusercontent\.com|pbs\.twimg\.com)\//.test(str(c.pic)) ? c.pic : "";
  const lines = (Array.isArray(c.lines) ? c.lines : [])
    .filter((l) => l && typeof l.t === "string" && typeof l.v === "string")
    .map((l) => ({ v: l.v, t: l.t, href: /^https:\/\//.test(str(l.href)) ? l.href : "" }));
  const since = c.since && typeof c.since.v === "string" ? { v: c.since.v, ...(Number.isFinite(c.since.t) ? { t: c.since.t } : {}) } : null;
  const role = Object.hasOwn(NAMES, str(c.role)) ? c.role : "builder";
  const focus = OF[role] && /^[A-Za-z][\w.-]{0,19}$/.test(str(c.focus)) ? c.focus : "";
  return { id: c.id, name: c.name, no: c.no, n: Number.isInteger(c.n) ? c.n : 0, github: str(c.github), x: str(c.x), pic, since, lines, role, focus };
}

const idOf = (raw) => {
  try { return decodeURIComponent(raw); } catch { return ""; }
};

// the card's file as written, and the card read from it; null when there's
// none or it can't be read
async function load(env, url, id) {
  if (!id || /[/\\]/.test(id) || id === "." || id === "..") return null;
  try {
    const got = await env.ASSETS.fetch(new Request(new URL(`/contributors/c/${encodeURIComponent(id)}.json`, url)));
    if (!got.ok) return null;
    const text = await got.text();
    const card = clean(JSON.parse(text));
    return card ? { card, text } : null;
  } catch { return null; }
}

// The link preview's picture: the card's front, drawn by card.js itself in
// Cloudflare's headless browser (the BROWSER binding) from the page with
// ?og, kept in R2 (CARDS) under a hash of everything that draws it: the
// card's file, card.js as served (its ETag) and the language. A new
// release or a new card.js is a new picture, and an old one is never drawn
// twice. The page asks for it with that hash in the address, so a site
// that keeps previews by address asks again when it changes.
async function lookOf(env, url) {
  try {
    const got = await env.ASSETS.fetch(new Request(new URL("/card.js", url)));
    return got.headers.get("ETag") || "";
  } catch { return ""; }
}
async function ogKey(env, url, text, lang) {
  const look = await lookOf(env, url);
  const d = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(`${text}\n${look}\n${lang}`));
  return Array.from(new Uint8Array(d), (b) => b.toString(16).padStart(2, "0")).join("");
}

// headless Chrome on Cloudflare loads the card's ?og page, where card.js
// leaves the picture in window.__og (a JPEG data URL)
export async function browserShot(env, at) {
  const { default: puppeteer } = await import("@cloudflare/puppeteer");
  const browser = await puppeteer.launch(env.BROWSER);
  try {
    const tab = await browser.newPage();
    await tab.setViewport({ width: 1000, height: 800, deviceScaleFactor: 1 });
    await tab.goto(at, { waitUntil: "load", timeout: 20000 });
    await tab.waitForFunction("typeof window.__og === 'string'", { timeout: 20000 });
    const got = await tab.evaluate("window.__og");
    const m = /^data:image\/jpeg;base64,(.+)$/.exec(got || "");
    if (!m) throw new Error("card.js gave no picture");
    return Uint8Array.from(atob(m[1]), (ch) => ch.charCodeAt(0));
  } finally {
    await browser.close().catch(() => {});
  }
}

// one drawing per picture at a time in this isolate
const drawing = new Map();
async function ensure(env, url, id, key, lang, shoot) {
  const at = `og/${key}.jpg`;
  const have = await env.CARDS.get(at);
  if (have) return have.body;
  if (!env.BROWSER && shoot === browserShot) throw new Error("no browser");
  if (!drawing.has(at)) {
    drawing.set(at, (async () => {
      try {
        const bytes = await shoot(env, `${url.origin}/c/${encodeURIComponent(id)}?og=1&lang=${lang}`);
        await env.CARDS.put(at, bytes, { httpMetadata: { contentType: "image/jpeg" } });
        return bytes;
      } finally { drawing.delete(at); }
    })());
  }
  return drawing.get(at);
}

// /c/<id>/og.jpg: the picture, or the site's own og.png while it can't be
// drawn, so a preview always shows something
export async function ogImage(req, env, ctx, shoot = browserShot) {
  const url = new URL(req.url);
  const m = url.pathname.match(OG_ROUTE);
  if (!m || (req.method !== "GET" && req.method !== "HEAD")) return null;
  const id = idOf(m[1]).toLowerCase();
  const asked = (url.searchParams.get("lang") || "").toLowerCase();
  const lang = WORDS[asked] ? asked : "en";
  const got = await load(env, url, id);
  if (!got) return new Response("no such card", { status: 404, headers: { "Cache-Control": "public, max-age=60" } });
  let body;
  try {
    body = await ensure(env, url, id, await ogKey(env, url, got.text, lang), lang, shoot);
  } catch (e) {
    console.log(`card picture for ${id} not drawn: ${e && e.message}`);
    return new Response(null, { status: 302, headers: { Location: `${SITE}/img/og.png`, "Cache-Control": "no-store" } });
  }
  return new Response(req.method === "HEAD" ? null : body, {
    headers: { "Content-Type": "image/jpeg", "Cache-Control": "public, max-age=86400", "X-Content-Type-Options": "nosniff" },
  });
}

export async function page(req, env, ctx, shoot = browserShot) {
  const url = new URL(req.url);
  const m = url.pathname.match(ROUTE);
  if (!m || (req.method !== "GET" && req.method !== "HEAD")) return null;
  const id = idOf(m[1]);
  const lang = langOf(req);
  const w = WORDS[lang];
  // ids are lowercase (credits.mjs slug): a link typed in capitals lands
  if (id && id !== id.toLowerCase()) return Response.redirect(`${SITE}/c/${encodeURIComponent(id.toLowerCase())}${url.search}`, 301);
  const got = await load(env, url, id);
  const card = got && got.card;
  const nonce = nonceOf();
  if (!card) {
    const body = `<main class="cc-gone"><p>${esc(w.gone)}</p><p><a href="${w.wall}">${esc(w.back)} →</a></p></main>`;
    return new Response(req.method === "HEAD" ? null : shell({ w, nonce, title: esc(w.back) + " · magpie", desc: esc(w.gone), robots: true, body }), { status: 404, headers: headers(nonce, "public, max-age=60") });
  }
  const link = `${SITE}/c/${encodeURIComponent(card.id)}`;
  const title = w.title(card.name, number(card.no));
  const role = titleOf(card, lang);
  const desc = role + " · " + w.desc(card.n, card.since && card.since.v) + (card.lines.length ? " " + card.lines.slice(0, 2).map((l) => l.t).join(" · ") : "");
  const list = card.lines.map((l) => `<li>${l.href ? `<a href="${esc(l.href)}"><code>${esc(l.v)}</code></a>` : `<code>${esc(l.v)}</code>`}<span>${esc(l.t)}</span></li>`).join("");
  const handle = card.github ? `https://github.com/${encodeURIComponent(card.github)}` : card.x ? `https://x.com/${encodeURIComponent(card.x)}` : "";
  const intent = `https://x.com/intent/post?text=${encodeURIComponent(w.post(number(card.no), role))}&url=${encodeURIComponent(link)}`;
  const body = `<header class="cc-top"><a href="${w.wall}#${encodeURIComponent(card.id)}"><img class="bird" src="/img/magpie-small.svg" alt="" width="22" height="22">${esc(w.back)}</a></header>
<main class="cc">
<div class="cc-stage" hidden><button class="cc-card" type="button" aria-label="${esc(w.flip)}" title="${esc(w.flip)}"><span class="cc-ground" aria-hidden="true"></span><span class="cc-flip"><span class="cc-edge" style="--z:-.75"></span><span class="cc-edge" style="--z:-.5"></span><span class="cc-edge" style="--z:-.25"></span><span class="cc-edge" style="--z:0"></span><span class="cc-edge" style="--z:.25"></span><span class="cc-edge" style="--z:.5"></span><span class="cc-edge" style="--z:.75"></span><span class="cc-face cc-front"><canvas aria-hidden="true"></canvas><span class="cc-shade"></span><span class="cc-glare"></span></span><span class="cc-face cc-back"><canvas aria-hidden="true"></canvas><span class="cc-shade"></span><span class="cc-glare"></span></span></span><span class="cc-sheen" aria-hidden="true"></span></button></div>
<div class="cc-acts"><button class="btn cc-save" type="button" hidden>${esc(w.save)}</button><button class="btn ghost cc-copy" type="button" hidden data-done="${esc(w.copied)}">${esc(w.copy)}</button><a class="btn ghost" href="${esc(intent)}" target="_blank" rel="noopener">${esc(w.share)}</a></div>
<section class="cc-list"><h1>${handle ? `<a href="${esc(handle)}">${esc(card.name)}</a>` : esc(card.name)} <span>No. ${number(card.no)}</span></h1><p class="cc-sub"><strong>${esc(role)}</strong> · ${esc(w.desc(card.n, card.since && card.since.v))}</p><ul>${list}</ul></section>
</main>
<script type="application/json" id="cc-data">${JSON.stringify({ card, link, w: { role, contributor: w.contributor, built: w.built, since: w.since, releases: w.releases, more: w.more(0).replace("0", "{n}"), lang: w.html } }).replace(/</g, "\\u003c")}</script>
<script src="/card.js" defer></script>`;
  // the preview's picture; drawn now, in the background, when it isn't yet,
  // so it is there by the time the link is shared (not for the drawing's
  // own load of this page)
  const key = await ogKey(env, url, got.text, lang);
  if (req.method === "GET" && !url.searchParams.has("og") && env.CARDS && ctx && ctx.waitUntil) {
    ctx.waitUntil(ensure(env, url, card.id, key, lang, shoot).catch((e) => console.log(`card picture for ${card.id} not drawn: ${e && e.message}`)));
  }
  const pic = `${SITE}/c/${encodeURIComponent(card.id)}/og.jpg?lang=${lang}&amp;v=${key.slice(0, 12)}`;
  const og = `<meta property="og:image" content="${pic}">
<meta property="og:image:width" content="2400">
<meta property="og:image:height" content="1260">
<meta property="og:image:alt" content="${esc(title)}">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:image" content="${pic}">`;
  const html = shell({ w, nonce, title: esc(title), desc: esc(desc), url: link, og, body });
  return new Response(req.method === "HEAD" ? null : html, { headers: headers(nonce, "public, max-age=300") });
}

function shell({ w, nonce, title, desc, url, og = "", robots, body }) {
  return `<!doctype html>
<html lang="${w.html}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${title}</title>
<meta name="description" content="${desc}">
<meta name="color-scheme" content="light dark">
<meta name="theme-color" content="#fafafa">
${robots ? '<meta name="robots" content="noindex">\n' : ""}<script nonce="${nonce}">${THEME}</script>
<link rel="icon" href="/favicon.png" type="image/png">
<link rel="stylesheet" href="/docs.css">
<link rel="stylesheet" href="/card.css">
<meta property="og:title" content="${title}">
<meta property="og:description" content="${desc}">
${url ? `<meta property="og:url" content="${url}">\n<link rel="canonical" href="${url}">\n` : ""}${og}
</head>
<body>
${body}
</body>
</html>
`;
}
