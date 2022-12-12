// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package boxes

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages2"
)

func TestFrame(t *testing.T) {
	text1 := "Von Toffany's fish "
	text2 := "et al. "

	out, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F1, err := builtin.Embed(out, "Times-Roman", "F1")
	// F1, err := truetype.Embed(out, "../font/truetype/ttf/FreeSerif.ttf", "F1")
	if err != nil {
		t.Fatal(err)
	}

	F2, err := builtin.Embed(out, "Times-Italic", "F2")
	if err != nil {
		t.Fatal(err)
	}

	pageTree := pages2.NewTree(out, nil)

	g, err := graphics.NewPage(out)
	if err != nil {
		t.Fatal(err)
	}

	gg1 := F1.Typeset(text1, 12)
	gg2 := F2.Typeset(text2, 12)
	box := &vBox{
		BoxExtent: BoxExtent{
			Width:  pages2.A5.URx - pages2.A5.LLx,
			Height: pages2.A5.URy - pages2.A5.LLy,
			Depth:  0,
		},
		Contents: []Box{
			Kern(30),
			&hBox{
				BoxExtent: BoxExtent{
					Width:  pages2.A5.URx - pages2.A5.LLx,
					Height: 10,
					Depth:  2,
				},
				Contents: []Box{
					Kern(36),
					&TextBox{
						Font:     F1,
						FontSize: 12,
						Glyphs:   gg1,
					},
					&TextBox{
						Font:     F2,
						FontSize: 12,
						Glyphs:   gg2,
					},
					&RuleBox{
						BoxExtent: BoxExtent{
							Width:  30,
							Height: 8,
							Depth:  1.8,
						},
					},
					&glue{
						Length: 0,
						Plus:   stretchAmount{1, 1},
					},
				},
			},
			&glue{
				Length: 0,
				Plus:   stretchAmount{1, 1},
			},
			&hBox{
				BoxExtent: BoxExtent{
					Width:  pages2.A5.URx - pages2.A5.LLx,
					Height: 10,
					Depth:  2,
				},
				Contents: []Box{
					&glue{
						Length: 0,
						Plus:   stretchAmount{1, 1},
					},
					&RuleBox{
						BoxExtent: BoxExtent{
							Width:  20,
							Height: 8,
							Depth:  0,
						},
					},
					&glue{
						Length: 0,
						Plus:   stretchAmount{1, 1},
					},
				},
			},
			Kern(30),
		},
	}

	box.Draw(g, 0, box.Depth)

	dict, err := g.Close()
	if err != nil {
		t.Fatal(err)
	}
	dict["MediaBox"] = pages2.A5

	_, err = pageTree.AppendPage(dict)
	if err != nil {
		t.Fatal(err)
	}

	ref, err := pageTree.Close()
	if err != nil {
		t.Error(err)
	}
	out.Catalog.Pages = ref

	err = out.Close()
	if err != nil {
		t.Error(err)
	}
}

// compile-time test: we implement the correct interfaces
var _ Box = &RuleBox{}
var _ Box = &vBox{}
var _ Box = Kern(0)
var _ Box = &glue{}
var _ Box = &TextBox{}
