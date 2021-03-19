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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/pages"
)

func TestFrame(t *testing.T) {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F1 := builtin.BuiltIn("Times-Roman", font.MacRomanEncoding)
	F1Dict, err := out.Write(pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("builtin"),
		"BaseFont": pdf.Name("Times-Roman"),
		"Encoding": pdf.Name("MacRomanEncoding"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	F2 := builtin.BuiltIn("Times-Italic", font.MacRomanEncoding)
	F2Dict, err := out.Write(pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("builtin"),
		"BaseFont": pdf.Name("Times-Italic"),
		"Encoding": pdf.Name("MacRomanEncoding"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	pageTree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{
				"F1": F1Dict,
				"F2": F2Dict,
			},
		},
		MediaBox: pages.A5,
		Rotate:   0,
	})

	text1, err := F1.TypeSet("Von Tiffany's fish ", 12)
	if err != nil {
		t.Fatal(err)
	}
	text2, err := F2.TypeSet("et al. ", 12)
	if err != nil {
		t.Fatal(err)
	}
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
						layout: text1,
					},
					&text{
						font:   "F2",
						layout: text2,
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

	page, err := pageTree.AddPage(&pages.Attributes{
		MediaBox: &pdf.Rectangle{
			URx: box.Width,
			URy: box.Height + box.Depth,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	box.Draw(page, 0, box.Depth)

	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	pages, err := pageTree.Flush()
	if err != nil {
		t.Fatal(err)
	}

	err = out.SetCatalog(pdf.Struct(&pdf.Catalog{
		Pages: pages,
	}))
	if err != nil {
		t.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		t.Error(err)
	}
}

// compile time test: we implement the correct interfaces
var _ stuff = &rule{}
var _ stuff = &vBox{}
var _ stuff = kern(0)
var _ stuff = &glue{}
var _ stuff = &text{}
