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

package cff

import (
	"fmt"
	"math"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

var _ interface {
	font.EmbeddedLayouter
	font.Embedded
	pdf.Finisher
} = (*embeddedSimple)(nil)

// embeddedSimple represents an [Instance] which has been embedded in a PDF
// file if the Composite option is not set.  There should be at most one
// embeddedSimple for each [Instance] in a PDF file.
type embeddedSimple struct {
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

	*simpleenc.Simple

	finished bool
}

func newEmbeddedSimple(ref pdf.Reference, f *Instance) *embeddedSimple {
	e := &embeddedSimple{
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

		Simple: simpleenc.NewSimple(
			math.Round(f.Font.GlyphWidthPDF(0)),
			f.Font.FontName,
			&pdfenc.WinAnsi,
		),
	}
	return e
}

func (e *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	c, ok := e.Simple.GetCode(gid, text)
	if !ok {
		if e.finished {
			return s, 0
		}

		glyphName := e.Font.Outlines.Glyphs[gid].Name
		width := math.Round(e.Font.GlyphWidthPDF(gid))
		var err error
		c, err = e.Simple.Encode(gid, glyphName, text, width)
		if err != nil {
			return s, 0
		}
	}

	w := e.Simple.Width(c)
	return append(s, c), w / 1000
}

func (e *embeddedSimple) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	if err := e.Simple.Error(); err != nil {
		return pdf.Errorf("font %q: %w", e.Font.FontName, err)
	}

	fontInfo := e.Font.FontInfo
	outlines := e.Font.Outlines

	// subset the font, if needed
	glyphs := e.Simple.Glyphs()
	subsetTag := subset.Tag(glyphs, outlines.NumGlyphs())

	var subsetOutlines *cff.Outlines
	if subsetTag != "" {
		subsetOutlines = outlines.Subset(glyphs)
	} else {
		subsetOutlines = clone(outlines)
	}

	// convert to a simple font, if needed:
	if len(subsetOutlines.Private) != 1 {
		return fmt.Errorf("need exactly one private dict for a simple font")
	}
	subsetOutlines.ROS = nil
	subsetOutlines.GIDToCID = nil
	if len(subsetOutlines.FontMatrices) > 0 && subsetOutlines.FontMatrices[0] != matrix.Identity {
		fontInfo = clone(fontInfo)
		fontInfo.FontMatrix = subsetOutlines.FontMatrices[0].Mul(fontInfo.FontMatrix)
	}
	subsetOutlines.FontMatrices = nil
	for gid, origGID := range glyphs { // fill in the glyph names
		g := subsetOutlines.Glyphs[gid]
		glyphName := e.Simple.GlyphName(origGID)
		if g.Name == glyphName {
			continue
		}
		g = clone(g)
		g.Name = glyphName
		subsetOutlines.Glyphs[gid] = g
	}
	// The real encoding is set in the PDF font dictionary, so that readers can
	// know the meaning of codes without having to parse the font file. Here we
	// set the built-in encoding of the font to the standard encoding, to
	// minimise font size.
	subsetOutlines.Encoding = cff.StandardEncoding(subsetOutlines.Glyphs)

	subsetCFF := &cff.Font{
		FontInfo: fontInfo,
		Outlines: subsetOutlines,
	}

	// construct the font dictionary and font descriptor
	isSymbolic := false
	for _, gid := range glyphs {
		if gid == 0 {
			continue
		}
		if !pdfenc.StandardLatin.Has[e.Simple.GlyphName(gid)] {
			isSymbolic = true
			break
		}
	}

	qh := subsetCFF.FontMatrix[0] * 1000
	qv := subsetCFF.FontMatrix[3] * 1000

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, e.Font.FontName),
		FontFamily:   subsetCFF.FamilyName,
		FontStretch:  e.Stretch,
		FontWeight:   e.Weight,
		IsFixedPitch: subsetCFF.IsFixedPitch,
		IsSerif:      e.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     e.IsScript,
		IsItalic:     subsetCFF.ItalicAngle != 0,
		ForceBold:    subsetCFF.Private[0].ForceBold,
		FontBBox:     subsetCFF.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetCFF.ItalicAngle,
		Ascent:       e.Ascent,
		Descent:      e.Descent,
		Leading:      e.Leading,
		CapHeight:    e.CapHeight,
		XHeight:      e.XHeight,
		StemV:        math.Round(subsetCFF.Private[0].StdVW * qh),
		StemH:        math.Round(subsetCFF.Private[0].StdHW * qv),
		MissingWidth: e.Simple.DefaultWidth(),
	}
	dict := dict.Type1{
		PostScriptName: e.Font.FontName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       e.Simple.Encoding(),
		FontType:       glyphdata.CFFSimple,
		FontRef:        rm.Out.Alloc(),
		ToUnicode:      e.Simple.ToUnicode(e.Font.FontName),
	}
	for c, info := range e.Simple.MappedCodes() {
		dict.Width[c] = info.Width
	}

	err := dict.WriteToPDF(rm, e.Ref)
	if err != nil {
		return err
	}

	err = cffglyphs.Embed(rm.Out, dict.FontType, dict.FontRef, subsetCFF)
	if err != nil {
		return err
	}

	return nil
}
