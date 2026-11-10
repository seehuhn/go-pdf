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
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/language"
	"seehuhn.de/go/postscript/afm"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"
	"seehuhn.de/go/sfnt/parser"

	"seehuhn.de/go/pdf/font/pdfenc"
)

// builtinDir is the directory with the generated files.
const builtinDir = "../../builtin"

// readGz returns the uncompressed content of a generated file.
func readGz(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(builtinDir, name))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readFont(t *testing.T, name string) *sfnt.Font {
	t.Helper()
	data := readGz(t, name)
	f, err := sfnt.Read(bytes.NewReader(data), parser.NewBudget(int64(len(data))))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestQuireLatin(t *testing.T) {
	for _, family := range quireFamilies {
		for _, style := range quireStyles {
			fontName := "Quire" + family + "-" + style
			t.Run(fontName, func(t *testing.T) {
				f := readFont(t, "quire/"+fontName+".ttf.gz")
				if f.FontName != fontName {
					t.Errorf("wrong font name %q", f.FontName)
				}
				if f.FamilyName != "Quire "+family {
					t.Errorf("wrong family name %q", f.FamilyName)
				}
				if f.Copyright == "" || f.License == "" || f.LicenseURL == "" || f.Description == "" {
					t.Error("missing copyright or licence fields")
				}

				outl, ok := f.Outlines.(*glyf.Outlines)
				if !ok {
					t.Fatalf("wrong outline type %T", f.Outlines)
				}
				if outl.Names == nil {
					t.Fatal("no glyph names")
				}
				for _, tag := range []string{"fpgm", "prep", "cvt "} {
					if len(outl.Tables[tag]) == 0 {
						t.Errorf("missing %q table", tag)
					}
				}

				cm := mustCMap(t, f)

				// glyph names of the standard font which the face replaces
				metrics, err := afm.Read(bytes.NewReader(readGz(t, "adobe/"+standardName(fontName)+".afm.gz")))
				if err != nil {
					t.Fatal(err)
				}
				for name := range metrics.Glyphs {
					if lookupGlyph(outl.Names, cm, name) == 0 {
						t.Errorf("missing glyph %q", name)
					}
				}

				// macron is the spacing accent, not the wider bar of U+00AF
				if gid := slices.Index(outl.Names, "macron"); gid <= 0 || glyph.ID(gid) != cm.Lookup(0x02C9) {
					t.Errorf("glyph %d is named macron, want the glyph of U+02C9", gid)
				}
				if name := outl.Names[cm.Lookup(0x00AF)]; name != "uni00AF" {
					t.Errorf("the glyph of U+00AF is named %q", name)
				}

				// the spacing comma below, which no encoding reaches
				if gid := slices.Index(outl.Names, "commaaccent"); gid > 0 {
					box := f.GlyphBBox(glyph.ID(gid))
					if box.IsZero() || box.URy >= 0 {
						t.Errorf("commaaccent: wrong bounding box %v", box)
					}
					if outl.Widths[gid] == 0 {
						t.Error("commaaccent: zero advance width")
					}
					if family == "Mono" {
						want := outl.Widths[cm.Lookup(' ')]
						if got := outl.Widths[gid]; got != want {
							t.Errorf("commaaccent: width %d, want %d", got, want)
						}
					}
				} else {
					t.Error("no glyph named commaaccent")
				}

				var missing []rune
				for _, rg := range []struct{ lo, hi rune }{
					{0x20, 0x7E}, {0xA0, 0xFF}, {0x100, 0x17F}, {0x192, 0x192},
					{0x2C6, 0x2C7}, {0x2D8, 0x2DD},
					{0x391, 0x3A1}, {0x3A3, 0x3A9}, {0x3B1, 0x3C9},
					{0x410, 0x44F}, {0x2013, 0x2014}, {0x2018, 0x201E}, {0x2022, 0x2022},
					{0x20AC, 0x20AC}, {0x2122, 0x2122}, {0x2190, 0x2193}, {0x2202, 0x2202},
					{0xFB01, 0xFB02},
				} {
					for r := rg.lo; r <= rg.hi; r++ {
						if cm.Lookup(r) == 0 {
							missing = append(missing, r)
						}
					}
				}
				if len(missing) > 0 {
					t.Errorf("missing characters %U", missing)
				}

				// GPOS kern for Sans and Serif
				if family != "Mono" {
					if f.Gpos == nil || !hasFeature(f.Gpos.FeatureList, "kern") {
						t.Error("GPOS kern feature missing")
					}
				}
				if f.Gdef == nil {
					t.Error("GDEF missing")
				}
				if f.Gsub == nil || !hasFeature(f.Gsub.FeatureList, "sups") {
					t.Error("GSUB sups feature missing")
				}
				if hasFeature(f.Gsub.FeatureList, "liga") != (family != "Mono") {
					t.Error("GSUB liga feature wrongly present or absent")
				}
			})
		}
	}
}

func TestQuireSymbol(t *testing.T) {
	for _, spec := range symbolSpecs {
		t.Run(spec.fontName, func(t *testing.T) {
			f := readFont(t, "quire/"+spec.fontName+".otf.gz")
			if f.FontName != spec.fontName || f.FamilyName != spec.family {
				t.Errorf("wrong names %q, %q", f.FontName, f.FamilyName)
			}
			if f.Copyright == "" || f.License == "" {
				t.Error("missing copyright or licence fields")
			}
			outl, ok := f.Outlines.(*cff.Outlines)
			if !ok {
				t.Fatalf("wrong outline type %T", f.Outlines)
			}

			stdName := map[string]string{"QuireSymbol": "Symbol", "QuireDingbats": "ZapfDingbats"}[spec.fontName]
			m, err := afm.Read(bytes.NewReader(readGz(t, "adobe/"+stdName+".afm.gz")))
			if err != nil {
				t.Fatal(err)
			}

			byName := make(map[string]int)
			for i, g := range outl.Glyphs {
				byName[g.Name] = i
			}
			q := f.FontMatrix[0] * 1000
			for name := range spec.encoding.Has {
				i, ok := byName[name]
				if !ok {
					t.Errorf("missing glyph %q", name)
					continue
				}
				w := outl.Glyphs[i].Width * q
				if d := w - m.Glyphs[name].WidthX; d < -0.5 || d > 0.5 {
					t.Errorf("glyph %q: width %g, want %g", name, w, m.Glyphs[name].WidthX)
				}
			}

			// built-in encoding via the symbol subtable
			symCmap, err := f.CMapTable.GetNoLang(3, 0)
			if err != nil {
				t.Fatal(err)
			}
			for code, name := range spec.encoding.Encoding {
				if name == "" {
					continue
				}
				if gid := symCmap.Lookup(rune(0xF000 + code)); int(gid) != byName[name] {
					t.Errorf("code %d: got glyph %d, want %q", code, gid, name)
				}
			}

			uni := mustCMap(t, f)
			check := map[string]struct {
				r    rune
				name string
			}{
				"QuireSymbol":   {0x03B1, "alpha"},
				"QuireDingbats": {0x2701, "a1"},
			}[spec.fontName]
			if gid := uni.Lookup(check.r); outl.Glyphs[gid].Name != check.name {
				t.Errorf("U+%04X maps to %q, want %q", check.r, outl.Glyphs[gid].Name, check.name)
			}
		})
	}
}

func TestOutputFiles(t *testing.T) {
	// the font map names existing files
	mapData, err := os.ReadFile(filepath.Join(builtinDir, "font.map"))
	if err != nil {
		t.Fatal(err)
	}
	for line := range bytes.SplitSeq(bytes.TrimSpace(mapData), []byte("\n")) {
		fields := bytes.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("bad font.map line %q", line)
		}
		if _, err := os.Stat(filepath.Join(builtinDir, "..", string(fields[2]))); err != nil {
			t.Error(err)
		}
		if string(fields[1]) == "type1" {
			t.Errorf("unexpected type1 entry %q", line)
		}
	}
	if string(makeFontMap()) != string(mapData) {
		t.Error("font.map is out of date")
	}

	// the AFM files are unmodified
	for _, s := range afmSources {
		if err := verify(readGz(t, "adobe/"+s.name+".gz"), s.sha); err != nil {
			t.Errorf("%s differs from the source", s.name)
		}
	}
	raw, err := os.ReadFile(filepath.Join(builtinDir, "adobe", mustReadSource.name))
	if err != nil {
		t.Fatal(err)
	}
	if err := verify(raw, mustReadSource.sha); err != nil {
		t.Errorf("%s differs from the source", mustReadSource.name)
	}
}

