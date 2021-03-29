// seehuhn.de/go/pdf - support for reading and writing PDF files
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
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/pages"
)

type subset struct {
	chars map[rune]bool
}

func NewSubset() *subset {
	return &subset{
		chars: make(map[rune]bool),
	}
}

func (ccc *subset) Add(s string) {
	for _, r := range s {
		if unicode.IsGraphic(r) {
			ccc.chars[r] = true
		}
	}
}

func TestFrame(t *testing.T) {
	text1 := "Von Tiffany's fish "
	text2 := "et al. "

	out, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	subset := NewSubset()
	subset.Add(text1)
	subset.Add(text2)
	subset.Add("ﬁ")

	// F1, err := builtin.Embed(out, "Times-Roman", subset.chars)
	F1, err := truetype.Embed(out, "../font/truetype/FreeSerif.ttf", subset.chars)
	if err != nil {
		t.Fatal(err)
	}

	F2, err := builtin.Embed(out, "Times-Italic", subset.chars)
	if err != nil {
		t.Fatal(err)
	}

	page, err := pages.SinglePage(out, &pages.Attributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{
				"F1": F1.Ref,
				"F2": F2.Ref,
			},
		},
		MediaBox: pages.A5,
		Rotate:   0,
	})
	if err != nil {
		t.Fatal(err)
	}

	layout1 := F1.Typeset(text1, 12)
	layout2 := F2.Typeset(text2, 12)
	box := &vBox{
		stuffExtent: stuffExtent{
			Width:  pages.A5.URx - pages.A5.LLx,
			Height: pages.A5.URy - pages.A5.LLy,
			Depth:  0,
		},
		Contents: []stuff{
			kern(30),
			&hBox{
				stuffExtent: stuffExtent{
					Width:  pages.A5.URx - pages.A5.LLx,
					Height: 10,
					Depth:  2,
				},
				Contents: []stuff{
					kern(36),
					&text{
						font:   "F1",
						layout: layout1,
					},
					&text{
						font:   "F2",
						layout: layout2,
					},
					&rule{
						stuffExtent: stuffExtent{
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
				stuffExtent: stuffExtent{
					Width:  pages.A5.URx - pages.A5.LLx,
					Height: 10,
					Depth:  2,
				},
				Contents: []stuff{
					&glue{
						Length: 0,
						Plus:   stretchAmount{1, 1},
					},
					&rule{
						stuffExtent: stuffExtent{
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
			kern(30),
		},
	}

	box.Draw(page, 0, box.Depth)

	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		t.Error(err)
	}
}

// compile-time test: we implement the correct interfaces
var _ stuff = &rule{}
var _ stuff = &vBox{}
var _ stuff = kern(0)
var _ stuff = &glue{}
var _ stuff = &text{}
