// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package font_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/sfnt"
	sfntcff "seehuhn.de/go/sfnt/cff"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	pdfcff "seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/fonttypes"
)

// TestSpaceIsBlank tests that space characters of common fonts are blank.
func TestSpaceIsBlank(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			F := sample.MakeFont()
			gg := F.Layout(nil, 10, " ")
			if len(gg.Seq) != 1 {
				t.Fatalf("expected 1 glyph, got %d", len(gg.Seq))
			}
			geom := F.GetGeometry()
			if !geom.GlyphExtents[gg.Seq[0].GID].IsZero() {
				t.Errorf("expected blank glyph, got %v",
					geom.GlyphExtents[gg.Seq[0].GID])
			}
		})
	}
}

// TestResourceNameDefault asserts that every sample font returns an empty
// string from ResourceName by default.  Users must opt in to a specific
// resource name by setting the font type's Name field.
func TestResourceNameDefault(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			F := sample.MakeFont()
			if got := F.ResourceName(); got != "" {
				t.Errorf("ResourceName() = %q, want \"\"", got)
			}
		})
	}
}

// TestSpaceWidth checks that the encoder records a non-zero width for the
// space character.  The space glyph has an empty outline, so its bounding
// box is zero, but its advance width is non-zero.  A font that confuses
// bounding-box width with advance width would store zero here and, by
// extension, write a zero /W entry to the font dictionary.
func TestSpaceWidth(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			F := sample.MakeFont()
			seq := F.Layout(nil, 1, " ")
			if len(seq.Seq) != 1 {
				t.Fatalf("expected 1 glyph, got %d", len(seq.Seq))
			}
			code, ok := F.Encode(seq.Seq[0].GID, " ")
			if !ok {
				t.Fatal("failed to encode space")
			}
			s := F.Codec().AppendCode(nil, code)
			for info := range F.Codes(s) {
				if info.Width <= 0 {
					t.Errorf("space width = %v, expected > 0", info.Width)
				}
			}
		})
	}
}

func TestToUnicodeSimple1(t *testing.T) {
	for _, sample := range fonttypes.All {
		if sample.Composite {
			continue
		}
		t.Run(sample.Label, func(t *testing.T) {
			const fontSize = 10
			const fontName = "X"

			F := sample.MakeFont()
			seq := F.Layout(nil, fontSize, "ABC")
			if len(seq.Seq) != 3 {
				t.Fatalf("expected 3 glyphs, got %d", len(seq.Seq))
			}

			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)

			b := builder.New(content.Page, nil, pdf.V2_0)
			err := b.SetFontNameInternal(F, fontName)
			if err != nil {
				t.Fatal(err)
			}
			b.TextSetFont(F, fontSize)
			b.TextBegin()
			b.TextShowGlyphs(seq)
			b.TextEnd()

			if b.Err != nil {
				t.Fatal(b.Err)
			}

			// Embed the font and get its reference
			fontRef, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(buf)
			d, err := extract.Dict(pdf.CursorAt(x, nil), fontRef, false)
			if err != nil {
				t.Fatal(err)
			}

			tu := getToUnicode(d)
			if tu != nil {
				t.Errorf("expected ToUnicode file for %q", sample.Label)
			}
		})
	}
}

func TestToUnicodeSimple2(t *testing.T) {
	for _, sample := range fonttypes.All {
		if sample.Composite {
			continue
		}
		t.Run(sample.Label, func(t *testing.T) {
			const fontSize = 10
			const fontName = "X"

			F := sample.MakeFont()
			seq := F.Layout(nil, fontSize, "ABC")
			if len(seq.Seq) != 3 {
				t.Fatalf("expected 3 glyphs, got %d", len(seq.Seq))
			}
			seq.Seq[1].Text = "D" // one glyph with non-standard text

			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)

			b := builder.New(content.Page, nil, pdf.V2_0)
			err := b.SetFontNameInternal(F, fontName)
			if err != nil {
				t.Fatal(err)
			}
			b.TextSetFont(F, fontSize)
			b.TextBegin()
			b.TextShowGlyphs(seq)
			b.TextEnd()

			if b.Err != nil {
				t.Fatal(b.Err)
			}

			// Embed the font and get its reference
			fontRef, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(buf)
			d, err := extract.Dict(pdf.CursorAt(x, nil), fontRef, false)
			if err != nil {
				t.Fatal(err)
			}

			tu := getToUnicode(d)
			if tu == nil {
				t.Fatal("missing ToUnicode file")
			}
			if len(tu.Singles) != 1 {
				t.Fatalf("expected 1 single mapping, got %d", len(tu.Singles))
			}
			if tu.Singles[0].Value != "D" {
				t.Errorf("expected single mapping for 'D', got %q", tu.Singles[0].Value)
			}
		})
	}
}

