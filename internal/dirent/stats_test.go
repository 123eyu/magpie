package dirent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// Stats says of each entry what os.Lstat says: a file's size and time to
// the nanosecond, a folder, a link (to a file, a folder, or nothing) as a
// link, in a folder of a few entries and in one too long for one call.
func TestStatsAsLstat(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	write := func(name string, n int, at time.Time) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(strings.Repeat("x", n)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
	write("a.jsonl", 0, time.Unix(1_700_000_000, 123_456_789))
	write("日本語 session.jsonl", 70_000, time.Unix(1_800_000_000, 1))
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.WriteFile(filepath.Join(elsewhere, "f"), []byte("abc"), 0o644); err != nil {
			t.Fatal(err)
		}
		for name, to := range map[string]string{"tofile": filepath.Join(elsewhere, "f"), "tofolder": elsewhere, "broken": filepath.Join(elsewhere, "gone")} {
			if err := os.Symlink(to, filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	check := func(dir string, want int) {
		got, ok := Stats(dir)
		if !ok {
			if runtime.GOOS == "darwin" {
				t.Fatalf("%s: Stats read nothing on macOS", dir)
			}
			return
		}
		if !slices.IsSortedFunc(got, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) }) {
			t.Errorf("%s: Stats isn't in name order", dir)
		}
		if len(got) != want {
			t.Errorf("%s: Stats has %d entries, want %d", dir, len(got), want)
		}
		es, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range es {
			fi, err := os.Lstat(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			s, ok := got.Get(e.Name())
			if !ok {
				t.Errorf("%q: not in Stats", e.Name())
				continue
			}
			if s.Mode != fi.Mode().Type() || !s.Mod.Equal(fi.ModTime()) || fi.Mode().IsRegular() && s.Size != fi.Size() {
				t.Errorf("%q: Stats says %v %d %v, Lstat %v %d %v", e.Name(), s.Mode, s.Size, s.Mod, fi.Mode().Type(), fi.Size(), fi.ModTime())
			}
		}
	}
	want := 3
	if runtime.GOOS != "windows" {
		want += 3
	}
	check(dir, want)

	// ~3000 long names are more than one call's 64 KB answer
	many := t.TempDir()
	for i := range 3000 {
		if err := os.WriteFile(filepath.Join(many, fmt.Sprintf("rollout-2026-10-27T06-%04d-%s.jsonl", i, strings.Repeat("n", 60))), []byte(strings.Repeat("y", i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check(many, 3000)

	if _, ok := Stats(filepath.Join(dir, "a.jsonl")); ok {
		t.Error("Stats read a file as a folder")
	}
	if _, ok := Stats(filepath.Join(dir, "gone")); ok {
		t.Error("Stats read a folder that isn't there")
	}
}

// A listing given back (Done) is the next one's room, and holds nothing of
// its folder: a smaller folder read into it has its own entries only.
func TestStatsAfterDone(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("listed in one call a folder on macOS only")
	}
	big, small := t.TempDir(), t.TempDir()
	for i := range 40 {
		if err := os.WriteFile(filepath.Join(big, fmt.Sprintf("b%02d", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(small, "s"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		l, ok := Stats(big)
		if !ok || len(l) != 40 {
			t.Fatalf("big folder: %d entries, ok %t", len(l), ok)
		}
		l.Done()
		s, ok := Stats(small)
		if !ok || len(s) != 1 || s[0].Name != "s" || s[0].Size != 3 {
			t.Fatalf("small folder after a big one given back: %+v, ok %t", s, ok)
		}
		if rest := s[1:cap(s)]; slices.ContainsFunc(rest, func(e Entry) bool { return e.Name != "" }) {
			t.Fatalf("a listing given back kept its folder's entries: %+v", rest)
		}
		s.Done()
	}
}
