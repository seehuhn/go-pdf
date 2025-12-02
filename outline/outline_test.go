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

package outline

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics/color"
)

type testCase struct {
	name    string
	version pdf.Version
	outline *Outline
}

var testCases = []testCase{
	{
		name:    "single item",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{Title: "Chapter 1"},
			},
		},
	},
	{
		name:    "multiple items",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{Title: "Chapter 1"},
				{Title: "Chapter 2"},
				{Title: "Chapter 3"},
			},
		},
	},
	{
		name:    "nested items",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{
					Title: "Part I",
					Children: []*Item{
						{Title: "Chapter 1"},
						{Title: "Chapter 2"},
					},
				},
				{
					Title: "Part II",
					Children: []*Item{
						{Title: "Chapter 3"},
					},
				},
			},
		},
	},
	{
		name:    "deep nesting",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{
					Title: "Level 1",
					Children: []*Item{
						{
							Title: "Level 2",
							Children: []*Item{
								{
									Title: "Level 3",
									Children: []*Item{
										{Title: "Level 4"},
									},
								},
							},
						},
					},
				},
			},
		},
	},
	{
		name:    "open items",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{
					Title: "Open Section",
					Open:  true,
					Children: []*Item{
						{Title: "Visible Child 1"},
						{Title: "Visible Child 2"},
					},
				},
				{
					Title: "Closed Section",
					Open:  false,
					Children: []*Item{
						{Title: "Hidden Child"},
					},
				},
			},
		},
	},
	{
		name:    "with color and style",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{Title: "Normal"},
				{Title: "Bold", Bold: true},
				{Title: "Italic", Italic: true},
				{Title: "Bold Italic", Bold: true, Italic: true},
				{Title: "Red", Color: color.DeviceRGB{1, 0, 0}},
				{Title: "Green Bold", Color: color.DeviceRGB{0, 1, 0}, Bold: true},
			},
		},
	},
	{
		name:    "with URI action",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{
					Title:  "External Link",
					Action: &action.URI{URI: "https://example.com/"},
				},
			},
		},
	},
	{
		name:    "complex outline",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{
					Title: "Contents",
					Open:  true,
					Children: []*Item{
						{Title: "Introduction"},
						{Title: "Chapter 1", Bold: true},
						{Title: "Chapter 2", Bold: true},
					},
				},
				{
					Title:  "External Resources",
					Action: &action.URI{URI: "https://example.com/"},
					Color:  color.DeviceRGB{0, 0, 1},
					Italic: true,
				},
				{
					Title: "Appendix",
					Children: []*Item{
						{Title: "A", Color: color.DeviceRGB{0.5, 0.5, 0.5}},
						{Title: "B", Color: color.DeviceRGB{0.5, 0.5, 0.5}},
					},
				},
			},
		},
	},
	{
		name:    "PDF 2.0 outline",
		version: pdf.V2_0,
		outline: &Outline{
			Items: []*Item{
				{
					Title: "Modern Document",
					Open:  true,
					Children: []*Item{
						{Title: "Section 1", Bold: true, Color: color.DeviceRGB{0.2, 0.4, 0.6}},
						{Title: "Section 2", Italic: true},
					},
				},
			},
		},
	},
}

func testRoundTrip(t *testing.T, v pdf.Version, o *Outline) {
	t.Helper()

	buf := &bytes.Buffer{}
	doc, err := document.WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, v, nil)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	err = o.Write(doc.RM)
	if err != nil {
		t.Fatalf("write outline: %v", err)
	}

	err = doc.Close()
	if err != nil {
		t.Fatalf("close document: %v", err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatalf("open document: %v", err)
	}
	defer r.Close()

	decoded, err := Read(r)
	if err != nil {
		t.Fatalf("read outline: %v", err)
	}

	if diff := cmp.Diff(o, decoded); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testRoundTrip(t, tc.version, tc.outline)
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		buf := &bytes.Buffer{}
		doc, err := document.WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, tc.version, opt)
		if err != nil {
			continue
		}

		err = tc.outline.Write(doc.RM)
		if err != nil {
			continue
		}

		err = doc.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Bytes())
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		defer r.Close()

		outline, err := Read(r)
		if err != nil {
			t.Skip("malformed outline")
		}
		if outline == nil || len(outline.Items) == 0 {
			t.Skip("no outline")
		}

		testRoundTrip(t, pdf.GetVersion(r), outline)
	})
}

func TestReadLoop(t *testing.T) {
	buf := &bytes.Buffer{}

	for _, good := range []bool{true, false} {
		buf.Reset()
		doc, err := document.WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, pdf.V1_7, nil)
		if err != nil {
			t.Fatal(err)
		}

		out := doc.Out

		refRoot := out.Alloc()
		refA := out.Alloc()
		refB := out.Alloc()
		refC := out.Alloc()

		var A pdf.Dict
		if good {
			A = pdf.Dict{
				"Title":  pdf.TextString("A"),
				"Next":   refB,
				"Parent": refRoot,
			}
		} else {
			// Create a loop in the outline tree.
			// This causes Acrobat reader to hang (version 2022.003.20310).
			// Let's make sure we do better.
			A = pdf.Dict{
				"Title":  pdf.TextString("A"),
				"Next":   refA,
				"Prev":   refA,
				"Parent": refRoot,
			}
		}
		B := pdf.Dict{
			"Title":  pdf.TextString("B"),
			"Prev":   refA,
			"Next":   refC,
			"Parent": refRoot,
		}
		C := pdf.Dict{
			"Title":  pdf.TextString("C"),
			"Prev":   refB,
			"Parent": refRoot,
		}
		root := pdf.Dict{
			"First": refA,
			"Last":  refC,
		}

		err = out.Put(refA, A)
		if err != nil {
			t.Fatal(err)
		}
		err = out.Put(refB, B)
		if err != nil {
			t.Fatal(err)
		}
		err = out.Put(refC, C)
		if err != nil {
			t.Fatal(err)
		}
		err = out.Put(refRoot, root)
		if err != nil {
			t.Fatal(err)
		}

		out.GetMeta().Catalog.Outlines = refRoot

		err = doc.Close()
		if err != nil {
			t.Fatal(err)
		}

		r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
		if err != nil {
			t.Fatal(err)
		}

		_, err = Read(r)
		if (err == nil) != good {
			t.Errorf("good=%v, err=%v", good, err)
		}

		r.Close()
	}
}
