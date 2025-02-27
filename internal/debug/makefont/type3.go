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
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/type1/names"

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
			d := &drawer{g: origGlyph}
			g.Draw = d.Draw
		}

		font.Glyphs = append(font.Glyphs, g)
	}

	cmap := make(map[rune]glyph.ID)
	for gid, g := range font.Glyphs {
		rr := names.ToUnicode(g.Name, font.PostScriptName == "ZapfDingbats")
		if len(rr) != 1 {
			continue
		}
		cmap[rr[0]] = glyph.ID(gid)
	}

	inst := &type3.Instance{
		Font: font,
		CMap: cmap,
	}
	return inst, nil
}

type drawer struct {
	g *glyf.Glyph
}

func (d *drawer) Draw(w *graphics.Writer) {
	origGlyph := d.g

	switch g := origGlyph.Data.(type) {
	case glyf.SimpleGlyph:
		glyphInfo, err := g.Decode()
		if err != nil {
			panic(err)
		}

		for _, cc := range glyphInfo.Contours {
			var extended glyf.Contour
			var prev glyf.Point
			onCurve := true
			for _, cur := range cc {
				if !onCurve && !cur.OnCurve {
					extended = append(extended, glyf.Point{
						X:       (cur.X + prev.X) / 2,
						Y:       (cur.Y + prev.Y) / 2,
						OnCurve: true,
					})
				}
				extended = append(extended, cur)
				prev = cur
				onCurve = cur.OnCurve
			}
			n := len(extended)

			var offs int
			for i := 0; i < len(extended); i++ {
				if extended[i].OnCurve {
					offs = i
					break
				}
			}

			w.MoveTo(float64(extended[offs].X), float64(extended[offs].Y))

			i := 0
			for i < n {
				i0 := (i + offs) % n
				if !extended[i0].OnCurve {
					panic("not on curve")
				}
				i1 := (i0 + 1) % n
				if extended[i1].OnCurve {
					if i == n-1 {
						break
					}
					w.LineTo(float64(extended[i1].X), float64(extended[i1].Y))
					i++
				} else {
					// See the following link for converting truetype outlines
					// to CFF outlines:
					// https://pomax.github.io/bezierinfo/#reordering
					i2 := (i1 + 1) % n
					w.CurveTo(
						float64(extended[i0].X)/3+float64(extended[i1].X)*2/3,
						float64(extended[i0].Y)/3+float64(extended[i1].Y)*2/3,
						float64(extended[i1].X)*2/3+float64(extended[i2].X)/3,
						float64(extended[i1].Y)*2/3+float64(extended[i2].Y)/3,
						float64(extended[i2].X),
						float64(extended[i2].Y))
					i += 2
				}
			}

			w.ClosePath()
		}
		w.Fill()
	case glyf.CompositeGlyph:
		panic("not implemented")
	}
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}
