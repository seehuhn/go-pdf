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

package truetype

import (
	"errors"
	"math"
	"slices"

	"golang.org/x/text/language"
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

type OptionsSimple struct {
	Language     language.Tag
	GsubFeatures map[string]bool
	GposFeatures map[string]bool
}

// Simple represents a TrueType font which can be embedded in a PDF file.
// This implements the [font.Layouter] interface.
type Simple struct {
	*sfnt.Font

	*font.Geometry
	layouter *sfnt.Layouter

	*simpleenc.Simple
}

var _ font.Layouter = (*Simple)(nil)

// NewSimple makes a PDF TrueType font from a sfnt.Font.
// The font info must be an OpenType/TrueType font with glyf outlines.
func NewSimple(info *sfnt.Font, opt *OptionsSimple) (*Simple, error) {
	if !info.IsGlyf() {
		return nil, errors.New("no glyf outlines in font")
	}

	if opt == nil {
		opt = &OptionsSimple{}
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

	f := &Simple{
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
// extract the the glyph corresponding to a character identifier. The
// result is a pointer to one of the FontInfo* types defined in the
// font/dict package.
func (f *Simple) FontInfo() any {
	return &dict.FontInfoSimple{
		PostScriptName: "",
		FontFile:       &glyphdata.Stream{},
		Encoding:       f.Simple.Encoding(),
		IsSymbolic:     f.isSymbolic(),
	}
}

// Encode converts a glyph ID to a character code (for use with the
// instance's codec).  The arguments width and text are hints for choosing
// an appropriate advance width and text representation for the character
// code, in case a new code is allocated.
//
// The function returns the character code, and a boolean indicating
// whether the encoding was successful.  If the function returns false, the
// glyph ID cannot be encoded with this font instance.
//
// Use the Codec to append the character code to PDF strings.
//
// Encode converts a glyph ID to a character code.
func (f *Simple) Encode(gid glyph.ID, text string) (charcode.Code, bool) {
	if c, ok := f.Simple.GetCode(gid, text); ok {
		return charcode.Code(c), true
	}

	width := math.Round(f.Font.GlyphWidthPDF(gid))
	c, err := f.Simple.Encode(gid, f.Font.GlyphName(gid), text, width)
	return charcode.Code(c), err == nil
}

// Layout appends a string to a glyph sequence.  The string is typeset at
// the given point size and the resulting GlyphSeq is returned.
//
// If seq is nil, a new glyph sequence is allocated.  If seq is not
// nil, the return value is guaranteed to be equal to seq.
func (f *Simple) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
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

func (f *Simple) isSymbolic() bool {
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
		if pdfenc.MacRoman.Encoding[code] != glyphName {
			canMacRoman = false
		}
		if pdfenc.WinAnsi.Encoding[code] != glyphName {
			canWinAnsi = false
		}
		if !(canMacRoman || canWinAnsi) {
			break
		}
	}

	return !(canMacRoman || canWinAnsi)
}

// Embed converts the Go representation of the object into a PDF object,
// corresponding to the PDF version of the output file.
//
// The first return value is the PDF representation of the object.
// If the object is embedded in the PDF file, this may be a reference.
//
// The second return value is a Go representation of the embedded object.
// In most cases, this value is not used and T can be set to [Unused].
func (f *Simple) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "simple TrueType fonts", pdf.V1_2); err != nil {
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

func (f *Simple) makeDict() (*dict.TrueType, error) {
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
				{PlatformID: 3, EncodingID: 1}: subtable.Encode(0),
			}
		}
	}

	qv := subsetFont.FontMatrix[3] * 1000
	ascent := math.Round(float64(subsetFont.Ascent) * qv)
	descent := math.Round(float64(subsetFont.Descent) * qv)
	leading := math.Round(float64(subsetFont.Ascent-subsetFont.Descent+subsetFont.LineGap) * qv)
	capHeight := math.Round(float64(subsetFont.CapHeight) * qv)
	xHeight := math.Round(float64(subsetFont.XHeight) * qv)

	italicAngle := math.Round(subsetFont.ItalicAngle*10) / 10

	postScriptName := subsetFont.PostScriptName()

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, postScriptName),
		FontFamily:   subsetFont.FamilyName,
		FontStretch:  subsetFont.Width,
		FontWeight:   subsetFont.Weight,
		IsFixedPitch: subsetFont.IsFixedPitch(),
		IsSerif:      subsetFont.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     subsetFont.IsScript,
		IsItalic:     subsetFont.IsItalic,
		FontBBox:     subsetFont.FontBBoxPDF().Rounded(),
		ItalicAngle:  italicAngle,
		Ascent:       ascent,
		Descent:      descent,
		Leading:      leading,
		CapHeight:    capHeight,
		XHeight:      xHeight,
		MissingWidth: f.Simple.DefaultWidth(),
	}

	dict := &dict.TrueType{
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       dictEnc,
		FontFile:       sfntglyphs.ToStream(subsetFont, glyphdata.TrueType),
		ToUnicode:      f.Simple.ToUnicode(postScriptName),
	}
	for c, info := range f.Simple.MappedCodes() {
		dict.Width[c] = info.Width
	}
	return dict, nil
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
