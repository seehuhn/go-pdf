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

package cff

import (
	"errors"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/opentype/gtab"
)

type embedder struct {
	sfnt *sfnt.Font
}

// New makes a PDF CFF font from a sfnt.Font.
// The font info must be an OpenType font with CFF outlines.
// The font can be embedded as a simple font or as a composite font.
func New(info *sfnt.Font) (font.Embedder, error) {
	if !info.IsCFF() {
		return nil, errors.New("no CFF outlines in font")
	}

	return embedder{sfnt: info}, nil
}

func (f embedder) Embed(w pdf.Putter, opt *font.Options) (font.Layouter, error) {
	if opt == nil {
		opt = &font.Options{}
	}

	info := f.sfnt

	fontCmap, err := info.CMapTable.GetBest()
	if err != nil {
		return nil, err
	}

	gsubFeatures := opt.GsubFeatures
	if gsubFeatures == nil {
		gsubFeatures = gtab.GsubDefaultFeatures
	}
	gsubLookups := info.Gsub.FindLookups(opt.Language, gsubFeatures)

	gposFeatures := opt.GposFeatures
	if gposFeatures == nil {
		gposFeatures = gtab.GposDefaultFeatures
	}
	gposLookups := info.Gpos.FindLookups(opt.Language, gposFeatures)

	geometry := &font.Geometry{
		GlyphExtents: bboxesToPDF(info.GlyphBBoxes(), info.FontMatrix[:]),
		Widths:       info.WidthsPDF(),

		Ascent:             float64(info.Ascent) * info.FontMatrix[3],
		Descent:            float64(info.Descent) * info.FontMatrix[3],
		BaseLineDistance:   float64(info.Ascent-info.Descent+info.LineGap) * info.FontMatrix[3],
		UnderlinePosition:  float64(info.UnderlinePosition) * info.FontMatrix[3],
		UnderlineThickness: float64(info.UnderlineThickness) * info.FontMatrix[3],
	}

	resource := font.Res{Ref: w.Alloc(), DefName: opt.ResName}

	var res font.Layouter
	if !opt.Composite {
		err := pdf.CheckVersion(w, "simple CFF fonts", pdf.V1_2)
		if err != nil {
			return nil, err
		}
		res = &embeddedSimple{
			w:        w,
			Res:      resource,
			Geometry: geometry,

			sfnt:        info,
			cmap:        fontCmap,
			gsubLookups: gsubLookups,
			gposLookups: gposLookups,

			SimpleEncoder: encoding.NewSimpleEncoder(),
		}
	} else {
		err := pdf.CheckVersion(w, "composite CFF fonts", pdf.V1_3)
		if err != nil {
			return nil, err
		}

		var gidToCID cmap.GIDToCID
		if opt.MakeGIDToCID != nil {
			gidToCID = opt.MakeGIDToCID()
		} else {
			gidToCID = cmap.NewGIDToCIDIdentity()
		}

		var cidEncoder cmap.CIDEncoder
		if opt.MakeEncoder != nil {
			cidEncoder = opt.MakeEncoder(gidToCID)
		} else {
			cidEncoder = cmap.NewCIDEncoderIdentity(gidToCID)
		}

		res = &embeddedComposite{
			w:        w,
			Res:      resource,
			Geometry: geometry,

			sfnt:        info,
			cmap:        fontCmap,
			gsubLookups: gsubLookups,
			gposLookups: gposLookups,

			GIDToCID:   gidToCID,
			CIDEncoder: cidEncoder,
		}
	}
	w.AutoClose(res)

	return res, nil
}

func bboxesToPDF(bboxes []funit.Rect16, M []float64) []pdf.Rectangle {
	res := make([]pdf.Rectangle, len(bboxes))
	for i, b := range bboxes {
		bPDF := pdf.Rectangle{
			LLx: math.Inf(+1),
			LLy: math.Inf(+1),
			URx: math.Inf(-1),
			URy: math.Inf(-1),
		}
		corners := []struct{ x, y funit.Int16 }{
			{b.LLx, b.LLy},
			{b.LLx, b.URy},
			{b.URx, b.LLy},
			{b.URx, b.URy},
		}
		for _, c := range corners {
			xf := float64(c.x)
			yf := float64(c.y)
			x, y := M[0]*xf+M[2]*yf+M[4], M[1]*xf+M[3]*yf+M[5]
			bPDF.LLx = min(bPDF.LLx, x)
			bPDF.LLy = min(bPDF.LLy, y)
			bPDF.URx = max(bPDF.URx, x)
			bPDF.URy = max(bPDF.URy, y)
		}
		res[i] = bPDF
	}
	return res
}
