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

package main

import (
	"fmt"
	"math"
	"os"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(fname string) error {
	page, err := document.CreateSinglePage(fname, document.A5, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	F, err := makeFont()
	if err != nil {
		return err
	}

	page.TextBegin()
	page.TextSetFont(F, 64)
	page.TextFirstLine(72, 520)
	page.TextShowRaw(pdf.String("HELLO"))
	page.TextEnd()

	page.SetFillColor(color.Red)
	page.Circle(72, 520, 5)
	page.Fill()

	return page.Close()
}

func makeFont() (font.Font, error) {
	info := makefont.OpenType()

	var glyphs []glyph.ID
	var gidToCID []cid.CID
	cmap, err := info.CMapTable.GetBest()
	if err != nil {
		return nil, err
	}
	for code := 0; code < 128; code++ {
		var gid glyph.ID
		if code > 0 {
			gid = cmap.Lookup(rune(code))
			if gid == 0 {
				continue
			}
		}
		glyphs = append(glyphs, gid)
		gidToCID = append(gidToCID, cid.CID(code))
	}
	info = info.Subset(glyphs)

	fontInfo := &type1.FontInfo{
		FontName:           info.PostScriptName(),
		Version:            info.Version.String(),
		Notice:             info.Trademark,
		Copyright:          info.Copyright,
		FullName:           info.FullName(),
		FamilyName:         info.FamilyName,
		Weight:             info.Weight.String(),
		ItalicAngle:        info.ItalicAngle,
		IsFixedPitch:       info.IsFixedPitch(),
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		FontMatrix:         info.FontMatrix,
	}
	outlines := info.Outlines.(*cff.Outlines)
	ros := &cid.SystemInfo{
		Registry:   "Quire",
		Ordering:   "ASCII",
		Supplement: 0,
	}
	outlines.MakeCIDKeyed(ros, gidToCID)
	cffFont := &cff.Font{
		FontInfo: fontInfo,
		Outlines: outlines,
	}

	qv := info.FontMatrix[3] * 1000
	ascent := math.Round(float64(info.Ascent) * qv)
	descent := math.Round(float64(info.Descent) * qv)
	leading := math.Round(float64(info.Ascent-info.Descent+info.LineGap) * qv)
	capHeight := math.Round(float64(info.CapHeight) * qv)
	glyphExtents := make([]rect.Rect, len(cffFont.Glyphs))
	for gid := range cffFont.Glyphs {
		glyphExtents[gid] = cffFont.GlyphBBoxPDF(cffFont.FontMatrix, glyph.ID(gid))
	}
	geom := &font.Geometry{
		Ascent:             ascent / 1000,
		Descent:            descent / 1000,
		Leading:            leading / 1000,
		UnderlinePosition:  float64(info.UnderlinePosition) * qv / 1000,
		UnderlineThickness: float64(info.UnderlineThickness) * qv / 1000,

		GlyphExtents: glyphExtents,
		Widths:       info.WidthsPDF(),
	}

	f := &testFont{
		Font:      cffFont,
		Geometry:  geom,
		ROS:       ros,
		Ascent:    ascent,
		Descent:   descent,
		CapHeight: capHeight,
	}
	return f, nil
}

type testFont struct {
	Font *cff.Font
	*font.Geometry
	ROS       *cid.SystemInfo
	Ascent    float64
	Descent   float64
	CapHeight float64
}

func (f *testFont) PostScriptName() string {
	return f.Font.FontName
}

func (f *testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	subsetFont := f.Font
	subsetTag := "ABCDEF"

	defaultWidth := math.Round(subsetFont.GlyphWidthPDF(0))
	widths := make(map[cid.CID]float64)
	for gid, cid := range subsetFont.Outlines.GIDToCID {
		w := math.Round(subsetFont.GlyphWidthPDF(glyph.ID(gid)))
		if w == defaultWidth {
			continue
		}
		widths[cid] = w
	}

	csASCII := charcode.CodeSpaceRange{
		{Low: []byte{0x00}, High: []byte{0x7F}},
	}
	encoding := &cmap.File{
		Name:           "Quire-ASCII-V",
		ROS:            f.ROS,
		WMode:          font.Vertical,
		CodeSpaceRange: csASCII,
		CIDRanges: []cmap.Range{
			{First: []byte{0x00}, Last: []byte{0x7F}, Value: 0},
		},
	}
	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, subsetFont.FontName),
		IsFixedPitch: f.Font.IsFixedPitch,
		FontBBox:     subsetFont.FontBBoxPDF().Rounded(),
		Ascent:       math.Round(f.Ascent),
		Descent:      math.Round(f.Descent),
		CapHeight:    math.Round(f.CapHeight),
	}
	fontDictRef := rm.Out.Alloc()
	dict := &dict.CIDFontType0{
		PostScriptName:  f.Font.FontName,
		SubsetTag:       subsetTag,
		Descriptor:      fd,
		ROS:             f.ROS,
		CMap:            encoding,
		Width:           widths,
		DefaultWidth:    defaultWidth,
		VMetrics:        nil,
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		Text: &cmap.ToUnicodeFile{
			CodeSpaceRange: csASCII,
			Ranges: []cmap.ToUnicodeRange{
				{First: []byte{0x00}, Last: []byte{0x7f}, Values: []string{"\000"}},
			},
		},
		FontType: glyphdata.CFF,
		FontRef:  rm.Out.Alloc(),
	}

	err := dict.WriteToPDF(rm, fontDictRef)
	if err != nil {
		return nil, nil, err
	}

	err = cffglyphs.Embed(rm.Out, dict.FontType, dict.FontRef, f.Font)
	if err != nil {
		return nil, nil, err
	}

	e, err := dict.MakeFont()
	if err != nil {
		return nil, nil, err
	}

	return fontDictRef, e, nil
}
