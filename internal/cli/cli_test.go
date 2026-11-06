package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/meop/golie/ui"

	"github.com/meop/goup/internal/config"
	"github.com/meop/goup/internal/godl"
)

// a pin is ghpm's with the package left off, and the key list prints is
// taken too
func TestParsePin(t *testing.T) {
	for arg, want := range map[string]string{
		"latest": "", "LATEST": "", "go": "", "go@latest": "",
		"1.26": "1.26", "go@1.26": "1.26", "go1.26": "1.26", "1.26.4": "1.26.4", "go@1.28rc1": "1.28rc1",
	} {
		if got, err := parsePin(arg); err != nil || got != want {
			t.Errorf("parsePin(%q) = %q, %v; want %q", arg, got, err, want)
		}
	}
	for _, arg := range []string{"", "nonsense", "gopls", "v1.26"} {
		if got, err := parsePin(arg); err == nil {
			t.Errorf("parsePin(%q) = %q, want an error", arg, got)
		}
	}
	if keyOf("") != "go" || keyOf("1.26") != "go@1.26" {
		t.Errorf("keyOf = %q, %q", keyOf(""), keyOf("1.26"))
	}
}

// capture points both of a command's outputs at buffers and returns what it
// wrote since the last call, names and prose together.
func capture(t *testing.T) func() string {
	t.Helper()
	var names, prose bytes.Buffer
	stdout = &names
	ui.Reset()
	ui.SetOutput(&prose)
	t.Cleanup(func() { stdout = os.Stdout; ui.SetOutput(os.Stdout); ui.Reset() })
	return func() string {
		defer ui.Reset()
		defer names.Reset()
		defer prose.Reset()
		return names.String() + prose.String()
	}
}

// goDev serves releases as go.dev's list, each with an archive for this
// machine unless its version is in elsewhere.
func goDev(t *testing.T, elsewhere []string, versions ...string) {
	t.Helper()
	var list []godl.Release
	for _, v := range versions {
		goos := runtime.GOOS
		if slices.Contains(elsewhere, v) {
			goos = "plan9"
		}
		list = append(list, godl.Release{Version: v, Stable: !strings.ContainsAny(v[2:], "abcdefghijklmnopqrstuvwxyz"), Files: []godl.File{
			{Filename: v + ".tar.gz", OS: goos, Arch: runtime.GOARCH, Kind: "archive"},
		}})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(list)
	}))
	old := godl.BaseURL
	godl.BaseURL = srv.URL + "/"
	t.Cleanup(func() { godl.BaseURL = old; srv.Close() })
}

func save(t *testing.T, extracts map[string]config.Entry) *config.Manifest {
	t.Helper()
	m := &config.Manifest{Repos: map[string]string{config.Package: config.Source}, Extracts: extracts}
	if err := config.SaveManifest(m); err != nil {
		t.Fatal(err)
	}
	return m
}

func installed() map[string]config.Entry {
	return map[string]config.Entry{
		"go":        {Pin: "latest", Version: "1.27.1", Bin: map[string]string{"go": "bin/go", "gofmt": "bin/gofmt"}},
		"go@1.26":   {Pin: "minor", Version: "1.26.9", Bin: map[string]string{"go@1.26": "bin/go"}},
		"go@1.25.3": {Pin: "fixed", Version: "1.25.3", Bin: map[string]string{"go@1.25.3": "bin/go"}},
	}
}

// names mode is for a script: it prints the keys, which every other command
// takes, and with nothing installed it prints nothing at all rather than a
// sentence the script would have to know
func TestListNamesMode(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	output := capture(t)
	list := newListCmd()
	run := func(flag *bool, args ...string) string {
		t.Helper()
		if flag != nil {
			*flag = true
			defer func() { *flag = false }()
		}
		if err := list.RunE(list, args); err != nil {
			t.Fatal(err)
		}
		return output()
	}

	if got := run(nil); got != msgNoneInstalled+"\n" {
		t.Errorf("without the flag the empty list is said, got %q", got)
	}
	for _, flag := range []*bool{&longNames, &shortNames} {
		if got := run(flag); got != "" {
			t.Errorf("names mode printed %q with nothing installed", got)
		}
	}

	save(t, installed())
	if got := run(&longNames); got != "go\ngo@1.25.3\ngo@1.26\n" {
		t.Errorf("--long-names printed %q", got)
	}
	if got := run(&shortNames, "1.26"); got != "go@1.26\n" {
		t.Errorf("--short-names 1.26 printed %q", got)
	}
	if got := run(nil, "1.9"); got != msgNoMatch+"\n" {
		t.Errorf("a filter that finds nothing printed %q", got)
	}
	// ghpm's list: a row per shim, under the key, the release and the pin level
	want := "name    version pin   uri       type target\n----    ------- ---   ---       ---- ------\ngo@1.26 1.26.9  minor go.dev/dl bin  go@1.26\n"
	if got := run(nil, "1.26"); got != want {
		t.Errorf("list 1.26 printed %q", got)
	}
}

