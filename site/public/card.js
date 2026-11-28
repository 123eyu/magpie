// A contributor's card (cards.js serves the page, its data in #cc-data):
// a black metal card drawn on two canvases, front and back. The front has
// the bird, the person's picture where a chip would be, their number by
// when they were first credited, their name with what magpie calls them
// under it (cards.js titleOf) and since when; the back what they built,
// newest first. The fine lines and the rosette are made from
// the person's id, so no two cards are cut alike. It tilts under a mouse,
// turns over on a click, and saves as one picture of both sides.
(function () {
  const data = document.getElementById("cc-data");
  const card = document.querySelector(".cc-card");
  if (!data || !card) return;
  let got;
  try { got = JSON.parse(data.textContent); } catch (e) { return; }
  const c = got.card, w = got.w;

  // the card in its own units; a canvas is these times its scale
  const W = 1000, H = 631, R = 38, M = 64;
  const SANS = '-apple-system, BlinkMacSystemFont, "SF Pro Display", "Inter", "Segoe UI", "PingFang SC", "Hiragino Sans", "Microsoft YaHei", "Noto Sans CJK SC", system-ui, sans-serif';
  const MONO = 'ui-monospace, "SF Mono", "JetBrains Mono", Menlo, Consolas, monospace';
  const PLAT = "#dedee0", DIM = "#8e8e93", FAINT = "#5f5f64";
  const no = String(c.no).padStart(4, "0");

  function seeded(s) {
    let h = 1779033703 ^ s.length;
    for (let i = 0; i < s.length; i++) { h = Math.imul(h ^ s.charCodeAt(i), 3432918353); h = (h << 13) | (h >>> 19); }
    return function () {
      h = Math.imul(h ^ (h >>> 16), 2246822507); h = Math.imul(h ^ (h >>> 13), 3266489909);
      return ((h ^= h >>> 16) >>> 0) / 4294967296;
    };
  }

  function round(x, px, py, pw, ph, r) {
    x.beginPath();
    x.moveTo(px + r, py);
    x.arcTo(px + pw, py, px + pw, py + ph, r);
    x.arcTo(px + pw, py + ph, px, py + ph, r);
    x.arcTo(px, py + ph, px, py, r);
    x.arcTo(px, py, px + pw, py, r);
    x.closePath();
  }

  // black metal: a deep, even body, brushed finely across, a little darker
  // towards the edge
  function body(x, rand) {
    round(x, 0, 0, W, H, R);
    x.clip();
    x.fillStyle = "#0d0d0f";
    x.fillRect(0, 0, W, H);
    for (let y = 0; y < H; y += 1) {
      const a = rand() * 0.02;
      x.fillStyle = rand() < 0.5 ? `rgba(255,255,255,${a})` : `rgba(0,0,0,${a * 3})`;
      x.fillRect(0, y, W, 1);
    }
    const v = x.createRadialGradient(W * 0.42, H * 0.3, H * 0.2, W * 0.5, H * 0.5, W * 0.72);
    v.addColorStop(0, "rgba(0,0,0,0)");
    v.addColorStop(1, "rgba(0,0,0,0.38)");
    x.fillStyle = v;
    x.fillRect(0, 0, W, H);
    // a hairline just inside the edge, as a machined bevel catches light
    round(x, 1.5, 1.5, W - 3, H - 3, R - 1.5);
    x.strokeStyle = "rgba(255,255,255,0.07)";
    x.lineWidth = 1.5;
    x.stroke();
  }

  // a guilloché rosette, its petals and turns the person's own; cut so
  // finely it shows only as the light moves over it
  function rosette(x, rand, cx, cy, r0) {
    const petals = 7 + Math.floor(rand() * 7), turn = rand() * Math.PI * 2, rings = 30;
    const depth = 0.07 + rand() * 0.05;
    x.lineWidth = 0.8;
    for (let k = 0; k < rings; k++) {
      const r = r0 * (0.3 + 0.7 * (k / (rings - 1))), ph = turn + k * (0.16 + rand() * 0.02);
      x.beginPath();
      for (let i = 0; i <= 360; i++) {
        const t = (i / 360) * Math.PI * 2;
        const rr = r * (1 + depth * Math.sin(petals * t + ph));
        const px = cx + rr * Math.cos(t), py = cy + rr * Math.sin(t);
        if (i) x.lineTo(px, py); else x.moveTo(px, py);
      }
      x.strokeStyle = `rgba(255,255,255,${0.016 + 0.012 * (k % 4 === 0)})`;
      x.stroke();
    }
  }

  // letters set apart, as a card's small capitals are; text with no
  // spaces between words (Chinese, Japanese) is set as written
  const caps = (s) => (/[\u3000-\u9fff\uff00-\uffef]/.test(s) ? s : s.toUpperCase());
  function spaced(x, s, px, py, gap, align) {
    if (/[　-鿿＀-￯]/.test(s)) gap = 0;
    const ws = Array.from(s, (ch) => x.measureText(ch).width);
    const all = ws.reduce((a, b) => a + b, 0) + gap * (ws.length - 1);
    let at = align === "right" ? px - all : align === "center" ? px - all / 2 : px;
    const was = x.textAlign;
    x.textAlign = "left";
    Array.from(s).forEach((ch, i) => { x.fillText(ch, at, py); at += ws[i] + gap; });
    x.textAlign = was;
    return all;
  }

  // pressed into the metal: a shadow under, a catch of light over
  function emboss(x, draw, ink) {
    x.save();
    x.fillStyle = "rgba(0,0,0,0.75)"; x.translate(0, 2.2); draw(); x.translate(0, -2.2);
    x.fillStyle = "rgba(255,255,255,0.22)"; x.translate(0, -1.1); draw(); x.translate(0, 1.1);
    x.fillStyle = ink; draw();
    x.restore();
  }

  function fit(x, s, max) {
    if (x.measureText(s).width <= max) return s;
    let a = Array.from(s);
    while (a.length > 1 && x.measureText(a.join("") + "…").width > max) a.pop();
    return a.join("").trimEnd() + "…";
  }

  const assets = { bird: null, face: null };

  function bird(x, px, py, size, ink) {
    if (!assets.bird) return;
    const k = 4, o = document.createElement("canvas");
    o.width = o.height = size * k;
    const ox = o.getContext("2d");
    ox.drawImage(assets.bird, 0, 0, o.width, o.height);
    ox.globalCompositeOperation = "source-in";
    ox.fillStyle = ink;
    ox.fillRect(0, 0, o.width, o.height);
    x.drawImage(o, px, py, size, size);
  }

  const since = (() => {
    if (!c.since) return "";
    if (!c.since.t) return c.since.v;
    const d = new Date(c.since.t * 1000);
    return `${c.since.v}  ${String(d.getUTCMonth() + 1).padStart(2, "0")}/${String(d.getUTCFullYear() % 100).padStart(2, "0")}`;
  })();
  const handle = c.github ? `@${c.github}` : c.x ? `@${c.x}` : "";

  // a line set wide, shrunk to fit; written as is when it still won't
  const CJK = /[　-鿿＀-￯]/;
  function span(x, s, gap) {
    if (CJK.test(s)) return x.measureText(s).width;
    const a = Array.from(s);
    return a.reduce((n, ch) => n + x.measureText(ch).width, 0) + gap * (a.length - 1);
  }
  // small type in Chinese or Japanese is set a size up, as it reads smaller
  const sm = (px, s) => (CJK.test(s) ? Math.round(px * 1.3) : px);
  function line(x, s, px, py, gap, max, align) {
    if (span(x, s, gap) <= max) return spaced(x, s, px, py, gap, align);
    const t = fit(x, s, max), was = x.textAlign;
    x.textAlign = align || "left";
    x.fillText(t, px, py);
    x.textAlign = was;
    return x.measureText(t).width;
  }

  function front(x) {
    const rand = seeded(c.id);
    x.save();
    body(x, rand);
    rosette(x, rand, W * 0.86, H * 0.5, H * 0.62);
    x.textBaseline = "alphabetic";
    // the house: a small bird and its name, set wide
    bird(x, M - 2, 62, 38, PLAT);
    x.fillStyle = PLAT;
    x.font = `500 25px ${SANS}`;
    spaced(x, "magpie", M + 46, 92, 2.4);
    x.font = `500 ${sm(12, w.contributor)}px ${SANS}`;
    x.fillStyle = DIM;
    spaced(x, caps(w.contributor), W - M, 90, 5, "right");
    // where a chip would be: the person, in grey
    const ax = M, ay = 176, as = 86, ar = 15;
    x.save();
    round(x, ax, ay, as, as, ar);
    x.clip();
    x.fillStyle = "#1a1a1d";
    x.fillRect(ax, ay, as, as);
    if (assets.face) x.drawImage(assets.face, ax, ay, as, as);
    else {
      x.fillStyle = DIM;
      x.font = `500 36px ${SANS}`;
      x.textAlign = "center";
      x.fillText(Array.from(c.name)[0].toUpperCase(), ax + as / 2, ay + as / 2 + 13);
      x.textAlign = "left";
    }
    x.restore();
    round(x, ax + 0.5, ay + 0.5, as - 1, as - 1, ar);
    x.strokeStyle = "rgba(255,255,255,0.16)";
    x.lineWidth = 1;
    x.stroke();
    // the number, pressed in lightly
    x.font = `400 40px ${MONO}`;
    emboss(x, () => spaced(x, "No.", M, 372, 1), "#7c7c82");
    emboss(x, () => spaced(x, no, M + 96, 372, 14), "#c9c9cd");
    // the name, in a card's capitals, and what magpie calls them under it
    let size = 36;
    const name = caps(c.name);
    x.font = `500 ${size}px ${SANS}`;
    while (size > 22 && span(x, name, 2.2) > W - 2 * M) { size -= 2; x.font = `500 ${size}px ${SANS}`; }
    emboss(x, () => line(x, name, M, 456, 2.2, W - 2 * M), PLAT);
    if (w.role) {
      x.font = `400 ${sm(15, w.role)}px ${SANS}`;
      x.fillStyle = "#a4a4a9";
      line(x, caps(w.role), M, 492, 3, W - 2 * M);
    }
    // since, how many, who
    const label = (s, px, align) => { x.font = `500 ${sm(11, s)}px ${SANS}`; x.fillStyle = "#6e6e74"; spaced(x, caps(s), px, 548, 3, align); };
    const value = (s, px, align) => { x.font = `400 18px ${MONO}`; x.fillStyle = "#b4b4b9"; x.textAlign = align || "left"; x.fillText(s, px, 576); x.textAlign = "left"; };
    let at = M;
    if (since) {
      label(w.since, at);
      value(since, at);
      x.font = `400 18px ${MONO}`;
      at += Math.max(x.measureText(since).width, 110) + 52;
    }
    label(w.releases, at);
    value(String(c.n), at);
    if (handle) {
      x.font = `400 18px ${MONO}`;
      value(fit(x, handle, W - M - at - 150), W - M, "right");
    }
    x.restore();
  }

  function back(x) {
    const rand = seeded(c.id + "/back");
    x.save();
    body(x, rand);
    // the stripe
    x.fillStyle = "#040405";
    x.fillRect(0, 64, W, 86);
    for (let y = 64; y < 150; y += 2) {
      x.fillStyle = `rgba(255,255,255,${rand() * 0.018})`;
      x.fillRect(0, y, W, 1);
    }
    rosette(x, rand, W * 0.9, H * 0.88, H * 0.36);
    x.textBaseline = "alphabetic";
    x.font = `500 ${sm(12, w.built)}px ${SANS}`;
    x.fillStyle = DIM;
    spaced(x, caps(w.built), M, 214, 4);
    const shown = c.lines.slice(0, 4);
    let y = 266;
    for (const l of shown) {
      x.font = `400 15px ${MONO}`;
      x.fillStyle = "#6e6e74";
      x.fillText(l.v, M, y);
      const vw = Math.max(x.measureText(l.v).width + 24, 124);
      x.font = `400 21px ${SANS}`;
      x.fillStyle = "#cfcfd3";
      x.fillText(fit(x, l.t, W - 2 * M - vw), M + vw, y);
      y += 54;
    }
    const more = c.lines.length - shown.length;
    if (more > 0) {
      x.font = `400 15px ${SANS}`;
      x.fillStyle = DIM;
      x.fillText(w.more.replace("{n}", String(more)), M, y - 6);
    }
    x.font = `400 16px ${MONO}`;
    x.fillStyle = "#8a8a8f";
    x.fillText(fit(x, "usemagpie.ai/c/" + c.id, W - M * 2 - 240), M, H - 50);
    x.textAlign = "right";
    x.fillText(`No. ${no}`, W - M, H - 50);
    x.textAlign = "left";
    bird(x, W - M - x.measureText(`No. ${no}`).width - 36, H - 71, 26, "#8a8a8f");
    x.restore();
  }

  const canvases = [card.querySelector(".cc-front canvas"), card.querySelector(".cc-back canvas")];
  function draw() {
    const scale = (card.clientWidth / W) * (window.devicePixelRatio || 1);
    if (!scale) return;
    // half the card's thickness, a little more than a real card's
    // half a real card's thickness: 0.76mm over 85.6mm
    card.style.setProperty("--t", `${(card.clientWidth * 0.0044).toFixed(2)}px`);
    canvases.forEach((cv, i) => {
      cv.width = Math.round(W * scale);
      cv.height = Math.round(H * scale);
      const x = cv.getContext("2d");
      x.setTransform(scale, 0, 0, scale, 0, 0);
      (i ? back : front)(x);
    });
  }

  function load(src, cors) {
    return new Promise((ok) => {
      const im = new Image();
      if (cors) { im.crossOrigin = "anonymous"; im.referrerPolicy = "no-referrer"; }
      im.onload = () => ok(im);
      im.onerror = () => ok(null);
      im.src = src;
    });
  }

  // the picture in grey, as engraved; left in colour if it can't be read
  function grey(im) {
    const s = 256, o = document.createElement("canvas");
    o.width = o.height = s;
    const ox = o.getContext("2d");
    const k = Math.min(im.naturalWidth, im.naturalHeight);
    ox.drawImage(im, (im.naturalWidth - k) / 2, (im.naturalHeight - k) / 2, k, k, 0, 0, s, s);
    try {
      const d = ox.getImageData(0, 0, s, s), p = d.data;
      for (let i = 0; i < p.length; i += 4) {
        const l = 0.2126 * p[i] + 0.7152 * p[i + 1] + 0.0722 * p[i + 2];
        const v = Math.max(0, Math.min(255, (l - 128) * 1.08 + 120));
        p[i] = p[i + 1] = p[i + 2] = v;
      }
      ox.putImageData(d, 0, 0);
    } catch (e) {}
    return o;
  }

  const stage = document.querySelector(".cc-stage");
  stage.hidden = false;
  draw();
  if (window.ResizeObserver) {
    let was = card.clientWidth;
    new ResizeObserver(() => { if (card.clientWidth !== was) { was = card.clientWidth; draw(); } }).observe(card);
  }
  const ready = Promise.all([
    load("/img/magpie-small.svg").then((im) => { assets.bird = im; }),
    c.pic ? load(c.pic, true).then((im) => { if (im) assets.face = grey(im); }) : null,
  ]).then(() => { draw(); card.dataset.ready = "1"; });

  // the link preview's picture (cards.js has headless Chrome load this page
  // with ?og and read window.__og): the front on a quiet light ground,
  // 1200x630 at twice that
  if (/[?&]og=/.test(location.search)) {
    ready.then(() => {
      const OW = 1200, OH = 630, k = 2, s = (OH * 0.8) / H;
      const o = document.createElement("canvas");
      o.width = OW * k;
      o.height = OH * k;
      const x = o.getContext("2d");
      x.setTransform(k, 0, 0, k, 0, 0);
      x.fillStyle = "#ececee";
      x.fillRect(0, 0, OW, OH);
      x.translate((OW - W * s) / 2, (OH - H * s) / 2);
      x.scale(s, s);
      x.save();
      x.shadowColor = "rgba(0, 0, 0, .3)";
      x.shadowBlur = 90;
      x.shadowOffsetY = 44;
      round(x, 0, 0, W, H, R);
      x.fillStyle = "#121214";
      x.fill();
      x.restore();
      front(x);
      window.__og = o.toDataURL("image/jpeg", 0.92);
    });
  }

  // turn over. The turn is played here, not by a CSS transition, so the
  // light can follow it: the card lifts a little to turn, its edge shows,
  // each face darkens as it turns from the light (above right, in front),
  // a glare crosses it where the light glances off, and the shadow under
  // it narrows as it goes edge-on.
  const still = matchMedia("(prefers-reduced-motion: reduce)");
  const mouse = matchMedia("(hover: hover) and (pointer: fine)");
  const flip = card.querySelector(".cc-flip"), ground = card.querySelector(".cc-ground");
  const faces = [card.querySelector(".cc-front"), card.querySelector(".cc-back")];
  const RX = 0, RY = 0, TURN = 560, D = Math.PI / 180;
  const unit = (x, y, z) => { const l = Math.hypot(x, y, z); return [x / l, y / l, z / l]; };
  const L = unit(0.4, -0.5, 0.77), HALF = unit(L[0], L[1], L[2] + 1);
  // cubic-bezier(.77, 0, .175, 1), the turn's ease-in-out
  const ease = (p) => {
    const bz = (t, a, b) => 3 * a * t * (1 - t) * (1 - t) + 3 * b * t * t * (1 - t) + t * t * t;
    let lo = 0, hi = 1, t = p;
    for (let i = 0; i < 24; i++) { t = (lo + hi) / 2; if (bz(t, 0.77, 0.175) < p) lo = t; else hi = t; }
    return bz(t, 0, 1);
  };
  let a = 0, tx = RX, ty = RY, anim = 0;
  function light() {
    const b = (a + ty) * D, x = tx * D;
    // the front's normal under rotateX(x) rotateY(b); the back's is opposite
    const n = [Math.sin(b), -Math.cos(b) * Math.sin(x), Math.cos(b) * Math.cos(x)];
    faces.forEach((f, i) => {
      const m = i ? n.map((v) => -v) : n;
      const d = m[0] * L[0] + m[1] * L[1] + m[2] * L[2];
      const dx = m[0] - HALF[0], dy = m[1] - HALF[1];
      f.style.setProperty("--dim", Math.min(0.62, 0.62 * (1 - Math.max(0, d) / L[2])).toFixed(3));
      // brushed metal: the highlight is a band, long across the turn
      f.style.setProperty("--glare", (0.3 * Math.exp(-(dx * dx) / 0.05 - (dy * dy) / 0.25)).toFixed(3));
      f.style.setProperty("--gx", `${(50 + (i ? 1 : -1) * dx * 260).toFixed(1)}%`);
    });
    // the edge that shows is the one turned toward the screen
    const e = Math.sin(b) > 0 ? [-Math.cos(b), 0, Math.sin(b)] : [Math.cos(b), 0, -Math.sin(b)];
    const k = Math.max(0, e[0] * L[0] + e[2] * L[2]);
    const v = Math.round(30 + k * 120);
    flip.style.setProperty("--edge", `rgb(${v}, ${v}, ${v + 4})`);
    card.classList.toggle("cc-past", Math.cos(b) < 0);
    ground.style.setProperty("--sx", Math.max(0.05, Math.abs(Math.cos(a * D))).toFixed(3));
    ground.style.setProperty("--lift", Math.abs(Math.sin(a * D)).toFixed(3));
  }
  function pose() {
    const s = Math.sin(a * D);
    flip.style.transform = a % 180 ? `translateZ(${(s * card.clientWidth * 0.07).toFixed(2)}px) rotateY(${a.toFixed(2)}deg)` : `rotateY(${a}deg)`;
    light();
  }
  function turn(to) {
    cancelAnimationFrame(anim);
    if (still.matches) { a = to; light(); flip.style.transform = ""; return; }
    const from = a, ms = TURN * Math.max(0.4, Math.abs(to - from) / 180);
    let t0 = 0;
    card.classList.add("cc-turning");
    const step = (now) => {
      if (!t0) t0 = now;
      const p = Math.min(1, (now - t0) / ms);
      a = p < 1 ? from + (to - from) * ease(p) : to;
      pose();
      if (p < 1) anim = requestAnimationFrame(step);
      else card.classList.remove("cc-turning");
    };
    anim = requestAnimationFrame(step);
  }
  card.setAttribute("aria-pressed", "false");
  card.addEventListener("click", () => {
    const on = !card.classList.contains("cc-flipped");
    card.classList.toggle("cc-flipped", on);
    card.setAttribute("aria-pressed", String(on));
    turn(on ? 180 : 0);
  });
  light();

  // tilt under a mouse, the light following it
  stage.addEventListener("pointermove", (e) => {
    if (still.matches || !mouse.matches || e.pointerType !== "mouse") return;
    const r = card.getBoundingClientRect();
    const px = Math.max(-1, Math.min(1, ((e.clientX - r.left) / r.width) * 2 - 1));
    const py = Math.max(-1, Math.min(1, ((e.clientY - r.top) / r.height) * 2 - 1));
    tx = -py * 7;
    ty = px * 9;
    card.style.setProperty("--rx", `${tx.toFixed(2)}deg`);
    card.style.setProperty("--ry", `${ty.toFixed(2)}deg`);
    card.style.setProperty("--mx", `${((px + 1) * 50).toFixed(1)}%`);
    card.style.setProperty("--my", `${((py + 1) * 50).toFixed(1)}%`);
    card.classList.add("cc-lit");
    light();
  });
  stage.addEventListener("pointerleave", () => {
    tx = RX;
    ty = RY;
    card.style.removeProperty("--rx");
    card.style.removeProperty("--ry");
    card.classList.remove("cc-lit");
    light();
  });

  // both sides, one picture, at twice the card's units
  const save = document.querySelector(".cc-save");
  if (save && HTMLCanvasElement.prototype.toBlob) {
    save.hidden = false;
    save.addEventListener("click", async () => {
      await ready;
      const k = 2, gap = 48;
      const o = document.createElement("canvas");
      o.width = W * k;
      o.height = (H * 2 + gap) * k;
      const x = o.getContext("2d");
      x.setTransform(k, 0, 0, k, 0, 0);
      front(x);
      x.translate(0, H + gap);
      back(x);
      o.toBlob((b) => {
        if (!b) return;
        const a = document.createElement("a");
        a.href = URL.createObjectURL(b);
        a.download = `magpie-card-${c.id}.png`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        setTimeout(() => URL.revokeObjectURL(a.href), 4000);
      }, "image/png");
    });
  }

  const copy = document.querySelector(".cc-copy");
  if (copy && navigator.clipboard && navigator.clipboard.writeText) {
    copy.hidden = false;
    const was = copy.textContent;
    let t = 0;
    copy.addEventListener("click", () => {
      navigator.clipboard.writeText(got.link).then(() => {
        copy.textContent = copy.dataset.done;
        clearTimeout(t);
        t = setTimeout(() => { copy.textContent = was; }, 1600);
      }, () => {});
    });
  }
})();
