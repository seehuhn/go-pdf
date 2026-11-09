// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
)

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// testServer serves body and counts the requests.
func testServer(t *testing.T, body []byte) (url string, hits *atomic.Int32) {
	t.Helper()
	hits = new(atomic.Int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/file.bin", hits
}

func setup(t *testing.T) {
	t.Helper()
	oldCache, oldSrc := cacheDir, srcDir
	cacheDir, srcDir = filepath.Join(t.TempDir(), "cache"), ""
	t.Cleanup(func() { cacheDir, srcDir = oldCache, oldSrc })
}

func TestFetch(t *testing.T) {
	setup(t)
	body := []byte("hello")
	url, hits := testServer(t, body)

	got, err := fetch(url, sha(body))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("wrong data %q", got)
	}

	// second call uses the cache
	if _, err := fetch(url, sha(body)); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("got %d requests, want 1", n)
	}
}

func TestFetchBadChecksum(t *testing.T) {
	setup(t)
	url, _ := testServer(t, []byte("hello"))

	if _, err := fetch(url, sha([]byte("other"))); err == nil {
		t.Error("bad checksum not detected")
	}
	entries, _ := os.ReadDir(cacheDir)
	if len(entries) != 0 {
		t.Errorf("unverified download cached: %d files", len(entries))
	}
}

func TestFetchCorruptCache(t *testing.T) {
	setup(t)
	body := []byte("hello")
	url, hits := testServer(t, body)

	if _, err := fetch(url, sha(body)); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(cacheDir, "*file.bin"))
	if len(files) != 1 {
		t.Fatalf("got %d cache files, want 1", len(files))
	}
	if err := os.WriteFile(files[0], []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := fetch(url, sha(body))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("wrong data %q", got)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("got %d requests, want 2", n)
	}
	repaired, _ := os.ReadFile(files[0])
	if !bytes.Equal(repaired, body) {
		t.Error("cache not repaired")
	}
}

func TestFetchHTTPError(t *testing.T) {
	setup(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := fetch(srv.URL+"/x", sha(nil)); err == nil {
		t.Error("HTTP error not reported")
	}
}

func TestFetchSrcDir(t *testing.T) {
	setup(t)
	dir := t.TempDir()
	body := []byte("local")
	if err := os.WriteFile(filepath.Join(dir, "file.bin"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	srcDir = dir

	// the URL is never contacted
	got, err := fetch("http://invalid.invalid/path/file.bin", sha(body))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("wrong data %q", got)
	}
	if _, err := fetch("http://invalid.invalid/path/file.bin", sha([]byte("x"))); err == nil {
		t.Error("bad checksum not detected")
	}
	if _, err := fetch("http://invalid.invalid/path/missing.bin", sha(body)); err == nil {
		t.Error("missing file not reported")
	}
}

func TestSources(t *testing.T) {
	if len(sources) != 4+14+1 {
		t.Errorf("got %d sources, want 19", len(sources))
	}
	seen := make(map[string]bool)
	for _, s := range sources {
		if seen[s.name] {
			t.Errorf("duplicate source %s", s.name)
		}
		seen[s.name] = true
		if len(s.sha) != 64 {
			t.Errorf("%s: bad checksum length", s.name)
		}
		if _, err := hex.DecodeString(s.sha); err != nil {
			t.Errorf("%s: %v", s.name, err)
		}
	}
}

func makeArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		hdr := &tar.Header{Name: n, Mode: 0o644, Size: int64(len(files[n])), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(files[n])); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractLiberation(t *testing.T) {
	files := map[string]string{
		"liberation-fonts-ttf-2.1.5/LICENSE":                          "licence text",
		"liberation-fonts-ttf-2.1.5/README.md":                        "ignored",
		"liberation-fonts-ttf-2.1.5/LiberationSansNarrow-Regular.ttf": "ignored",
	}
	for name := range liberationFonts {
		files["liberation-fonts-ttf-2.1.5/"+name] = "data of " + name
	}

	fonts, license, err := extractLiberation(makeArchive(t, files))
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 12 {
		t.Errorf("got %d fonts, want 12", len(fonts))
	}
	for name := range liberationFonts {
		if string(fonts[name]) != "data of "+name {
			t.Errorf("%s: wrong content", name)
		}
	}
	if string(license) != "licence text" {
		t.Errorf("wrong licence %q", license)
	}

	// missing font
	delete(files, "liberation-fonts-ttf-2.1.5/LiberationMono-Bold.ttf")
	if _, _, err := extractLiberation(makeArchive(t, files)); err == nil {
		t.Error("missing font not detected")
	}

	// missing licence
	files["liberation-fonts-ttf-2.1.5/LiberationMono-Bold.ttf"] = "x"
	delete(files, "liberation-fonts-ttf-2.1.5/LICENSE")
	if _, _, err := extractLiberation(makeArchive(t, files)); err == nil {
		t.Error("missing licence not detected")
	}

	// not a gzip file
	if _, _, err := extractLiberation([]byte("junk")); err == nil {
		t.Error("invalid archive not detected")
	}
}