func TestGzipBytes(t *testing.T) {
	a, err := gzipBytes([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := gzipBytes([]byte("hello"))
	if !bytes.Equal(a, b) {
		t.Error("gzip output not deterministic")
	}
	zr, err := gzip.NewReader(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	if zr.Name != "" || !zr.ModTime.IsZero() {
		t.Error("gzip header not empty")
	}
}

func mustCMap(t *testing.T, f *sfnt.Font) cmap.Subtable {
	t.Helper()
	cm, err := f.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}
	return cm
}

// TestAddLiga checks that addLiga adds the ligatures to every language
// system, keeps the feature list sorted and keeps the other features of each
// language system.  Language systems may share their values, so the test
// also checks that the input script list is left unchanged.
func TestAddLiga(t *testing.T) {
	src := readFont(t, "quire/QuireSans-Italic.ttf.gz")

	// remove the liga feature, so that addLiga can be applied again
	data := &bytes.Buffer{}
	if _, err := src.Write(data); err != nil {
		t.Fatal(err)
	}
	f, err := sfnt.Read(bytes.NewReader(data.Bytes()), parser.NewBudget(int64(data.Len())))
	if err != nil {
		t.Fatal(err)
	}
	gsub := f.Gsub
	ligaIdx := slices.IndexFunc(gsub.FeatureList, func(ft *gtab.Feature) bool { return ft.Tag == "liga" })
	if ligaIdx < 0 {
		t.Fatal("no liga feature")
	}
	gsub.FeatureList = slices.Delete(gsub.FeatureList, ligaIdx, ligaIdx+1)
	// The font was read above and is used nowhere else, so its values can
	// be changed in place, each shared value once.
	seen := make(map[*gtab.Features]bool)
	for _, ls := range gsub.ScriptList {
		if seen[ls] {
			continue
		}
		seen[ls] = true
		switch {
		case ls.Required == 0xFFFF:
		case int(ls.Required) == ligaIdx:
			ls.Required = 0xFFFF
		case int(ls.Required) > ligaIdx:
			ls.Required--
		}
		ls.Optional = slices.DeleteFunc(ls.Optional, func(i gtab.FeatureIndex) bool { return int(i) == ligaIdx })
		for i, idx := range ls.Optional {
			if int(idx) > ligaIdx {
				ls.Optional[i]--
			}
		}
	}

	// Grek is made to share Latn's value, so the test does not rely on the
	// reader's sharing.  Latn and Cyrl get required features which sort
	// after and before liga.
	featureIdx := func(tag string) gtab.FeatureIndex {
		i := slices.IndexFunc(gsub.FeatureList, func(ft *gtab.Feature) bool { return ft.Tag == tag })
		if i < 0 {
			t.Fatalf("no %s feature", tag)
		}
		return gtab.FeatureIndex(i)
	}
	latn := language.MustParse("und-Latn-x-latn")
	grek := language.MustParse("und-Grek-x-grek")
	cyrl := language.MustParse("und-Cyrl-x-cyrl")
	for _, tag := range []language.Tag{latn, grek, cyrl} {
		if gsub.ScriptList[tag] == nil {
			t.Fatalf("no language system %s", tag)
		}
	}
	if gsub.ScriptList[latn] == gsub.ScriptList[cyrl] {
		t.Fatal("unexpected sharing")
	}
	gsub.ScriptList[grek] = gsub.ScriptList[latn]
	gsub.ScriptList[latn].Required = featureIdx("subs")
	gsub.ScriptList[cyrl].Required = featureIdx("dlig")

	// the feature tags of a language system, the required one marked by "*"
	tags := func(ls *gtab.Features) []string {
		var res []string
		if ls.Required != 0xFFFF {
			res = append(res, "*"+gsub.FeatureList[ls.Required].Tag)
		}
		for _, idx := range ls.Optional {
			res = append(res, gsub.FeatureList[idx].Tag)
		}
		slices.Sort(res)
		return res
	}
	before := make(map[string][]string)
	for lang, ls := range gsub.ScriptList {
		before[lang.String()] = tags(ls)
	}
	if len(before) < 2 {
		t.Fatalf("only %d language systems", len(before))
	}

	snapshot := func(scripts gtab.ScriptListInfo) map[string]gtab.Features {
		res := make(map[string]gtab.Features, len(scripts))
		for lang, ls := range scripts {
			res[lang.String()] = gtab.Features{
				Required: ls.Required,
				Optional: slices.Clone(ls.Optional),
			}
		}
		return res
	}
	input := gsub.ScriptList
	inputBefore := snapshot(input)

	if err := addLiga(f); err != nil {
		t.Fatal(err)
	}
	if !slices.IsSortedFunc(gsub.FeatureList, func(a, b *gtab.Feature) int { return strings.Compare(a.Tag, b.Tag) }) {
		t.Error("feature list not sorted")
	}
	for lang, ls := range gsub.ScriptList {
		want := slices.Sorted(slices.Values(slices.Concat(before[lang.String()], []string{"liga"})))
		if got := tags(ls); !slices.Equal(got, want) {
			t.Errorf("%s: features %v, want %v", lang, got, want)
		}
	}
	if diff := cmp.Diff(inputBefore, snapshot(input)); diff != "" {
		t.Errorf("input script list modified (-before +after):\n%s", diff)
	}

	if err := addLiga(f); err == nil {
		t.Error("second addLiga succeeded")
	}
}

func hasFeature(list gtab.FeatureListInfo, tag string) bool {
	for _, f := range list {
		if f.Tag == tag && len(f.Lookups) > 0 {
			return true
		}
	}
	return false
}

// offlineTransport rejects all requests except those to the local machine.
type offlineTransport struct{}

func (offlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Hostname() {
	case "127.0.0.1", "::1", "localhost":
		return http.DefaultTransport.RoundTrip(req)
	}
	return nil, errors.New("network access in test")
}

func TestMain(m *testing.M) {
	httpClient = &http.Client{Transport: offlineTransport{}}
	os.Exit(m.Run())
}

func TestQuireSymbolBuiltinEncoding(t *testing.T) {
	cases := []struct {
		file string
		enc  *pdfenc.Encoding
	}{
		{"quire/QuireSymbol.otf.gz", &pdfenc.Symbol},
		{"quire/QuireDingbats.otf.gz", &pdfenc.ZapfDingbats},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			f := readFont(t, c.file)
			outlines, ok := f.Outlines.(*cff.Outlines)
			if !ok {
				t.Fatal("not a CFF font")
			}
			got := outlines.BuiltinEncoding()
			if got == nil {
				t.Fatal("no built-in encoding")
			}
			for code, want := range c.enc.Encoding {
				if want == "" {
					want = ".notdef"
				}
				if got[code] != want {
					t.Errorf("code %d: got glyph %q, want %q", code, got[code], want)
				}
			}
		})
	}
}

