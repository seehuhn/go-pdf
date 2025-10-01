// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package opentype

import (
	"errors"
	"math"
	"slices"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

// SimpleGlyf represents a simple OpenType font with glyf outlines.
// This implements the font.Layouter interface.
type SimpleGlyf struct {
	*sfnt.Font

	*font.Geometry
	layouter *sfnt.Layouter

	*simpleenc.Simple
}

var _ font.Layouter = (*SimpleGlyf)(nil)

// newSimpleGlyf creates a simple OpenType font with glyf outlines.
func newSimpleGlyf(info *sfnt.Font, opt *OptionsSimple) (*SimpleGlyf, error) {
	if !info.IsGlyf() {
		return nil, errors.New("no glyf outlines in font")
	}

	geometry := &font.Geometry{
		GlyphExtents: scaleBoxesGlyf(info.GlyphBBoxes(), info.UnitsPerEm),
		Widths:       info.WidthsPDF(),

		Ascent:             float64(info.Ascent) / float64(info.UnitsPerEm),
		Descent:            float64(info.Descent) / float64(info.UnitsPerEm),
		Leading:            float64(info.Ascent-info.Descent+info.LineGap) / float64(info.UnitsPerEm),
		UnderlinePosition:  float64(info.UnderlinePosition) / float64(info.UnitsPerEm),
		UnderlineThickness: float64(info.UnderlineThickness) / float64(info.UnitsPerEm),
	}

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	f := &SimpleGlyf{
		Font:     info,
		Geometry: geometry,
		layouter: layouter,
	}

	notdefWidth := math.Round(info.GlyphWidthPDF(0))
	f.Simple = simpleenc.NewSimple(
		notdefWidth,
		info.PostScriptName(),
		&pdfenc.WinAnsi,
	)

	return f, nil
}

// FontInfo returns information required to load the font file and to
// extract the the glyph corresponding to a character identifier.
func (f *SimpleGlyf) FontInfo() any {
	return &dict.FontInfoSimple{
		PostScriptName: f.Font.PostScriptName(),
		FontFile:       &glyphdata.Stream{},
		Encoding:       f.Simple.Encoding(),
		IsSymbolic:     f.isSymbolic(),
	}
}

// Embed adds the font to a PDF file.
func (f *SimpleGlyf) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "OpenType fonts", pdf.V1_6); err != nil {
		return nil, err
	}

	ref := e.Alloc()
	e.Defer(func(eh *pdf.EmbedHelper) error {
		dict, err := f.makeDict()
		if err != nil {
			return err
		}
		_, err = eh.EmbedAt(ref, dict)
		return err
	})

	return ref, nil
}

// Encode converts a glyph ID to a character code.
func (f *SimpleGlyf) Encode(gid glyph.ID, text string) (charcode.Code, bool) {
	if c, ok := f.Simple.GetCode(gid, text); ok {
		return charcode.Code(c), true
	}

	width := math.Round(f.Font.GlyphWidthPDF(gid))
	c, err := f.Simple.Encode(gid, f.Font.GlyphName(gid), text, width)
	return charcode.Code(c), err == nil
}

// Layout appends a string to a glyph sequence.
func (f *SimpleGlyf) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	buf := f.layouter.Layout(s)
	seq.Seq = slices.Grow(seq.Seq, len(buf))
	for _, g := range buf {
		xOffset := float64(g.XOffset) * ptSize * f.Font.FontMatrix[0]
		if len(seq.Seq) == 0 {
			seq.Skip += xOffset
		} else {
			seq.Seq[len(seq.Seq)-1].Advance += xOffset
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     g.GID,
			Advance: float64(g.Advance) * ptSize * f.Font.FontMatrix[0],
			Rise:    float64(g.YOffset) * ptSize * f.Font.FontMatrix[3],
			Text:    string(g.Text),
		})
	}
	return seq
}

// IsBlank reports whether the glyph is blank.
func (f *SimpleGlyf) IsBlank(gid glyph.ID) bool {
	if int(gid) >= len(f.Geometry.GlyphExtents) {
		gid = 0
	}
	return f.Geometry.GlyphExtents[gid].IsZero()
}

func (f *SimpleGlyf) isSymbolic() bool {
	// Follow the advice of section 9.6.5.4 of ISO 32000-2:2020, we
	// only make the font as non-symbolic, if it can be encoded either
	// using "MacRomanEncoding" or "WinAnsiEncoding".
	canMacRoman := true
	canWinAnsi := true
	for code := range 256 {
		gid := f.Simple.GID(byte(code))
		if gid == 0 {
			continue
		}
		glyphName := f.Simple.GlyphName(gid)
		if !pdfenc.MacRoman.Has[glyphName] {
			canMacRoman = false
		}
		if !pdfenc.WinAnsi.Has[glyphName] {
			canWinAnsi = false
		}
	}
	return !canMacRoman && !canWinAnsi
}

