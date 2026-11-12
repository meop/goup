package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/meop/golie/manifest"

	"github.com/meop/goup/internal/config"
	"github.com/meop/goup/internal/godl"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the installed go and its commands",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := config.LoadManifest()
			if err != nil {
				return fail("could not load manifest: %v", err)
			}
			if len(m.Extracts) == 0 {
				print(msgNoneInstalled)
				return nil
			}
			keys := m.Keys()
			if printNameList(keys) {
				return nil
			}
			var rows [][]string
			for _, key := range keys {
				e := m.Extracts[key]
				rows = entryRows(rows, []string{key, e.Version, e.Pin, m.Repos[pkgGo]}, e)
			}
			colors := []func(string) string{nil, colorfn("info"), nil, nil, nil, nil}
			printTable([]string{"name", "version", "pin", "uri", "type", "target"}, rows, colors)
			return nil
		},
	}
	addNameFormatFlags(cmd)
	return cmd
}

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Say whether a newer release is out for go's channel",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := config.LoadManifest()
			if err != nil {
				return fail("could not load manifest: %v", err)
			}
			if len(m.Extracts) == 0 {
				print(msgNoneInstalled)
				return nil
			}
			matched := m.Keys()
			var r resolver
			var rows [][]string
			var keys []string
			hadErrors := false
			for _, key := range matched {
				e := m.Extracts[key]
				// held to one release, so there is nowhere newer for it to go
				if e.Pin == manifest.PinFixed {
					continue
				}
				_, pin, _ := manifest.ParseKey(key)
				target, err := r.resolve(context.Background(), pin)
				if err != nil {
					printFail("%s: %v", key, err)
					hadErrors = true
					continue
				}
				if godl.Compare(target.Version, e.Version) > 0 {
					rows = entryRows(rows, []string{key, e.Version, godl.Display(target.Version), e.Pin, m.Repos[pkgGo]}, e)
					keys = append(keys, key)
				}
			}
			switch {
			case printNameList(keys):
			case len(rows) == 0:
				if !hadErrors {
					print(msgAllUpToDate)
				}
			default:
				colors := []func(string) string{nil, colorfn("old"), colorfn("new"), nil, nil, nil, nil}
				printTable([]string{"name", "version", "update", "pin", "uri", "type", "target"}, rows, colors)
			}
			if hadErrors {
				return errSilent
			}
			return nil
		},
	}
	addNameFormatFlags(cmd)
	return cmd
}