func getToUnicode(d dict.Dict) *cmap.ToUnicodeFile {
	switch d := d.(type) {
	case *dict.Type1:
		return d.ToUnicode
	case *dict.TrueType:
		return d.ToUnicode
	case *dict.Type3:
		return d.ToUnicode
	case *dict.CIDFontType0:
		return d.ToUnicode
	case *dict.CIDFontType2:
		return d.ToUnicode
	default:
		panic("unknown font dictionary type")
	}
}

// The PostScript name of a Type 1 or CFF font is the FontName of its font
// program, so an embedded subset must call itself by the tagged name the
// /BaseFont and /FontName entries use.  A "glyf" font keeps its name in the
// "name" table, of which this library writes the smallest form which can hold
// one.
func TestSubsetTagReachesFontProgram(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			F := sample.MakeFont()
			ref, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range F.Layout(nil, 10, "Hail Rome").Seq {
				F.Encode(g.GID, g.Text)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			fontDict, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}

			tag, psName, stream := subsetInfo(fontDict)
			if tag == "" || stream == nil {
				t.Skip("not an embedded subset")
			}
			inner, ok := programName(t, stream)
			if !ok {
				t.Skip("the font program carries no name of its own")
			}
			if want := subset.Join(tag, psName); inner != want {
				t.Errorf("the embedded program calls itself %q, want %q", inner, want)
			}
		})
	}
}

// subsetInfo returns the subset tag, the untagged PostScript name and the
// embedded font program of a font dictionary.
func subsetInfo(d dict.Dict) (string, string, *glyphdata.Stream) {
	switch d := d.(type) {
	case *dict.Type1:
		return d.SubsetTag, d.PostScriptName, d.FontFile
	case *dict.TrueType:
		return d.SubsetTag, d.PostScriptName, d.FontFile
	case *dict.CIDFontType0:
		return d.SubsetTag, d.PostScriptName, d.FontFile
	case *dict.CIDFontType2:
		return d.SubsetTag, d.PostScriptName, d.FontFile
	}
	return "", "", nil
}

// programName returns the name the embedded font program gives itself, and
// whether it carries one at all.
func programName(t *testing.T, stream *glyphdata.Stream) (string, bool) {
	t.Helper()

	switch stream.Type {
	case glyphdata.Type1:
		f, err := type1glyphs.FromStream(stream)
		if err != nil {
			t.Fatal(err)
		}
		return f.FontInfo.FontName, true
	case glyphdata.CFF, glyphdata.CFFSimple:
		f, err := cffglyphs.FromStream(stream)
		if err != nil {
			t.Fatal(err)
		}
		return f.FontInfo.FontName, true
	case glyphdata.TrueType, glyphdata.OpenTypeGlyf,
		glyphdata.OpenTypeCFF, glyphdata.OpenTypeCFFSimple:
		f, err := sfntglyphs.FromStream(stream)
		if err != nil {
			t.Fatal(err)
		}
		name := f.FontName
		return name, name != ""
	}
	return "", false
}

// A font program embedded in a PDF file must still know its own name when it
// is read back out, so that a tool which rewrites the file can embed it again
// without inventing one.  Embedding must also not compound subset tags: the
// program carries the tagged name of the subset it is, and a further subset of
// it is named for the font it came from, not for that subset.
func TestFontNameSurvivesRoundTrip(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			first := embedAndExtract(t, sample.MakeFont())
			tag, psName, stream := subsetInfo(first)
			if stream == nil {
				t.Skip("the font is not embedded")
			}
			if tag == "" {
				t.Skip("the font is not a subset")
			}

			inner, ok := programName(t, stream)
			if !ok {
				t.Fatal("the embedded program carries no name")
			}
			if want := subset.Join(tag, psName); inner != want {
				t.Fatalf("the embedded program is named %q, want %q", inner, want)
			}

			// the name the program reports must not gain a second tag when the
			// PDF layer splits it up again
			gotTag, gotName := subset.Split(inner)
			if gotTag != tag || gotName != psName {
				t.Errorf("the name splits into %q + %q, want %q + %q",
					gotTag, gotName, tag, psName)
			}
		})
	}
}

