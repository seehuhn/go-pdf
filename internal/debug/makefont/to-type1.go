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

package makefont

import (
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
)

// toType1 constructs a Type1 font from an sfnt.
func toType1(info *sfnt.Font) (*type1.Font, error) {
	// TODO(voss): base this on ToCFF()

	info = clone(info)
	info.EnsureGlyphNames()

	newOutlines := make(map[string]*type1.Glyph)

	// convert glypf outlines to type1 outlines
	origOutlines := info.Outlines.(*glyf.Outlines)
	for i, origGlyph := range origOutlines.Glyphs {
		gid := glyph.ID(i)
		name := info.GlyphName(gid)
		newGlyph := &type1.Glyph{
			WidthX: info.GlyphWidth(gid),
		}

		if origGlyph != nil {
			glyphPath := origOutlines.Glyphs.Path(gid)
			cubicPath := glyphPath.ToCubic()
			for cmd, pts := range cubicPath {
				switch cmd {
				case path.CmdMoveTo:
					newGlyph.MoveTo(pts[0].X, pts[0].Y)
				case path.CmdLineTo:
					newGlyph.LineTo(pts[0].X, pts[0].Y)
				case path.CmdCubeTo:
					newGlyph.CurveTo(pts[0].X, pts[0].Y, pts[1].X, pts[1].Y, pts[2].X, pts[2].Y)
				case path.CmdClose:
					newGlyph.ClosePath()
				}
			}
		}

		newOutlines[name] = newGlyph
	}

	encoding := make([]string, 256)
	for i := 0; i < 256; i++ {
		name := psenc.StandardEncoding[i]
		if _, ok := newOutlines[name]; ok {
			encoding[i] = name
		} else {
			encoding[i] = ".notdef"
		}
	}

	cmap, err := info.CMapTable.GetBest()
	if err != nil {
		panic("unreachable")
	}

	var topMin, topMax funit.Int16
	var bottomMin, bottomMax funit.Int16
	for c := 'A'; c <= 'Z'; c++ {
		gid := cmap.Lookup(c)

		ext := info.GlyphBBox(gid)
		top := ext.URy
		if c == 'A' || top < topMin {
			topMin = top
		}
		if c == 'A' || top > topMax {
			topMax = top
		}

		if c == 'Q' {
			continue
		}
		bottom := ext.LLy
		if c == 'A' || bottom < bottomMin {
			bottomMin = bottom
		}
		if c == 'A' || bottom > bottomMax {
			bottomMax = bottom
		}
	}

	Private := &type1.PrivateDict{
		BlueValues: []funit.Int16{
			bottomMin, bottomMax, topMin, topMax,
		},
	}

	res := &type1.Font{
		FontInfo:     info.GetFontInfo(),
		Glyphs:       newOutlines,
		Private:      Private,
		Encoding:     encoding,
		CreationDate: info.CreationTime,
	}

	return res, nil
}

func toAFM(info *sfnt.Font) (*afm.Metrics, error) {
	info = clone(info)
	info.EnsureGlyphNames()

	qh := info.FontMatrix[0] * 1000
	qv := info.FontMatrix[3] * 1000

	n := info.NumGlyphs()
	newGlyphs := make(map[string]*afm.GlyphInfo, n)
	for i := 0; i < n; i++ {
		gid := glyph.ID(i)
		name := info.GlyphName(gid)
		bbox := info.GlyphBBox(gid)
		bboxAFM := rect.Rect{ // TODO(voss): use the font matrix
			LLx: float64(bbox.LLx) * qh,
			LLy: float64(bbox.LLy) * qh,
			URx: float64(bbox.URx) * qh,
			URy: float64(bbox.URy) * qh,
		}
		newGlyphs[name] = &afm.GlyphInfo{
			WidthX: info.GlyphWidthPDF(gid),
			BBox:   bboxAFM,
			// TODO(voss): ligatures
		}
	}

	encoding := make([]string, 256)
	for i := 0; i < 256; i++ {
		name := psenc.StandardEncoding[i]
		if _, ok := newGlyphs[name]; ok {
			encoding[i] = name
		} else {
			encoding[i] = ".notdef"
		}
	}

	res := &afm.Metrics{
		Glyphs:             newGlyphs,
		Encoding:           encoding,
		FontName:           info.PostScriptName(),
		FullName:           info.FullName(),
		CapHeight:          info.CapHeight.AsFloat(qv),
		XHeight:            info.XHeight.AsFloat(qv),
		Ascent:             info.Ascent.AsFloat(qv),
		Descent:            info.Descent.AsFloat(qv),
		UnderlinePosition:  float64(info.UnderlinePosition) * qv,
		UnderlineThickness: float64(info.UnderlineThickness) * qv,
		ItalicAngle:        info.ItalicAngle,
		IsFixedPitch:       info.IsFixedPitch(),
		// TODO(voss): kerning
	}

	return res, nil
}
