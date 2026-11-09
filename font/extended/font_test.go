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

package extended

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/opentype/gtab"
	"seehuhn.de/go/sfnt/parser"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// the standard font each constant extends
var equivalent = map[Font]standard.Font{
	QuireSansRegular:     standard.Helvetica,
	QuireSansBold:        standard.HelveticaBold,
	QuireSansItalic:      standard.HelveticaOblique,
	QuireSansBoldItalic:  standard.HelveticaBoldOblique,
	QuireSerifRegular:    standard.TimesRoman,
	QuireSerifBold:       standard.TimesBold,
	QuireSerifItalic:     standard.TimesItalic,
	QuireSerifBoldItalic: standard.TimesBoldItalic,
	QuireMonoRegular:     standard.Courier,
	QuireMonoBold:        standard.CourierBold,
	QuireMonoItalic:      standard.CourierOblique,
	QuireMonoBoldItalic:  standard.CourierBoldOblique,
	QuireSymbol:          standard.Symbol,
	QuireDingbats:        standard.ZapfDingbats,
}

// sampleText returns text which the font covers.
func sampleText(f Font) string {
	switch f {
	case QuireSymbol:
		return "\u2200\u2208\u03b1\u211c"
	case QuireDingbats:
		return "\u2660\u2665"
	default:
		return "Hello, World! \u00e9\u0105"
	}
}

// TestNewAll checks that every font embeds as a simple and as a composite
// font, and that the result can be read back.
func TestNewAll(t *testing.T) {
	if len(All) != len(equivalent) {
		t.Fatalf("%d fonts, want %d", len(All), len(equivalent))
	}

	for _, f := range All {
		for _, composite := range []bool{false, true} {
			name := fmt.Sprintf("%s/composite=%t", f.PostScriptName(), composite)
			t.Run(name, func(t *testing.T) {
				var opt *embed.Options
				if composite {
					opt = &embed.Options{CMap: cmap.IdentityH}
				}

				w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
				rm := pdf.NewResourceManager(w)

				F, err := f.New(opt)
				if err != nil {
					t.Fatal(err)
				}
				ref, err := rm.Embed(F)
				if err != nil {
					t.Fatal(err)
				}
				text := sampleText(f)
				seq := F.Layout(nil, 10, text)
				if len(seq.Seq) == 0 {
					t.Fatal("empty layout")
				}
				for _, g := range seq.Seq {
					if g.GID == 0 {
						t.Errorf("glyph for %q is .notdef", g.Text)
					}
					if _, ok := F.Encode(g.GID, g.Text); !ok {
						t.Fatalf("cannot encode %q", g.Text)
					}
				}
				if err := rm.Close(); err != nil {
					t.Fatal(err)
				}

				x := pdf.NewExtractor(w)
				d, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
				if err != nil {
					t.Fatal(err)
				}

				var desc *font.Descriptor
				var psName string
				switch d := d.(type) {
				case *dict.TrueType:
					desc, psName = d.Descriptor, d.PostScriptName
				case *dict.Type1:
					desc, psName = d.Descriptor, d.PostScriptName
				case *dict.CIDFontType2:
					desc, psName = d.Descriptor, d.PostScriptName
				case *dict.CIDFontType0:
					desc, psName = d.Descriptor, d.PostScriptName
				default:
					t.Fatalf("unexpected font dictionary type %T", d)
				}
				if composite != isComposite(d) {
					t.Errorf("composite=%t, but got %T", composite, d)
				}
				if psName != f.PostScriptName() {
					t.Errorf("PostScript name %q, want %q", psName, f.PostScriptName())
				}

				// Quire Mono has no serifs, unlike Courier
				std := font.Must(f.Standard().New())
				wantSerif := std.Descriptor.IsSerif && !isMono(f)
				if desc.IsSerif != wantSerif {
					t.Errorf("IsSerif=%v, want %v", desc.IsSerif, wantSerif)
				}
				if (f == QuireSymbol || f == QuireDingbats) && !desc.IsSymbolic {
					t.Error("IsSymbolic=false")
				}
				if desc.IsFixedPitch != std.Descriptor.IsFixedPitch {
					t.Errorf("IsFixedPitch=%v, but %s has %v",
						desc.IsFixedPitch, f.Standard(), std.Descriptor.IsFixedPitch)
				}
			})
		}
	}
}