// embedAndExtract writes the font into a document of its own and reads the
// font dictionary back out.
func embedAndExtract(t *testing.T, F font.Layouter) dict.Dict {
	t.Helper()

	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range F.Layout(nil, 10, "Hail Rome").Seq {
		F.Encode(g.GID, g.Text)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(w)
	fontDict, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	return fontDict
}

// Embedding must leave the font data it was given alone.  The same data may be
// used by a second instance, in this or in another document, and a glyph
// renamed here would be renamed there too.
func TestEmbedLeavesSourceGlyphNamesAlone(t *testing.T) {
	for _, tc := range []struct {
		label string
		embed func(*sfnt.Font) (font.Layouter, error)
	}{
		{"cff.Simple", func(f *sfnt.Font) (font.Layouter, error) { return pdfcff.NewSimple(f, nil) }},
		{"cff.Composite", func(f *sfnt.Font) (font.Layouter, error) { return pdfcff.NewComposite(f, nil) }},
		{"opentype.SimpleCFF", func(f *sfnt.Font) (font.Layouter, error) { return opentype.NewSimple(f, nil) }},
		{"opentype.CompositeCFF", func(f *sfnt.Font) (font.Layouter, error) { return opentype.NewComposite(f, nil) }},
	} {
		t.Run(tc.label, func(t *testing.T) {
			src := sourceWithDuplicateGlyphName(t)
			outlines := src.Outlines.(*sfntcff.Outlines)
			before := glyphNames(outlines)

			F, err := tc.embed(src)
			if err != nil {
				t.Fatal(err)
			}

			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(F); err != nil {
				t.Fatal(err)
			}
			for gid := range glyph.ID(src.NumGlyphs()) {
				if _, ok := F.Encode(gid, ""); !ok {
					t.Fatalf("no code was allocated for glyph %d", gid)
				}
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(before, glyphNames(outlines)); diff != "" {
				t.Errorf("embedding renamed the glyphs of the source font (-before +after):\n%s", diff)
			}
		})
	}
}

// sourceWithDuplicateGlyphName returns a font small enough that a document can
// use every glyph, so that nothing is subsetted away, and in which two glyphs
// share a name, as a font program taken out of a PDF file may.  Naming the
// glyphs of such a font means renaming one of the two.
func sourceWithDuplicateGlyphName(t *testing.T) *sfnt.Font {
	t.Helper()

	src, err := makefont.OpenType().Subset([]glyph.ID{0, 1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	subtable := sfntcmap.Format4{'A': 1, 'B': 2, 'C': 3}
	src.CMapTable = sfntcmap.Table{
		{PlatformID: 3, EncodingID: 1}: subtable.Encode(0),
	}

	outlines := src.Outlines.(*sfntcff.Outlines)
	outlines.Glyphs[2] = &sfntcff.Glyph{
		Name:  outlines.Glyphs[1].Name,
		Cmds:  outlines.Glyphs[2].Cmds,
		Width: outlines.Glyphs[2].Width,
	}
	return src
}

// glyphNames returns the name of each glyph, in glyph ID order.
func glyphNames(o *sfntcff.Outlines) []string {
	names := make([]string, len(o.Glyphs))
	for i, g := range o.Glyphs {
		names[i] = g.Name
	}
	return names
}

// embedders lists the ways an sfnt font can be embedded, together with the
// kind of source font each one takes: the CFF embedders need CFF outlines and
// the "glyf" ones need TrueType outlines.
var embedders = []struct {
	label  string
	source func() *sfnt.Font
	embed  func(*sfnt.Font) (font.Layouter, error)
}{
	{"cff.Simple", makefont.OpenType, func(f *sfnt.Font) (font.Layouter, error) { return pdfcff.NewSimple(f, nil) }},
	{"cff.Composite", makefont.OpenType, func(f *sfnt.Font) (font.Layouter, error) { return pdfcff.NewComposite(f, nil) }},
	{"opentype.SimpleCFF", makefont.OpenType, func(f *sfnt.Font) (font.Layouter, error) { return opentype.NewSimple(f, nil) }},
	{"opentype.CompositeCFF", makefont.OpenType, func(f *sfnt.Font) (font.Layouter, error) { return opentype.NewComposite(f, nil) }},
	{"truetype.Simple", makefont.TrueType, func(f *sfnt.Font) (font.Layouter, error) { return truetype.NewSimple(f, nil) }},
	{"truetype.Composite", makefont.TrueType, func(f *sfnt.Font) (font.Layouter, error) { return truetype.NewComposite(f, nil) }},
	{"opentype.SimpleGlyf", makefont.TrueType, func(f *sfnt.Font) (font.Layouter, error) { return opentype.NewSimple(f, nil) }},
	{"opentype.CompositeGlyf", makefont.TrueType, func(f *sfnt.Font) (font.Layouter, error) { return opentype.NewComposite(f, nil) }},
}

// A subset is named by prefixing a tag to the name of the font it was made
// from, and a font file cannot carry a name longer than [subset.MaxNameLen].
// The name a font arrives with may already use every byte of that, so it is
// the untagged name which must leave room for the tag.
func TestEmbedLongFontName(t *testing.T) {
	longName := strings.Repeat("x", subset.MaxNameLen)

	for _, tc := range embedders {
		t.Run(tc.label, func(t *testing.T) {
			src := tc.source()
			src.FontName = longName

			F, err := tc.embed(src)
			if err != nil {
				t.Fatal(err)
			}
			d := embedAndExtract(t, F)

			tag, psName, _ := subsetInfo(d)
			if len(subset.Join(tag, psName)) > subset.MaxNameLen {
				t.Errorf("the font is named %q, which is %d bytes long",
					subset.Join(tag, psName), len(subset.Join(tag, psName)))
			}
			if !strings.HasPrefix(longName, psName) {
				t.Errorf("the font is named %q, which the original name does not start with",
					psName)
			}
		})
	}
}

// A font may name itself in a way it cannot store: a "name" table holds only a
// subset of ASCII, whereas the name may have come from a CFF Name INDEX or the
// /BaseFont entry of a PDF file, where a CJK font is commonly named in UTF-8.
// Embedding settles on a name the font can carry rather than failing, and the
// dictionaries and the embedded program still agree on it.
func TestEmbedFontNameTheProgramCannotCarry(t *testing.T) {
	for _, tc := range embedders {
		t.Run(tc.label, func(t *testing.T) {
			src := tc.source()
			src.FontName = "宋体-Regular"

			F, err := tc.embed(src)
			if err != nil {
				t.Fatal(err)
			}
			d := embedAndExtract(t, F)

			tag, psName, stream := subsetInfo(d)
			if psName == "" {
				t.Error("the font was left unnamed")
			}
			if stream == nil {
				t.Fatal("the font program was not embedded")
			}
			// a "glyf" font cannot store the name it arrived with, so it is
			// named after its family instead; a CFF font keeps the name
			if inner, ok := programName(t, stream); ok {
				if want := subset.Join(tag, psName); inner != want {
					t.Errorf("the embedded program calls itself %q, want %q",
						inner, want)
				}
			}
		})
	}
}

// Writing a font, reading it back and writing it again must name the font the
// same way every time.  The name is what ties the /BaseFont entry, the
// descriptor and the embedded program together, so a name which shifted on
// each pass would eventually name three different things.
func TestFontNameStableAcrossWriteReadWrite(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			first := embedAndExtract(t, sample.MakeFont())
			tag1, psName1, stream1 := subsetInfo(first)

			// write the dictionary we read back out, and read it again
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			ref, err := rm.Embed(first)
			if err != nil {
				t.Fatal(err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
			second, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			tag2, psName2, stream2 := subsetInfo(second)

			if psName1 != psName2 || tag1 != tag2 {
				t.Errorf("the font was named %q, then %q",
					subset.Join(tag1, psName1), subset.Join(tag2, psName2))
			}
			if (stream1 == nil) != (stream2 == nil) {
				t.Error("the font program appeared or vanished")
			}
			if stream1 == nil {
				return
			}

			// the name the program gives itself must be settled too
			name1, ok1 := programName(t, stream1)
			name2, ok2 := programName(t, stream2)
			if ok1 != ok2 || name1 != name2 {
				t.Errorf("the program called itself %q, then %q", name1, name2)
			}
		})
	}
}
