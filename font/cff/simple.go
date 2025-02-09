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
	"math/bits"
	"slices"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

var _ interface {
	font.EmbeddedLayouter
	font.Scanner
	pdf.Finisher
} = (*embeddedSimple)(nil)

// embeddedSimple represents an [Instance] which has been embedded in a PDF
// file if the Composite option is not set.  There should be at most on
// embeddedSimple for each [Instance] in a PDF file.
type embeddedSimple struct {
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

	*dict.Type1
	Code     map[key]byte
	Encoding map[byte]string

	GidToGlyph    map[glyph.ID]string
	GlyphNameUsed map[string]bool
}

type key struct {
	Gid  glyph.ID
	Text string
}

func (e *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	key := key{Gid: gid, Text: text}
	code, ok := e.Code[key]
	if !ok {
		glyphName := e.GlyphName(gid, text)
		if len(e.Code) < 256 {
			code = e.AllocateCode(glyphName, e.Font.FontName == "ZapfDingbats", &pdfenc.Standard)
			e.Encoding[code] = glyphName
			e.Text[code] = text
			e.Width[gid] = math.Round(e.Font.GlyphWidthPDF(gid))
		}
		e.Code[key] = code
	}
	return append(s, code), e.Width[gid]
}

// GlyphName returns a name for the given glyph.
//
// If the glyph name is not known, the function constructs a new name,
// based on the text of the glyph.
func (e *embeddedSimple) GlyphName(gid glyph.ID, text string) string {
	if glyphName, ok := e.GidToGlyph[gid]; ok {
		return glyphName
	}

	// For compatibility with old readers, we try to keep glyph names below or
	// at 31 characters, on a best-effort basis.
	glyphName := e.Font.Outlines.Glyphs[gid].Name
	if glyphName == "" {
		if gid == 0 {
			glyphName = ".notdef"
		} else if text == "" {
			glyphName = fmt.Sprintf("orn%03d", len(e.GlyphNameUsed)+1)
		} else {
			var parts []string
			for _, r := range text {
				parts = append(parts, names.FromUnicode(r))
			}

			for i := range parts {
				if len(glyphName)+1+len(parts[i]) > 31-5 { // try to leave space for a suffix
					break
				}
				if glyphName != "" {
					glyphName += "_"
				}
				glyphName += parts[i]
			}
		}
	}

	// add a suffix to make the name unique, if needed
	base := glyphName
	alt := 0
	for e.GlyphNameUsed[glyphName] {
		alt++
		glyphName = fmt.Sprintf("%s.alt%d", base, alt)
	}
	e.GidToGlyph[gid] = glyphName
	e.GlyphNameUsed[glyphName] = true

	return glyphName
}

func (e *embeddedSimple) AllocateCode(glyphName string, dingbats bool, target *pdfenc.Encoding) byte {
	var r rune
	rr := names.ToUnicode(glyphName, dingbats)
	if len(rr) > 0 {
		r = rr[0]
	}

	bestScore := -1
	bestCode := byte(0)
	for codeInt := 0; codeInt < 256; codeInt++ {
		code := byte(codeInt)
		if _, alreadyUsed := e.Encoding[code]; alreadyUsed {
			continue
		}
		var score int
		stdName := target.Encoding[code]
		if stdName == glyphName {
			// If the glyph is in the target encoding, and the corresponding
			// code is still free, then use it.
			bestCode = code
			break
		} else if stdName == ".notdef" || stdName == "" {
			// fill up the unused slots first
			score += 100
		} else if !(code == 32 && glyphName != "space") {
			// Try to keep code 32 for the space character,
			// in order to not break the PDF word spacing parameter.
			score += 10
		}
		score += bits.TrailingZeros16(uint16(r) ^ uint16(code))

		if score > bestScore {
			bestScore = score
			bestCode = code
		}
	}

	return bestCode
}

// Finish is called when the resource manager is closed.
// At this point the subset of glyphs to be embedded is known.
func (e *embeddedSimple) Finish(rm *pdf.ResourceManager) error {
	if len(e.Code) > 256 {
		return fmt.Errorf("too many distinct glyphs used in font %q", e.Font.FontName)
	}

	fontInfo := e.Font.FontInfo
	outlines := e.Font.Outlines

	// subset the font
	gidIsUsed := make(map[glyph.ID]struct{})
	gidIsUsed[0] = struct{}{} // always include .notdef
	for key := range e.Code {
		gidIsUsed[key.Gid] = struct{}{}
	}
	glyphs := maps.Keys(gidIsUsed)
	slices.Sort(glyphs)
	subsetOutlines := outlines.Subset(glyphs)
	subsetTag := subset.Tag(glyphs, outlines.NumGlyphs())
	e.SubsetTag = subsetTag

	// convert to a simple font, if needed:
	if len(subsetOutlines.Private) != 1 {
		return fmt.Errorf("need exactly one private dict for a simple font")
	}
	subsetOutlines.ROS = nil
	subsetOutlines.GIDToCID = nil
	if outlines.IsCIDKeyed() && subsetOutlines.FontMatrices[0] != matrix.Identity {
		fontInfo = clone(fontInfo)
		fontInfo.FontMatrix = subsetOutlines.FontMatrices[0].Mul(fontInfo.FontMatrix)
	}
	subsetOutlines.FontMatrices = nil
	for gid, origGID := range glyphs { // fill in the glyph names
		g := subsetOutlines.Glyphs[gid]
		glyphName := e.GidToGlyph[origGID]
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

	isSymbolic := false
	for _, gid := range glyphs {
		if gid == 0 {
			continue
		}
		if !pdfenc.StandardLatin.Has[e.GidToGlyph[gid]] {
			isSymbolic = true
			break
		}
	}

	qh := subsetCFF.FontMatrix[0] * 1000
	qv := subsetCFF.FontMatrix[3] * 1000

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, subsetCFF.FontName),
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
		MissingWidth: math.Round(subsetCFF.GlyphWidthPDF(0)),
	}
	e.Descriptor = fd

	e.FontType = glyphdata.CFFSimple
	e.FontRef = rm.Out.Alloc()

	err := e.Type1.WriteToPDF(rm)
	if err != nil {
		return err
	}

	err = cffglyphs.Embed(rm.Out, e.FontType, e.FontRef, subsetCFF)
	if err != nil {
		return err
	}

	return nil
}

func clone[T any](obj *T) *T {
	new := *obj
	return &new
}
