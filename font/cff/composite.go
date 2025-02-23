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

package cff

import (
	"math"
	"slices"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/subset"
)

var _ interface {
	font.EmbeddedLayouter
	font.Scanner
	pdf.Finisher
} = (*embeddedComposite)(nil)

// embeddedComposite represents an [Instance] which has been embedded in a PDF
// file if the Composite font option is set.  There should be at most one
// embeddedComposite for each [Instance] in a PDF file.
type embeddedComposite struct {
	Ref  pdf.Reference
	Font *cff.Font

	Stretch  os2.Width
	Weight   os2.Weight
	IsSerif  bool
	IsScript bool

	Ascent    float64 // PDF glyph space units
	Descent   float64 // PDF glyph space units
	Leading   float64 // PDF glyph space units
	CapHeight float64 // PDF glyph space units
	XHeight   float64 // PDF glyph space units

	font.GIDToCID
	font.CIDEncoder

	finished bool
}

func (e *embeddedComposite) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	cid := e.GIDToCID.CID(gid, []rune(text))
	c, ok := e.CIDEncoder.GetCode(cid, text)
	if !ok {
		if e.finished {
			return s, 0
		}

		width := math.Round(e.Font.GlyphWidthPDF(gid))
		var err error
		c, err = e.CIDEncoder.AllocateCode(cid, text, width)
		if err != nil {
			return s, 0
		}
	}

	w := e.CIDEncoder.Width(c)
	return e.CIDEncoder.Codec().AppendCode(s, c), w / 1000
}

// Finish is called when the resource manager is closed.
// At this point the subset of glyphs to be embedded is known.
func (e *embeddedComposite) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	fontInfo := e.Font.FontInfo
	outlines := e.Font.Outlines

	// subset the font
	glyphSet := make(map[glyph.ID]struct{})
	glyphSet[e.GIDToCID.GID(0)] = struct{}{}
	for _, info := range e.CIDEncoder.MappedCodes() {
		glyphSet[e.GIDToCID.GID(info.CID)] = struct{}{}
	}

	glyphs := maps.Keys(glyphSet)
	slices.Sort(glyphs)

	// subset the font, if needed
	subsetTag := subset.Tag(glyphs, outlines.NumGlyphs())
	var subsetOutlines *cff.Outlines
	if subsetTag != "" {
		subsetOutlines = outlines.Subset(glyphs)
	} else {
		subsetOutlines = clone(outlines)
	}

	origGIDToCID := e.GIDToCID.GIDToCID(outlines.NumGlyphs())
	gidToCID := make([]cid.CID, subsetOutlines.NumGlyphs())
	for i, gid := range glyphs {
		gidToCID[i] = origGIDToCID[gid]
	}

	ros := e.ROS()

	m := make(map[charcode.Code]font.Code)
	for code, val := range e.CIDEncoder.MappedCodes() {
		m[code] = *val
	}
	cmapInfo := &cmap.File{
		Name:  "",
		ROS:   ros,
		WMode: e.CIDEncoder.WritingMode(),
	}
	cmapInfo.SetMapping(e.CIDEncoder.Codec(), m)
	cmapInfo.UpdateName()
	toUnicode := cmap.NewToUnicodeFile(e.CIDEncoder.Codec(), m)

	// If the CFF font is CID-keyed, then the `charset` table in the CFF font
	// describes the mapping from CIDs to glyphs.  Otherwise, the CID is used
	// as the glyph index directly.
	isIdentity := true
	for GID, CID := range gidToCID {
		if CID != 0 && CID != cid.CID(GID) {
			isIdentity = false
			break
		}
	}

	mustUseCID := len(subsetOutlines.Private) > 1
	if isIdentity && !mustUseCID { // convert to simple font
		subsetOutlines.ROS = nil
		subsetOutlines.GIDToCID = nil
		if len(subsetOutlines.FontMatrices) > 0 && subsetOutlines.FontMatrices[0] != matrix.Identity {
			fontInfo = clone(fontInfo)
			fontInfo.FontMatrix = subsetOutlines.FontMatrices[0].Mul(fontInfo.FontMatrix)
		}
		subsetOutlines.FontMatrices = nil
		subsetOutlines.Encoding = cff.StandardEncoding(subsetOutlines.Glyphs)
	} else { // Make the font CID-keyed.
		subsetOutlines.Encoding = nil
		var sup int32
		if ros.Supplement > 0 && ros.Supplement < 0x1000_0000 {
			sup = int32(ros.Supplement)
		}
		subsetOutlines.ROS = &cid.SystemInfo{
			Registry:   ros.Registry,
			Ordering:   ros.Ordering,
			Supplement: sup,
		}
		subsetOutlines.GIDToCID = gidToCID
		for len(subsetOutlines.FontMatrices) < len(subsetOutlines.Private) {
			// TODO(voss): I think it would be more normal to have the identity
			// matrix in the top dict, and the real font matrix here?
			subsetOutlines.FontMatrices = append(subsetOutlines.FontMatrices, matrix.Identity)
		}
	}

	subsetFont := &cff.Font{
		FontInfo: fontInfo,
		Outlines: subsetOutlines,
	}

	ww := make(map[cmap.CID]float64)
	for gid, cid := range gidToCID {
		ww[cid] = math.Round(subsetFont.GlyphWidthPDF(glyph.ID(gid)))
	}
	dw := math.Round(subsetFont.GlyphWidthPDF(0))

	isSymbolic := false // TODO(voss): set this correctly

	qh := subsetFont.FontMatrix[0] * 1000
	qv := subsetFont.FontMatrix[3] * 1000

	italicAngle := math.Round(subsetFont.ItalicAngle*10) / 10

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, e.Font.FontName),
		FontFamily:   subsetFont.FamilyName,
		FontStretch:  e.Stretch,
		FontWeight:   e.Weight,
		IsFixedPitch: subsetFont.IsFixedPitch,
		IsSerif:      e.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     e.IsScript,
		IsItalic:     italicAngle != 0,
		ForceBold:    subsetFont.Private[0].ForceBold,
		FontBBox:     subsetFont.FontBBoxPDF().Rounded(),
		ItalicAngle:  italicAngle,
		Ascent:       math.Round(e.Ascent),
		Descent:      math.Round(e.Descent),
		Leading:      math.Round(e.Leading),
		CapHeight:    math.Round(e.CapHeight),
		XHeight:      math.Round(e.XHeight),
		StemV:        math.Round(subsetOutlines.Private[0].StdVW * qh),
		StemH:        math.Round(subsetOutlines.Private[0].StdHW * qv),
	}

	dict := &dict.CIDFontType0{
		Ref:            e.Ref,
		PostScriptName: e.Font.FontName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		ROS:            ros,
		Encoding:       cmapInfo,
		Width:          ww,
		DefaultWidth:   dw,
		Text:           toUnicode,
		FontType:       glyphdata.CFF,
		FontRef:        rm.Out.Alloc(),
	}

	err := dict.WriteToPDF(rm)
	if err != nil {
		return err
	}

	err = cffglyphs.Embed(rm.Out, dict.FontType, dict.FontRef, subsetFont)
	if err != nil {
		return err
	}

	return nil
}
