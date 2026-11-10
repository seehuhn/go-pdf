// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package loader

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"seehuhn.de/go/pdf/internal/stdmtx"
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/parser"
)

// TestBuiltin verifies that the builtin fonts are available.
// This test can catch typos in the builtin font map file.
func TestBuiltin(t *testing.T) {
	loader := NewFontLoader()

	for key, val := range loader.lookup {
		if !val.isBuiltin {
			t.Errorf("%s/%d: non-builtin font in builtin map", key.psname, key.fontType)
		}

		// Make sure the font can be loaded.
		r, err := loader.Open(key.psname, key.fontType)
		if err != nil {
			t.Errorf("%s/%d: error loading builtin font: %v", key.psname, key.fontType, err)
			continue
		}
		_, err = io.Copy(io.Discard, r)
		if err != nil {
			t.Errorf("%s/%d: error reading builtin font: %v", key.psname, key.fontType, err)
		}
		err = r.Close()
		if err != nil {
			t.Errorf("%s/%d: error closing builtin font: %v", key.psname, key.fontType, err)
		}
	}
}

func TestIsBuiltin(t *testing.T) {
	l := NewFontLoader()
	if !l.IsBuiltin("Helvetica", FontTypeSfnt) {
		t.Error("Helvetica sfnt not builtin")
	}
	if l.IsBuiltin("Helvetica", FontTypeType1) {
		t.Error("absent entry reported as builtin")
	}
	if l.IsBuiltin("Unknown", FontTypeSfnt) {
		t.Error("unknown font reported as builtin")
	}
	l.AddFont("Helvetica", FontTypeSfnt, "/nonexistent.ttf")
	if l.IsBuiltin("Helvetica", FontTypeSfnt) {
		t.Error("overridden entry reported as builtin")
	}
	if !l.IsBuiltin("Helvetica", FontTypeAFM) {
		t.Error("Helvetica afm not builtin")
	}
}

// TestStandardFonts checks that the 14 standard fonts are available.
func TestStandardFonts(t *testing.T) {
	loader := NewFontLoader()

	for name := range stdmtx.Metrics {
		t.Run(name, func(t *testing.T) {
			r, err := loader.Open(name, FontTypeSfnt)
			if err != nil {
				t.Fatalf("error loading font %q: %v", name, err)
			}
			_, err = sfnt.Read(r, parser.NewBudget(1<<26))
			if err != nil {
				t.Errorf("error reading font %q: %v", name, err)
			}
			err = r.Close()
			if err != nil {
				t.Errorf("error closing font %q: %v", name, err)
			}

			r, err = loader.Open(name, FontTypeAFM)
			if err != nil {
				t.Fatalf("error loading afm file %q: %v", name, err)
			}
			metrics, err := afm.Read(r)
			if err != nil {
				t.Errorf("error reading afm file %q: %v", name, err)
			}
			err = r.Close()
			if err != nil {
				t.Errorf("error closing afm file %q: %v", name, err)
			}
			if metrics != nil && metrics.FontName != name {
				t.Errorf("font name %q, want %q", metrics.FontName, name)
			}
		})
	}
}

func TestHelvetica(t *testing.T) {
	loader := NewFontLoader()

	r, err := loader.Open("Helvetica", FontTypeSfnt)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	info, err := sfnt.Read(r, parser.NewBudget(1<<26))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.FontName; got != "QuireSans-Regular" {
		t.Errorf("PostScript name %q, want QuireSans-Regular", got)
	}
}

// TestNoType1 checks that no Type 1 fonts are bundled.
func TestNoType1(t *testing.T) {
	loader := NewFontLoader()
	_, err := loader.Open("Helvetica", FontTypeType1)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("got %v, want fs.ErrNotExist", err)
	}
}

