package debug

import (
	"bytes"
	"errors"
	"fmt"

	"golang.org/x/image/font/gofont/goregular"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt/glyf"
	"seehuhn.de/go/pdf/font/sfntcff"
)

// Build a font for use in unit tests.
func Build() (*sfntcff.Info, error) {
	info, err := sfntcff.Read(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, err
	}

	outlines := info.Outlines.(*sfntcff.GlyfOutlines)
	cffOutlines := &cff.Outlines{}
	for c := 'A'; c <= 'Z'; c++ {
		gid := info.CMap.Lookup(c)
		gl := outlines.Glyphs[gid]
		g := gl.Data.(glyf.SimpleGlyph)
		glyphInfo, err := g.Decode()
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Println("glyph", c)
		cffGlyph := cff.NewGlyph(info.GlyphName(gid), info.FGlyphWidth(gid))
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
		cffOutlines.Glyphs = append(cffOutlines.Glyphs, cffGlyph)
	}

	info.Outlines = cffOutlines
	// info.CMap = ...

	return info, errors.New("not implemented")
}