func TestAddCommaAccent(t *testing.T) {
	// a font which has the glyph is left unchanged
	f := readFont(t, "quire/QuireSans-Regular.ttf.gz")
	n := f.NumGlyphs()
	if err := addCommaAccent(f, "Sans"); err != nil {
		t.Fatal(err)
	}
	if f.NumGlyphs() != n {
		t.Error("glyph added to a font which has commaaccent")
	}

	// Serif: the glyph is found under its Liberation name
	f = readFont(t, "quire/QuireSerif-Regular.ttf.gz")
	outl := f.Outlines.(*glyf.Outlines)
	gid := slices.Index(outl.Names, "commaaccent")
	outl.Names[gid] = "uniF004"
	if err := addCommaAccent(f, "Serif"); err != nil {
		t.Fatal(err)
	}
	if outl.Names[gid] != "commaaccent" {
		t.Errorf("glyph %d is named %q", gid, outl.Names[gid])
	}

	// Mono: the glyph is built from the comma below of scommaaccent
	f = readFont(t, "quire/QuireMono-Regular.ttf.gz")
	outl = f.Outlines.(*glyf.Outlines)
	gid = slices.Index(outl.Names, "commaaccent")
	if gid != len(outl.Glyphs)-1 {
		t.Fatalf("commaaccent is glyph %d of %d", gid, len(outl.Glyphs))
	}
	want := outl.Glyphs[gid]
	outl.Glyphs = outl.Glyphs[:gid]
	outl.Widths = outl.Widths[:gid]
	outl.Names = outl.Names[:gid]
	if err := addCommaAccent(f, "Mono"); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, outl.Glyphs[gid]); diff != "" {
		t.Errorf("wrong commaaccent glyph (-want +got):\n%s", diff)
	}
	if outl.Widths[gid] != outl.Widths[mustCMap(t, f).Lookup('s')] {
		t.Errorf("wrong advance width %d", outl.Widths[gid])
	}
}

