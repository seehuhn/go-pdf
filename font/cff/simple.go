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
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
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

	gd       *GlyphData
	finished bool
}

type codeInfo struct {
	Width float64
	Text  string
}

type GlyphData struct {
	code          map[glyphKey]byte
	info          map[byte]*codeInfo
	notdef        *codeInfo
	GlyphName     map[glyph.ID]string
	glyphNameUsed map[string]bool

	isZapfDingbats bool
	encoding       *pdfenc.Encoding
}

type glyphKey struct {
	gid  glyph.ID
	text string
}

// newGlyphData creates a glyphData and registers the .notdef glyph.
func newGlyphData(notdefWidth float64, isZapfDingbats bool, base *pdfenc.Encoding) *GlyphData {
	gd := &GlyphData{
		code:           make(map[glyphKey]byte),
		info:           make(map[byte]*codeInfo),
		notdef:         &codeInfo{Width: notdefWidth},
		GlyphName:      make(map[glyph.ID]string),
		glyphNameUsed:  make(map[string]bool),
		isZapfDingbats: isZapfDingbats,
		encoding:       base,
	}
	gd.GlyphName[0] = ".notdef"
	gd.glyphNameUsed[".notdef"] = true
	return gd
}

// GetCode looks up the code for a (gid,text) pair, returning the code byte
// and whether it already exists.
func (gd *GlyphData) GetCode(gid glyph.ID, text string) (byte, bool) {
	k := glyphKey{gid: gid, text: text}
	c, ok := gd.code[k]
	return c, ok
}

// new method added to GlyphData
func (gd *GlyphData) SetCode(gid glyph.ID, text string, c byte) {
	gd.code[glyphKey{gid: gid, text: text}] = c
}

// GetData returns the CID, width and text for the given code,
// substituting .notdef if it doesn't exist.
func (gd *GlyphData) GetData(c byte) (cid.CID, float64, string) {
	info, ok := gd.info[c]
	if !ok {
		info = gd.notdef
		return 0, info.Width, info.Text
	}
	return cid.CID(c) + 1, info.Width, info.Text
}

// AllocateCode finds a free code slot (0–255) for the glyph, then stores the
// corresponding codeInfo in gd.info.
func (gd *GlyphData) AllocateCode(gid glyph.ID, defaultGlyphName, text string, width float64) byte {
	if len(gd.code) >= 256 {
		return 0
	}

	glyphName := gd.makeGlyphName(gid, defaultGlyphName, text)

	var r rune
	rr := names.ToUnicode(glyphName, gd.isZapfDingbats)
	if len(rr) > 0 {
		r = rr[0]
	}
	bestScore := -1
	bestCode := byte(0)
	for codeInt := 0; codeInt < 256; codeInt++ {
		code := byte(codeInt)
		if _, used := gd.info[code]; used {
			continue
		}
		score := 0
		stdName := gd.encoding.Encoding[code]
		if stdName == glyphName {
			bestCode = code
			break
		} else if stdName == ".notdef" || stdName == "" {
			score += 100
		} else if !(code == 32 && glyphName != "space") {
			score += 10
		}
		score += bits.TrailingZeros16(uint16(r) ^ uint16(code))
		if score > bestScore {
			bestScore = score
			bestCode = code
		}
	}
	gd.info[bestCode] = &codeInfo{Width: width, Text: text}
	return bestCode
}

