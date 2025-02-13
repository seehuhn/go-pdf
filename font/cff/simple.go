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
	"iter"
	"math"
	"math/bits"
	"slices"
	"strings"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
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

	code   map[key]byte
	info   map[byte]*codeInfo
	notdef *codeInfo

	GlyphName     map[glyph.ID]string
	GlyphNameUsed map[string]bool

	finished bool
}

type key struct {
	Gid  glyph.ID
	Text string
}

type codeInfo struct {
	Width float64
	Text  string
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

		code: make(map[key]byte),
		info: make(map[byte]*codeInfo),
		notdef: &codeInfo{
			Width: math.Round(f.Font.GlyphWidthPDF(0)),
		},

		GlyphName:     make(map[glyph.ID]string),
		GlyphNameUsed: make(map[string]bool),
	}
	e.GlyphName[0] = ".notdef"
	e.GlyphNameUsed[".notdef"] = true

	return e
}

func (e *embeddedSimple) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

// Codes returns an iterator over the characters in the PDF string. Each code
// includes the CID, width, and associated text. Missing glyphs map to CID 0
// (notdef).
func (e *embeddedSimple) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for _, c := range s {
			info, ok := e.info[c]
			if !ok {
				info = e.notdef
				code.CID = 0 // CID 0 for .notdef
			} else {
				code.CID = cid.CID(c) + 1 // other CIDs start at 1
			}
			code.Width = info.Width
			code.Text = info.Text

			if !yield(&code) {
				return
			}
		}
	}
}

func (e *embeddedSimple) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	c := s[0]
	info, ok := e.info[c]
	if !ok {
		info = e.notdef
	}
	return info.Width / 1000, 1
}

func (e *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	key := key{Gid: gid, Text: text}
	c, ok := e.code[key]
	if !ok {
		glyphName := e.makeGlyphName(gid, text)
		if len(e.code) < 256 && !e.finished {
			c = e.AllocateCode(glyphName, e.Font.FontName == "ZapfDingbats", &pdfenc.WinAnsi)
			e.info[c] = &codeInfo{
				Width: math.Round(e.Font.GlyphWidthPDF(gid)),
				Text:  text,
			}
		}
		e.code[key] = c
	}
	return append(s, c), e.info[c].Width / 1000
}

// makeGlyphName returns a name for the given glyph.
//
// If the glyph name is not known, the function constructs a new name,
// based on the text of the glyph.
func (e *embeddedSimple) makeGlyphName(gid glyph.ID, text string) string {
	if glyphName, ok := e.GlyphName[gid]; ok {
		return glyphName
	}

	glyphName := e.Font.Outlines.Glyphs[gid].Name
	if glyphName == "" {
		var parts []string
		for _, r := range text {
			parts = append(parts, names.FromUnicode(r))
		}
		if len(parts) > 0 {
			glyphName = strings.Join(parts, "_")
		}
	}

	// add a suffix to make the name unique, if needed
	alt := 0
	base := glyphName
nameLoop:
	for {
		if len(glyphName) <= 31 && !e.GlyphNameUsed[glyphName] {
			break
		}

		// older software may not support glyph names longer than 31 characters
		if len(glyphName) > 31 {
			for idx := len(e.GlyphNameUsed); idx >= 0; idx-- {
				glyphName = fmt.Sprintf("orn%03d", idx)
				if !e.GlyphNameUsed[glyphName] {
					break nameLoop
				}
			}
		}

		alt++
		glyphName = fmt.Sprintf("%s.alt%d", base, alt)
	}

	e.GlyphName[gid] = glyphName
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
		if _, alreadyUsed := e.info[code]; alreadyUsed {
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
			// fill up unused slots first
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
	if e.finished {
		return nil
	}
	e.finished = true

	if len(e.code) > 256 {
		return fmt.Errorf("too many distinct glyphs used in font %q", e.Font.FontName)
	}

	fontInfo := e.Font.FontInfo
	outlines := e.Font.Outlines

	// find the set of used glyphs
	gidIsUsed := make(map[glyph.ID]struct{})
	gidIsUsed[0] = struct{}{} // always include .notdef
	for key := range e.code {
		gidIsUsed[key.Gid] = struct{}{}
	}
	glyphs := maps.Keys(gidIsUsed)
	slices.Sort(glyphs)

	// subset the font, if needed
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
		glyphName := e.GlyphName[origGID]
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
		if !pdfenc.StandardLatin.Has[e.GlyphName[gid]] {
			isSymbolic = true
			break
		}
	}

	enc := make(map[byte]string)
	for key, c := range e.code {
		enc[c] = e.GlyphName[key.Gid]
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
		MissingWidth: e.notdef.Width,
	}
	dict := dict.Type1{
		Ref:            e.Ref,
		PostScriptName: e.Font.FontName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       func(c byte) string { return enc[c] },
		FontType:       glyphdata.CFFSimple,
		FontRef:        rm.Out.Alloc(),
	}
	for c, info := range e.info {
		dict.Width[c] = info.Width
		dict.Text[c] = info.Text
	}

	err := dict.WriteToPDF(rm)
	if err != nil {
		return err
	}

	err = cffglyphs.Embed(rm.Out, dict.FontType, dict.FontRef, subsetCFF)
	if err != nil {
		return err
	}

	return nil
}