func TestAddCommaAccentErrors(t *testing.T) {
	// readCut returns a font of the given family without commaaccent and with
	// the given glyph renamed
	readCut := func(fontName, glyphName string) *sfnt.Font {
		f := readFont(t, "quire/"+fontName+".ttf.gz")
		outl := f.Outlines.(*glyf.Outlines)
		outl.Names[slices.Index(outl.Names, "commaaccent")] = "x"
		if i := slices.Index(outl.Names, glyphName); i >= 0 {
			outl.Names[i] = "y"
		}
		return f
	}

	for _, c := range []struct {
		fontName, family, glyphName string
	}{
		{"QuireSans-Regular", "Sans", ""},
		{"QuireSerif-Regular", "Serif", "uniF004"},
		{"QuireMono-Regular", "Mono", "scommaaccent"},
		{"QuireMono-Regular", "Mono", "uni0326"},
	} {
		f := readCut(c.fontName, c.glyphName)
		if err := addCommaAccent(f, c.family); err == nil {
			t.Errorf("%s without %q: no error", c.fontName, c.glyphName)
		}
	}

	// scommaaccent which is not a composite
	f := readCut("QuireMono-Regular", "")
	outl := f.Outlines.(*glyf.Outlines)
	i := slices.Index(outl.Names, "scommaaccent")
	outl.Glyphs[i] = outl.Glyphs[slices.Index(outl.Names, "s")]
	if err := addCommaAccent(f, "Mono"); err == nil {
		t.Error("simple scommaaccent: no error")
	}
}

