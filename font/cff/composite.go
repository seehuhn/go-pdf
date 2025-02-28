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
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

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

var _ interface {
	font.EmbeddedLayouter
	font.Scanner
	pdf.Finisher
} = (*embeddedComposite)(nil)

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

	cmap.GIDToCID
	cidenc.CIDEncoder

	finished bool
}

func newEmbeddedComposite(ref pdf.Reference, f *Instance) *embeddedComposite {
	opt := f.Opt
	if opt == nil {
		opt = &Options{}
	}

	makeGIDToCID := cmap.NewGIDToCIDSequential
	if opt.MakeGIDToCID != nil {
		makeGIDToCID = opt.MakeGIDToCID
	}
	gidToCID := makeGIDToCID()

	// TODO(voss): make sure we use `/Identity-H` instead of writing a CMap stream.
	makeEncoder := cidenc.NewCompositeIdentity
	if opt.MakeEncoder != nil {
		makeEncoder = opt.MakeEncoder
	}
	notdefWidth := math.Round(f.Font.GlyphWidthPDF(0))
	encoder := makeEncoder(notdefWidth, opt.WritingMode)

	e := &embeddedComposite{
		Ref:  ref,
		Font: f.Font,

		Stretch:  f.Stretch,
		Weight:   f.Weight,
		IsSerif:  f.IsSerif,
		IsScript: f.IsScript,

		Ascent:    f.Ascent,
		Descent:   f.Descent,
		Leading:   f.Leading,
		CapHeight: f.CapHeight,
		XHeight:   f.XHeight,

		GIDToCID:   gidToCID,
		CIDEncoder: encoder,
	}
	return e
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

func (e *embeddedComposite) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	fontInfo := e.Font.FontInfo
	origOutlines := e.Font.Outlines
	postScriptName := fontInfo.FontName

	// Subset the font, if needed.
	// To minimise file size, we arrange the glyphs in order of increasing CID.
	cidSet := make(map[cid.CID]struct{})
	cidSet[0] = struct{}{}
	for _, info := range e.CIDEncoder.MappedCodes() {
		cidSet[info.CID] = struct{}{}
	}
	cidList := maps.Keys(cidSet)
	slices.Sort(cidList)

	glyphs := make([]glyph.ID, len(cidList))
	for i, cidVal := range cidList {
		glyphs[i] = e.GIDToCID.GID(cidVal)
	}
	subsetTag := subset.Tag(glyphs, origOutlines.NumGlyphs())

	var subsetOutlines *cff.Outlines
	if subsetTag != "" {
		subsetOutlines = origOutlines.Subset(glyphs)
	} else {
		subsetOutlines = clone(origOutlines)
	}

	ros := e.ROS()

	m := make(map[charcode.Code]font.Code)
	for code, val := range e.CIDEncoder.MappedCodes() {
		m[code] = *val
	}
	toUnicode := cmap.NewToUnicodeFile(e.CIDEncoder.Codec(), m)

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
		for _, info := range e.CIDEncoder.MappedCodes() {
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
			sup = int32(ros.Supplement)
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
	dw := math.Round(subsetFont.GlyphWidthPDF(0))
	ww := make(map[cmap.CID]float64)
	for _, info := range e.CIDEncoder.MappedCodes() {
		ww[info.CID] = info.Width
	}

	isSymbolic := false
	for _, info := range e.CIDEncoder.MappedCodes() {
		// TODO(voss): if the font is simple, use the existing glyph names
		rr := []rune(info.Text)
		if len(rr) != 1 {
			isSymbolic = true
			break
		}
		glyphName := names.FromUnicode(rr[0])
		if !pdfenc.StandardLatin.Has[glyphName] {
			isSymbolic = true
			break
		}
	}

	qh := subsetFont.FontMatrix[0] * 1000 // TODO(voss): is this correct for CID-keyed fonts?
	qv := subsetFont.FontMatrix[3] * 1000

	italicAngle := math.Round(subsetFont.ItalicAngle*10) / 10

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, postScriptName),
		FontFamily:   subsetFont.FamilyName,
		FontStretch:  e.Stretch,
		FontWeight:   e.Weight,
		IsFixedPitch: subsetFont.IsFixedPitch,
		IsSerif:      e.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     e.IsScript,
		IsItalic:     italicAngle != 0,
		ForceBold:    subsetOutlines.Private[0].ForceBold,
		FontBBox:     subsetFont.FontBBoxPDF().Rounded(),
		ItalicAngle:  italicAngle,
		Ascent:       e.Ascent,
		Descent:      e.Descent,
		Leading:      e.Leading,
		CapHeight:    e.CapHeight,
		XHeight:      e.XHeight,
		StemV:        math.Round(subsetOutlines.Private[0].StdVW * qh),
		StemH:        math.Round(subsetOutlines.Private[0].StdHW * qv),
	}

	dict := &dict.CIDFontType0{
		Ref:            e.Ref,
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		ROS:            ros,
		Encoding:       e.CIDEncoder.CMap(ros),
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
