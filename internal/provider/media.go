package provider

import "strings"

// The formats a provider's images and videos are asked in (MediaAPI).
const (
	// MediaOpenAI is OpenAI's: /images/generations and /images/edits,
	// /videos.
	MediaOpenAI = "openai"
	// MediaArk is Volcengine Ark's (火山方舟): its own /images/generations
	// body, and video at /contents/generations/tasks.
	MediaArk = "ark"
)

// MediaAPIOf is v as MediaAPI keeps it: MediaOpenAI, MediaArk, or "" for
// auto, which anything else is.
func MediaAPIOf(v string) string {
	switch v = strings.ToLower(strings.TrimSpace(v)); v {
	case MediaOpenAI, MediaArk:
		return v
	case "volcengine", "火山方舟":
		return MediaArk
	}
	return ""
}

// MediaBase is the root p's images and videos APIs are under when the
// user gave one apart from its chat URL (MediaURL), else "".
func (p Provider) MediaBase() string {
	return strings.TrimRight(strings.TrimSpace(p.MediaURL), "/")
}