// makeDict creates the PDF font dictionary for this font.
func (f *SimpleGlyf) makeDict() (*dict.TrueType, error) {
	if err := f.Simple.Error(); err != nil {
		return nil, pdf.Errorf("font %q: %w", f.Font.PostScriptName(), err)
	}

	// subset the font
	origFont := f.Font.Clone()
	origFont.CMapTable = nil
	origFont.Gdef = nil
	origFont.Gsub = nil
	origFont.Gpos = nil

	glyphs := f.Simple.Glyphs()
	subsetTag := subset.Tag(glyphs, origFont.NumGlyphs())
	var subsetFont *sfnt.Font
	if subsetTag != "" {
		subsetFont = origFont.Subset(glyphs)
	} else {
		subsetFont = origFont
	}

	toSubset := make(map[glyph.ID]glyph.ID, len(glyphs))
	for subsetGID, origGID := range glyphs {
		toSubset[origGID] = glyph.ID(subsetGID)
	}

	isSymbolic := f.isSymbolic()

	var dictEnc encoding.Simple
	if isSymbolic {
		// Use the built-in encoding, defined by a (1,0) "cmap" subtable which
		// maps codes to a GIDs.

		dictEnc = encoding.Builtin

		subtable := sfntcmap.Format4{}
		for code := range 256 {
			gid := f.Simple.GID(byte(code))
			if gid == 0 {
				continue
			}
			subtable[uint16(code)] = toSubset[gid]
		}
		subsetFont.CMapTable = sfntcmap.Table{
			{PlatformID: 1, EncodingID: 0}: subtable.Encode(0),
		}
	} else {
		// Use the encoding to map codes to names, use the Adobe Glyph List to
		// map the names to unicode, and use a (3,1) "cmap" subtable to map
		// unicode to GIDs.

		dictEnc = f.Simple.Encoding()

		var needsFormat12 bool
		for _, origGid := range glyphs {
			glyphName := f.Simple.GlyphName(origGid)
			rr := []rune(names.ToUnicode(glyphName, subsetFont.PostScriptName()))
			if len(rr) != 1 {
				continue
			}
			if rr[0] > 0xFFFF {
				needsFormat12 = true
				break
			}
		}

		if !needsFormat12 {
			subtable := sfntcmap.Format4{}
			for gid, origGid := range glyphs {
				glyphName := f.Simple.GlyphName(origGid)
				rr := []rune(names.ToUnicode(glyphName, subsetFont.PostScriptName()))
				if len(rr) != 1 {
					continue
				}
				subtable[uint16(rr[0])] = glyph.ID(gid)
			}
			subsetFont.CMapTable = sfntcmap.Table{
				{PlatformID: 3, EncodingID: 1}: subtable.Encode(0),
			}
		} else {
			subtable := sfntcmap.Format12{}
			for gid, origGid := range glyphs {
				glyphName := f.Simple.GlyphName(origGid)
				rr := []rune(names.ToUnicode(glyphName, subsetFont.PostScriptName()))
				if len(rr) != 1 {
					continue
				}
				subtable[uint32(rr[0])] = glyph.ID(gid)
			}
			subsetFont.CMapTable = sfntcmap.Table{
				{PlatformID: 3, EncodingID: 1}: subtable.Encode(1),
			}
		}
	}

	// construct the font dictionary and font descriptor
	var widths [256]float64
	for code, info := range f.Simple.MappedCodes() {
		widths[code] = info.Width
	}

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, subsetFont.PostScriptName()),
		FontFamily:   subsetFont.FamilyName,
		FontStretch:  subsetFont.Width,
		FontWeight:   subsetFont.Weight,
		IsFixedPitch: subsetFont.IsFixedPitch(),
		IsSerif:      subsetFont.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     subsetFont.IsScript,
		IsItalic:     subsetFont.IsItalic,
		FontBBox:     subsetFont.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetFont.ItalicAngle,
		Ascent:       float64(subsetFont.Ascent) / float64(subsetFont.UnitsPerEm) * 1000,
		Descent:      float64(subsetFont.Descent) / float64(subsetFont.UnitsPerEm) * 1000,
		Leading:      float64(subsetFont.LineGap) / float64(subsetFont.UnitsPerEm) * 1000,
		CapHeight:    float64(subsetFont.CapHeight) / float64(subsetFont.UnitsPerEm) * 1000,
		XHeight:      float64(subsetFont.XHeight) / float64(subsetFont.UnitsPerEm) * 1000,
		StemV:        0, // not specified
		StemH:        0, // not specified
		AvgWidth:     0, // not specified
		MaxWidth:     0, // not specified
		MissingWidth: f.Simple.DefaultWidth(),
	}

	fontDict := &dict.TrueType{
		PostScriptName: subsetFont.PostScriptName(),
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       dictEnc,
		Width:          widths,
		ToUnicode:      f.Simple.ToUnicode(subsetFont.PostScriptName()),
		FontFile:       sfntglyphs.ToStream(subsetFont, glyphdata.TrueType),
	}

	return fontDict, nil
}

func scaleBoxesGlyf(bboxes []funit.Rect16, unitsPerEm uint16) []rect.Rect {
	res := make([]rect.Rect, len(bboxes))
	for i, b := range bboxes {
		res[i] = rect.Rect{
			LLx: float64(b.LLx) / float64(unitsPerEm),
			LLy: float64(b.LLy) / float64(unitsPerEm),
			URx: float64(b.URx) / float64(unitsPerEm),
			URy: float64(b.URy) / float64(unitsPerEm),
		}
	}
	return res
}