// reading is wider than acting: a term keeps a toolchain by the start of
// what it is pinned to or of the release it is on
func TestFindKeys(t *testing.T) {
	m := &config.Manifest{Extracts: installed()}
	for _, tc := range []struct {
		terms []string
		want  []string
	}{
		{nil, []string{"go", "go@1.25.3", "go@1.26"}},
		{[]string{"latest"}, []string{"go"}},
		{[]string{"go"}, []string{"go"}},
		{[]string{"1.27"}, []string{"go"}},
		{[]string{"go@1.26"}, []string{"go@1.26"}},
		{[]string{"1.26.9"}, []string{"go@1.26"}},
		{[]string{"1.2"}, []string{}},
		{[]string{"1"}, []string{"go", "go@1.25.3", "go@1.26"}},
		{[]string{"1.25", "1.27"}, []string{"go", "go@1.25.3"}},
		{[]string{"1.3", "nonsense"}, []string{}},
	} {
		if got := findKeys(m, tc.terms); !slices.Equal(got, tc.want) {
			t.Errorf("findKeys(%v) = %v, want %v", tc.terms, got, tc.want)
		}
	}
}

// a bare install follows go's newest release, a pin is recorded at ghpm's
// level for it, and installing another channel says which one it replaces
func TestInstallGate(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	goDev(t, nil, "go1.28rc1", "go1.27.2", "go1.27.1", "go1.26.9", "go1.25.3")
	output := capture(t)
	dryRun = true
	defer func() { dryRun = false }()
	install := newInstallCmd()
	run := func(args ...string) string {
		t.Helper()
		_ = install.RunE(install, args)
		return output()
	}

	latest := "name version pin    uri       replaces\n---- ------- ---    ---       --------\ngo   1.27.2  latest go.dev/dl \n\n" + msgDryRun + "\n"
	if got := run(); got != latest {
		t.Errorf("install printed %q", got)
	}
	if got := run("latest"); got != latest {
		t.Errorf("install latest printed %q", got)
	}
	for arg, row := range map[string]string{
		"1":       "go@1 1.27.2  major",
		"go@1.26": "go@1.26 1.26.9  minor",
		"1.25.3":  "go@1.25.3 1.25.3  fixed",
		"1.28rc1": "go@1.28rc1 1.28rc1 fixed",
	} {
		if got := run(arg); !strings.Contains(got, "\n"+row+" go.dev/dl") {
			t.Errorf("install %s printed %q, want a row %q", arg, got, row)
		}
	}
	if err := install.Args(install, []string{"1.26", "1.27"}); err == nil {
		t.Error("install took two channels")
	}

	save(t, map[string]config.Entry{"go": {Pin: "latest", Version: "1.27.1", Bin: map[string]string{"go": "bin/go", "gofmt": "bin/gofmt"}}})
	want := "name    version pin   uri       replaces\n----    ------- ---   ---       --------\ngo@1.26 1.26.9  minor go.dev/dl go 1.27.1\n\n" + msgDryRun + "\n"
	if got := run("1.26"); got != want {
		t.Errorf("install of another channel printed %q", got)
	}
	if got := run("latest"); got != "go: already installed → 1.27.1\n" {
		t.Errorf("install of what is installed printed %q", got)
	}
	// --force is not --reinstall: it is about files in the way of a shim, and
	// what is installed stays installed
	force = true
	if got := run(); got != "go: already installed → 1.27.1\n" {
		t.Errorf("install --force of what is installed printed %q", got)
	}
	force = false
	reinstall = true
	defer func() { reinstall = false }()
	if got := run(); got != latest {
		t.Errorf("install --reinstall printed %q", got)
	}
}

// default with a channel is install; without one it says what go follows
func TestDefault(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	output := capture(t)
	def := newDefaultCmd()
	if err := def.RunE(def, nil); err != nil || output() != msgNoneInstalled+"\n" {
		t.Errorf("default with nothing installed: %v", err)
	}
	save(t, map[string]config.Entry{"go@1.26": {Pin: "minor", Version: "1.26.9"}})
	if err := def.RunE(def, nil); err != nil {
		t.Fatal(err)
	}
	if got := output(); got != "go@1.26 → 1.26.9 (minor)\n" {
		t.Errorf("default printed %q", got)
	}
}

