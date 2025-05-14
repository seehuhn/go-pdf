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

package type3

import (
	"fmt"
	"math"
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyph"
)

var _ interface {
	font.EmbeddedLayouter
	font.Embedded
	pdf.Finisher
} = (*embeddedSimple)(nil)

// embeddedSimple represents an [Instance] which has been embedded in a PDF
// file.
type embeddedSimple struct {
	Ref  pdf.Reference
	Font *Font

	*simpleenc.Simple

	finished bool
}

func newEmbeddedSimple(ref pdf.Reference, font *Font) *embeddedSimple {
	e := &embeddedSimple{
		Ref:    ref,
		Font:   font,
		Simple: simpleenc.NewSimple(0, "", &pdfenc.Standard),
	}
	return e
}

func (e *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	c, ok := e.Simple.GetCode(gid, text)
	if !ok {
		if e.finished {
			return s, 0
		}
		g := e.Font.Glyphs[gid]

		glyphName := g.Name
		width := math.Round(g.Width)

		var err error
		c, err = e.Simple.AllocateCode(gid, glyphName, text, width)
		if err != nil {
			return s, 0
		}
	}

	w := e.Simple.Width(c)
	return append(s, c), w * e.Font.FontMatrix[0]
}

func (e *embeddedSimple) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	if err := e.Simple.Error(); err != nil {
		return pdf.Errorf("Type3 font: %w", err)
	}

	glyphs := e.Simple.Glyphs()

	// Write the glyphs first, so that we can construct the resources
	// dictionary. Here we use a common builder for all glyphs, so that a
	// common resources dictionary for the whole font can be accumulated.
	//
	// TODO(voss):
	//   - consider the discussion at
	//     https://pdf-issues.pdfa.org/32000-2-2020/clause07.html#H7.8.3
	//   - check where different PDF versions put the Resources dictionary
	//   - make it configurable whether to use per-glyph resource dictionaries?
	page := graphics.NewWriter(nil, rm)
	charProcs := make(map[pdf.Name]pdf.Reference)
	for _, gid := range glyphs {
		g := e.Font.Glyphs[gid]
		if g.Name == "" {
			continue
		}
		gRef := rm.Out.Alloc()

		charProcs[pdf.Name(g.Name)] = gRef

		stm, err := rm.Out.OpenStream(gRef, nil, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		page.NewStream(stm)

		// TODO(voss): move "d0" and "d1" to the graphics package, and restrict
		// the list of allowed operators depending on the choice.
		if g.Color {
			fmt.Fprintf(stm, "%s 0 d0\n", format(g.Width))
		} else {
			fmt.Fprintf(stm,
				"%s 0 %s %s %s %s d1\n",
				format(g.Width),
				format(g.BBox.LLx),
				format(g.BBox.LLy),
				format(g.BBox.URx),
				format(g.BBox.URy))
		}
		if g.Draw != nil {
			err = g.Draw(page)
			if err != nil {
				return err
			}
		}
		if page.Err != nil {
			return page.Err
		}
		err = stm.Close()
		if err != nil {
			return err
		}
	}
	resources := page.Resources

	italicAngle := math.Round(e.Font.ItalicAngle*10) / 10

	fd := &font.Descriptor{
		FontName:     e.Font.PostScriptName,
		FontFamily:   e.Font.FontFamily,
		FontStretch:  e.Font.FontStretch,
		FontWeight:   e.Font.FontWeight,
		IsFixedPitch: e.Font.IsFixedPitch,
		IsSerif:      e.Font.IsSerif,
		IsSymbolic:   e.Simple.IsSymbolic(),
		IsScript:     e.Font.IsScript,
		IsItalic:     italicAngle != 0,
		IsAllCap:     e.Font.IsAllCap,
		IsSmallCap:   e.Font.IsSmallCap,
		ItalicAngle:  italicAngle,
		Ascent:       e.Font.Ascent,
		Descent:      e.Font.Descent,
		Leading:      e.Font.Leading,
		CapHeight:    e.Font.CapHeight,
		XHeight:      e.Font.XHeight,
		StemV:        -1,
		MissingWidth: e.Simple.DefaultWidth(),
	}
	dict := &dict.Type3{
		Name:       pdf.Name(e.Font.PostScriptName),
		Descriptor: fd,
		Encoding:   e.Simple.Encoding(),
		CharProcs:  charProcs,
		// FontBBox:   &pdf.Rectangle{},
		FontMatrix: e.Font.FontMatrix,
		Resources:  resources,
	}
	m := make(map[charcode.Code]string)
	for c, info := range e.Simple.MappedCodes() {
		dict.Width[c] = info.Width
		implied := names.ToUnicode(dict.Encoding(byte(c)), "")
		if info.Text != implied {
			m[charcode.Code(c)] = info.Text
		}
	}
	if len(m) > 0 {
		tuInfo, err := cmap.NewToUnicodeFile(charcode.Simple, m)
		if err != nil {
			return err
		}
		dict.ToUnicode = tuInfo
	}

	err := dict.WriteToPDF(rm, e.Ref)
	if err != nil {
		return err
	}

	return nil
}

func (e *embeddedSimple) ToTextSpace(x float64) float64 {
	return x * e.Font.FontMatrix[0]
}

func format(x float64) string {
	return strconv.FormatFloat(x, 'f', -1, 64)
}
