package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meop/golie/manifest"
	"github.com/meop/golie/pointer"

	"github.com/meop/goup/internal/config"
	"github.com/meop/goup/internal/store"
)

// doctor on a home with nothing in it says what is missing, and what puts it
// there, in the lines every tool built on golie prints
func TestDoctorSaysWhatIsMissingAndWhatPutsItThere(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	t.Setenv("PATH", "")
	out := capture(t)

	if err := runDoctor(nil, nil); err != nil {
		t.Fatal(err)
	}
	got := out()
	for _, want := range []string{
		"goup doctor\n",
		"  [PASS] lock — no concurrent process detected\n",
		"  [WARN] vendor - wedge — not present — `goup pull` installs it\n",
		"  [WARN] go — not installed — `goup add` installs one\n",
		"is not on PATH — go won't be found\n",
		"  [PASS] cache — 0 B — ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor printed\n%s\nwith no %q", got, want)
		}
	}
}

// the go doctor finds is the one its version pointer names: a reinstall puts
// the new copy beside the old, as 1.27.3+<n>, rather than over it
func TestDoctorFindsTheGoItsPointerNames(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	out := capture(t)
	l, err := store.Layout()
	if err != nil {
		t.Fatal(err)
	}
	placed := l.VersionPath("go", "1.27.3+1")
	if err := os.MkdirAll(placed, 0755); err != nil {
		t.Fatal(err)
	}
	if err := pointer.Point(l.VersionFilePath("go"), placed); err != nil {
		t.Fatal(err)
	}
	m := &manifest.Manifest{Extracts: map[string]manifest.Entry{"go": {Version: "1.27.3"}}}
	if err := config.SaveManifest(m); err != nil {
		t.Fatal(err)
	}

	if err := runDoctor(nil, nil); err != nil {
		t.Fatal(err)
	}
	if want := "  [PASS] go — present [1.27.3] — " + filepath.Clean(placed) + "\n"; !strings.Contains(out(), want) {
		t.Errorf("doctor did not say %q", want)
	}
}