// makeGlyphName returns a name for the given glyph, adding any needed suffix.
func (gd *GlyphData) makeGlyphName(gid glyph.ID, defaultGlyphName, text string) string {
	if name, ok := gd.GlyphName[gid]; ok {
		return name
	}
	name := defaultGlyphName
	if name == "" {
		var parts []string
		for _, r := range text {
			parts = append(parts, names.FromUnicode(r))
		}
		if len(parts) > 0 {
			name = strings.Join(parts, "_")
		}
	}
	alt := 0
	base := name
nameLoop:
	for {
		if len(name) <= 31 && !gd.glyphNameUsed[name] {
			break
		}
		if len(name) > 31 {
			for idx := len(gd.glyphNameUsed); idx >= 0; idx-- {
				name = fmt.Sprintf("orn%03d", idx)
				if !gd.glyphNameUsed[name] {
					break nameLoop
				}
			}
		}
		alt++
		name = fmt.Sprintf("%s.alt%d", base, alt)
	}
	gd.GlyphName[gid] = name
	gd.glyphNameUsed[name] = true
	return name
}

// Overflow checks if more than 256 codes have been allocated.
func (gd *GlyphData) Overflow() bool {
	return len(gd.code) > 256
}

// Subset returns a sorted list of the glyphs used in this encoding.
func (gd *GlyphData) Subset() []glyph.ID {
	gidIsUsed := make(map[glyph.ID]struct{})
	gidIsUsed[0] = struct{}{} // always include .notdef
	for k := range gd.code {
		gidIsUsed[k.gid] = struct{}{}
	}
	glyphs := maps.Keys(gidIsUsed)
	slices.Sort(glyphs)
	return glyphs
}

func (gd *GlyphData) Encoding() encoding.Type1 {
	enc := make(map[byte]string)
	for k, c := range gd.code {
		enc[c] = gd.GlyphName[k.gid]
	}
	return func(c byte) string { return enc[c] }
}

func (gd *GlyphData) MissingWidth() float64 {
	return gd.notdef.Width
}

func newEmbeddedSimple(ref pdf.Reference, f *Instance) *embeddedSimple {
	e := &embeddedSimple{
		Ref:       ref,
		Font:      f.Font,
		Stretch:   f.Stretch,
		Weight:    f.Weight,
		IsSerif:   f.IsSerif,
		IsScript:  f.IsScript,
		Ascent:    f.Ascent,
		Descent:   f.Descent,
		Leading:   f.Leading,
		CapHeight: f.CapHeight,
		XHeight:   f.XHeight,
		gd: newGlyphData(
			math.Round(f.Font.GlyphWidthPDF(0)),
			f.Font.FontName == "ZapfDingbats",
			&pdfenc.WinAnsi,
		),
	}

	return e
}

func (*embeddedSimple) WritingMode() font.WritingMode {
	return font.Horizontal
}

func (e *embeddedSimple) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for _, c := range s {
			code.CID, code.Width, code.Text = e.gd.GetData(c)
			code.UseWordSpacing = (c == 0x20)
			if !yield(&code) {
				return
			}
		}
	}
}

func (e *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	c, ok := e.gd.GetCode(gid, text)
	if !ok {
		if !e.finished {
			width := math.Round(e.Font.GlyphWidthPDF(gid))
			c = e.gd.AllocateCode(gid, e.Font.Outlines.Glyphs[gid].Name, text, width)
		}
		e.gd.SetCode(gid, text, c)
	}

	_, w, _ := e.gd.GetData(c)
	return append(s, c), w / 1000
}

func (e *embeddedSimple) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	if e.gd.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q", e.Font.FontName)
	}

	fontInfo := e.Font.FontInfo
	outlines := e.Font.Outlines

	// subset the font, if needed
	glyphs := e.gd.Subset()
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
		glyphName := e.gd.GlyphName[origGID]
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
		if !pdfenc.StandardLatin.Has[e.gd.GlyphName[gid]] {
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
		MissingWidth: e.gd.MissingWidth(),
	}
	dict := dict.Type1{
		Ref:            e.Ref,
		PostScriptName: e.Font.FontName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       e.gd.Encoding(),
		FontType:       glyphdata.CFFSimple,
		FontRef:        rm.Out.Alloc(),
	}
	for c := range 256 {
		_, dict.Width[c], dict.Text[c] = e.gd.GetData(byte(c))
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
