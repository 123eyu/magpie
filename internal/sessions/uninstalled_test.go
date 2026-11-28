package sessions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yetone/magpie/internal/proc"
)

// _SekiBetu_ on X: OpenCode uninstalled, its sessions stayed listed with a
// Resume and no Delete. With its command gone Resume is not offered, Delete
// takes the row off the list (its database is not touched), and Restore
// lists it again.
func TestUninstalledAgentSessionsAreForgotten(t *testing.T) {
	inZone(t, 0)
	data, _ := setupMore(t)
	ocMakeDB(t, filepath.Join(data, "opencode", "opencode.db"))
	t.Cleanup(ForceCLIGone())

	row := func() Managed {
		t.Helper()
		for _, m := range ListAgent("opencode") {
			if m.ID == ocMain {
				return m
			}
		}
		t.Fatal("no OpenCode session")
		return Managed{}
	}
	if m := row(); m.Resume == "" || m.Deletable {
		t.Fatalf("installed: %+v", m)
	}
	if _, err := Forget("opencode", ocMain); err == nil {
		t.Fatal("forgot a session of an installed agent")
	}

	defer ForceCLIGone("opencode")()
	if oc := find(t, List(0), "opencode", ocMain); oc.Resume != "" {
		t.Fatalf("resume offered with opencode gone: %q", oc.Resume)
	}
	if m := row(); m.Resume != "" || !m.Deletable || !m.Forget {
		t.Fatalf("uninstalled: resume %q deletable %v forget %v", m.Resume, m.Deletable, m.Forget)
	}
	m := row()
	tr, err := Forget("opencode", ocMain)
	if err != nil {
		t.Fatal(err)
	}
	if !tr.Forgot || !GatewayHidden()("opencode", ocMain, m.Last) {
		t.Fatalf("not hidden after Forget: %+v", tr)
	}
	if got := Trash(); len(got) != 1 || !got[0].Forgot {
		t.Fatalf("trash %+v", got)
	}
	// the Usage page's list (ForgottenOut) leaves it out, and only while the
	// command is gone
	has := func() bool {
		for _, x := range ForgottenOut(List(0)) {
			if x.Agent == "opencode" && x.ID == ocMain {
				return true
			}
		}
		return false
	}
	if has() {
		t.Fatal("a forgotten session is still in the Usage list")
	}
	if _, err := Restore(tr.Key); err != nil {
		t.Fatal(err)
	}
	if GatewayHidden()("opencode", ocMain, m.Last) {
		t.Fatal("still hidden after Restore")
	}
	if !has() {
		t.Fatal("a restored session is not in the Usage list")
	}
}

// eachgo on Discord: Pi's rows on the Sessions page lost Resume. Pi was
// installed, in a folder a user's tools go in (a Node manager's, bun's)
// that magpie's own PATH didn't reach, so the Agents page found it and the
// sessions took its command for gone. Found as the Agents page finds it,
// Pi keeps its Resume; with no pi anywhere it has none.
func TestCLIFoundOffPath(t *testing.T) {
	inZone(t, 0)
	setupMore(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	for _, v := range []string{"BUN_INSTALL", "NVM_DIR", "FNM_DIR", "MISE_DATA_DIR"} {
		t.Setenv(v, "")
	}
	t.Setenv("PATH", t.TempDir())
	proc.Forget()
	t.Cleanup(proc.Forget)
	old := cliGone
	cliGone = cliMissing
	t.Cleanup(func() { cliGone = old })

	if !cliMissing("pi") {
		t.Fatal("pi found with none installed")
	}
	if r := resumeFor("", "pi", piFirst, "/work/pi", ""); r != "" {
		t.Fatalf("resume offered with no pi: %q", r)
	}

	bin, name := filepath.Join(home, ".bun", "bin"), "pi"
	if runtime.GOOS == "windows" {
		name = "pi.cmd"
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	proc.Forget()
	if cliMissing("pi") {
		t.Fatal("pi in ~/.bun/bin, off PATH, taken for uninstalled")
	}
	if p := find(t, List(0), "pi", piFirst); p.Resume == "" {
		t.Fatal("Pi's session has no Resume with pi installed off PATH")
	}
	for _, m := range ListAgent("pi") {
		if m.ID == piFirst && m.Resume == "" {
			t.Fatal("the Sessions page's Pi row has no Resume with pi installed off PATH")
		}
	}
}
