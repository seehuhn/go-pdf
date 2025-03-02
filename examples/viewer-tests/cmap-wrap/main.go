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
	"iter"
	"os"

	"golang.org/x/image/font/gofont/goregular"

	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(fname string) error {
	opts := &pdf.WriterOptions{
		HumanReadable: true,
	}
	out, err := document.CreateSinglePage(fname, document.A5r, pdf.V2_0, opts)
	if err != nil {
		return err
	}

	cmap := &cmap.File{
		Name: "Test",
		ROS: &cmap.CIDSystemInfo{
			Registry:   "Adobe",
			Ordering:   "Japan1",
			Supplement: 0,
		},
		CodeSpaceRange: charcode.CodeSpaceRange{
			{Low: []byte{0x30, 0x30}, High: []byte{0x32, 0x32}},
		},
		CIDRanges: []cmap.Range{
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
	cmap  *cmap.File
	codec *charcode.Codec
}

// NewTestFont creates a new test font with the given cmap.
func NewTestFont(rm *pdf.ResourceManager, cmap *cmap.File) (*testFont, error) {
	// Create a font with just the nine glyphs for "ABCDEFGHI".
	ttf, err := sfnt.Read(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, fmt.Errorf("gofont: %w", err)
	}
	ttf.Gdef = nil
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
	ttf = ttf.Subset(glyphs)

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

	cidToGID := make([]glyph.ID, 43)
	cidToGID[34] = 1 // CID 34 = GID 1 (A)
	cidToGID[35] = 2 // CID 35 = GID 2 (B)
	cidToGID[36] = 3 // CID 36 = GID 3 (C)
	cidToGID[37] = 4 // CID 37 = GID 4 (D)
	cidToGID[38] = 5 // CID 38 = GID 5 (E)
	cidToGID[39] = 6 // CID 39 = GID 6 (F)
	cidToGID[40] = 7 // CID 40 = GID 7 (G)
	cidToGID[41] = 8 // CID 41 = GID 8 (H)
	cidToGID[42] = 9 // CID 42 = GID 9 (I)

	q := 1000 / float64(f.ttf.UnitsPerEm)

	fd := &font.Descriptor{
		FontName:  "ABCDEF+Test",
		FontBBox:  f.ttf.FontBBoxPDF(),
		Ascent:    f.ttf.Ascent.AsFloat(q),
		Descent:   f.ttf.Descent.AsFloat(q),
		CapHeight: f.ttf.CapHeight.AsFloat(q),
	}
	dict := &dict.CIDFontType2{
		Ref:            fontDictRef,
		PostScriptName: "Test",
		SubsetTag:      "ABCDEF",
		Descriptor:     fd,
		ROS:            f.cmap.ROS,
		Encoding:       f.cmap,
		DefaultWidth:   2000,
		CIDToGID:       cidToGID,
		FontType:       glyphdata.TrueType,
		FontRef:        rm.Out.Alloc(),
	}

	err := dict.WriteToPDF(rm)
	if err != nil {
		return nil, nil, err
	}

	err = opentypeglyphs.Embed(rm.Out, dict.FontType, dict.FontRef, f.ttf)
	if err != nil {
		return nil, nil, err
	}

	return fontDictRef, f, nil
}

// This implements the [font.Font] and [font.Embedded] interfaces.
func (f *testFont) WritingMode() font.WritingMode {
	return font.Horizontal
}

// This implements the [font.Embedded] interface.
func (f *testFont) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		code.Width = 2000

		for len(s) > 0 {
			c, k, _ := f.codec.Decode(s)
			switch c {
			case 0x3030: // "00"
				code.CID = 34
				code.Text = "A"
			case 0x3130: // "01"
				code.CID = 35
				code.Text = "B"
			case 0x3230: // "02"
				code.CID = 36
				code.Text = "C"
			case 0x3031: // "10"
				code.CID = 34
				code.Text = "D"
			case 0x3131: // "11"
				code.CID = 35
				code.Text = "E"
			case 0x3231: // "12"
				code.CID = 36
				code.Text = "F"
			case 0x3032: // "20"
				code.CID = 34
				code.Text = "G"
			case 0x3132: // "21"
				code.CID = 35
				code.Text = "H"
			case 0x3232: // "22"
				code.CID = 36
				code.Text = "I"
			default:
				code.CID = 0
				code.Text = ""
			}
			if !yield(&code) {
				break
			}
			s = s[k:]
		}
	}
}

// This implements the [font.Embedded] interface.
func (f *testFont) DecodeWidth(s pdf.String) (float64, int) {
	_, k, _ := f.codec.Decode(s)
	return 2, k
}
