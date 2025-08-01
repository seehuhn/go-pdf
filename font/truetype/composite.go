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
	"math"
	"slices"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

var _ interface {
	font.EmbeddedLayouter
	font.Embedded
	pdf.Finisher
} = (*embeddedComposite)(nil)

type embeddedComposite struct {
	Ref  pdf.Reference
	Font *sfnt.Font

	cmap.GIDToCID
	cidenc.CIDEncoder

	finished bool
	usedCIDs map[cid.CID]struct{}
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

	makeEncoder := cidenc.NewCompositeIdentity
	if opt.MakeEncoder != nil {
		makeEncoder = opt.MakeEncoder
	}
	notdefWidth := math.Round(f.Font.GlyphWidthPDF(0))
	encoder := makeEncoder(notdefWidth, opt.WritingMode)

	e := &embeddedComposite{
		Ref:  ref,
		Font: f.Font,

		GIDToCID:   gidToCID,
		CIDEncoder: encoder,
		usedCIDs:   make(map[cid.CID]struct{}),
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
		c, err = e.CIDEncoder.Encode(cid, text, width)
		if err != nil {
			return s, 0
		}
	}

	// Track that this CID has been used
	e.usedCIDs[cid] = struct{}{}

	w := e.CIDEncoder.Width(c)
	return e.CIDEncoder.Codec().AppendCode(s, c), w / 1000
}

func (e *embeddedComposite) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	origFont := e.Font
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
	for cidVal := range e.usedCIDs {
		cidSet[cidVal] = struct{}{}
	}
	cidList := maps.Keys(cidSet)
	slices.Sort(cidList)

	glyphs := make([]glyph.ID, len(cidList))
	for i, cidVal := range cidList {
		glyphs[i] = e.GIDToCID.GID(cidVal)
	}
	subsetTag := subset.Tag(glyphs, origFont.NumGlyphs())

	var subsetFont *sfnt.Font
	if subsetTag != "" {
		subsetFont = origFont.Subset(glyphs)
	} else {
		subsetFont = origFont
	}

	ros := e.ROS()

	// construct the font dictionary and font descriptor
	dw := math.Round(subsetFont.GlyphWidthPDF(0))
	ww := make(map[cmap.CID]float64)
	isSymbolic := false

	for _, info := range e.CIDEncoder.MappedCodes() {
		// Only include information for CIDs that were actually used
		if _, used := e.usedCIDs[info.CID]; used || info.CID == 0 {
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

	qv := subsetFont.FontMatrix[3] * 1000
	ascent := math.Round(float64(subsetFont.Ascent) * qv)
	descent := math.Round(float64(subsetFont.Descent) * qv)
	leading := math.Round(float64(subsetFont.Ascent-subsetFont.Descent+subsetFont.LineGap) * qv)
	capHeight := math.Round(float64(subsetFont.CapHeight) * qv)
	xHeight := math.Round(float64(subsetFont.XHeight) * qv)

	italicAngle := math.Round(subsetFont.ItalicAngle*10) / 10

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

	dict := &dict.CIDFontType2{
		PostScriptName:  postScriptName,
		SubsetTag:       subsetTag,
		Descriptor:      fd,
		ROS:             ros,
		CMap:            e.CIDEncoder.CMap(ros),
		Width:           ww,
		DefaultWidth:    dw,
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		ToUnicode:       e.CIDEncoder.ToUnicode(),
		FontType:        glyphdata.TrueType,
		FontRef:         rm.Out.Alloc(),
	}
	if !isIdentity {
		dict.CIDToGID = cidToGID
	}

	err := dict.WriteToPDF(rm, e.Ref)
	if err != nil {
		return err
	}

	err = opentypeglyphs.Embed(rm.Out, dict.FontType, dict.FontRef, subsetFont)
	if err != nil {
		return err
	}

	return nil
}
