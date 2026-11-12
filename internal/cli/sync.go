package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/golie/fetch"
	"github.com/meop/golie/journal"
	"github.com/meop/golie/layout"
	"github.com/meop/golie/manifest"
	"github.com/meop/golie/tidy"

	"github.com/meop/goup/internal/config"
	"github.com/meop/goup/internal/godl"
	"github.com/meop/goup/internal/store"
)

// reinstall and force are the two flags install and update share, and they are
// separate because they are different things to ask for: --reinstall redoes
// goup's own work and touches nothing that is not goup's, while --force is the
// only way a shim replaces a file goup did not stamp. Neither implies the
// other.
var reinstall, force bool

func addInstallFlags(cmd *cobra.Command, reinstallUsage string) {
	cmd.Flags().BoolVarP(&reinstall, "reinstall", "r", false, reinstallUsage)
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Replace a file in the bin directory that is not a goup shim")
}

// change is one toolchain a command is about to move: its manifest key, what
// it is pinned to, what the manifest says of it now (the zero value when it
// is being added), and the release it is going to.
type change struct {
	key    string
	pin    string
	prev   config.Entry
	target godl.Release
}

const addLong = "There is one go, run as go and gofmt, and it follows the channel it was installed with.\n" +
	"Adding another channel replaces it.\n\n" +
	"  goup add              follow go's newest stable release\n" +
	"  goup add latest       the same, said aloud\n" +
	"  goup add 1.26         follow the newest 1.26.x\n" +
	"  goup add 1.26.4       stay on exactly 1.26.4\n\n" +
	"A project's go.mod picks its own toolchain on top of this one: that is go's doing (GOTOOLCHAIN)."

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [channel]",
		Short: "Add go, following latest, a line (1.26) or one release (1.26.4)",
		Long:  addLong,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runAdd,
	}
	addInstallFlags(cmd, "Reinstall even if already installed")
	return cmd
}

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync go to the newest release its channel allows",
		Args:  cobra.NoArgs,
		RunE:  runSync,
	}
	addInstallFlags(cmd, "Reinstall even if up to date")
	return cmd
}

// runAdd puts go on the channel asked for, latest when none is. The
// channel is the manifest key ghpm would use ("go", "go@1.26"), and only one
// is kept: once the new one is in, any other is dropped, its shims among
// them unless the new one stamped the same names.
func runAdd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	unlock, err := config.AcquireLock()
	if err != nil {
		return fail("%v", err)
	}
	defer unlock()
	m, err := config.LoadManifest()
	if err != nil {
		return fail("could not load manifest: %v", err)
	}
	arg := config.PinLatest
	if len(args) == 1 {
		arg = args[0]
	}
	pin, err := parsePin(arg)
	if err != nil {
		return fail("%s: %v", arg, err)
	}
	key := keyOf(pin)
	prev, exists := m.Extracts[key]
	if exists && !reinstall {
		print("%s: already installed → %s", key, prev.Version)
		return nil
	}
	var r resolver
	target, err := r.resolve(ctx, pin)
	if err != nil {
		return fail("%s: %v", key, err)
	}

	others := slices.DeleteFunc(m.Keys(), func(k string) bool { return k == key })
	replaces := make([]string, 0, len(others))
	for _, k := range others {
		replaces = append(replaces, fmt.Sprintf("%s %s", k, m.Extracts[k].Version))
	}
	rows := [][]string{{key, godl.Display(target.Version), manifest.Pin(pin), config.Source, strings.Join(replaces, ", ")}}
	colors := []func(string) string{nil, colorfn("new"), nil, nil, colorfn("old")}
	if !gate([]string{"name", "version", "pin", "uri", "replaces"}, rows, colors, "install "+key) {
		return nil
	}
	done, failed := applyAll(ctx, m, []change{{key: key, pin: pin, prev: prev, target: target}})
	if done == 0 {
		return closing(failed)
	}
	if len(others) > 0 {
		failed = append(failed, dropAll(m, others, m.Extracts[key].Bin)...)
	}
	printPass("installed %s → %s", key, godl.Display(target.Version))
	warnPath()
	return closing(failed)
}

// runSync moves what was installed and nothing else: with nothing installed
// it installs none, since which channel to follow is install's choice. A go
// held to one release has nowhere to move, and is said to be.
func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	unlock, err := config.AcquireLock()
	if err != nil {
		return fail("%v", err)
	}
	defer unlock()
	m, err := config.LoadManifest()
	if err != nil {
		return fail("could not load manifest: %v", err)
	}
	if len(m.Extracts) == 0 {
		print(msgNoneInstalled)
		return nil
	}

	var keys []string
	var failed []failedItem
	for _, key := range m.Keys() {
		if e := m.Extracts[key]; e.Pin == manifest.PinFixed {
			print("%s: fixed at %s, skipping", key, e.Version)
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return closing(failed)
	}

	var r resolver
	var changes []change
	for _, key := range keys {
		_, pin, _ := manifest.ParseKey(key)
		prev := m.Extracts[key]
		target, err := r.resolve(ctx, pin)
		if err != nil {
			printFail("%s: %v", key, err)
			failed = append(failed, failedItem{name: key, reason: err.Error()})
			continue
		}
		if godl.Compare(target.Version, prev.Version) > 0 || reinstall {
			changes = append(changes, change{key: key, pin: pin, prev: prev, target: target})
		}
	}
	if len(changes) == 0 {
		if len(failed) == 0 {
			print(msgAllUpToDate)
		}
		return closing(failed)
	}
	var rows [][]string
	for _, c := range changes {
		rows = append(rows, []string{c.key, c.prev.Version, godl.Display(c.target.Version), c.prev.Pin, m.Repos[pkgGo]})
	}
	colors := []func(string) string{nil, colorfn("old"), colorfn("new"), nil, nil}
	if gate([]string{"name", "version", "update", "pin", "uri"}, rows, colors, fmt.Sprintf("update %d toolchain(s)", len(changes))) {
		done, bad := applyAll(ctx, m, changes)
		failed = append(failed, bad...)
		if done > 0 {
			printPass("updated %d toolchain(s)", done)
		}
	}
	return closing(failed)
}