// update moves what can move, and takes no channel: a go held to one release
// is said to be
func TestUpdateGate(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	goDev(t, nil, "go1.27.2", "go1.27.1", "go1.26.9", "go1.25.14", "go1.25.3")
	output := capture(t)
	dryRun = true
	defer func() { dryRun = false }()
	update := newUpdateCmd()
	run := func() string {
		t.Helper()
		_ = update.RunE(update, nil)
		return output()
	}

	if got := run(); got != msgNoneInstalled+"\n" {
		t.Errorf("update with nothing installed printed %q", got)
	}
	if err := update.Args(update, []string{"1.26"}); err == nil {
		t.Error("update took a channel")
	}
	save(t, map[string]config.Entry{"go": {Pin: "latest", Version: "1.27.1"}})
	outdated := "name version update pin    uri\n---- ------- ------ ---    ---\ngo   1.27.1  1.27.2 latest go.dev/dl\n\n" + msgDryRun + "\n"
	if got := run(); got != outdated {
		t.Errorf("update printed %q", got)
	}
	save(t, map[string]config.Entry{"go@1.25.3": {Pin: "fixed", Version: "1.25.3"}})
	if got := run(); got != "go@1.25.3: fixed at 1.25.3, skipping\n" {
		t.Errorf("update of a fixed go printed %q", got)
	}
	save(t, map[string]config.Entry{"go@1.26": {Pin: "minor", Version: "1.26.9"}})
	if got := run(); got != msgAllUpToDate+"\n" {
		t.Errorf("update of a current go printed %q", got)
	}
	// --reinstall moves what is current too, and --force alone does not
	force = true
	if got := run(); got != msgAllUpToDate+"\n" {
		t.Errorf("update --force of a current go printed %q", got)
	}
	force = false
	reinstall = true
	defer func() { reinstall = false }()
	current := "name    version update pin   uri\n----    ------- ------ ---   ---\ngo@1.26 1.26.9  1.26.9 minor go.dev/dl\n\n" + msgDryRun + "\n"
	if got := run(); got != current {
		t.Errorf("update --reinstall of a current go printed %q", got)
	}
}

// find lists what install can install here: the newest release of each line, or
// every release a term starts, beside the toolchains already on them
func TestFind(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	goDev(t, []string{"go1.4.3"}, "go1.28rc1", "go1.27.2", "go1.27.1", "go1.26.9", "go1.4.3")
	output := capture(t)
	find := newFindCmd()
	run := func(args ...string) string {
		t.Helper()
		if err := find.RunE(find, args); err != nil {
			t.Fatal(err)
		}
		return output()
	}

	// a candidate is no line's newest release, and a release with no archive
	// for this machine is not one install can put here
	if got := run(); got != "version\n-------\n1.27.2\n1.26.9\n" {
		t.Errorf("find printed %q", got)
	}
	if got := run("1.27"); got != "version\n-------\n1.27.2\n1.27.1\n" {
		t.Errorf("find 1.27 printed %q", got)
	}
	if got := run("1.28"); got != "version\n-------\n1.28rc1\n" {
		t.Errorf("find 1.28 printed %q", got)
	}
	if got := run("latest"); got != "version\n-------\n1.27.2\n" {
		t.Errorf("find latest printed %q", got)
	}
	if got := run("1.4"); got != "no releases matching \"1.4\"\n" {
		t.Errorf("find 1.4 printed %q", got)
	}

	save(t, map[string]config.Entry{"go": {Version: "1.27.2"}, "go@1.27": {Version: "1.27.2"}})
	if got := run("1.27"); got != "version name\n------- ----\n1.27.2  go, go@1.27\n1.27.1  \n" {
		t.Errorf("find 1.27 with toolchains installed printed %q", got)
	}
	longNames = true
	defer func() { longNames = false }()
	if got := run("1.27"); got != "1.27.2\n1.27.1\n" {
		t.Errorf("find 1.27 --long-names printed %q", got)
	}
	if got := run("1.4"); got != "" {
		t.Errorf("find 1.4 --long-names printed %q", got)
	}
}

// tidy is ghpm's over ~/.goup: with everything accounted for it says so
func TestTidyNothing(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	output := capture(t)
	tidy := newTidyCmd()
	if err := tidy.RunE(tidy, nil); err != nil {
		t.Fatal(err)
	}
	if got := output(); got != "nothing to tidy\n" {
		t.Errorf("tidy printed %q", got)
	}
}
