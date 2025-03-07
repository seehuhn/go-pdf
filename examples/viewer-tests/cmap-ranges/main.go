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

	"seehuhn.de/go/postscript/cid"
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
		ROS: &cid.SystemInfo{
			Registry:   "Adobe",
			Ordering:   "Japan1",
			Supplement: 0,
		},
		CodeSpaceRange: charcode.CodeSpaceRange{
			{Low: []byte{0x00}, High: []byte{0x41}},
			{Low: []byte{0x42, 0x00}, High: []byte{0x42, 0xFF}},
			{Low: []byte{0x43}, High: []byte{0xFF}},
		},
		CIDRanges: []cmap.Range{
			{
				First: []byte{0x41},
				Last:  []byte{0x43},
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
	out.TextShowRaw(pdf.String{0x41, 0x43})
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
	// Create a font with just the three glyphs for "ABC".
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
	for _, r := range "ABC" {
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

	cidToGID := make([]glyph.ID, 37)
	cidToGID[34] = 1 // CID 34 = GID 1 (A)
	cidToGID[35] = 2 // CID 35 = GID 2 (B)
	cidToGID[36] = 3 // CID 36 = GID 3 (C)

	q := 1000 / float64(f.ttf.UnitsPerEm)

	fd := &font.Descriptor{
		FontName:  "AABBCC+Test",
		FontBBox:  f.ttf.FontBBoxPDF(),
		Ascent:    f.ttf.Ascent.AsFloat(q),
		Descent:   f.ttf.Descent.AsFloat(q),
		CapHeight: f.ttf.CapHeight.AsFloat(q),
	}
	dict := &dict.CIDFontType2{
		Ref:             fontDictRef,
		PostScriptName:  "Test",
		SubsetTag:       "AABBCC",
		Descriptor:      fd,
		ROS:             f.cmap.ROS,
		Encoding:        f.cmap,
		DefaultWidth:    2000,
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		CIDToGID:        cidToGID,
		FontType:        glyphdata.TrueType,
		FontRef:         rm.Out.Alloc(),
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
			case 0x41:
				code.CID = 34
				code.Text = "A"
			case 0x42:
				code.CID = 35
				code.Text = "B"
			case 0x43:
				code.CID = 36
				code.Text = "C"
			default:
				code.CID = 0
				code.Text = ""
			}
			code.UseWordSpacing = (k == 1 && c == 0x20)
			if !yield(&code) {
				break
			}
			s = s[k:]
		}
	}
}