// applyAll makes each change in turn, after making sure mallet is there to
// stamp with. One that fails is said and does not stop the rest.
func applyAll(ctx context.Context, m *config.Manifest, changes []change) (done int, failed []failedItem) {
	l, err := store.Layout()
	if err == nil {
		err = syncWedge(ctx, l)
	}
	if err != nil {
		printFail("wedge: %v", err)
		for _, c := range changes {
			failed = append(failed, failedItem{name: c.key, reason: fmt.Sprintf("wedge: %v", err)})
		}
		return 0, failed
	}
	for _, c := range changes {
		if err := apply(ctx, m, l, c); err != nil {
			printFail("%s: %v", c.key, err)
			failed = append(failed, failedItem{name: c.key, reason: err.Error()})
			continue
		}
		done++
	}
	return done, failed
}

// apply brings one toolchain to its target release, the way ghpm installs a
// package: the archive into the download cache, unpacked into the install's
// own extract dir, a shim per command, and then the manifest entry.
//
// The manifest is the commit point. Everything before it is journalled, and a
// failure rolls the journal back to the release the manifest still names. The
// release the version file left is removed only after the commit, because
// that rollback restores from it.
func apply(ctx context.Context, m *config.Manifest, l layout.Layout, c change) error {
	version := godl.Display(c.target.Version)
	file, err := c.target.Archive()
	if err != nil {
		return err
	}
	downloads, err := l.DownloadDir(config.Source, version)
	if err != nil {
		return err
	}
	archive := filepath.Join(downloads, file.Filename)
	// an archive already in the cache is not fetched again, and says nothing
	if !cached(archive, file.SHA256) {
		print("%s: asset fetching [%s]...", c.key, file.Filename)
		if err := fetch.Download(ctx, file.URL(), archive, file.SHA256); err != nil {
			return err
		}
	}
	s, err := stamper()
	if err != nil {
		return err
	}

	vfile, stage := l.VersionFilePath(c.key), l.StagePath(c.key, version)
	jrn := &journal.Journal{}
	rollback := func(err error) error {
		for _, undo := range jrn.Rollback() {
			printWarn("%s: could not roll back: %v", c.key, undo)
		}
		return err
	}

	if err := unpack(archive, stage); err != nil {
		return err
	}
	tree, undo, err := journal.Place(stage, l.VersionPath(c.key, version))
	if err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	jrn.Did("extract", undo)
	undo, err = journal.Point(vfile, tree)
	if err != nil {
		return rollback(err)
	}
	jrn.Did("version file", undo)
	keys, err := treeBins(tree)
	if err != nil {
		return rollback(err)
	}
	bins := make(map[string]string, len(keys))
	for _, binKey := range keys {
		// one go at a time, so its commands keep their own names whatever
		// channel it follows
		name := filepath.Base(binKey)
		print("%s: bin found [%s]", c.key, binKey)
		jrn.Did("shim "+name, journal.ShimUndo(s, name))
		if err := s.Create(name, filepath.Join(vfile, filepath.FromSlash(binKey)), force); err != nil {
			return rollback(fmt.Errorf("%s: could not create shim: %w", name, err))
		}
		bins[name] = binKey
	}

	m.Repos[pkgGo] = config.Source
	m.Extracts[c.key] = config.Entry{Pin: manifest.Pin(c.pin), Version: version, Assets: []string{file.Filename}, Bin: bins}
	if err := config.SaveManifest(m); err != nil {
		m.Extracts[c.key] = c.prev
		if c.prev.Version == "" {
			m.RemoveExtract(c.key)
		}
		return rollback(fmt.Errorf("could not save manifest: %w", err))
	}

	// after the commit: a shim the new release no longer ships, and the
	// release this one replaced
	for name := range c.prev.Bin {
		if _, ok := bins[name]; !ok {
			if err := s.Remove(name); err != nil {
				printWarn("%s: could not remove stale shim %s: %v", c.key, name, err)
			}
		}
	}
	for _, st := range tidy.PruneInstall(l.ExtractPath(c.key)) {
		printWarn("%s: could not remove %s, left for tidy: %v", c.key, st.Path, st.Err)
	}
	return nil
}

// unpack makes stage the go the archive holds, for journal.Swap.
func unpack(archive, stage string) error {
	if err := os.MkdirAll(filepath.Dir(stage), 0755); err != nil {
		return err
	}
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	// the archive holds one top level "go" directory
	if err := fetch.Extract(archive, stage, 1); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	return nil
}

// warnPath says so when the shims were stamped somewhere PATH does not reach.
func warnPath() {
	l, err := store.Layout()
	if err != nil {
		return
	}
	binDir, err := l.BinDir()
	if err != nil {
		return
	}
	// windows spells the same directory in more than one case
	if slices.ContainsFunc(filepath.SplitList(os.Getenv("PATH")), func(dir string) bool {
		return strings.EqualFold(filepath.Clean(dir), binDir)
	}) {
		return
	}
	printWarn("%s is not on PATH", binDir)
}
