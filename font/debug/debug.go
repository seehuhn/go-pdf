// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package debug

import (
	"bytes"
	"math"
	"time"

	"golang.org/x/image/font/gofont/goregular"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/glyf"
	"seehuhn.de/go/pdf/font/type1"
)

// MakeSimpleFont creates a simple font for use in unit tests.
func MakeSimpleFont() *sfnt.Info {
	info, err := sfnt.Read(bytes.NewReader(goregular.TTF))
	if err != nil {
		panic(err)
	}

	var includeGid []font.GlyphID
	cmap := cmap.Format4{}
	encoding := make([]font.GlyphID, 256)

	includeGid = append(includeGid, 0, 1, 2, 3)
	cmap[0x000D] = font.GlyphID(2)
	cmap[0x0020] = font.GlyphID(3)
	encoding[0] = font.GlyphID(1)
	encoding[0x000D] = font.GlyphID(2)
	encoding[0x0020] = font.GlyphID(3)

	var topMin, topMax funit.Int16
	var bottomMin, bottomMax funit.Int16
	for c := 'A'; c <= 'Z'; c++ {
		gid := info.CMap.Lookup(c)
		cmap[uint16(c)] = font.GlyphID(len(includeGid))
		encoding[c] = font.GlyphID(len(includeGid))
		includeGid = append(includeGid, gid)

		ext := info.GlyphExtent(gid)
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

	origOutlines := info.Outlines.(*glyf.Outlines)
	newOutlines := &cff.Outlines{
		Private: []*type1.PrivateDict{
			{
				BlueValues: []funit.Int16{
					bottomMin, bottomMax, topMin, topMax,
				},
			},
		},
		FdSelect: func(font.GlyphID) int {
			return 0
		},
		Encoding: encoding,
	}

	for _, gid := range includeGid {
		origGlyph := origOutlines.Glyphs[gid]
		cffGlyph := cff.NewGlyph(info.GlyphName(gid), info.GlyphWidth(gid))

		var g glyf.SimpleGlyph
		var ok bool
		if origGlyph != nil {
			g, ok = origGlyph.Data.(glyf.SimpleGlyph)
		}
		if !ok {
			newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)
			continue
		}
		glyphInfo, err := g.Decode()
		if err != nil {
			continue
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

			cffGlyph.MoveTo(float64(extended[offs].X), float64(extended[offs].Y))

			i := 0
			for i < n {
				i0 := (i + offs) % n
				if !extended[i0].OnCurve {
					panic("not on curve") // TODO(voss): remove
				}
				i1 := (i0 + 1) % n
				if extended[i1].OnCurve {
					if i == n-1 {
						break
					}
					cffGlyph.LineTo(float64(extended[i1].X), float64(extended[i1].Y))
					i++
				} else {
					// See the following link for converting truetype outlines
					// to CFF outlines:
					// https://pomax.github.io/bezierinfo/#reordering
					i2 := (i1 + 1) % n
					cffGlyph.CurveTo(
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
		newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)
	}

	ext := info.GlyphExtent(info.CMap.Lookup('M'))
	xMid := math.Round(float64(ext.URx+ext.LLx) / 2)
	yMid := math.Round(float64(ext.URy+ext.LLy) / 2)
	a := math.Round(math.Min(xMid, yMid) * 0.8)

	cffGlyph := cff.NewGlyph("marker.left", ext.URx)
	cffGlyph.MoveTo(xMid, yMid)
	cffGlyph.LineTo(xMid-a, yMid-a)
	cffGlyph.LineTo(xMid-a, yMid+a)
	encoding['>'] = font.GlyphID(len(newOutlines.Glyphs))
	cmap[uint16('>')] = font.GlyphID(len(newOutlines.Glyphs))
	newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)

	cffGlyph = cff.NewGlyph("marker.right", ext.URx)
	cffGlyph.MoveTo(xMid, yMid)
	cffGlyph.LineTo(xMid+a, yMid+a)
	cffGlyph.LineTo(xMid+a, yMid-a)
	encoding['<'] = font.GlyphID(len(newOutlines.Glyphs))
	cmap[uint16('<')] = font.GlyphID(len(newOutlines.Glyphs))
	newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)

	cffGlyph = cff.NewGlyph("marker", ext.URx)
	cffGlyph.MoveTo(xMid, yMid)
	cffGlyph.LineTo(xMid-a, yMid-a)
	cffGlyph.LineTo(xMid-a, yMid+a)
	cffGlyph.LineTo(xMid, yMid)
	cffGlyph.LineTo(xMid+a, yMid+a)
	cffGlyph.LineTo(xMid+a, yMid-a)
	encoding['='] = font.GlyphID(len(newOutlines.Glyphs))
	cmap[uint16('=')] = font.GlyphID(len(newOutlines.Glyphs))
	newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)

	now := time.Now()
	res := &sfnt.Info{
		FamilyName:       "Debug",
		Width:            info.Width,
		Weight:           info.Weight,
		Version:          0,
		CreationTime:     now,
		ModificationTime: now,

		UnitsPerEm:         info.UnitsPerEm,
		Ascent:             info.Ascent,
		Descent:            info.Descent,
		LineGap:            info.LineGap,
		CapHeight:          info.CapHeight,
		XHeight:            info.XHeight,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		IsRegular:          true,

		CMap:     cmap,
		Outlines: newOutlines,
	}

	return res
}

