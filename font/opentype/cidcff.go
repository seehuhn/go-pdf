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
	"maps"
	"math"
	"slices"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

// CompositeCFF represents a composite OpenType font with CFF outlines.
// This implements the font.Layouter interface.
type CompositeCFF struct {
	*sfnt.Font

	*font.Geometry
	layouter *sfnt.Layouter

	gidToCID cmap.GIDToCID
	cidenc.CIDEncoder
	usedCIDs map[cid.CID]struct{}
}

var _ font.Layouter = (*CompositeCFF)(nil)

// newCompositeCFF creates a composite OpenType font with CFF outlines.
func newCompositeCFF(info *sfnt.Font, opt *OptionsComposite) (*CompositeCFF, error) {
	if !info.IsCFF() {
		return nil, errors.New("no CFF outlines in font")
	}

	geom := &font.Geometry{
		Ascent:             float64(info.Ascent) * info.FontMatrix[3],
		Descent:            float64(info.Descent) * info.FontMatrix[3],
		Leading:            float64(info.Ascent-info.Descent+info.LineGap) * info.FontMatrix[3],
		UnderlinePosition:  float64(info.UnderlinePosition) * info.FontMatrix[3],
		UnderlineThickness: float64(info.UnderlineThickness) * info.FontMatrix[3],

		GlyphExtents: scaleBoxesCFF(info.GlyphBBoxes(), info.FontMatrix[:]),
		Widths:       info.WidthsPDF(),
	}

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	makeGIDToCID := cmap.NewGIDToCIDSequential
	if opt.MakeGIDToCID != nil {
		makeGIDToCID = opt.MakeGIDToCID
	}
	makeEncoder := cidenc.NewCompositeIdentity
	if opt.MakeEncoder != nil {
		makeEncoder = opt.MakeEncoder
	}
	notdefWidth := math.Round(info.GlyphWidthPDF(0))

	f := &CompositeCFF{
		Font: info,

		Geometry: geom,
		layouter: layouter,

		gidToCID:   makeGIDToCID(),
		CIDEncoder: makeEncoder(notdefWidth, opt.WritingMode),
		usedCIDs:   make(map[cid.CID]struct{}),
	}

	return f, nil
}

// FontInfo returns information required to load the font file and to
// extract the the glyph corresponding to a character identifier. The
// result is a pointer to one of the FontInfo* types defined in the
// font/dict package.
func (f *CompositeCFF) FontInfo() any {
	dict, _ := f.makeDict()
	if dict == nil {
		return nil
	}
	return dict.FontInfo()
}

// Embed adds the font to a PDF file.
func (f *CompositeCFF) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "OpenType fonts", pdf.V1_6); err != nil {
		return nil, err
	}

	ref := e.Alloc()
	e.Defer(func(rm *pdf.EmbedHelper) error {
		dict, err := f.makeDict()
		if err != nil {
			return err
		}
		_, err = rm.EmbedAt(ref, dict)
		return err
	})

	return ref, nil
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
func (f *CompositeCFF) Encode(gid glyph.ID, text string) (charcode.Code, bool) {
	cid := f.gidToCID.CID(gid, []rune(text))
	f.usedCIDs[cid] = struct{}{}

	if c, ok := f.CIDEncoder.GetCode(cid, text); ok {
		return c, true
	}

	width := math.Round(f.Font.GlyphWidthPDF(gid))
	c, err := f.CIDEncoder.Encode(cid, text, width)
	return c, err == nil
}

// Layout appends a string to a glyph sequence.
func (f *CompositeCFF) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	qh := ptSize * f.Font.FontMatrix[0]
	qv := ptSize * f.Font.FontMatrix[3]

	buf := f.layouter.Layout(s)
	seq.Seq = slices.Grow(seq.Seq, len(buf))
	for _, g := range buf {
		xOffset := float64(g.XOffset) * qh
		if len(seq.Seq) == 0 {
			seq.Skip += xOffset
		} else {
			seq.Seq[len(seq.Seq)-1].Advance += xOffset
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     g.GID,
			Advance: float64(g.Advance) * qh,
			Rise:    float64(g.YOffset) * qv,
			Text:    string(g.Text),
		})
	}
	return seq
}

