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
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics"
)

// Type3 returns a Type3 font.
func Type3() (*type3.Instance, error) {
	info := clone(TrueType())
	info.EnsureGlyphNames()

	font := &type3.Font{
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

	// convert glypf outlines to type 3 outlines
	origOutlines := info.Outlines.(*glyf.Outlines)
	for i, origGlyph := range origOutlines.Glyphs {
		gid := glyph.ID(i)
		glyphName := info.GlyphName(gid)
		if glyphName == ".notdef" {
			continue
		}

		g := &type3.Glyph{
			Name:  glyphName,
			Width: info.GlyphWidth(gid),
		}

		if origGlyph != nil {
			bbox := origGlyph.Rect16
			g.BBox = rect.Rect{
				LLx: float64(bbox.LLx),
				LLy: float64(bbox.LLy),
				URx: float64(bbox.URx),
				URy: float64(bbox.URy),
			}
			d := &drawer{glyphs: origOutlines.Glyphs, gid: gid}
			g.Draw = d.Draw
		}

		font.Glyphs = append(font.Glyphs, g)
	}

	return type3.New(font)
}

type drawer struct {
	glyphs glyf.Glyphs
	gid    glyph.ID
}

func (d *drawer) Draw(w *graphics.Writer) error {
	glyphPath := d.glyphs.Path(d.gid)
	for contour := range glyphPath.Contours() {
		cubicContour := path.ToCubic(contour)
		for cmd, pts := range cubicContour {
			switch cmd {
			case path.CmdMoveTo:
				w.MoveTo(pts[0].X, pts[0].Y)
			case path.CmdLineTo:
				w.LineTo(pts[0].X, pts[0].Y)
			case path.CmdCubeTo:
				w.CurveTo(pts[0].X, pts[0].Y, pts[1].X, pts[1].Y, pts[2].X, pts[2].Y)
			case path.CmdClose:
				w.ClosePath()
			}
		}
	}
	w.Fill()
	return nil
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}
