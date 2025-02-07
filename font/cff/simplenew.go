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
	"errors"
	"fmt"
	"math"
	"math/bits"
	"slices"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"
)

var _ interface {
	font.Layouter
} = (*InstanceNew)(nil)

type InstanceNew struct {
	Font *cff.Font
	*font.Geometry
	Layouter *sfnt.Layouter

	Stretch  os2.Width
	Weight   os2.Weight
	IsSerif  bool
	IsScript bool
}

func NewNew(info *sfnt.Font, opt *font.Options) (*InstanceNew, error) {
	if opt == nil {
		opt = &font.Options{}
	}

	fontInfo := &type1.FontInfo{
		FontName:           info.PostScriptName(),
		Version:            info.Version.String(),
		Notice:             info.Trademark,
		Copyright:          info.Copyright,
		FullName:           info.FullName(),
		FamilyName:         info.FamilyName,
		Weight:             info.Weight.String(),
		ItalicAngle:        info.ItalicAngle,
		IsFixedPitch:       info.IsFixedPitch(),
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		FontMatrix:         info.FontMatrix,
	}
	outlines, ok := info.Outlines.(*cff.Outlines)
	if !ok {
		return nil, errors.New("no CFF outlines in font")
	}
	cffFont := &cff.Font{
		FontInfo: fontInfo,
		Outlines: outlines,
	}

	glyphExtents := make([]rect.Rect, len(outlines.Glyphs))
	for gid := range outlines.Glyphs {
		glyphExtents[gid] = outlines.GlyphBBoxPDF(info.FontMatrix, glyph.ID(gid))
	}

	geometry := &font.Geometry{
		Ascent:             float64(info.Ascent) * info.FontMatrix[3],
		Descent:            float64(info.Descent) * info.FontMatrix[3],
		BaseLineDistance:   float64(info.Ascent-info.Descent+info.LineGap) * info.FontMatrix[3],
		UnderlinePosition:  float64(info.UnderlinePosition) * info.FontMatrix[3],
		UnderlineThickness: float64(info.UnderlineThickness) * info.FontMatrix[3],

		GlyphExtents: glyphExtents,
		Widths:       info.WidthsPDF(),
	}

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	f := &InstanceNew{
		Font:     cffFont,
		Geometry: geometry,
		Layouter: layouter,

		Stretch:  info.Width,
		Weight:   info.Weight,
		IsSerif:  info.IsSerif,
		IsScript: info.IsScript,
	}
	return f, nil
}

func (f *InstanceNew) PostScriptName() string {
	return f.Font.FontName
}

func (f *InstanceNew) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	qh := ptSize * f.Font.FontMatrix[0]
	qv := ptSize * f.Font.FontMatrix[3]

	buf := f.Layouter.Layout(s)
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
			Advance: float64(g.Advance) * ptSize * qh,
			Rise:    float64(g.YOffset) * ptSize * qv,
			Text:    g.Text,
		})
	}
	return seq
}

func (f *InstanceNew) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	ref := rm.Out.Alloc()

	enc := make(map[byte]string)
	dict := &dict.Type1{
		Ref:            ref,
		PostScriptName: f.Font.FontName,
		// SubsetTag will be set later
		// Descriptor will be set later
		Encoding: func(code byte) string {
			return enc[code]
		},
		// FontType will be set later
		// FontRef will be set later
	}
	e := &embeddedSimpleNew{
		Type1:    dict,
		Code:     make(map[key]byte),
		Encoding: enc,

		Font:     f.Font,
		Stretch:  f.Stretch,
		Weight:   f.Weight,
		IsSerif:  f.IsSerif,
		IsScript: f.IsScript,
	}

	return e.Ref, e, nil
}

var _ interface {
	font.EmbeddedLayouter
	font.Scanner
	pdf.Finisher
} = (*embeddedSimpleNew)(nil)

type embeddedSimpleNew struct {
	*dict.Type1
	Code     map[key]byte
	Encoding map[byte]string

	GidToGlyph    map[glyph.ID]string
	GlyphNameUsed map[string]bool

	Font     *cff.Font
	Stretch  os2.Width
	Weight   os2.Weight
	IsSerif  bool
	IsScript bool
}

type key struct {
	Gid  glyph.ID
	Text string
}

func (e *embeddedSimpleNew) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	key := key{Gid: gid, Text: text}
	if code, ok := e.Code[key]; !ok {
		return append(s, code), e.Width[gid]
	}

	glyphName := e.GlyphName(gid, text)

	var code byte
	if len(e.Code) < 256 {
		code = e.AllocateCode(glyphName, e.Font.FontName == "ZapfDingbats", &pdfenc.Standard)
	}

	e.Code[key] = code
	e.Encoding[code] = glyphName
	e.Text[code] = text
	e.Width[gid] = math.Round(e.Font.GlyphWidthPDF(gid))
	return append(s, code), e.Width[gid]
}

// GlyphName returns the name of the glyph with the given ID.
//
// If the glyph name is not known, the function constructs a new name,
// based on the text of the glyph.
func (e *embeddedSimpleNew) GlyphName(gid glyph.ID, text string) string {
	if glyphName, ok := e.GidToGlyph[gid]; ok {
		return glyphName
	}

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
				if len(glyphName)+1+len(parts[i]) > 25 {
					break
				}
				if glyphName != "" {
					glyphName += "_"
				}
				glyphName += parts[i]
			}
		}
	}

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

func (e *embeddedSimpleNew) AllocateCode(glyphName string, dingbats bool, target *pdfenc.Encoding) byte {
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

func (e *embeddedSimpleNew) Finish(rm *pdf.ResourceManager) error {
	if len(e.Code) > 256 {
		return fmt.Errorf("too many distinct glyphs used in font %q", e.Font.FontName)
	}

	// subset the font
	gidIsUsed := make(map[glyph.ID]struct{})
	gidIsUsed[0] = struct{}{} // always include .notdef
	for key := range e.Code {
		gidIsUsed[key.Gid] = struct{}{}
	}
	glyphs := maps.Keys(gidIsUsed)
	slices.Sort(glyphs)

	subsetCFF := &cff.Font{
		FontInfo: e.Font.FontInfo,
		Outlines: e.Font.Outlines.Subset(glyphs),
	}

	// TODO(voss): convert to simple font, if needed

	// TODO(voss) set e.Type1.SubsetTag
	var subsetTag string
	e.SubsetTag = subsetTag

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, subsetCFF.FontName),
		FontFamily:   subsetCFF.FamilyName,
		FontStretch:  e.Stretch,
		FontWeight:   e.Weight,
		IsFixedPitch: subsetCFF.IsFixedPitch,
		IsSerif:      e.IsSerif,
		IsSymbolic:   false, // TODO
		IsScript:     e.IsScript,
		IsItalic:     subsetCFF.ItalicAngle != 0,
		ForceBold:    subsetCFF.Private[0].ForceBold,
		FontBBox:     subsetCFF.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetCFF.ItalicAngle,
		Ascent:       0, // TODO
		Descent:      0, // TODO
		Leading:      0, // TODO
		CapHeight:    0, // TODO
		XHeight:      0, // TODO
		StemV:        0, // TODO
		StemH:        0, // TODO
		MaxWidth:     0, // TODO
		AvgWidth:     0, // TODO
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
