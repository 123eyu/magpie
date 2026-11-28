package cli

import (
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/settings"
)

// magpie quota auto-reset wait sets how near its renewal a used-up week
// is waited for instead of spending a reset (Steve055s on X), says it
// alone, turns it off, and refuses a value out of range or not whole
// hours, writing nothing then.
func TestQuotaResetWaitCmd(t *testing.T) {
	groupsHome(t)
	if out, err := runOut(t, "quota", "auto-reset", "wait"); err != nil || !strings.Contains(out, "however soon it renews (the default)") {
		t.Fatalf("unset: %q %v", out, err)
	}
	for _, c := range []struct {
		arg  string
		want int
	}{{"24h", 24}, {"1h", 1}, {"168h", 168}, {"12", 12}, {"48H", 48}} {
		if out, err := runOut(t, "quota", "auto-reset", "wait", c.arg); err != nil || settings.Load().CodexResetWait != c.want {
			t.Fatalf("%s: %q %v, set %d", c.arg, out, err, settings.Load().CodexResetWait)
		}
	}
	if out, err := runOut(t, "quota", "auto-reset", "wait"); err != nil || !strings.Contains(out, "renews within 48h") || strings.Contains(out, "default") {
		t.Fatalf("set: %q %v", out, err)
	}
	for _, bad := range []string{"169h", "0.5h", "30m", "-1h", "soon", "1h30m", "--help"} {
		if bad == "--help" {
			if out, err := runOut(t, "quota", "auto-reset", "wait", bad); err != nil || !strings.Contains(out, "auto-reset wait") {
				t.Fatalf("--help: %q %v", out, err)
			}
		} else if _, err := runOut(t, "quota", "auto-reset", "wait", bad); err == nil {
			t.Fatalf("%s: taken", bad)
		}
		if got := settings.Load().CodexResetWait; got != 48 {
			t.Fatalf("%s: wrote %d", bad, got)
		}
	}
	if out, err := runOut(t, "quota", "auto-reset", "wait", "off"); err != nil || settings.Load().CodexResetWait != 0 || !strings.Contains(out, "default") {
		t.Fatalf("off: %q %v", out, err)
	}
}
