package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/golie/tidy"

	"github.com/meop/goup/internal/config"
	"github.com/meop/goup/internal/store"
)

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "uninstall",
		Aliases: []string{"un", "unin", "remove", "rm", "r"},
		Short:   "Uninstall go",
		Args:    cobra.NoArgs,
		RunE:    runUninstall,
	}
}

func runUninstall(cmd *cobra.Command, args []string) error {
	unlock, err := config.AcquireLock()
	if err != nil {
		return fail("%v", err)
	}
	defer unlock()
	m, err := config.LoadManifest()
	if err != nil {
		return fail("could not load manifest: %v", err)
	}
	keys := m.Keys()
	if len(keys) == 0 {
		print(msgNoneInstalled)
		return nil
	}
	var rows [][]string
	for _, key := range keys {
		e := m.Extracts[key]
		rows = append(rows, []string{key, e.Version, e.Pin, m.Repos[pkgGo], strings.Join(e.Assets, ", ")})
	}
	colors := []func(string) string{nil, colorfn("info"), nil, nil, nil}
	if !gate([]string{"name", "version", "pin", "uri", "assets"}, rows, colors, "uninstall "+strings.Join(keys, ", ")) {
		return nil
	}
	failed := dropAll(m, keys, nil)
	if len(failed) < len(keys) {
		printPass("uninstalled go")
	}
	return closing(failed)
}

// dropAll deletes each toolchain's shims, then its extract dir, and only then
// forgets it. Nothing here can be rolled back, so that order is the safety: a
// shim that will not delete leaves the manifest describing a toolchain that
// is still fully there, for the retry to finish. A shim named in keep belongs
// to the toolchain that replaced these, and stays.
func dropAll(m *config.Manifest, keys []string, keep map[string]string) []failedItem {
	var failed []failedItem
	l, err := store.Layout()
	if err != nil {
		return append(failed, failedItem{name: strings.Join(keys, ", "), reason: err.Error()})
	}
	s, err := stamper()
	if err != nil {
		return append(failed, failedItem{name: strings.Join(keys, ", "), reason: err.Error()})
	}
	for _, key := range keys {
		e := m.Extracts[key]
		reason := ""
		for name := range e.Bin {
			if _, kept := keep[name]; kept {
				continue
			}
			if err := s.Remove(name); err != nil {
				printFail("%s: %s: could not remove shim: %v", key, name, err)
				if reason == "" {
					reason = fmt.Sprintf("%s: could not remove shim: %v", name, err)
				}
			}
		}
		if reason == "" {
			if err := tidy.PruneExtract(l.ExtractPath(key), l.ExtractsDir()); err != nil && !os.IsNotExist(err) {
				reason = fmt.Sprintf("could not remove extract dir: %v", err)
				printFail("%s: %s", key, reason)
			}
		}
		if reason == "" {
			m.RemoveExtract(key)
			if err := config.SaveManifest(m); err != nil {
				reason = fmt.Sprintf("could not save manifest: %v", err)
				printFail("%s: %s", key, reason)
			}
		}
		if reason != "" {
			failed = append(failed, failedItem{name: key, reason: reason})
		}
	}
	return failed
}