// MakeCompleteFont creates a font for use in unit tests.
func MakeCompleteFont() *sfnt.Info {
	info, err := sfnt.Read(bytes.NewReader(goregular.TTF))
	if err != nil {
		panic(err)
	}

	var topMin, topMax funit.Int16
	var bottomMin, bottomMax funit.Int16
	for c := 'A'; c <= 'Z'; c++ {
		gid := info.CMap.Lookup(c)

		ext := info.GlyphExtent(gid)
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

	origOutlines := info.Outlines.(*glyf.Outlines)
	newOutlines := &cff.Outlines{
		Private: []*type1.PrivateDict{
			{
				BlueValues: []funit.Int16{
					bottomMin, bottomMax, topMin, topMax,
				},
			},
		},
		FdSelect: func(font.GlyphID) int {
			return 0
		},
	}

	for i, origGlyph := range origOutlines.Glyphs {
		gid := font.GlyphID(i)
		cffGlyph := cff.NewGlyph(info.GlyphName(gid), info.GlyphWidth(gid))

		var g glyf.SimpleGlyph
		var ok bool
		if origGlyph != nil {
			g, ok = origGlyph.Data.(glyf.SimpleGlyph)
		}
		if !ok {
			newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)
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

			cffGlyph.MoveTo(float64(extended[offs].X), float64(extended[offs].Y))

			i := 0
			for i < n {
				i0 := (i + offs) % n
				if !extended[i0].OnCurve {
					panic("not on curve") // TODO(voss): remove
				}
				i1 := (i0 + 1) % n
				if extended[i1].OnCurve {
					if i == n-1 {
						break
					}
					cffGlyph.LineTo(float64(extended[i1].X), float64(extended[i1].Y))
					i++
				} else {
					// See the following link for converting truetype outlines
					// to CFF outlines:
					// https://pomax.github.io/bezierinfo/#reordering
					i2 := (i1 + 1) % n
					cffGlyph.CurveTo(
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
		newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)
	}

	ext := info.GlyphExtent(info.CMap.Lookup('M'))
	xMid := math.Round(float64(ext.URx+ext.LLx) / 2)
	yMid := math.Round(float64(ext.URy+ext.LLy) / 2)
	a := math.Round(math.Min(xMid, yMid) * 0.8)

	cffGlyph := cff.NewGlyph("marker.left", ext.URx)
	cffGlyph.MoveTo(xMid, yMid)
	cffGlyph.LineTo(xMid-a, yMid-a)
	cffGlyph.LineTo(xMid-a, yMid+a)
	newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)

	cffGlyph = cff.NewGlyph("marker.right", ext.URx)
	cffGlyph.MoveTo(xMid, yMid)
	cffGlyph.LineTo(xMid+a, yMid+a)
	cffGlyph.LineTo(xMid+a, yMid-a)
	newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)

	cffGlyph = cff.NewGlyph("marker", ext.URx)
	cffGlyph.MoveTo(xMid, yMid)
	cffGlyph.LineTo(xMid-a, yMid-a)
	cffGlyph.LineTo(xMid-a, yMid+a)
	cffGlyph.LineTo(xMid, yMid)
	cffGlyph.LineTo(xMid+a, yMid+a)
	cffGlyph.LineTo(xMid+a, yMid-a)
	newOutlines.Glyphs = append(newOutlines.Glyphs, cffGlyph)

	now := time.Now()
	res := &sfnt.Info{
		FamilyName:       "Debug",
		Width:            info.Width,
		Weight:           info.Weight,
		Version:          0,
		CreationTime:     now,
		ModificationTime: now,

		UnitsPerEm:         info.UnitsPerEm,
		Ascent:             info.Ascent,
		Descent:            info.Descent,
		LineGap:            info.LineGap,
		CapHeight:          info.CapHeight,
		XHeight:            info.XHeight,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		IsRegular:          true,

		CMap:     info.CMap,
		Outlines: newOutlines,
	}

	return res
}
