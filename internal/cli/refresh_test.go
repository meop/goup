package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/meop/golie/wedge"
)

// wedgeReleases serves wedge's release page with latest as its newest,
// unsigned: the signature check is golie's, and tested there
func wedgeReleases(t *testing.T, latest *string) {
	t.Helper()
	r, err := wedge.Here()
	if err != nil {
		t.Skip(err)
	}
	archive := func(version string) []byte {
		var buf bytes.Buffer
		if runtime.GOOS == "windows" {
			zw := zip.NewWriter(&buf)
			for _, name := range r.Files {
				w, _ := zw.Create(name)
				_, _ = w.Write([]byte(version))
			}
			_ = zw.Close()
			return buf.Bytes()
		}
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		for _, name := range r.Files {
			_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(version)), Typeflag: tar.TypeReg})
			_, _ = tw.Write([]byte(version))
		}
		_ = tw.Close()
		_ = gz.Close()
		return buf.Bytes()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/tag/v"+*latest, http.StatusFound)
	})
	mux.HandleFunc("/download/{tag}/{file}", func(w http.ResponseWriter, req *http.Request) {
		version := strings.TrimPrefix(req.PathValue("tag"), "v")
		name, _ := r.Archive(version)
		switch req.PathValue("file") {
		case name:
			_, _ = w.Write(archive(version))
		case "SHA256SUMS":
			sum := sha256.Sum256(archive(version))
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"))
		default:
			http.NotFound(w, req)
		}
	})
	srv := httptest.NewServer(mux)
	origURL, origKey := wedge.ReleasesURL, wedge.Key
	wedge.ReleasesURL, wedge.Key = srv.URL+"/", nil
	t.Cleanup(func() { wedge.ReleasesURL, wedge.Key = origURL, origKey; srv.Close() })
}

// refresh installs wedge where there is none, says when it is the latest,
// and says what it moved from and to, and that shims keep their wedge
func TestRefreshSaysWhatItDid(t *testing.T) {
	t.Setenv("GOUP_TEST_HOME", t.TempDir())
	latest := "1.0.0"
	wedgeReleases(t, &latest)
	out := capture(t)

	for _, step := range []struct{ latest, want string }{
		{"1.0.0", "vendor - wedge: asset fetching [" + mustArchive(t, "1.0.0") + "]...\n✓ vendor - wedge: installed [1.0.0]\n"},
		{"1.0.0", "✓ vendor - wedge: up to date [1.0.0]\n"},
		{"2.0.0", "vendor - wedge: asset fetching [" + mustArchive(t, "2.0.0") + "]...\n✓ vendor - wedge: updated [1.0.0 → 2.0.0]\nvendor - wedge: shims keep the wedge that stamped them until [goup update --reinstall]\n"},
	} {
		latest = step.latest
		if err := runRefresh(nil, nil); err != nil {
			t.Fatal(err)
		}
		if got := out(); got != step.want {
			t.Errorf("refresh to %s printed %q, want %q", step.latest, got, step.want)
		}
	}
}

func mustArchive(t *testing.T, version string) string {
	t.Helper()
	r, err := wedge.Here()
	if err != nil {
		t.Skip(err)
	}
	name, _ := r.Archive(version)
	return name
}