// TestSimpleSymbolic checks the symbolic flag and the cmap of simple fonts.
// A font is symbolic if it uses glyphs outside the Adobe Standard Latin
// character set, so the Latin faces are non-symbolic for text within that
// set, and then carry a (3,1) cmap subtable in the embedded font program.
func TestSimpleSymbolic(t *testing.T) {
	for _, f := range All {
		t.Run(f.PostScriptName(), func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			F, err := f.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			ref, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			text := "Hello, World! \u00e9"
			if f == QuireSymbol || f == QuireDingbats {
				text = sampleText(f)
			}
			for _, g := range F.Layout(nil, 10, text).Seq {
				if _, ok := F.Encode(g.GID, g.Text); !ok {
					t.Fatalf("cannot encode %q", g.Text)
				}
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			d, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}

			switch d := d.(type) {
			case *dict.TrueType:
				if d.Descriptor.IsSymbolic {
					t.Error("Latin face is symbolic")
				}
				checkUnicodeCMap(t, d)
			case *dict.Type1:
				if f != QuireSymbol && f != QuireDingbats {
					t.Fatalf("Latin face is a %T", d)
				}
				if !d.Descriptor.IsSymbolic {
					t.Error("font is not symbolic")
				}
			default:
				t.Fatalf("unexpected font dictionary type %T", d)
			}
		})
	}
}

// checkUnicodeCMap checks that the embedded font program of a simple TrueType
// font has a (3,1) cmap subtable, so that the font can be used with a
// non-symbolic font descriptor.
func checkUnicodeCMap(t *testing.T, d *dict.TrueType) {
	t.Helper()
	if d.FontFile == nil {
		t.Error("font is not embedded")
		return
	}
	buf := &bytes.Buffer{}
	if err := d.FontFile.WriteTo(buf, &glyphdata.Lengths{}); err != nil {
		t.Fatal(err)
	}
	info, err := sfnt.Read(bytes.NewReader(buf.Bytes()), parser.NewBudget(int64(buf.Len())))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := info.CMapTable[sfntcmap.Key{PlatformID: 3, EncodingID: 1}]; !ok {
		t.Error("embedded font program has no (3,1) cmap subtable")
	}
}

func isMono(f Font) bool {
	return f >= QuireMonoRegular && f <= QuireMonoBoldItalic
}

func isComposite(d dict.Dict) bool {
	switch d.(type) {
	case *dict.CIDFontType2, *dict.CIDFontType0:
		return true
	}
	return false
}

// TestStandard checks that each font extends the standard font of the same
// family and style.
func TestStandard(t *testing.T) {
	for _, f := range All {
		if got, want := f.Standard(), equivalent[f]; got != want {
			t.Errorf("%s: Standard()=%s, want %s", f, got, want)
		}
	}
}

// TestStandardUnknownFont checks that a Font value this package does not name
// has no standard counterpart.
func TestStandardUnknownFont(t *testing.T) {
	for _, f := range []Font{-1, Font(len(All))} {
		if got := f.Standard(); got != "" {
			t.Errorf("font %d: Standard()=%q", int(f), got)
		}
	}
}

func TestPostScriptName(t *testing.T) {
	seen := make(map[string]bool)
	for _, f := range All {
		name := f.PostScriptName()
		if name == "" || seen[name] {
			t.Errorf("font %d: bad or duplicate name %q", int(f), name)
		}
		seen[name] = true
		if name != f.String() {
			t.Errorf("%q: String()=%q", name, f.String())
		}

		// the name is the one of the font file
		info, err := parsed[f]()
		if err != nil {
			t.Fatal(err)
		}
		if info.FontName != name {
			t.Errorf("font file is called %q, want %q", info.FontName, name)
		}
		if info.IsVariable() {
			t.Errorf("%s: unexpected variable font", name)
		}
	}
}

// TestGoString checks that %#v prints the names of the constants.
func TestGoString(t *testing.T) {
	want := map[Font]string{
		QuireSansRegular:     "extended.QuireSansRegular",
		QuireSansBold:        "extended.QuireSansBold",
		QuireSansItalic:      "extended.QuireSansItalic",
		QuireSansBoldItalic:  "extended.QuireSansBoldItalic",
		QuireSerifRegular:    "extended.QuireSerifRegular",
		QuireSerifBold:       "extended.QuireSerifBold",
		QuireSerifItalic:     "extended.QuireSerifItalic",
		QuireSerifBoldItalic: "extended.QuireSerifBoldItalic",
		QuireMonoRegular:     "extended.QuireMonoRegular",
		QuireMonoBold:        "extended.QuireMonoBold",
		QuireMonoItalic:      "extended.QuireMonoItalic",
		QuireMonoBoldItalic:  "extended.QuireMonoBoldItalic",
		QuireSymbol:          "extended.QuireSymbol",
		QuireDingbats:        "extended.QuireDingbats",
		Font(-1):             "extended.Font(-1)",
		Font(len(All)):       fmt.Sprintf("extended.Font(%d)", len(All)),
	}
	if len(want) != len(All)+2 {
		t.Fatalf("test table has %d entries, want %d", len(want), len(All)+2)
	}
	for f, name := range want {
		if got := fmt.Sprintf("%#v", f); got != name {
			t.Errorf("font %d: %%#v gives %q, want %q", int(f), got, name)
		}
	}
}

