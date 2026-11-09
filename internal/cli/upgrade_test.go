package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/meop/goup/internal/store"
)

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// runAs has upgrade take path for the running binary.
func runAs(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("goup"), 0755); err != nil {
		t.Fatal(err)
	}
	orig := self.Path
	self.Path = func() (string, error) { return path, nil }
	t.Cleanup(func() { self.Path = orig })
}

// installedPath is where the install script leaves goup in the test home, the
// one copy upgrade replaces.
func installedPath(t *testing.T) string {
	t.Helper()
	l, err := store.Layout()
	if err != nil {
		t.Fatal(err)
	}
	binDir, err := l.BinDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(binDir, exeName("goup"))
}

// serveReleases stands up goup's release page and points upgrade at it.
func serveReleases(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	orig := self.ReleasesURL
	self.ReleasesURL = srv.URL + "/"
	t.Cleanup(func() { self.ReleasesURL = orig })
}

// the goup its installer placed is asked of the release page as before
func TestUpgradeOfTheInstalledGoup(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	runAs(t, installedPath(t))
	serveReleases(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tag/v"+version, http.StatusFound)
	})
	output := capture(t)

	if err := runUpgrade(newUpgradeCmd(), nil); err != nil {
		t.Fatal(err)
	}
	if got := output(); got != msgGoupUpToDate+"\n" {
		t.Errorf("expected the single up-to-date line, got %q", got)
	}
}

// a goup that runs from anywhere but ~/.goup/bin was put there by something
// else — ghpm, go install, a build — and replacing it in place would leave
// that describing a binary it did not install. It is refused in one line,
// before the release page is asked anything, and whether or not the install
// script's own copy is also on the machine
func TestUpgradeRefusesAGoupItsInstallerDidNotPlace(t *testing.T) {
	for name, alsoInstalled := range map[string]bool{"alone": false, "beside an installed copy": true} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("GOUP_TEST_HOME", t.TempDir())
			if alsoInstalled {
				runAs(t, installedPath(t))
			}
			elsewhere := filepath.Join(t.TempDir(), exeName("goup"))
			runAs(t, elsewhere)
			serveReleases(t, func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("a goup that will not be replaced asked for %s", r.URL.Path)
			})
			output := capture(t)

			if err := runUpgrade(newUpgradeCmd(), nil); err != errSilent {
				t.Fatalf("runUpgrade = %v, want the refusal", err)
			}
			if got, want := strings.TrimSpace(output()), "✗ goup: cannot upgrade itself, running from "+elsewhere+": upgrade it with what installed it"; got != want {
				t.Errorf("expected the single refusal line %q, got:\n%s", want, got)
			}
			if got, _ := os.ReadFile(elsewhere); string(got) != "goup" {
				t.Errorf("a goup the installer did not place was replaced: %q", got)
			}
		})
	}
}
