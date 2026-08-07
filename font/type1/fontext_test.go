// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package type1_test

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/fonttypes"
	pdfpage "seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/reader"
	"seehuhn.de/go/postscript/afm"
	pstype1 "seehuhn.de/go/postscript/type1"
)

func TestEmbed(t *testing.T) {
	// step 1: embed a font instance into a simple PDF file
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	fontData := makefont.Type1()
	fontMetrics := makefont.AFM()
	fontInstance, err := type1.New(fontData, fontMetrics)
	if err != nil {
		t.Fatal(err)
	}

	ref, err := rm.Embed(fontInstance)
	if err != nil {
		t.Fatal(err)
	}

	// make sure a few glyphs are included and encoded
	fontInstance.Layout(nil, 12, "Hello")

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// step 2: read back the font and verify that everything is as expected
	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	fontDict, ok := dictObj.(*dict.Type1)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}

	if fontDict.PostScriptName != fontData.FontName {
		t.Errorf("wrong PostScript name: expected %v, got %v",
			fontData.FontName, fontDict.PostScriptName)
	}
	if len(fontDict.SubsetTag) != 6 {
		t.Errorf("wrong subset tag: %q", fontDict.SubsetTag)
	}

	// TODO(voss): more tests
}

// The metrics and the font program come from separate files and need not name
// the same glyphs.  A ligature whose result the font program does not have
// must be ignored: applying it would replace the text with ".notdef".
func TestLigatureToMissingGlyphIgnored(t *testing.T) {
	metrics := makefont.AFM()
	metrics.Glyphs["H"].Ligatures = map[string]string{"a": "NoSuchGlyph"}

	F, err := type1.New(makefont.Type1(), metrics)
	if err != nil {
		t.Fatal(err)
	}

	seq := F.Layout(nil, 10, "Ha")
	if len(seq.Seq) != 2 {
		t.Fatalf("%q laid out as %d glyphs, want 2", "Ha", len(seq.Seq))
	}
	for _, g := range seq.Seq {
		if g.GID == 0 {
			t.Errorf("%q was laid out as .notdef", g.Text)
		}
	}
}

// A kern pair naming a glyph the font program does not have must be ignored:
// an unknown name resolves to glyph ID 0, so the adjustment would be applied
// whenever ".notdef" is laid out next to another glyph.
func TestKernWithMissingGlyphIgnored(t *testing.T) {
	metrics := makefont.AFM()
	metrics.Kern = append(metrics.Kern, afm.KernPair{
		Left: "NoSuchGlyph", Right: "A", Adjust: -500,
	})

	F, err := type1.New(makefont.Type1(), metrics)
	if err != nil {
		t.Fatal(err)
	}

	// "中" is not in the font, so it is laid out as .notdef
	seq := F.Layout(nil, 1000, "中A")
	var want float64
	for _, g := range seq.Seq {
		want += F.Widths[g.GID] * 1000
	}
	if got := seq.TotalWidth(); got != want {
		t.Errorf("a kern of %v was applied next to .notdef", got-want)
	}
}

// A font program without a ".notdef" glyph still gets one in its glyph list,
// and the metrics need not describe it.
func TestMissingNotdefMetrics(t *testing.T) {
	psFont := makefont.Type1()
	metrics := makefont.AFM()
	delete(psFont.Glyphs, ".notdef")
	delete(metrics.Glyphs, ".notdef")

	if _, err := type1.New(psFont, metrics); err != nil {
		t.Fatal(err)
	}
}