// TestNewConcurrent checks that instances built concurrently from the shared
// parsed font data can be used and embedded at the same time.  Run with -race.
func TestNewConcurrent(t *testing.T) {
	fonts := []Font{QuireSansRegular, QuireSerifBoldItalic, QuireMonoRegular, QuireSymbol}
	var wg sync.WaitGroup
	for _, f := range fonts {
		for _, composite := range []bool{false, true} {
			for range 2 {
				wg.Go(func() {
					var opt *embed.Options
					if composite {
						opt = &embed.Options{CMap: cmap.IdentityH}
					}
					w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
					rm := pdf.NewResourceManager(w)
					F, err := f.New(opt)
					if err != nil {
						t.Error(err)
						return
					}
					if _, err := rm.Embed(F); err != nil {
						t.Error(err)
						return
					}
					for _, g := range F.Layout(nil, 10, sampleText(f)).Seq {
						F.Encode(g.GID, g.Text)
					}
					if err := rm.Close(); err != nil {
						t.Error(err)
					}
				})
			}
		}
	}
	wg.Wait()
}

// TestGreekCyrillic checks that Greek and Cyrillic text lays out without
// .notdef glyphs in a composite font.
func TestGreekCyrillic(t *testing.T) {
	texts := []string{
		"\u0391\u03b1\u0392\u03b2\u0393\u03b3 \u03c9\u03a9\u03ac\u03ce",
		"\u0410\u0431\u0432\u0433\u0434 \u042f\u044f\u0401\u0451",
	}
	for _, f := range All[:QuireMonoBoldItalic+1] {
		F, err := f.New(&embed.Options{CMap: cmap.IdentityH})
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range texts {
			seq := F.Layout(nil, 10, text)
			if len(seq.Seq) != len([]rune(text)) {
				t.Errorf("%s: %d glyphs for %q", f, len(seq.Seq), text)
			}
			for _, g := range seq.Seq {
				if g.GID == 0 && strings.TrimSpace(g.Text) != "" {
					t.Errorf("%s: no glyph for %q", f, g.Text)
				}
			}
		}
	}
}

// TestNewIndependent checks that two instances of the same font allocate
// character codes independently.
func TestNewIndependent(t *testing.T) {
	first, err := QuireSerifRegular.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := QuireSerifRegular.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	gid := first.Layout(nil, 10, "A").Seq[0].GID
	if _, ok := first.Encode(gid, "A"); !ok {
		t.Fatal("no code was allocated")
	}
	if first.CodesRemaining() == second.CodesRemaining() {
		t.Error("a code allocated in one instance was taken from the other")
	}
}

// TestNewVariations checks that variations are rejected, since the fonts are
// static.
func TestNewVariations(t *testing.T) {
	_, err := QuireSansRegular.New(&embed.Options{
		Variations: map[string]float64{"wght": 700},
	})
	if err == nil {
		t.Error("expected an error for variations of a static font")
	}
}

// TestNewUnknownFont checks that a Font value this package does not name
// is reported as an error.
func TestNewUnknownFont(t *testing.T) {
	for _, f := range []Font{-1, Font(len(All))} {
		if _, err := f.New(nil); err == nil {
			t.Errorf("expected an error for font %d", int(f))
		}
		if name := f.PostScriptName(); name != "" {
			t.Errorf("font %d has name %q", int(f), name)
		}
	}
}

