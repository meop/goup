package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/meop/golie/layout"
	"github.com/meop/golie/pointer"
	"github.com/meop/goup/internal/config"
	"github.com/meop/goup/internal/godl"
	"github.com/meop/goup/internal/store"
)

// release is a go release whose archive is already in the download cache: a go
// tree that says its version and ships no commands, so nothing is stamped
func release(t *testing.T, l layout.Layout, version string) godl.Release {
	t.Helper()
	downloads, err := l.DownloadDir(config.Source, version)
	if err != nil {
		t.Fatal(err)
	}
	name := "go" + version + ".tar.gz"
	f, err := os.Create(filepath.Join(downloads, name))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	body := []byte("go" + version)
	for _, hdr := range []*tar.Header{
		{Name: "go/bin/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "go/VERSION", Typeflag: tar.TypeReg, Mode: 0644, Size: int64(len(body))},
	} {
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write(body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(downloads, name))
	if err != nil {
		t.Fatal(err)
	}
	h.Write(b)
	return godl.Release{Version: "go" + version, Stable: true, Files: []godl.File{
		{Filename: name, OS: runtime.GOOS, Arch: runtime.GOARCH, Kind: "archive", SHA256: hex.EncodeToString(h.Sum(nil))},
	}}
}

func TestApplyPointsTheVersionFileAtEachReleaseAndLeavesNoOldOne(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	l, err := store.Layout()
	if err != nil {
		t.Fatal(err)
	}
	m := save(t, map[string]config.Entry{})

	for _, version := range []string{"1.27.1", "1.27.2"} {
		c := change{key: "go", pin: "latest", prev: m.Extracts["go"], target: release(t, l, version)}
		if err := apply(context.Background(), m, l, c); err != nil {
			t.Fatal(err)
		}
		tree, ok := pointer.Target(l.VersionFilePath("go"))
		if !ok || tree != l.VersionPath("go", version) {
			t.Fatalf("after %s the version file names %s, %v", version, tree, ok)
		}
		if got, err := os.ReadFile(filepath.Join(tree, "VERSION")); err != nil || string(got) != "go"+version {
			t.Errorf("after %s the tree says %q, %v", version, got, err)
		}
		if m.Extracts["go"].Version != version {
			t.Errorf("the manifest says %s after %s", m.Extracts["go"].Version, version)
		}
	}
	entries, err := os.ReadDir(l.ExtractPath("go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "1.27.2,version" {
		t.Errorf("the install holds %v", names)
	}
}

// installing another channel keeps only the new one: the old one's tree and
// entry go once the new one is in
func TestInstallAnotherChannelDropsTheOld(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	l, err := store.Layout()
	if err != nil {
		t.Fatal(err)
	}
	m := save(t, map[string]config.Entry{})
	for _, c := range []change{
		{key: "go", pin: "", target: release(t, l, "1.27.2")},
		{key: "go@1.26", pin: "1.26", target: release(t, l, "1.26.9")},
	} {
		if err := apply(context.Background(), m, l, c); err != nil {
			t.Fatal(err)
		}
	}
	if failed := dropAll(m, []string{"go"}, m.Extracts["go@1.26"].Bin); len(failed) > 0 {
		t.Fatalf("dropAll: %v", failed)
	}
	if keys := m.Keys(); len(keys) != 1 || keys[0] != "go@1.26" {
		t.Errorf("the manifest holds %v", keys)
	}
	if _, err := os.Stat(l.ExtractPath("go")); !os.IsNotExist(err) {
		t.Errorf("the old channel's tree is still there: %v", err)
	}
	if tree, ok := pointer.Target(l.VersionFilePath("go@1.26")); !ok || tree != l.VersionPath("go@1.26", "1.26.9") {
		t.Errorf("the new channel's version file names %s, %v", tree, ok)
	}
}
