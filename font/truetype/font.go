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

// New makes a PDF TrueType font from a sfnt.Font.
// The font info must be an OpenType/TrueType font with glyf outlines.
// The font can be embedded as a simple font or as a composite font.
func New(info *sfnt.Font) (font.Embedder, error) {
	if !info.IsGlyf() {
		return nil, errors.New("no glyf outlines in font")
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
	gsubLookups := info.Gsub.FindLookups(opt.Language, opt.GsubFeatures)

	gposFeatures := opt.GposFeatures
	if gposFeatures == nil {
		gposFeatures = gtab.GposDefaultFeatures
	}
	gposLookups := info.Gpos.FindLookups(opt.Language, opt.GposFeatures)

	resource := font.Res{Ref: w.Alloc(), DefName: opt.ResName}

	geometry := &font.Geometry{
		GlyphExtents: scaleBboxesGlyf(info.GlyphBBoxes(), info.UnitsPerEm),
		Widths:       info.WidthsPDF(),

		Ascent:             float64(info.Ascent) / float64(info.UnitsPerEm),
		Descent:            float64(info.Descent) / float64(info.UnitsPerEm),
		BaseLineDistance:   float64(info.Ascent-info.Descent+info.LineGap) / float64(info.UnitsPerEm),
		UnderlinePosition:  float64(info.UnderlinePosition) / float64(info.UnitsPerEm),
		UnderlineThickness: float64(info.UnderlineThickness) / float64(info.UnitsPerEm),
	}

	var res font.Layouter
	if !opt.Composite {
		err := pdf.CheckVersion(w, "simple TrueType fonts", pdf.V1_1)
		if err != nil {
			return nil, err
		}

		res = &embeddedSimple{
			w:             w,
			Res:           resource,
			Geometry:      geometry,
			sfnt:          f.sfnt,
			cmap:          fontCmap,
			gsubLookups:   gsubLookups,
			gposLookups:   gposLookups,
			SimpleEncoder: encoding.NewSimpleEncoder(),
		}
	} else {
		err := pdf.CheckVersion(w, "composite TrueType fonts", pdf.V1_3)
		if err != nil {
			return nil, err
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
			w:           w,
			Res:         resource,
			Geometry:    geometry,
			sfnt:        f.sfnt,
			cmap:        fontCmap,
			gsubLookups: gsubLookups,
			gposLookups: gposLookups,
			GIDToCID:    gidToCID,
			CIDEncoder:  cidEncoder,
		}
	}

	w.AutoClose(res)

	return res, nil
}

func scaleBboxesGlyf(bboxes []funit.Rect16, unitsPerEm uint16) []pdf.Rectangle {
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
