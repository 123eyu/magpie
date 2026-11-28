// Settings' Text size in a browser tab (`magpie web`, WeltAn_X on X). The
// app's windows zoom natively (textsize.go); a page can't set its tab's
// zoom, so here the root is zoomed with CSS `zoom`, the size kept in this
// browser (magpie.textSize), never the app's. CSS zoom leaves the page two
// sizes of pixel: what it lays out in (styles, offsets, scrollTop) and what
// it measures in (rects, a pointer's place, innerWidth, media queries, vw
// and vh), which is the larger by the zoom. A menu put at an anchor's rect,
// or a drag following the pointer, landed that far off. What measures is
// told the layout's pixels here, as a zoomed window tells them, so the rest
// of the page is the same at every size. Loaded before app.js, so every
// measure and media list made is told; the app's windows skip it.
(() => {
  if (!window.bootPrefs?.web) return;
  let z = 1, stamp = 0, unitsDone = false;
  const de = document.documentElement;
  const rect = (r) => (z === 1 ? r : new DOMRect(r.x / z, r.y / z, r.width / z, r.height / z));
  // rects and a point read in the layout's pixels
  for (const P of [Element.prototype, Range.prototype]) {
    const box = P.getBoundingClientRect, all = P.getClientRects;
    P.getBoundingClientRect = function () { return rect(box.call(this)); };
    P.getClientRects = function () { const l = all.call(this); return z === 1 ? l : Array.from(l, rect); };
  }
  const coords = (P, names) => {
    for (const k of names) {
      const d = Object.getOwnPropertyDescriptor(P, k);
      if (d?.get) Object.defineProperty(P, k, { ...d, get() { const v = d.get.call(this); return z === 1 ? v : v / z; } });
    }
  };
  coords(MouseEvent.prototype, ["clientX", "clientY", "pageX", "pageY", "x", "y", "offsetX", "offsetY", "movementX", "movementY"]);
  if (window.Touch) coords(Touch.prototype, ["clientX", "clientY", "pageX", "pageY"]);
  for (const k of ["innerWidth", "innerHeight"]) {
    const d = Object.getOwnPropertyDescriptor(window, k) || Object.getOwnPropertyDescriptor(Window.prototype, k);
    if (d?.get) Object.defineProperty(window, k, { configurable: true, enumerable: true, get() { return d.get.call(window) / z; } });
  }
  // and a canvas drawn as sharp as at 100%, as a zoomed window's is
  const dpr = Object.getOwnPropertyDescriptor(window, "devicePixelRatio") || Object.getOwnPropertyDescriptor(Window.prototype, "devicePixelRatio");
  if (dpr?.get) Object.defineProperty(window, "devicePixelRatio", { configurable: true, enumerable: true, get() { return dpr.get.call(window) * z; } });
  for (const k of ["elementFromPoint", "elementsFromPoint"]) {
    const f = Document.prototype[k];
    Document.prototype[k] = function (x, y) { return f.call(this, x * z, y * z); };
  }
  // a width or height in a query is the layout's: the viewport's over z
  const scaled = (q) => (z === 1 ? q : q.replace(/(\d*\.?\d+)px/g, (_, n) => +(n * z).toFixed(3) + "px"));
  const mm = window.matchMedia.bind(window);
  const lists = new Set();
  // a query list with px follows the zoom: matches is read anew, and a
  // list someone listens to says change when a zoom flips it
  window.matchMedia = (q) => {
    if (!/px/.test(q)) return mm(q);
    const o = new EventTarget();
    let l, at = -1, on, heard = false;
    const now = () => {
      if (at !== stamp) { if (l) l.onchange = null; l = mm(scaled(q)); at = stamp; if (heard) l.onchange = tell; }
      return l.matches;
    };
    const tell = () => {
      if (now() === on) return;
      on = l.matches;
      const e = Object.assign(new Event("change"), { matches: on, media: q });
      o.dispatchEvent(e);
      o.onchange?.(e);
    };
    on = now();
    const add = o.addEventListener.bind(o);
    Object.defineProperties(o, { matches: { get: now }, media: { value: q }, onchange: { value: null, writable: true } });
    // only a list listened to is held (the few made once, at the start);
    // one made to read matches goes when it is dropped
    o.addEventListener = (...a) => { if (!heard) { heard = true; l.onchange = tell; lists.add(tell); } add(...a); };
    o.addListener = (f) => o.addEventListener("change", f);
    o.removeListener = (f) => o.removeEventListener("change", f);
    return o;
  };
  const rules = (list, f) => { for (const r of list) { f(r); if (r.cssRules) rules(r.cssRules, f); } };
  const sheets = (f) => { for (const s of document.styleSheets) { try { rules(s.cssRules, f); } catch { /* another origin's */ } } };
  const was = new WeakMap();
  window.webZoom = (n) => {
    z = Math.min(2, Math.max(1, (n || 100) / 100));
    stamp++;
    de.style.zoom = z === 1 ? "" : z;
    // vw and vh are zoomed too, so each is over z (--wz), done once
    de.style.setProperty("--wz", z);
    if (z !== 1 && !unitsDone) {
      unitsDone = true;
      sheets((r) => {
        const st = r.style;
        if (!st) return;
        for (const p of [...st]) {
          const v = st.getPropertyValue(p);
          if (/\d(?:[dsl]?v[wh]|vmin|vmax)\b/.test(v) && !v.includes("--wz"))
            st.setProperty(p, v.replace(/(\d*\.?\d+(?:[dsl]?v[wh]|vmin|vmax))\b/g, "calc($1 / var(--wz, 1))"), st.getPropertyPriority(p));
        }
      });
    }
    sheets((r) => {
      if (!r.media) return;
      const t = was.get(r) ?? r.media.mediaText;
      if (!/px/.test(t)) return;
      was.set(r, t);
      r.media.mediaText = scaled(t);
    });
    for (const f of lists) f();
    // as a zoomed window's: whatever fits the page to its width runs again
    dispatchEvent(new Event("resize"));
  };
  let n = 100;
  try { n = JSON.parse(localStorage.getItem("magpie.textSize")) || 100; } catch { /* the default */ }
  if (Number.isInteger(n) && n > 100 && n <= 200) webZoom(n);
})();
