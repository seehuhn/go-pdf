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

package main

import (
	"bytes"
	"fmt"
	"os"

	"golang.org/x/image/font/gofont/goregular"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
)

func main() {
	err := generateSampleFile("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func generateSampleFile(fname string) error {
	opts := &pdf.WriterOptions{
		HumanReadable: true,
	}
	out, err := document.CreateSinglePage(fname, document.A5r, pdf.V2_0, opts)
	if err != nil {
		return err
	}

	cmap := &cmap.InfoNew{
		Name: "Test",
		ROS: &cmap.CIDSystemInfo{
			Registry:   "Adobe",
			Ordering:   "Japan1",
			Supplement: 0,
		},
		CodeSpaceRange: charcode.CodeSpaceRange{
			{Low: []byte{0x30, 0x30}, High: []byte{0x32, 0x32}},
		},
		CIDRanges: []cmap.RangeNew{
			{
				First: []byte{0x30, 0x30},
				Last:  []byte{0x32, 0x32},
				Value: 34, // 'A'
			},
		},
	}

	F, err := NewTestFont(out.RM, cmap)
	if err != nil {
		return err
	}

	out.TextSetFont(F, 20)
	out.TextBegin()
	out.TextFirstLine(50, 370)
	out.TextShowRaw(pdf.String("000102"))
	out.TextSecondLine(0, -40)
	out.TextShowRaw(pdf.String("101112"))
	out.TextNextLine()
	out.TextShowRaw(pdf.String("202122"))
	out.TextEnd()

	err = out.Close()
	if err != nil {
		return err
	}
	return nil
}

// testFont is a small subset of the Go Regular font, with a configurable cmap.
// This is only useful for testing.
//
// For simplicity we use this structure both as the `font.Font` before
// embedding, and as the `font.Embedded` after embedding.
type testFont struct {
	ttf   *sfnt.Font
	cmap  *cmap.InfoNew
	codec *charcode.Codec
}

// NewTestFont creates a new test font with the given cmap.
func NewTestFont(rm *pdf.ResourceManager, cmap *cmap.InfoNew) (*testFont, error) {
	// Create a font with just the glyphs for "ABC".
	ttf, err := sfnt.Read(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, fmt.Errorf("gofont: %w", err)
	}
	ttf.Gsub = nil
	ttf.Gpos = nil

	glyphs := []glyph.ID{0} // always include the .notdef glyph
	lookup, err := ttf.CMapTable.GetBest()
	if err != nil {
		return nil, err
	}
	for _, r := range "ABCDEFGHI" {
		gid := lookup.Lookup(r)
		glyphs = append(glyphs, gid)
	}
	ttf, err = ttf.Subset(glyphs)
	if err != nil {
		return nil, err
	}

	// fix all glyph widths to 2000 PDF glyphs space units
	outlines := ttf.Outlines.(*glyf.Outlines)
	dw := funit.Int16(2 * ttf.UnitsPerEm)
	for i := range outlines.Widths {
		outlines.Widths[i] = dw
	}

	codec, err := charcode.NewCodec(cmap.CodeSpaceRange)
	if err != nil {
		return nil, err
	}

	return &testFont{
		ttf:   ttf,
		cmap:  cmap,
		codec: codec,
	}, nil
}

// PostScriptName returns the PostScript name of the font.
// This implements the [font.Font] interface.
func (f *testFont) PostScriptName() string {
	return "Test"
}

// Embed adds the font to a PDF file.
// This implements the [font.Font] interface.
func (f *testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Out.Alloc()
	cidFontDictRef := rm.Out.Alloc()
	fontDescriptorRef := rm.Out.Alloc()
	cidToGIDRef := rm.Out.Alloc()
	fontFileRef := rm.Out.Alloc()

	rosRef, _, err := pdf.ResourceManagerEmbed(rm, f.cmap.ROS)
	if err != nil {
		return nil, nil, err
	}
	cmapRef, _, err := pdf.ResourceManagerEmbed(rm, f.cmap)
	if err != nil {
		return nil, nil, err
	}

	bbox := f.ttf.BBox()
	q := 1000 / float64(f.ttf.UnitsPerEm)
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name("Test"),
		"Encoding":        cmapRef,
		"DescendantFonts": pdf.Array{cidFontDictRef},
	}
	err = rm.Out.Put(fontDictRef, fontDict)
	if err != nil {
		return nil, nil, err
	}

	cidFontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType2"),
		"BaseFont":       pdf.Name("Test"),
		"CIDSystemInfo":  rosRef,
		"FontDescriptor": fontDescriptorRef,
		"DW":             pdf.Integer(2000),
		"CIDToGIDMap":    cidToGIDRef,
	}
	err = rm.Out.Put(cidFontDictRef, cidFontDict)
	if err != nil {
		return nil, nil, err
	}

	cidToGID := make([]byte, 43*2)
	cidToGID[2*34+1] = 1 // CID 34 = GID 1 (A)
	cidToGID[2*35+1] = 2 // CID 35 = GID 2 (B)
	cidToGID[2*36+1] = 3 // CID 36 = GID 3 (C)
	cidToGID[2*37+1] = 4 // CID 37 = GID 4 (D)
	cidToGID[2*38+1] = 5 // CID 38 = GID 5 (E)
	cidToGID[2*39+1] = 6 // CID 39 = GID 6 (F)
	cidToGID[2*40+1] = 7 // CID 40 = GID 7 (G)
	cidToGID[2*41+1] = 8 // CID 41 = GID 8 (H)
	cidToGID[2*42+1] = 9 // CID 42 = GID 9 (I)
	cidToGIDStream, err := rm.Out.OpenStream(cidToGIDRef, nil, pdf.FilterASCIIHex{})
	if err != nil {
		return nil, nil, err
	}
	_, err = cidToGIDStream.Write(cidToGID)
	if err != nil {
		return nil, nil, err
	}
	err = cidToGIDStream.Close()
	if err != nil {
		return nil, nil, err
	}

	fd := &font.Descriptor{
		FontName:  "Test",
		FontBBox:  fontBBox,
		Ascent:    f.ttf.Ascent.AsFloat(q),
		Descent:   f.ttf.Descent.AsFloat(q),
		CapHeight: f.ttf.CapHeight.AsFloat(q),
	}
	fontDescriptor := fd.AsDict()
	fontDescriptor["FontFile2"] = fontFileRef
	err = rm.Out.Put(fontDescriptorRef, fontDescriptor)
	if err != nil {
		return nil, nil, err
	}

	var filters []pdf.Filter
	opt := rm.Out.GetOptions()
	if opt.HasAny(pdf.OptPretty) {
		filters = append(filters, pdf.FilterASCII85{})
	}
	filters = append(filters, pdf.FilterCompress{})

	length1 := pdf.NewPlaceholder(rm.Out, 10)
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("TrueType"),
		"Length1": length1,
	}
	fontFileStream, err := rm.Out.OpenStream(fontFileRef, fontFileDict, filters...)
	if err != nil {
		return nil, nil, err
	}
	n, err := f.ttf.WriteTrueTypePDF(fontFileStream)
	if err != nil {
		return nil, nil, err
	}
	err = length1.Set(pdf.Integer(n))
	if err != nil {
		return nil, nil, err
	}
	err = fontFileStream.Close()
	if err != nil {
		return nil, nil, err
	}

	return fontDictRef, f, nil
}

// WritingMode returns [cmap.Horizontal].
// This implements the [font.Font] interface.
func (f *testFont) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

// DecodeWidth reads one character code from the given string and returns
// the width of the corresponding glyph.
// This implements the [font.Embedded] interface.
func (f *testFont) DecodeWidth(s pdf.String) (float64, int) {
	_, k, _ := f.codec.Decode(s)
	return 2000, k
}

// CodeAndWidth encodes the given glyph ID as a PDF character code.
// This implements the [font.Embedded] interface.
func (f *testFont) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	// We don't need this method here, since we directly write the character
	// codes.
	panic("not implemented")
}
