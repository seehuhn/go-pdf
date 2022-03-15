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

package main

import (
	"log"
	"os"
	"time"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/names"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/sfntcff"
	"seehuhn.de/go/pdf/font/type1"
)

func main() {
	now := time.Now()
	info := &sfntcff.Info{
		FamilyName: "Test",
		Weight:     font.WeightNormal,
		Width:      font.WidthNormal,

		Version:          0x00010000,
		CreationTime:     now,
		ModificationTime: now,

		Copyright: "Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>",
		PermUse:   os2.PermInstall,

		UnitsPerEm: 1000,
		Ascent:     700,
		Descent:    -300,
		LineGap:    200,
	}

	cffInfo := &cff.Outlines{}

	g := cff.NewGlyph(".notdef", 550)
	g.MoveTo(0, 0)
	g.LineTo(500, 0)
	g.LineTo(500, 700)
	g.LineTo(0, 700)
	cffInfo.Glyphs = append(cffInfo.Glyphs, g)

	g = cff.NewGlyph("space", 550)
	cffInfo.Glyphs = append(cffInfo.Glyphs, g)

	g = cff.NewGlyph("A", 550)
	g.MoveTo(0, 0)
	g.LineTo(500, 0)
	g.LineTo(250, 710)
	cffInfo.Glyphs = append(cffInfo.Glyphs, g)

	g = cff.NewGlyph("B", 550)
	g.MoveTo(0, 0)
	g.LineTo(200, 0)
	g.CurveTo(300, 0, 500, 75, 500, 175)
	g.CurveTo(500, 275, 300, 350, 200, 350)
	g.CurveTo(300, 350, 500, 425, 500, 525)
	g.CurveTo(500, 625, 300, 700, 200, 700)
	g.LineTo(0, 700)
	cffInfo.Glyphs = append(cffInfo.Glyphs, g)
	cffInfo.Private = []*type1.PrivateDict{
		{
			BlueValues: []int32{-10, 0, 700, 710}, // TODO(voss)
		},
	}
	cffInfo.FdSelect = func(gi font.GlyphID) int { return 0 }

	info.Font = cffInfo
	info.CMap = makeCMap(cffInfo.Glyphs)

	// ----------------------------------------------------------------------

	out, err := os.Create("test.otf")
	if err != nil {
		log.Fatal(err)
	}
	_, err = info.Write(out)
	if err != nil {
		log.Fatal(err)
	}
	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func makeCMap(gg []*cff.Glyph) cmap.Subtable {
	cmap := cmap.Format4{}
	for i, g := range gg {
		rr := names.ToUnicode(string(g.Name), false)
		if len(rr) == 1 {
			r := uint16(rr[0])
			if _, ok := cmap[r]; !ok {
				cmap[r] = font.GlyphID(i)
			}
		}
	}
	return cmap
}
