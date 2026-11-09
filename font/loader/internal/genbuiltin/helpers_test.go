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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"seehuhn.de/go/sfnt/glyph"
)

func TestMustReadText(t *testing.T) {
	page := []byte(`<html><head><title>x</title></head>
<body bgcolor="white"><font color="white">hidden text</font>
<p>Copyright &copy; 2001  Adobe,
<b>all</b> rights reserved.</p></body></html>`)
	got := mustReadText(page)
	want := "Copyright © 2001 Adobe, all rights reserved."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// no body tag
	if got := mustReadText([]byte("<p>a</p>\n<p>b</p>")); got != "a b" {
		t.Errorf("got %q, want %q", got, "a b")
	}
}

func TestMakeLicenses(t *testing.T) {
	got := makeLicenses([]byte("LIBERATION TEXT\n\n"), []byte("FOXIT TEXT"), []byte("<body>ADOBE <i>TEXT</i></body>"))
	for _, s := range []string{"LIBERATION TEXT\n\n", "FOXIT TEXT\n\n", "ADOBE TEXT\n"} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q", s)
		}
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Error("output should end in exactly one newline")
	}
}

func TestJoinNotices(t *testing.T) {
	if got := joinNotices("", "b"); got != "b" {
		t.Errorf("got %q, want b", got)
	}
	if got := joinNotices("a", "b"); got != "a b" {
		t.Errorf("got %q, want a b", got)
	}
}

func TestWriteOutputs(t *testing.T) {
	root := t.TempDir()
	oldOut := outDir
	outDir = filepath.Join(root, "builtin")
	t.Cleanup(func() { outDir = oldOut })

	// stale content in the generated directories must disappear,
	// other content must stay
	for _, p := range []string{"quire/stale", "adobe/stale", "keep"} {
		full := filepath.Join(outDir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string][]byte{
		"quire/a.gz":   []byte("1"),
		"adobe/b.gz":   []byte("22"),
		"adobe/c":      []byte("333"),
		"font.map":     []byte("map"),
		"Licenses.txt": []byte("lic"),
	}
	if err := writeOutputs(files); err != nil {
		t.Fatal(err)
	}
	for p, want := range files {
		got, err := os.ReadFile(filepath.Join(outDir, p))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s: got %q, want %q", p, got, want)
		}
	}
	for _, p := range []string{"quire/stale", "adobe/stale"} {
		if _, err := os.Stat(filepath.Join(outDir, p)); err == nil {
			t.Errorf("%s not removed", p)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "keep")); err != nil {
		t.Error("unrelated file removed")
	}

	// no temporary directories left behind
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("%d entries next to outDir, want 1", len(entries))
	}
}

func TestWriteOutputsFailure(t *testing.T) {
	root := t.TempDir()
	oldOut := outDir
	outDir = filepath.Join(root, "builtin")
	t.Cleanup(func() { outDir = oldOut })

	if err := writeOutputs(map[string][]byte{"quire/a": []byte("old")}); err != nil {
		t.Fatal(err)
	}

	// "x" is both a file and a directory, so writing fails part-way
	err := writeOutputs(map[string][]byte{"quire/b": []byte("new"), "quire/x": []byte("1"), "quire/x/y": []byte("2")})
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(filepath.Join(outDir, "quire/a")); err != nil {
		t.Error("previous output damaged by failed run")
	}
	if _, err := os.Stat(filepath.Join(outDir, "quire/b")); err == nil {
		t.Error("partial output visible")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Errorf("%d entries next to outDir, want 1", len(entries))
	}
}

func TestWriteOutputsBadDir(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	oldOut := outDir
	outDir = filepath.Join(blocker, "builtin")
	t.Cleanup(func() { outDir = oldOut })
	if err := writeOutputs(nil); err == nil {
		t.Error("expected an error")
	}
}

func TestLoadNameMismatch(t *testing.T) {
	s := source{name: "a.txt", url: "https://example.com/b.txt", sha: "00"}
	if _, err := load(s); err == nil {
		t.Error("mismatch not detected")
	}
}

func TestLookupGlyph(t *testing.T) {
	f := readFont(t, "quire/QuireSans-Regular.ttf.gz")
	cm := mustCMap(t, f)

	// via the unicode value
	if got, want := lookupGlyph(nil, cm, "A"), cm.Lookup('A'); got == 0 || got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	// via the post names
	names := []string{".notdef", "foo", "bar"}
	if got := lookupGlyph(names, cm, "bar"); got != glyph.ID(2) {
		t.Errorf("got %d, want 2", got)
	}
	// name mapping to several runes
	if got := lookupGlyph(nil, cm, "f_i"); got != 0 {
		t.Errorf("multi-rune name: got %d, want 0", got)
	}
	// unknown name
	if got := lookupGlyph(nil, cm, "nosuchglyphname"); got != 0 {
		t.Errorf("unknown name: got %d, want 0", got)
	}
}

func TestAddLigaErrors(t *testing.T) {
	// no GSUB table
	f := readFont(t, "quire/QuireSans-Regular.ttf.gz")
	f.Gsub = nil
	if err := addLiga(f); err == nil {
		t.Error("missing GSUB not detected")
	}

	// feature variations
	f = readFont(t, "quire/QuireSans-Regular.ttf.gz")
	f.Gsub.VariationsRaw = []byte{0}
	if err := addLiga(f); err == nil {
		t.Error("feature variations not detected")
	}

	// existing liga feature
	f = readFont(t, "quire/QuireSans-Regular.ttf.gz")
	if err := addLiga(f); err == nil {
		t.Error("existing liga feature not detected")
	}

	// missing glyph: the symbol font has no Latin letters
	f = readFont(t, "quire/QuireSymbol.otf.gz")
	if err := addLiga(f); err == nil {
		t.Error("missing glyph not detected")
	}

	// no usable cmap
	f = readFont(t, "quire/QuireSans-Regular.ttf.gz")
	f.CMapTable = nil
	if err := addLiga(f); err == nil {
		t.Error("missing cmap not detected")
	}
}