// A subsetted font names itself the same way inside and out.  The PostScript
// name of a Type 1 font is the FontName of its font program, and for a subset
// that name carries the tag, so the dictionary, the descriptor and the
// embedded program must all agree on it.
func TestSubsetTagReachesFontProgram(t *testing.T) {
	F, err := type1.New(makefont.Type1(), makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}
	encodeAll(t, F, "Hail")

	fontDict := embedAlone(t, F)
	if len(fontDict.SubsetTag) != 6 {
		t.Fatalf("subset tag %q, want six letters", fontDict.SubsetTag)
	}
	want := subset.Join(fontDict.SubsetTag, fontDict.PostScriptName)

	if got := fontDict.Descriptor.FontName; got != want {
		t.Errorf("descriptor names the font %q, want %q", got, want)
	}

	psFont, err := type1glyphs.FromStream(fontDict.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := psFont.FontInfo.FontName; got != want {
		t.Errorf("the embedded program calls itself %q, want %q", got, want)
	}
}

// Naming the embedded subset must leave the font data it was built from alone:
// that data is shared with every other instance of the same font, so a write
// there would rename the font in every other document too.
func TestSubsetTagLeavesSharedDataAlone(t *testing.T) {
	psFont := makefont.Type1()
	before := psFont.FontName

	F, err := type1.New(psFont, makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}
	encodeAll(t, F, "Hail")
	embedAlone(t, F)

	if psFont.FontName != before {
		t.Errorf("embedding renamed the shared font data to %q, want %q",
			psFont.FontName, before)
	}
}

// A font program whose glyphs have the metrics of one of the standard fonts
// need not be embedded, since the standard font can stand in for it.  A
// program which arrived as a subset is different: the tag says a file carried
// glyph outlines of its own, and those need not look like the standard font
// they happen to be metrically compatible with.
func TestStandardMetricsDoNotDiscardASubset(t *testing.T) {
	// a bare font program, the way one taken out of a PDF file arrives
	psFont := font.Must(standard.Helvetica.New()).Font

	// the untagged font is metrically standard, so the program is left out
	plain, err := type1.New(psFont, nil)
	if err != nil {
		t.Fatal(err)
	}
	encodeAll(t, plain, "Hail")
	if fontDict := embedAlone(t, plain); fontDict.FontFile != nil {
		t.Error("the standard font was embedded although it need not be")
	}

	// the same program, named the way a subset is named
	tagged, err := type1.New(withName(psFont, "ABCDEF+Helvetica"), nil)
	if err != nil {
		t.Fatal(err)
	}
	encodeAll(t, tagged, "Hail")

	fontDict := embedAlone(t, tagged)
	if fontDict.FontFile == nil {
		t.Error("the subset was replaced by the standard font")
	}
	if fontDict.PostScriptName != "Helvetica" {
		t.Errorf("the font is named %q, want %q", fontDict.PostScriptName, "Helvetica")
	}
	if !subset.IsValidTag(fontDict.SubsetTag) {
		t.Errorf("subset tag %q, want six letters", fontDict.SubsetTag)
	}
}

// The dictionary describes the program it carries, so the font is named after
// the program.  Metrics are never embedded, and the name they give the font
// need not be the one the program answers to.
func TestFontNameComesFromTheProgram(t *testing.T) {
	psFont := makefont.Type1()
	metrics := makefont.AFM()
	metrics.FontName = "Other-Regular"

	F, err := type1.New(psFont, metrics)
	if err != nil {
		t.Fatal(err)
	}
	encodeAll(t, F, "Hail")

	fontDict := embedAlone(t, F)
	if got, want := fontDict.PostScriptName, psFont.FontName; got != want {
		t.Errorf("the dictionary names the font %q, want %q", got, want)
	}
}

// The subset tag describes the glyphs taken out of the font program, so
// metrics covering glyphs the program does not have must not change it: the
// glyph IDs a subset is made of index the program's glyph list.
func TestSubsetTagIgnoresTheMetricsGlyphCount(t *testing.T) {
	plain, err := type1.New(makefont.Type1(), makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}
	encodeAll(t, plain, "Hail")

	padded := makefont.AFM()
	padded.Glyphs["glyphTheProgramLacks"] = &afm.GlyphInfo{}
	extra, err := type1.New(makefont.Type1(), padded)
	if err != nil {
		t.Fatal(err)
	}
	encodeAll(t, extra, "Hail")

	want := embedAlone(t, plain).SubsetTag
	if got := embedAlone(t, extra).SubsetTag; got != want {
		t.Errorf("a glyph added to the metrics changed the tag from %q to %q",
			want, got)
	}
}

// withName returns a copy of the font program under a different PostScript
// name, leaving the data it was copied from alone.
func withName(f *pstype1.Font, name string) *pstype1.Font {
	fontInfo := *f.FontInfo
	fontInfo.FontName = name
	other := *f
	other.FontInfo = &fontInfo
	return &other
}

// The font descriptor describes the subset which is actually embedded, so its
// bounding box covers the glyphs the document uses rather than the whole font.
func TestDescriptorBBoxCoversSubsetOnly(t *testing.T) {
	small, err := type1.New(makefont.Type1(), makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}
	encodeAll(t, small, ".")

	tall, err := type1.New(makefont.Type1(), makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}
	encodeAll(t, tall, ".H")

	smallBBox := embedAlone(t, small).Descriptor.FontBBox
	tallBBox := embedAlone(t, tall).Descriptor.FontBBox

	if smallBBox.URy >= tallBBox.URy {
		t.Errorf("a full stop reaches to %v, as high as %q at %v",
			smallBBox.URy, "H", tallBBox.URy)
	}
}

// Clone copies the Instance struct wholesale and then replaces the one field
// which carries per-document state.  A field added later is shared silently,
// and sharing one which turns out to hold per-document state would let two
// documents corrupt each other.  Pinning the count forces that decision to be
// made rather than missed.
func TestCloneConsidersEveryField(t *testing.T) {
	const reviewed = 13
	if n := reflect.TypeFor[type1.Instance]().NumField(); n != reviewed {
		t.Errorf("Instance has %d fields, want %d: decide whether Clone must "+
			"give the new field a value of its own, then update this count", n, reviewed)
	}
}

// A clone serves a document of its own: the codes it allocates are its own,
// and the document it is embedded into shows the text laid out with it.
func TestCloneEncodesForItsOwnDocument(t *testing.T) {
	original, err := type1.New(makefont.Type1(), makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}
	clone := original.Clone()

	// The two texts share no glyphs, so a code carried over from one instance
	// to the other would decode to the wrong character.
	const firstText = "Hail"
	const secondText = "Rome"

	firstCodes := encodeAll(t, original, firstText)
	secondCodes := encodeAll(t, clone, secondText)

	firstDict := embedAlone(t, original)
	secondDict := embedAlone(t, clone)

	if got := decode(firstDict, firstCodes); got != firstText {
		t.Errorf("first document shows %q, want %q", got, firstText)
	}
	if got := decode(secondDict, secondCodes); got != secondText {
		t.Errorf("second document shows %q, want %q", got, secondText)
	}

	// Neither document carries the other's glyphs, so each embeds the four its
	// own instance used and no others.
	if diff := cmp.Diff([]string{".notdef", "H", "a", "i", "l"},
		embeddedGlyphs(t, firstDict)); diff != "" {
		t.Errorf("first document (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{".notdef", "R", "e", "m", "o"},
		embeddedGlyphs(t, secondDict)); diff != "" {
		t.Errorf("second document (-want +got):\n%s", diff)
	}
}

// A clone is interchangeable with an instance built from the font data afresh:
// same layout, same codes, same font dictionary.
func TestCloneMatchesFreshInstance(t *testing.T) {
	base, err := type1.New(makefont.Type1(), makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}
	clone := base.Clone()

	fresh, err := type1.New(makefont.Type1(), makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}

	const text = "Wavy fjord ffi 123"

	if diff := cmp.Diff(fresh.Layout(nil, 10, text), clone.Layout(nil, 10, text)); diff != "" {
		t.Errorf("layout differs (-fresh +clone):\n%s", diff)
	}

	freshCodes := encodeAll(t, fresh, text)
	cloneCodes := encodeAll(t, clone, text)
	if diff := cmp.Diff(freshCodes, cloneCodes); diff != "" {
		t.Errorf("codes differ (-fresh +clone):\n%s", diff)
	}

	freshDict := embedAlone(t, fresh)
	cloneDict := embedAlone(t, clone)

	if freshDict.PostScriptName != cloneDict.PostScriptName {
		t.Errorf("PostScript name %q, want %q",
			cloneDict.PostScriptName, freshDict.PostScriptName)
	}
	if freshDict.SubsetTag != cloneDict.SubsetTag {
		t.Errorf("subset tag %q, want %q", cloneDict.SubsetTag, freshDict.SubsetTag)
	}
	if diff := cmp.Diff(freshDict.Descriptor, cloneDict.Descriptor); diff != "" {
		t.Errorf("descriptor differs (-fresh +clone):\n%s", diff)
	}
	if diff := cmp.Diff(freshDict.Width, cloneDict.Width); diff != "" {
		t.Errorf("widths differ (-fresh +clone):\n%s", diff)
	}
	if diff := cmp.Diff(encodingVector(freshDict), encodingVector(cloneDict)); diff != "" {
		t.Errorf("encoding differs (-fresh +clone):\n%s", diff)
	}
	if diff := cmp.Diff(embeddedGlyphs(t, freshDict), embeddedGlyphs(t, cloneDict)); diff != "" {
		t.Errorf("embedded glyphs differ (-fresh +clone):\n%s", diff)
	}
	if diff := cmp.Diff(decode(freshDict, freshCodes), decode(cloneDict, cloneCodes)); diff != "" {
		t.Errorf("text differs (-fresh +clone):\n%s", diff)
	}
}

// Using one instance must leave the other where it was, whichever way round
// the two are used.
func TestCloneLeavesOriginalAlone(t *testing.T) {
	original, err := type1.New(makefont.Type1(), makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}
	clone := original.Clone()

	const text = "Hail"

	// the clone runs first, so any code it allocates predates the original's
	viaClone := encodeAll(t, clone, text)
	viaOriginal := encodeAll(t, original, text)

	if diff := cmp.Diff(viaClone, viaOriginal); diff != "" {
		t.Errorf("the first use influenced the second (-clone +original):\n%s", diff)
	}
}

// encodeAll lays out text and allocates a character code for every glyph,
// returning the string a content stream would show.
func encodeAll(t *testing.T, F *type1.Instance, text string) pdf.String {
	t.Helper()

	var codes pdf.String
	for _, g := range F.Layout(nil, 10, text).Seq {
		c, ok := F.Encode(g.GID, g.Text)
		if !ok {
			t.Fatalf("no code allocated for %q", g.Text)
		}
		codes = append(codes, byte(c))
	}
	return codes
}

// embedAlone writes F into a document of its own and reads the font dictionary
// back out.
func embedAlone(t *testing.T, F *type1.Instance) *dict.Type1 {
	t.Helper()

	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	fontDict, ok := dictObj.(*dict.Type1)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return fontDict
}

// decode reads codes back as text, the way a viewer extracting text would.
func decode(fontDict *dict.Type1, codes pdf.String) string {
	s := &strings.Builder{}
	for code := range fontDict.MakeFont().Codes(codes) {
		s.WriteString(code.Text)
	}
	return s.String()
}

// embeddedGlyphs returns the sorted names of the glyphs in the font program
// the document carries.  A subset holds exactly the glyphs its instance
// encoded, together with ".notdef", so this is the set the document can draw.
func embeddedGlyphs(t *testing.T, fontDict *dict.Type1) []string {
	t.Helper()

	psFont, err := type1glyphs.FromStream(fontDict.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	return slices.Sorted(maps.Keys(psFont.Glyphs))
}

// encodingVector returns the glyph name the font dictionary gives for each of
// the 256 character codes.
func encodingVector(fontDict *dict.Type1) []string {
	names := make([]string, 256)
	for c := range names {
		names[c] = fontDict.Encoding(byte(c))
	}
	return names
}

func TestTextContent(t *testing.T) {
	text := `“Hello World!”`

	// step 1: embed a Type 1 font into a simple PDF document
	// and make sure all the characters from the text are included.
	mem := memfile.New()
	page, err := document.WriteSinglePage(mem, document.A5, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}
	pageRef := page.Out.Alloc() // fix the reference for the page dictionary
	page.Ref = pageRef

	F := fonttypes.Type1WithMetrics()
	page.TextBegin()
	page.TextSetFont(F, 12)
	page.TextFirstLine(100, 100)
	page.TextShow(text)
	page.TextEnd()

	// keep a reference to the font
	ref, _ := page.RM.Embed(F)

	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	// os.WriteFile("debug.pdf", mem.Data, 0644)

	// step 2: extract the encoded string from the content stream
	var textString pdf.String
	x := pdf.NewExtractor(page.Out)
	r := reader.New(x)
	r.EveryOp = func(op string, args []pdf.Object) error {
		switch op {
		case "Tj":
			textString = append(textString, args[0].(pdf.String)...)
		case "TJ":
			a := args[0].(pdf.Array)
			for _, arg := range a {
				switch arg := arg.(type) {
				case pdf.String:
					textString = append(textString, arg...)
				}
			}
		}
		return nil
	}
	pg, err := pdf.Decode(pdf.CursorAt(x, nil), pageRef, pdfpage.Decode)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ProcessPage(pg); err != nil {
		t.Fatal(err)
	}

	// step 3: read back the font dictionary to inspect it.
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	fontDict, ok := dictObj.(*dict.Type1)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}

	s := &strings.Builder{}
	E := fontDict.MakeFont()
	for code := range E.Codes(textString) {
		s.WriteString(code.Text)
	}
	if s.String() != text {
		t.Fatalf("expected %q, got %q", text, s.String())
	}
}