// TestLayoutFeatures checks the layout features of the font files against the
// list in the package documentation.
func TestLayoutFeatures(t *testing.T) {
	for _, f := range All {
		info, err := parsed[f]()
		if err != nil {
			t.Fatal(err)
		}

		var gsub, gpos []string
		switch {
		case f == QuireSymbol || f == QuireDingbats:
			if info.Gsub != nil || info.Gpos != nil {
				t.Errorf("%s: unexpected layout tables", f)
			}
			continue
		case isMono(f):
			gsub = []string{"subs", "sups"}
			if f != QuireMonoRegular {
				gpos = []string{"mark"}
			}
		case f <= QuireSansBoldItalic:
			gsub = []string{"liga", "subs", "sups"}
			gpos = []string{"kern", "mark", "mkmk"}
		default:
			gsub = []string{"liga", "subs", "sups"}
			gpos = []string{"kern", "mark"}
		}
		italic := f == QuireSansItalic || f == QuireSansBoldItalic ||
			f == QuireSerifItalic || f == QuireSerifBoldItalic
		if italic {
			gsub = append(gsub, "locl")
		}

		if got := usedFeatures(info.Gsub); !slices.Equal(got, slices.Sorted(slices.Values(gsub))) {
			t.Errorf("%s: GSUB features %v, want %v", f, got, gsub)
		}
		if got := usedFeatures(info.Gpos); !slices.Equal(got, gpos) {
			t.Errorf("%s: GPOS features %v, want %v", f, got, gpos)
		}

		// "ccmp" and "dlig" are declared without lookups
		if info.Gsub != nil {
			for _, ft := range info.Gsub.FeatureList {
				if (ft.Tag == "ccmp" || ft.Tag == "dlig") && len(ft.Lookups) > 0 {
					t.Errorf("%s: feature %q has lookups", f, ft.Tag)
				}
			}
		}
	}
}

// TestLigatures checks that Quire Sans and Quire Serif form the fi and fl
// ligatures by default, that Quire Mono does not, and that the text of a
// ligature survives embedding.
func TestLigatures(t *testing.T) {
	for _, f := range []Font{QuireSansRegular, QuireSerifBoldItalic, QuireMonoRegular} {
		t.Run(f.String(), func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			F, err := f.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			ref, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}

			seq := F.Layout(nil, 10, "fifl")
			var want []string
			if isMono(f) {
				want = []string{"f", "i", "f", "l"}
			} else {
				want = []string{"fi", "fl"}
			}
			var got []string
			var codes []byte
			for _, g := range seq.Seq {
				got = append(got, g.Text)
				code, ok := F.Encode(g.GID, g.Text)
				if !ok {
					t.Fatalf("cannot encode %q", g.Text)
				}
				codes = append(codes, byte(code))
			}
			if !slices.Equal(got, want) {
				t.Fatalf("glyph texts %q, want %q", got, want)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			d, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			tt, ok := d.(*dict.TrueType)
			if !ok {
				t.Fatalf("unexpected font dictionary type %T", d)
			}
			for i, code := range codes {
				text, ok := "", false
				if tt.ToUnicode != nil {
					text, ok = tt.ToUnicode.Lookup([]byte{code})
				}
				if !ok {
					text = names.ToUnicode(tt.Encoding(code), "")
				}
				if text != want[i] {
					t.Errorf("code %d: text %q, want %q", code, text, want[i])
				}
			}
		})
	}
}

// usedFeatures returns the sorted tags of the features which have lookups.
func usedFeatures(info *gtab.Info) []string {
	if info == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, ft := range info.FeatureList {
		if len(ft.Lookups) > 0 {
			seen[string(ft.Tag)] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// TestSimpleSymbolGlyphs checks that glyphs of Quire Symbol and Quire
// Dingbats, embedded as simple fonts, resolve to the right glyph names when
// the font is read back.
func TestSimpleSymbolGlyphs(t *testing.T) {
	cases := []struct {
		font Font
		text string
		want string
	}{
		{QuireSymbol, "α", "alpha"},
		{QuireSymbol, "∀", "universal"},
		{QuireDingbats, "✁", "a1"},
		{QuireDingbats, "♠", "a109"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			F, err := c.font.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			ref, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			seq := F.Layout(nil, 10, c.text)
			if len(seq.Seq) != 1 {
				t.Fatalf("%d glyphs for %q", len(seq.Seq), c.text)
			}
			code, ok := F.Encode(seq.Seq[0].GID, c.text)
			if !ok {
				t.Fatalf("cannot encode %q", c.text)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			d, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			d1, ok := d.(*dict.Type1)
			if !ok {
				t.Fatalf("unexpected font dictionary type %T", d)
			}
			program, err := cffglyphs.FromStream(d1.FontFile)
			if err != nil {
				t.Fatal(err)
			}

			name := d1.Encoding(byte(code))
			if name == encoding.UseBuiltin {
				name = program.Outlines.BuiltinEncoding()[code]
			}
			if name != c.want {
				t.Errorf("code %d: got glyph %q, want %q", code, name, c.want)
			}
			if !slices.ContainsFunc(program.Outlines.Glyphs, func(g *cff.Glyph) bool {
				return g.Name == c.want
			}) {
				t.Errorf("glyph %q missing from the embedded font", c.want)
			}
		})
	}
}
