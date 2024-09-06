// seehuhn.de/pdf-examples - example code for seehuhn.de/go/pdf
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
		CodeSpaceRange: []charcode.Range{
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

type testFont struct {
	cmap *cmap.InfoNew
	ttf  *sfnt.Font
}

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

	return &testFont{
		cmap: cmap,
		ttf:  ttf,
	}, nil
}

func (f *testFont) PostScriptName() string {
	return "Test"
}

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
	cidToGID[2*34+1] = 1 // 'A' has GID 0x0001
	cidToGID[2*35+1] = 2 // 'B' has GID 0x0002
	cidToGID[2*36+1] = 3 // 'C' has GID 0x0003
	cidToGID[2*37+1] = 4 // 'D' has GID 0x0004
	cidToGID[2*38+1] = 5 // 'E' has GID 0x0005
	cidToGID[2*39+1] = 6 // 'F' has GID 0x0006
	cidToGID[2*40+1] = 7 // 'G' has GID 0x0007
	cidToGID[2*41+1] = 8 // 'H' has GID 0x0008
	cidToGID[2*42+1] = 9 // 'I' has GID 0x0009
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

	length1 := pdf.NewPlaceholder(rm.Out, 10)
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("TrueType"),
		"Length1": length1,
	}
	fontFileStream, err := rm.Out.OpenStream(fontFileRef, fontFileDict, pdf.FilterASCII85{}, pdf.FilterCompress{})
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

func (f *testFont) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

func (f *testFont) ForeachWidth(s pdf.String, yield func(width float64, isSpace bool)) {
	for i := 0; i < len(s); i++ {
		yield(2000, s[i] == ' ')
		if s[i] == 2 {
			i += 2
		} else {
			i++
		}
	}
}

func (f *testFont) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	panic("not implemented")
}
