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

	"golang.org/x/exp/maps"
	"golang.org/x/text/language"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

type OptionsComposite struct {
	Language     language.Tag
	GsubFeatures map[string]bool
	GposFeatures map[string]bool

	WritingMode  font.WritingMode
	MakeGIDToCID func() cmap.GIDToCID
	MakeEncoder  func(cid0Width float64, wMode font.WritingMode) cidenc.CIDEncoder
}

// Composite represents a TrueType font together with the font options.
// This implements the [font.Layouter] interface.
type Composite struct {
	*sfnt.Font

	*font.Geometry
	layouter *sfnt.Layouter

	gidToCID cmap.GIDToCID
	cidenc.CIDEncoder
	usedCIDs map[cid.CID]struct{}
}

var _ font.Layouter = (*Composite)(nil)

// NewComposite makes a PDF TrueType font from a sfnt.Font.
// The font info must be an OpenType/TrueType font with glyf outlines.
// The font can be embedded as a simple font or as a composite font.
func NewComposite(info *sfnt.Font, opt *OptionsComposite) (*Composite, error) {
	if !info.IsGlyf() {
		return nil, errors.New("no glyf outlines in font")
	}

	if opt == nil {
		opt = &OptionsComposite{}
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

	makeGIDToCID := cmap.NewGIDToCIDSequential
	if opt.MakeGIDToCID != nil {
		makeGIDToCID = opt.MakeGIDToCID
	}

	makeEncoder := cidenc.NewCompositeIdentity
	if opt.MakeEncoder != nil {
		makeEncoder = opt.MakeEncoder
	}
	notdefWidth := math.Round(info.GlyphWidthPDF(0))

	f := &Composite{
		Font:       info,
		Geometry:   geometry,
		layouter:   layouter,
		gidToCID:   makeGIDToCID(),
		CIDEncoder: makeEncoder(notdefWidth, opt.WritingMode),
		usedCIDs:   make(map[cid.CID]struct{}),
	}

	return f, nil
}

// FontInfo returns information required to load the font file and to
// extract the the glyph corresponding to a character identifier.
// The returned structure is of type [*dict.FontInfoGlyfEmbedded].
func (f *Composite) FontInfo() any {
	dict, _ := f.makeDict()
	if dict == nil {
		return nil
	}
	return dict.FontInfo()
}

// Embed adds the font to a PDF file.
func (f *Composite) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "composite CFF fonts", pdf.V1_3); err != nil {
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

// Encode converts a glyph ID to a character code.
func (f *Composite) Encode(gid glyph.ID, width float64, text string) (charcode.Code, bool) {
	cid := f.gidToCID.CID(gid, []rune(text))
	if c, ok := f.CIDEncoder.GetCode(cid, text); ok {
		return c, true
	}

	f.usedCIDs[cid] = struct{}{}

	if width <= 0 {
		width = math.Round(f.Font.GlyphWidthPDF(gid))
	}
	c, err := f.CIDEncoder.Encode(cid, text, width)
	return c, err == nil
}

// Layout appends a string to a glyph sequence.
func (f *Composite) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
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
func (f *Composite) makeDict() (*dict.CIDFontType2, error) {
	origFont := f.Font
	postScriptName := origFont.PostScriptName()

	origFont = origFont.Clone()
	origFont.CMapTable = nil
	origFont.Gdef = nil
	origFont.Gsub = nil
	origFont.Gpos = nil

	// Subset the font, if needed.
	// To minimise file size, we arrange the glyphs in order of increasing CID.
	cidSet := make(map[cid.CID]struct{})
	cidSet[0] = struct{}{} // Always include CID 0 (notdef)
	for cidVal := range f.usedCIDs {
		cidSet[cidVal] = struct{}{}
	}
	cidList := maps.Keys(cidSet)
	slices.Sort(cidList)

	glyphs := make([]glyph.ID, len(cidList))
	for i, cidVal := range cidList {
		glyphs[i] = f.gidToCID.GID(cidVal)
	}
	subsetTag := subset.Tag(glyphs, origFont.NumGlyphs())

	var subsetFont *sfnt.Font
	if subsetTag != "" {
		subsetFont = origFont.Subset(glyphs)
	} else {
		subsetFont = origFont
	}

	ros := f.gidToCID.ROS()

	// construct the font dictionary and font descriptor
	dw := math.Round(subsetFont.GlyphWidthPDF(0))
	ww := make(map[cid.CID]float64)
	isSymbolic := false

	for _, info := range f.CIDEncoder.MappedCodes() {
		// Only include information for CIDs that were actually used
		if _, used := f.usedCIDs[info.CID]; used || info.CID == 0 {
			ww[info.CID] = info.Width

			if !isSymbolic {
				glyphName := names.FromUnicode(info.Text)
				if !pdfenc.StandardLatin.Has[glyphName] {
					isSymbolic = true
				}
			}
		}
	}

	// The `CIDToGIDMap` entry in the CIDFont dictionary specifies the mapping
	// from CIDs to glyphs.
	maxCID := cidList[len(cidList)-1]
	isIdentity := true
	cidToGID := make([]glyph.ID, maxCID+1)
	for subsetGID, cidVal := range cidList {
		if cidVal != cid.CID(subsetGID) {
			isIdentity = false
		}
		cidToGID[cidVal] = glyph.ID(subsetGID)
	}

	qv := 1000 * subsetFont.FontMatrix[3]
	ascent := math.Round(float64(subsetFont.Ascent) * qv)
	descent := math.Round(float64(subsetFont.Descent) * qv)
	leading := math.Round(float64(subsetFont.Ascent-subsetFont.Descent+subsetFont.LineGap) * qv)
	capHeight := math.Round(float64(subsetFont.CapHeight) * qv)
	xHeight := math.Round(float64(subsetFont.XHeight) * qv)

	italicAngle := pdf.Round(subsetFont.ItalicAngle, 1)

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
	}

	fontDict := &dict.CIDFontType2{
		PostScriptName:  postScriptName,
		SubsetTag:       subsetTag,
		Descriptor:      fd,
		ROS:             ros,
		CMap:            f.CIDEncoder.CMap(ros),
		Width:           ww,
		DefaultWidth:    dw,
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		ToUnicode:       f.CIDEncoder.ToUnicode(),
		FontFile:        sfntglyphs.ToStream(subsetFont, glyphdata.TrueType),
	}
	if !isIdentity {
		fontDict.CIDToGID = cidToGID
	}

	return fontDict, nil
}
