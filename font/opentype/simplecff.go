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
	"fmt"
	"math"
	"math/bits"
	"slices"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

var _ interface {
	font.EmbeddedLayouter
	font.Scanner
	pdf.Finisher
} = (*embeddedCFFSimple)(nil)

// embeddedSimple represents an [Instance] which has been embedded in a PDF
// file if the Composite option is not set.  There should be at most one
// embeddedSimple for each [Instance] in a PDF file.
type embeddedCFFSimple struct {
	Font *sfnt.Font

	*dict.Type1
	Code     map[key]byte
	Encoding map[byte]string

	GidToGlyph    map[glyph.ID]string
	GlyphNameUsed map[string]bool

	finished bool
}

type key struct {
	Gid  glyph.ID
	Text string
}

func newEmbeddedCFFSimple(ref pdf.Reference, font *sfnt.Font) *embeddedCFFSimple {
	enc := make(map[byte]string)
	dict := &dict.Type1{
		Ref:            ref,
		PostScriptName: font.PostScriptName(),
		// SubsetTag will be set later, in Finish()
		// Descriptor will be set later, in Finish()
		Encoding: func(code byte) string {
			return enc[code]
		},
		// FontType will be set later, in Finish()
		// FontRef will be set later, in Finish()
	}

	e := &embeddedCFFSimple{
		Font: font,

		Type1:    dict,
		Code:     make(map[key]byte),
		Encoding: enc,

		GidToGlyph:    make(map[glyph.ID]string),
		GlyphNameUsed: make(map[string]bool),
	}
	e.GidToGlyph[0] = ".notdef"
	e.GlyphNameUsed[".notdef"] = true

	return e
}

func (e *embeddedCFFSimple) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	key := key{Gid: gid, Text: text}
	code, ok := e.Code[key]
	if !ok {
		glyphName := e.GlyphName(gid, text)
		if len(e.Code) < 256 {
			code = e.AllocateCode(glyphName, e.PostScriptName == "ZapfDingbats", &pdfenc.WinAnsi)
			e.Encoding[code] = glyphName
			e.Text[code] = text
			e.Width[code] = math.Round(e.Font.GlyphWidthPDF(gid))
		}
		e.Code[key] = code
	}
	return append(s, code), e.Width[code] / 1000
}

// GlyphName returns a name for the given glyph.
//
// If the glyph name is not known, the function constructs a new name,
// based on the text of the glyph.
func (e *embeddedCFFSimple) GlyphName(gid glyph.ID, text string) string {
	if glyphName, ok := e.GidToGlyph[gid]; ok {
		return glyphName
	}

	glyphName := e.Font.GlyphName(gid)
	if glyphName == "" {
		if text == "" {
			glyphName = fmt.Sprintf("orn%03d", len(e.GlyphNameUsed)+1)
		} else {
			var parts []string
			for _, r := range text {
				parts = append(parts, names.FromUnicode(r))
			}

			// For compatibility with old readers, we try to keep glyph names below or
			// at 31 characters, on a best-effort basis.
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

func (e *embeddedCFFSimple) AllocateCode(glyphName string, dingbats bool, target *pdfenc.Encoding) byte {
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
func (e *embeddedCFFSimple) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	if len(e.Code) > 256 {
		return fmt.Errorf("too many distinct glyphs used in font %q", e.PostScriptName)
	}

	// subset the font
	gidIsUsed := make(map[glyph.ID]struct{})
	gidIsUsed[0] = struct{}{} // always include .notdef
	for key := range e.Code {
		gidIsUsed[key.Gid] = struct{}{}
	}
	glyphs := maps.Keys(gidIsUsed)
	slices.Sort(glyphs)

	subsetSfnt, err := e.Font.Subset(glyphs)
	if err != nil {
		return err
	}
	subsetTag := subset.Tag(glyphs, e.Font.NumGlyphs())
	e.SubsetTag = subsetTag

	// convert to a simple font, if needed:
	outlines := subsetSfnt.Outlines.(*cff.Outlines)
	if len(outlines.Private) != 1 {
		return fmt.Errorf("need exactly one private dict for a simple font")
	}
	outlines.ROS = nil
	outlines.GIDToCID = nil
	if outlines.IsCIDKeyed() && outlines.FontMatrices[0] != matrix.Identity {
		subsetSfnt.FontMatrix = outlines.FontMatrices[0].Mul(subsetSfnt.FontMatrix)
	}
	outlines.FontMatrices = nil
	for gid, origGID := range glyphs { // fill in the glyph names
		g := outlines.Glyphs[gid]
		glyphName := e.GidToGlyph[origGID]
		if g.Name == glyphName {
			continue
		}
		g = clone(g)
		g.Name = glyphName
		outlines.Glyphs[gid] = g
	}
	// The real encoding is set in the PDF font dictionary, so that readers can
	// know the meaning of codes without having to parse the font file. Here we
	// set the built-in encoding of the font to the standard encoding, to
	// minimise font size.
	outlines.Encoding = cff.StandardEncoding(outlines.Glyphs)
	subsetSfnt.Outlines = outlines

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

	qh := subsetSfnt.FontMatrix[0] * 1000
	qv := subsetSfnt.FontMatrix[3] * 1000
	ascent := float64(subsetSfnt.Ascent) * qv
	descent := float64(subsetSfnt.Descent) * qv
	leading := float64(subsetSfnt.Ascent-subsetSfnt.Descent+subsetSfnt.LineGap) * qv
	capHeight := float64(subsetSfnt.CapHeight) * qv
	xHeight := float64(subsetSfnt.XHeight) * qv

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, e.PostScriptName),
		FontFamily:   subsetSfnt.FamilyName,
		FontStretch:  subsetSfnt.Width,
		FontWeight:   subsetSfnt.Weight,
		IsFixedPitch: subsetSfnt.IsFixedPitch(),
		IsSerif:      subsetSfnt.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     subsetSfnt.IsScript,
		IsItalic:     subsetSfnt.ItalicAngle != 0,
		ForceBold:    outlines.Private[0].ForceBold,
		FontBBox:     subsetSfnt.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetSfnt.ItalicAngle,
		Ascent:       math.Round(ascent),
		Descent:      math.Round(descent),
		Leading:      math.Round(leading),
		CapHeight:    math.Round(capHeight),
		XHeight:      math.Round(xHeight),
		StemV:        math.Round(outlines.Private[0].StdVW * qh),
		StemH:        math.Round(outlines.Private[0].StdHW * qv),
		MissingWidth: math.Round(subsetSfnt.GlyphWidthPDF(0)),
	}
	e.Descriptor = fd

	e.FontType = glyphdata.OpenTypeCFFSimple
	e.FontRef = rm.Out.Alloc()

	err = e.Type1.WriteToPDF(rm)
	if err != nil {
		return err
	}

	err = opentypeglyphs.Embed(rm.Out, e.FontType, e.FontRef, subsetSfnt)
	if err != nil {
		return err
	}

	return nil
}

func clone[T any](obj *T) *T {
	new := *obj
	return &new
}
