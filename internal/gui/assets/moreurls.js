// A custom provider's More endpoints, under its other protocols' URLs: the
// Models URL, and the Image and video URL with its format (chenxiang on
// Discord: a relay serving chat at /v1 in OpenAI's format and images and
// videos at /v3 in Volcengine Ark's) — the root its images and videos APIs
// are under, and the format they take: Auto (Ark on ark.*.volces.com only,
// as before), OpenAI, or Volcengine Ark. Empty and Auto change nothing.
// app.js imports this as the fold is drawn, so it weighs nothing on the
// core. The media pair goes into the draft only once changed: a save that
// leaves it out keeps what was saved.
window.moreURLFields = function (p) {
  const mu = input(draft.modelsURL, "https://…/v1/models", "url");
  mu.oninput = () => { draft.modelsURL = mu.value; };
  const u = input(draft.mediaURL ?? p?.mediaURL ?? "", "https://…/v3", "url");
  u.classList.add("media-url");
  u.oninput = () => { draft.mediaURL = u.value.trim(); };
  const api = segs([["", t("Auto")], ["openai", "OpenAI"], ["ark", t("Volcengine Ark")]], draft.mediaAPI ?? p?.mediaAPI ?? "", (v) => { draft.mediaAPI = v; });
  api.classList.add("media-api");
  api.setAttribute("role", "group");
  api.setAttribute("aria-label", t("Image and video format"));
  const pair = el("div", "pair media-ask");
  pair.append(api, u);
  return [
    ...field(t("Models URL"), mu, t("Where the vendor lists its models, when that isn't under the base URL; asked with the key")),
    ...field(t("Image and video URL"), pair, t("Where the vendor serves images and videos, if not under the base URL, and the format it takes")),
  ];
};
