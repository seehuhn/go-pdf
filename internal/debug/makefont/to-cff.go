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
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
)

// toCFF converts "glyf" outlines to "CFF" outlines.
//
// The result is inefficient, since we we are using the naive way to
// convert quadratic bezier curves to cubic bezier curves.  Do not use
// this function in production code.
func toCFF(info *sfnt.Font) (*sfnt.Font, error) {
	if info.IsCFF() {
		return info, nil
	}

	cmap, err := info.CMapTable.GetBest()
	if err != nil {
		return nil, err
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

	// convert glypf outlines to cff outlines
	origOutlines := info.Outlines.(*glyf.Outlines)
	encoding := make([]glyph.ID, 256)
	if origOutlines.Names != nil {
		rev := make(map[string]glyph.ID)
		for gid, name := range origOutlines.Names {
			rev[name] = glyph.ID(gid)
		}
		for i, name := range pdfenc.Standard.Encoding {
			encoding[i] = rev[name]
		}
	}
	newOutlines := &cff.Outlines{
		Private: []*type1.PrivateDict{
			{
				BlueValues: []funit.Int16{
					bottomMin, bottomMax, topMin, topMax,
				},
				BlueScale: 0.039625,
				BlueShift: 7,
				BlueFuzz:  1,
			},
		},
		Encoding: encoding,
		FDSelect: func(glyph.ID) int { return 0 },
	}

	for i, origGlyph := range origOutlines.Glyphs {
		gid := glyph.ID(i)
		newGlyph := cff.NewGlyph(info.GlyphName(gid), info.GlyphWidth(gid))

		var g glyf.SimpleGlyph
		var ok bool
		if origGlyph != nil {
			g, ok = origGlyph.Data.(glyf.SimpleGlyph)
		}
		if !ok {
			newOutlines.Glyphs = append(newOutlines.Glyphs, newGlyph)
			continue
		}
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

			newGlyph.MoveTo(float64(extended[offs].X), float64(extended[offs].Y))

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
					newGlyph.LineTo(float64(extended[i1].X), float64(extended[i1].Y))
					i++
				} else {
					// See the following link for converting truetype outlines
					// to CFF outlines:
					// https://pomax.github.io/bezierinfo/#reordering
					i2 := (i1 + 1) % n
					newGlyph.CurveTo(
						float64(extended[i0].X)/3+float64(extended[i1].X)*2/3,
						float64(extended[i0].Y)/3+float64(extended[i1].Y)*2/3,
						float64(extended[i1].X)*2/3+float64(extended[i2].X)/3,
						float64(extended[i1].Y)*2/3+float64(extended[i2].Y)/3,
						float64(extended[i2].X),
						float64(extended[i2].Y))
					i += 2
				}
			}
		}
		newOutlines.Glyphs = append(newOutlines.Glyphs, newGlyph)
	}
	info.Outlines = newOutlines

	return info, nil
}

// toCFFCID modifies a font to use CFF CIDFont operators.
func toCFFCID(info *sfnt.Font) (*sfnt.Font, error) {
	info, err := toCFF(info)
	if err != nil {
		return nil, err
	}

	outlines := clone(info.Outlines.(*cff.Outlines))
	outlines.Encoding = nil
	outlines.ROS = &cid.SystemInfo{
		Registry:   "Seehuhn",
		Ordering:   "Sonderbar",
		Supplement: 0,
	}
	outlines.GIDToCID = make([]cid.CID, len(outlines.Glyphs))
	for i := range outlines.GIDToCID {
		outlines.GIDToCID[i] = cid.CID(i)
	}
	outlines.FontMatrices = []matrix.Matrix{matrix.Identity}
	info.Outlines = outlines

	return info, nil
}

// toCFFCID2 modifies a font to use CFF CIDFont operators
// with multiple private dictionaries.
func toCFFCID2(info *sfnt.Font) (*sfnt.Font, error) {
	info, err := toCFFCID(info)
	if err != nil {
		return nil, err
	}

	outlines := info.Outlines.(*cff.Outlines)
	if len(outlines.Private) != 1 {
		panic("unexpected number of private dictionaries")
	}
	p := outlines.Private[0]
	outlines.Private = []*type1.PrivateDict{p, p}
	outlines.FDSelect = func(gid glyph.ID) int {
		if gid%2 == 0 {
			return 0
		}
		return 1
	}
	outlines.FontMatrices = []matrix.Matrix{matrix.Identity, matrix.Identity}

	return info, nil
}
