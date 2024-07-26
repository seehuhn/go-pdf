// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"errors"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt"
)

// Instance represents a TrueType font together with the font options.
// This implements the [font.Font] interface.
type Instance struct {
	*sfnt.Font
	*font.Geometry

	layouter *sfnt.Layouter

	Opt *font.Options
}

// New makes a PDF TrueType font from a sfnt.Font.
// The font info must be an OpenType/TrueType font with glyf outlines.
// The font can be embedded as a simple font or as a composite font.
func New(info *sfnt.Font, opt *font.Options) (*Instance, error) {
	if !info.IsGlyf() {
		return nil, errors.New("no glyf outlines in font")
	}

	if opt == nil {
		opt = &font.Options{}
	}

	geometry := &font.Geometry{
		GlyphExtents: scaleBoxesGlyf(info.GlyphBBoxes(), info.UnitsPerEm),
		Widths:       info.WidthsPDF(),

		Ascent:             float64(info.Ascent) / float64(info.UnitsPerEm),
		Descent:            float64(info.Descent) / float64(info.UnitsPerEm),
		BaseLineDistance:   float64(info.Ascent-info.Descent+info.LineGap) / float64(info.UnitsPerEm),
		UnderlinePosition:  float64(info.UnderlinePosition) / float64(info.UnitsPerEm),
		UnderlineThickness: float64(info.UnderlineThickness) / float64(info.UnitsPerEm),
	}

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	f := &Instance{
		Font:     info,
		Geometry: geometry,
		layouter: layouter,
		Opt:      opt,
	}
	return f, nil
}

// Layout implements the [font.Layouter] interface.
func (f *Instance) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	buf := f.layouter.Layout(s)
	seq.Seq = slices.Grow(seq.Seq, len(buf))
	for _, g := range buf {
		xOffset := float64(g.XOffset) * ptSize * f.Font.FontMatrix[0]
		if len(seq.Seq) == 0 {
			seq.Skip += xOffset
		} else {
			seq.Seq[len(seq.Seq)-1].Advance += xOffset
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     g.GID,
			Advance: float64(g.Advance) * ptSize * f.Font.FontMatrix[0],
			Rise:    float64(g.YOffset) * ptSize * f.Font.FontMatrix[3],
			Text:    g.Text,
		})
	}
	return seq
}

// Embed adds the font to a PDF file.
// This implements the [font.Font] interface.
func (f *Instance) Embed(rm *pdf.ResourceManager) (pdf.Object, font.Embedded, error) {
	opt := f.Opt
	if opt == nil {
		opt = &font.Options{}
	}

	w := rm.Out
	ref := w.Alloc()

	var res font.Embedded
	if opt.Composite {
		err := pdf.CheckVersion(w, "composite TrueType fonts", pdf.V1_3)
		if err != nil {
			return nil, nil, err
		}

		var gidToCID cmap.GIDToCID
		if opt.MakeGIDToCID != nil {
			gidToCID = opt.MakeGIDToCID()
		} else {
			gidToCID = cmap.NewGIDToCIDSequential()
		}

		var cidEncoder cmap.CIDEncoder
		if opt.MakeEncoder != nil {
			cidEncoder = opt.MakeEncoder(gidToCID)
		} else {
			cidEncoder = cmap.NewCIDEncoderIdentity(gidToCID)
		}

		res = &embeddedComposite{
			w:          w,
			ref:        ref,
			sfnt:       f.Font,
			GIDToCID:   gidToCID,
			CIDEncoder: cidEncoder,
		}
	} else {
		err := pdf.CheckVersion(w, "simple TrueType fonts", pdf.V1_1)
		if err != nil {
			return nil, nil, err
		}

		res = &embeddedSimple{
			w:             w,
			ref:           ref,
			sfnt:          f.Font,
			SimpleEncoder: encoding.NewSimpleEncoder(),
		}
	}

	return ref, res, nil
}

func scaleBoxesGlyf(bboxes []funit.Rect16, unitsPerEm uint16) []pdf.Rectangle {
	res := make([]pdf.Rectangle, len(bboxes))
	for i, b := range bboxes {
		res[i] = pdf.Rectangle{
			LLx: float64(b.LLx) / float64(unitsPerEm),
			LLy: float64(b.LLy) / float64(unitsPerEm),
			URx: float64(b.URx) / float64(unitsPerEm),
			URy: float64(b.URy) / float64(unitsPerEm),
		}
	}
	return res
}
