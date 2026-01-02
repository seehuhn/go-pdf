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

package makefont

import (
	"seehuhn.de/go/geom/path"

	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// Type3 returns a Type3 font.
func Type3() (font.Layouter, error) {
	info := clone(TrueType())
	info.EnsureGlyphNames()

	fnt := &type3.Font{
		Glyphs: []*type3.Glyph{
			{}, // .notdef
		},
		PostScriptName:     info.PostScriptName(),
		FontMatrix:         info.FontMatrix,
		FontFamily:         info.FamilyName,
		FontStretch:        info.Width,
		FontWeight:         info.Weight,
		IsFixedPitch:       info.IsFixedPitch(),
		IsSerif:            info.IsSerif,
		IsScript:           info.IsScript,
		ItalicAngle:        info.ItalicAngle,
		Ascent:             float64(info.Ascent),
		Descent:            float64(info.Descent),
		Leading:            float64(info.Ascent - info.Descent + info.LineGap),
		CapHeight:          float64(info.CapHeight),
		XHeight:            float64(info.XHeight),
		UnderlinePosition:  float64(info.UnderlinePosition),
		UnderlineThickness: float64(info.UnderlineThickness),
	}

	// convert glyf outlines to type 3 outlines
	origOutlines := info.Outlines.(*glyf.Outlines)
	for i, origGlyph := range origOutlines.Glyphs {
		gid := glyph.ID(i)
		glyphName := info.GlyphName(gid)
		if glyphName == ".notdef" {
			continue
		}

		width := info.GlyphWidth(gid)

		// Build content stream for the glyph
		b := builder.New(content.Glyph, nil)

		if origGlyph != nil {
			bbox := origGlyph.Rect16
			b.Type3UncoloredGlyph(width, 0,
				float64(bbox.LLx), float64(bbox.LLy),
				float64(bbox.URx), float64(bbox.URy))

			// Draw the glyph path
			glyphPath := origOutlines.Path(gid)
			cubicPath := glyphPath.ToCubic()
			for cmd, pts := range cubicPath {
				switch cmd {
				case path.CmdMoveTo:
					b.MoveTo(pts[0].X, pts[0].Y)
				case path.CmdLineTo:
					b.LineTo(pts[0].X, pts[0].Y)
				case path.CmdCubeTo:
					b.CurveTo(pts[0].X, pts[0].Y, pts[1].X, pts[1].Y, pts[2].X, pts[2].Y)
				case path.CmdClose:
					b.ClosePath()
				}
			}
			b.Fill()
		} else {
			// Empty glyph - just width, no drawing
			b.Type3UncoloredGlyph(width, 0, 0, 0, 0, 0)
		}

		stream, err := b.Harvest()
		if err != nil {
			return nil, err
		}

		g := &type3.Glyph{
			Name:    glyphName,
			Content: stream,
		}

		fnt.Glyphs = append(fnt.Glyphs, g)
	}

	return fnt.New()
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}