// makeDict creates the PDF font dictionary for this font.
func (f *CompositeCFF) makeDict() (*dict.CIDFontType0, error) {
	// get the CFF font data
	cffFont := f.Font.AsCFF()
	if cffFont == nil {
		return nil, errors.New("no CFF outlines in font")
	}

	fontInfo := cffFont.FontInfo
	origOutlines := cffFont.Outlines
	postScriptName := f.Font.PostScriptName()

	// Subset the font, if needed.
	// To minimise file size, we arrange the glyphs in order of increasing CID.
	cidSet := make(map[cid.CID]struct{})
	cidSet[0] = struct{}{}
	for cidVal := range f.usedCIDs {
		cidSet[cidVal] = struct{}{}
	}
	cidList := slices.Sorted(maps.Keys(cidSet))

	glyphs := make([]glyph.ID, len(cidList))
	for i, cidVal := range cidList {
		glyphs[i] = f.gidToCID.GID(cidVal)
	}
	subsetTag := subset.Tag(glyphs, origOutlines.NumGlyphs())

	var subsetOutlines *cff.Outlines
	if subsetTag != "" {
		subsetOutlines = origOutlines.Subset(glyphs)
	} else {
		subsetOutlines = clone(origOutlines)
	}

	ros := f.gidToCID.ROS()

	// Simple CFF fonts can only have one private dict, and ...
	canUseSimple := len(subsetOutlines.Private) == 1
	// ... they assume that CID values equal GID values.
	for subsetGID, CID := range cidList {
		if CID != 0 && CID != cid.CID(subsetGID) {
			canUseSimple = false
			break
		}
	}

	if canUseSimple { // convert to simple font
		if len(subsetOutlines.FontMatrices) > 0 && subsetOutlines.FontMatrices[0] != matrix.Identity {
			fontInfo = clone(fontInfo)
			fontInfo.FontMatrix = subsetOutlines.FontMatrices[0].Mul(fontInfo.FontMatrix)
		}

		cidToSubsetGID := make(map[cid.CID]glyph.ID)
		for subsetGID, CID := range cidList {
			cidToSubsetGID[CID] = glyph.ID(subsetGID)
		}
		glyphText := make(map[glyph.ID]string)
		for _, info := range f.CIDEncoder.MappedCodes() {
			// Only include information for CIDs that were actually used
			if _, used := f.usedCIDs[info.CID]; !used && info.CID != 0 {
				continue
			}
			subsetGID, ok := cidToSubsetGID[info.CID]
			if !ok {
				continue
			}
			glyphText[subsetGID] = info.Text
		}
		subsetOutlines.MakeSimple(glyphText)
	} else { // convert to CID-keyed font
		var sup int32
		if ros.Supplement > 0 && ros.Supplement < 0x1000_0000 {
			sup = ros.Supplement
		}
		ros := &cid.SystemInfo{
			Registry:   ros.Registry,
			Ordering:   ros.Ordering,
			Supplement: sup,
		}
		subsetOutlines.MakeCIDKeyed(ros, cidList)
	}

	subsetFont := &cff.Font{
		FontInfo: fontInfo,
		Outlines: subsetOutlines,
	}

	// construct the font dictionary and font descriptor
	dw := math.Round(f.Font.GlyphWidthPDF(0))
	ww := make(map[cid.CID]float64)
	for _, info := range f.CIDEncoder.MappedCodes() {
		// Only include information for CIDs that were actually used
		if _, used := f.usedCIDs[info.CID]; used || info.CID == 0 {
			ww[info.CID] = info.Width
		}
	}

	isSymbolic := false
	for _, info := range f.CIDEncoder.MappedCodes() {
		// TODO(voss): if the font is simple, use the existing glyph names?
		glyphName := names.FromUnicode(info.Text)
		if !pdfenc.StandardLatin.Has[glyphName] {
			isSymbolic = true
			break
		}
	}

	qh := subsetFont.FontMatrix[0] * 1000 // TODO(voss): is this correct for CID-keyed fonts?
	qv := subsetFont.FontMatrix[3] * 1000
	ascent := math.Round(float64(f.Font.Ascent) * qv)
	descent := math.Round(float64(f.Font.Descent) * qv)
	leading := math.Round(float64(f.Font.Ascent-f.Font.Descent+f.Font.LineGap) * qv)
	capHeight := math.Round(float64(f.Font.CapHeight) * qv)
	xHeight := math.Round(float64(f.Font.XHeight) * qv)

	italicAngle := pdf.Round(subsetFont.ItalicAngle, 1)

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, postScriptName),
		FontFamily:   subsetFont.FamilyName,
		FontStretch:  f.Font.Width,
		FontWeight:   f.Font.Weight,
		IsFixedPitch: f.Font.IsFixedPitch(),
		IsSerif:      f.Font.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     f.Font.IsScript,
		IsItalic:     f.Font.IsItalic,
		ForceBold:    subsetOutlines.Private[0].ForceBold,
		FontBBox:     subsetFont.FontBBoxPDF().Rounded(),
		ItalicAngle:  italicAngle,
		Ascent:       ascent,
		Descent:      descent,
		Leading:      leading,
		CapHeight:    capHeight,
		XHeight:      xHeight,
		StemV:        math.Round(subsetOutlines.Private[0].StdVW * qh),
		StemH:        math.Round(subsetOutlines.Private[0].StdHW * qv),
	}

	fontDict := &dict.CIDFontType0{
		PostScriptName:  postScriptName,
		SubsetTag:       subsetTag,
		Descriptor:      fd,
		ROS:             ros,
		CMap:            f.CIDEncoder.CMap(ros),
		Width:           ww,
		DefaultWidth:    dw,
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		ToUnicode:       f.CIDEncoder.ToUnicode(),
		FontFile:        cffglyphs.ToStream(subsetFont, glyphdata.CFF),
	}

	return fontDict, nil
}