func TestAFMGlyphNames(t *testing.T) {
	names, err := afmGlyphNames(readGz(t, "adobe/Courier.afm.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 315 || !slices.IsSorted(names) || !slices.Contains(names, "commaaccent") {
		t.Errorf("wrong glyph names: %d names", len(names))
	}

	if _, err := afmGlyphNames([]byte("not an AFM file")); err == nil {
		t.Error("invalid AFM data: no error")
	}
}

func TestStandardName(t *testing.T) {
	if got := standardName("QuireSerif-Italic"); got != "Times-Italic" {
		t.Errorf("wrong standard font %q", got)
	}
	if got := standardName("QuireSans"); got != "" {
		t.Errorf("standard font %q for an unknown name", got)
	}
}

func TestRenameMacronErrors(t *testing.T) {
	// already renamed: the glyph of U+00AF is no longer called macron
	f := readFont(t, "quire/QuireSans-Regular.ttf.gz")
	if err := renameMacron(f); err == nil {
		t.Error("renamed font: no error")
	}

	f = readFont(t, "quire/QuireSans-Regular.ttf.gz")
	f.CMapTable = nil
	if err := renameMacron(f); err == nil {
		t.Error("missing cmap: no error")
	}
}

func TestRenameMacron(t *testing.T) {
	f := readFont(t, "quire/QuireSerif-Italic.ttf.gz")
	outl := f.Outlines.(*glyf.Outlines)
	cm := mustCMap(t, f)
	bar, accent := cm.Lookup(0x00AF), cm.Lookup(0x02C9)
	outl.Names[bar] = "macron"
	outl.Names[accent] = "firsttonechinese"
	if err := renameMacron(f); err != nil {
		t.Fatal(err)
	}
	if outl.Names[bar] != "uni00AF" || outl.Names[accent] != "macron" {
		t.Errorf("wrong names %q and %q", outl.Names[bar], outl.Names[accent])
	}
}
