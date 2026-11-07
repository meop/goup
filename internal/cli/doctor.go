package cli

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/meop/golie/doctor"
	"github.com/meop/golie/lock"
	"github.com/meop/golie/pointer"
	"github.com/meop/golie/vendored"

	"github.com/meop/goup/internal/config"
	"github.com/meop/goup/internal/store"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check goup's own state: its lock, wedge, go, PATH and cache",
		Args:  cobra.NoArgs,
		RunE:  runDoctor,
	}
}

// runDoctor reads what is there and changes nothing, so it takes no lock: it
// says whether another goup holds one.
func runDoctor(cmd *cobra.Command, args []string) error {
	l, err := store.Layout()
	if err != nil {
		return fail("%v", err)
	}
	doctor.Start("goup")

	if err := lock.Check(l.LockPath(), "goup"); err != nil {
		doctor.Warn("lock", err.Error())
	} else {
		doctor.Pass("lock", "no concurrent process detected")
	}

	if dir, r, err := wedgeRelease(l); err != nil {
		doctor.Fail(vendored.Label("wedge"), err.Error())
	} else {
		doctor.Vendored("wedge", r, dir, "goup refresh")
	}

	m, err := config.LoadManifest()
	switch {
	case err != nil:
		doctor.Fail("manifest.json", err.Error())
	case len(m.Extracts) == 0:
		doctor.Warn(config.Package, "not installed — `goup install` installs one")
	default:
		for _, key := range m.Keys() {
			e := m.Extracts[key]
			// the version a shim runs is the one its pointer names, which a
			// reinstall puts beside the old copy rather than over it
			path, ok := pointer.Target(l.VersionFilePath(key))
			if !ok {
				doctor.Warn(key, "no version pointer — "+l.VersionFilePath(key))
			} else if _, err := os.Stat(path); err != nil {
				doctor.Warn(key, "extract missing — "+path)
			} else {
				doctor.Pass(key, doctor.Present(e.Version, path))
			}
		}
	}

	if bin, err := l.BinDir(); err == nil {
		if slices.Contains(filepath.SplitList(os.Getenv("PATH")), bin) {
			doctor.Pass("PATH", bin+" is on PATH")
		} else {
			doctor.Fail("PATH", bin+" is not on PATH — go won't be found")
		}
	}

	doctor.Pass("cache", doctor.Size(l.DownloadsDir())+" — "+l.DownloadsDir())
	return nil
}