func TestAddFontMap(t *testing.T) {
	l := NewFontLoader()
	m := "# comment\n% another comment\n\n  \nFoo type1 /fonts/foo.pfb\nFoo afm /fonts/foo.afm\nFoo sfnt /fonts/my font.ttf\n"
	if err := l.AddFontMap(strings.NewReader(m)); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tp   FontType
		want string
	}{
		{FontTypeType1, "/fonts/foo.pfb"},
		{FontTypeAFM, "/fonts/foo.afm"},
		{FontTypeSfnt, "/fonts/my font.ttf"},
	}
	for _, c := range cases {
		v := l.lookup[key{"Foo", c.tp}]
		if v == nil || v.fname != c.want || v.isBuiltin {
			t.Errorf("type %d: got %+v, want %q", c.tp, v, c.want)
		}
	}
	if len(l.lookup) != len(NewFontLoader().lookup)+3 {
		t.Error("comment or blank lines created entries")
	}
}

func TestAddFontMapErrors(t *testing.T) {
	for _, m := range []string{
		"Foo sfnt\n",
		"Foo\n",
		"Foo opentype /a/b\n",
	} {
		if err := NewFontLoader().AddFontMap(strings.NewReader(m)); err == nil {
			t.Errorf("%q: no error", m)
		}
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }

func TestAddFontMapReadError(t *testing.T) {
	if err := NewFontLoader().AddFontMap(errReader{}); err == nil {
		t.Error("read error not reported")
	}
}

func TestOpenAddedFont(t *testing.T) {
	dir := t.TempDir()
	fname := filepath.Join(dir, "x.ttf")
	if err := os.WriteFile(fname, []byte("font data"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := NewFontLoader()
	l.AddFont("X", FontTypeSfnt, fname)
	r, err := l.Open("X", FontTypeSfnt)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if string(data) != "font data" {
		t.Errorf("got %q", data)
	}

	// registered but missing file
	l.AddFont("Y", FontTypeSfnt, filepath.Join(dir, "missing.ttf"))
	if _, err := l.Open("Y", FontTypeSfnt); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("got %v, want fs.ErrNotExist", err)
	}

	// unknown font
	if _, err := l.Open("Z", FontTypeSfnt); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("got %v, want fs.ErrNotExist", err)
	}
}

func gzipped(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestOpenFS(t *testing.T) {
	fsys := fstest.MapFS{
		"plain.txt":     {Data: []byte("plain")},
		"packed.txt.gz": {Data: gzipped(t, "packed")},
		"bad.gz":        {Data: []byte("not gzip data")},
	}

	for name, want := range map[string]string{"plain.txt": "plain", "packed.txt.gz": "packed"} {
		r, err := openFS(fsys, name)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Close(); err != nil {
			t.Error(err)
		}
		if string(data) != want {
			t.Errorf("%s: got %q, want %q", name, data, want)
		}
	}

	if _, err := openFS(fsys, "missing.gz"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: got %v, want fs.ErrNotExist", err)
	}
	if _, err := openFS(fsys, "bad.gz"); err == nil {
		t.Error("invalid gzip data not detected")
	}
}

// failFile is an fs.File which fails to close.
type failFile struct {
	io.Reader
	err error
}

func (f failFile) Stat() (fs.FileInfo, error) { return nil, errors.New("no stat") }
func (f failFile) Read(p []byte) (int, error) { return f.Reader.Read(p) }
func (f failFile) Close() error               { return f.err }

func TestGzipReadCloserClose(t *testing.T) {
	newZR := func(data []byte) *gzip.Reader {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return zr
	}
	good := gzipped(t, "some data that is long enough to be interesting")
	closeErr := errors.New("close error")

	// file close error is reported
	g := &gzipReadCloser{zr: newZR(good), f: failFile{err: closeErr}}
	if err := g.Close(); err != closeErr {
		t.Errorf("got %v, want the file's close error", err)
	}

	// decompressor error takes precedence, but the file is still closed
	g = &gzipReadCloser{zr: newZR(good[:len(good)-12]), f: failFile{err: closeErr}}
	if _, err := io.Copy(io.Discard, g); err == nil {
		t.Fatal("truncated data not detected")
	}
	if err := g.Close(); err == nil || err == closeErr {
		t.Errorf("got %v, want the decompressor error", err)
	}
}
