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
	"maps"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
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
				{Title: pdf.Text("Chapter 1")},
			},
		},
	},
	{
		name:    "multiple items",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{Title: pdf.Text("Chapter 1")},
				{Title: pdf.Text("Chapter 2")},
				{Title: pdf.Text("Chapter 3")},
			},
		},
	},
	{
		name:    "nested items",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{
					Title: pdf.Text("Part I"),
					Children: []*Item{
						{Title: pdf.Text("Chapter 1")},
						{Title: pdf.Text("Chapter 2")},
					},
				},
				{
					Title: pdf.Text("Part II"),
					Children: []*Item{
						{Title: pdf.Text("Chapter 3")},
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
					Title: pdf.Text("Level 1"),
					Children: []*Item{
						{
							Title: pdf.Text("Level 2"),
							Children: []*Item{
								{
									Title: pdf.Text("Level 3"),
									Children: []*Item{
										{Title: pdf.Text("Level 4")},
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
					Title: pdf.Text("Open Section"),
					Open:  true,
					Children: []*Item{
						{Title: pdf.Text("Visible Child 1")},
						{Title: pdf.Text("Visible Child 2")},
					},
				},
				{
					Title: pdf.Text("Closed Section"),
					Open:  false,
					Children: []*Item{
						{Title: pdf.Text("Hidden Child")},
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
				{Title: pdf.Text("Normal")},
				{Title: pdf.Text("Bold"), Bold: true},
				{Title: pdf.Text("Italic"), Italic: true},
				{Title: pdf.Text("Bold Italic"), Bold: true, Italic: true},
				{Title: pdf.Text("Red"), Color: color.DeviceRGB{1, 0, 0}},
				{Title: pdf.Text("Green Bold"), Color: color.DeviceRGB{0, 1, 0}, Bold: true},
			},
		},
	},
	{
		name:    "with URI action",
		version: pdf.V1_7,
		outline: &Outline{
			Items: []*Item{
				{
					Title:  pdf.Text("External Link"),
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
					Title: pdf.Text("Contents"),
					Open:  true,
					Children: []*Item{
						{Title: pdf.Text("Introduction")},
						{Title: pdf.Text("Chapter 1"), Bold: true},
						{Title: pdf.Text("Chapter 2"), Bold: true},
					},
				},
				{
					Title:  pdf.Text("External Resources"),
					Action: &action.URI{URI: "https://example.com/"},
					Color:  color.DeviceRGB{0, 0, 1},
					Italic: true,
				},
				{
					Title: pdf.Text("Appendix"),
					Children: []*Item{
						{Title: pdf.Text("A"), Color: color.DeviceRGB{0.5, 0.5, 0.5}},
						{Title: pdf.Text("B"), Color: color.DeviceRGB{0.5, 0.5, 0.5}},
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
					Title: pdf.Text("Modern Document"),
					Open:  true,
					Children: []*Item{
						{Title: pdf.Text("Section 1"), Bold: true, Color: color.DeviceRGB{0.2, 0.4, 0.6}},
						{Title: pdf.Text("Section 2"), Italic: true},
					},
				},
			},
		},
	},
}

func testRoundTrip(t *testing.T, v pdf.Version, o *Outline) {
	t.Helper()

	w, buf := memfile.NewPDFWriter(t, v, nil)

	rm := pdf.NewResourceManager(w)
	outlineRef, err := rm.Store(o)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("write outline: %v", err)
	}
	w.GetMeta().Catalog.Outlines = outlineRef

	err = rm.Close()
	if err != nil {
		t.Fatalf("close resource manager: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("close writer: %v", err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatalf("open document: %v", err)
	}
	defer r.Close()

	decoded, err := Decode(pdf.NewCursor(r), r.GetMeta().Catalog.Outlines, false)
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
		w, buf := memfile.NewPDFWriter(f, tc.version, opt)

		rm := pdf.NewResourceManager(w)
		outlineRef, err := rm.Store(tc.outline)
		if err != nil {
			continue
		}
		w.GetMeta().Catalog.Outlines = outlineRef

		err = rm.Close()
		if err != nil {
			continue
		}
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		defer r.Close()

		outline, err := Decode(pdf.NewCursor(r), r.GetMeta().Catalog.Outlines, false)
		if err != nil {
			t.Skip("malformed outline")
		}
		if outline == nil || len(outline.Items) == 0 {
			t.Skip("no outline")
		}

		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		testRoundTrip(t, version, outline)
	})
}

func TestStructEntry(t *testing.T) {
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	// allocate a dummy structure element
	seRef := w.Alloc()
	err := w.Put(seRef, pdf.Dict{
		"Type": pdf.Name("StructElem"),
		"S":    pdf.Name("P"),
	})
	if err != nil {
		t.Fatal(err)
	}

	o := &Outline{
		Items: []*Item{
			{Title: pdf.Text("With SE"), StructEntry: seRef},
		},
	}

	rm := pdf.NewResourceManager(w)
	outlineRef, err := rm.Store(o)
	if err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Outlines = outlineRef

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	decoded, err := Decode(pdf.NewCursor(r), r.GetMeta().Catalog.Outlines, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(decoded.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(decoded.Items))
	}
	if decoded.Items[0].StructEntry == 0 {
		t.Error("StructEntry not preserved in round trip")
	}
}

func TestReadLoop(t *testing.T) {
	for _, good := range []bool{true, false} {
		w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)

		refRoot := w.Alloc()
		refA := w.Alloc()
		refB := w.Alloc()
		refC := w.Alloc()

		var A pdf.Dict
		if good {
			A = pdf.Dict{
				"Title":  pdf.Text("A"),
				"Next":   refB,
				"Parent": refRoot,
			}
		} else {
			// Create a loop in the outline tree.
			// This causes Acrobat reader to hang (version 2022.003.20310).
			// Let's make sure we do better.
			A = pdf.Dict{
				"Title":  pdf.Text("A"),
				"Next":   refA,
				"Prev":   refA,
				"Parent": refRoot,
			}
		}
		B := pdf.Dict{
			"Title":  pdf.Text("B"),
			"Prev":   refA,
			"Next":   refC,
			"Parent": refRoot,
		}
		C := pdf.Dict{
			"Title":  pdf.Text("C"),
			"Prev":   refB,
			"Parent": refRoot,
		}
		root := pdf.Dict{
			"First": refA,
			"Last":  refC,
		}

		err := w.Put(refA, A)
		if err != nil {
			t.Fatal(err)
		}
		err = w.Put(refB, B)
		if err != nil {
			t.Fatal(err)
		}
		err = w.Put(refC, C)
		if err != nil {
			t.Fatal(err)
		}
		err = w.Put(refRoot, root)
		if err != nil {
			t.Fatal(err)
		}

		w.GetMeta().Catalog.Outlines = refRoot

		err = w.Close()
		if err != nil {
			t.Fatal(err)
		}

		r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
		if err != nil {
			t.Fatal(err)
		}

		ol, err := Decode(pdf.NewCursor(r), r.GetMeta().Catalog.Outlines, false)
		if err != nil {
			t.Errorf("good=%v, err=%v", good, err)
		} else if good {
			if len(ol.Items) != 3 {
				t.Errorf("expected 3 items, got %d", len(ol.Items))
			}
		} else {
			// loop is silently broken
			if len(ol.Items) != 1 {
				t.Errorf("expected 1 item (loop broken), got %d", len(ol.Items))
			}
		}

		r.Close()
	}
}

// TestDeepFirstChainBounded guards against a stack-overflow DoS: a /First
// chain of distinct items is acyclic, so the cycle guard never trips, yet
// recursing one frame per level would exhaust the Go stack.  The depth cap
// must silently truncate the over-deep subtree, like a cycle, keeping the
// items above the cap.  A wide /Next sibling list, by contrast, is
// iterative and must read in full.
func TestDeepFirstChainBounded(t *testing.T) {
	t.Run("deep First chain", func(t *testing.T) {
		n := limits.MaxOutlineDepth + 10
		r := buildOutline(t, n, func(i int, refs []pdf.Reference) pdf.Dict {
			d := pdf.Dict{"Title": pdf.Text("x")}
			if i+1 < n {
				d["First"] = refs[i+1]
			}
			return d
		})
		defer r.Close()

		ol, err := Decode(pdf.NewCursor(r), r.GetMeta().Catalog.Outlines, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		depth := 0
		for items := ol.Items; len(items) > 0; items = items[0].Children {
			depth++
		}
		if depth != limits.MaxOutlineDepth {
			t.Errorf("expected depth %d, got %d", limits.MaxOutlineDepth, depth)
		}
	})

	t.Run("wide Next list", func(t *testing.T) {
		n := 2*limits.MaxOutlineDepth + 50
		r := buildOutline(t, n, func(i int, refs []pdf.Reference) pdf.Dict {
			d := pdf.Dict{"Title": pdf.Text("x")}
			if i+1 < n {
				d["Next"] = refs[i+1]
			}
			return d
		})
		defer r.Close()

		ol, err := Decode(pdf.NewCursor(r), r.GetMeta().Catalog.Outlines, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ol.Items) != n {
			t.Errorf("expected %d items, got %d", n, len(ol.Items))
		}
	})
}

// buildOutline writes a PDF whose outline has n items, each built by item(i).
// refs[i] is the reference of item i; the root's /First points at refs[0].
func buildOutline(t *testing.T, n int, item func(i int, refs []pdf.Reference) pdf.Dict) *pdf.Reader {
	t.Helper()

	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	refs := make([]pdf.Reference, n)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i := range refs {
		if err := w.Put(refs[i], item(i, refs)); err != nil {
			t.Fatal(err)
		}
	}

	refRoot := w.Alloc()
	if err := w.Put(refRoot, pdf.Dict{"First": refs[0]}); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Outlines = refRoot

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestColorOutOfRange checks that colour components outside the range the
// spec allows are clamped on read, so that anything the reader accepts can
// be written back out through the strict writer.
func TestColorOutOfRange(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	rootRef := w.Alloc()
	itemRef := w.Alloc()
	err := w.Put(itemRef, pdf.Dict{
		"Title":  pdf.Text("out of range"),
		"Parent": rootRef,
		"C":      pdf.Array{pdf.Number(2.5), pdf.Number(-1), pdf.Number(0.5)},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(rootRef, pdf.Dict{
		"Type":  pdf.Name("Outlines"),
		"First": itemRef,
		"Last":  itemRef,
		"Count": pdf.Integer(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	o, err := pdf.CursorAt(x, nil).Decode(rootRef, Decode)
	if err != nil {
		t.Fatal(err)
	}

	want := color.DeviceRGB{1, 0, 0.5}
	if got := o.Items[0].Color; got != want {
		t.Errorf("got colour %v, want %v", got, want)
	}

	// the clamped value must survive the strict writer
	w2, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w2)
	if _, err := o.Encode(rm); err != nil {
		t.Errorf("re-encoding a value read from a file failed: %v", err)
	}
}

// TestColorNaN checks that a NaN colour component is rejected by the writer.
// NaN passes any "out of range" test phrased with < and >, but has no valid
// PDF representation.
func TestColorNaN(t *testing.T) {
	o := &Outline{
		Items: []*Item{{
			Title: pdf.Text("not a number"),
			Color: color.DeviceRGB{math.NaN(), 0, 0},
		}},
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	if _, err := o.Encode(rm); err == nil {
		t.Error("expected an error for a NaN colour component")
	}
}

func decodeOutlineItem(t *testing.T, extras pdf.Dict) *Outline {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	rootRef := w.Alloc()
	itemRef := w.Alloc()
	item := pdf.Dict{
		"Title":  pdf.Text("item"),
		"Parent": rootRef,
	}
	maps.Copy(item, extras)
	if err := w.Put(itemRef, item); err != nil {
		t.Fatal(err)
	}
	err := w.Put(rootRef, pdf.Dict{
		"Type":  pdf.Name("Outlines"),
		"First": itemRef,
		"Last":  itemRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	o, err := pdf.CursorAt(x, nil).Decode(rootRef, Decode)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// TestDanglingReference checks that a reference to a free object reads the
// same as an absent entry.
func TestDanglingReference(t *testing.T) {
	dangling := pdf.NewReference(99, 0)
	for _, key := range []pdf.Name{"Dest", "A", "Count", "C", "F", "SE", "First", "Next"} {
		t.Run(string(key), func(t *testing.T) {
			want := decodeOutlineItem(t, nil)
			got := decodeOutlineItem(t, pdf.Dict{key: dangling})
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("dangling reference differs from absent entry (-want +got):\n%s", diff)
			}
		})
	}
}

// TestActionFallback checks that the action is used whenever Dest does not
// give a usable destination.
func TestActionFallback(t *testing.T) {
	act := pdf.Dict{
		"S":   pdf.Name("URI"),
		"URI": pdf.String("https://example.com/"),
	}
	want := decodeOutlineItem(t, pdf.Dict{"A": act})
	if want.Items[0].Action == nil {
		t.Fatal("action not read")
	}

	for name, dest := range map[string]pdf.Object{
		"dangling": pdf.NewReference(99, 0),
		"empty":    pdf.Array{},
		"wrong":    pdf.Integer(7),
	} {
		t.Run(name, func(t *testing.T) {
			got := decodeOutlineItem(t, pdf.Dict{"Dest": dest, "A": act})
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("unexpected outline (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("destination wins", func(t *testing.T) {
		got := decodeOutlineItem(t, pdf.Dict{"Dest": pdf.String("named"), "A": act})
		if got.Items[0].Destination == nil || got.Items[0].Action != nil {
			t.Errorf("got %v, want a destination and no action", got.Items[0])
		}
	})
}

// TestWrongTypeEntries checks that wrong-type values read like absent
// entries, and that an invalid title reads as an empty title.
func TestWrongTypeEntries(t *testing.T) {
	for _, key := range []pdf.Name{"Count", "C", "F", "A"} {
		t.Run(string(key), func(t *testing.T) {
			want := decodeOutlineItem(t, nil)
			got := decodeOutlineItem(t, pdf.Dict{key: pdf.Name("bad")})
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("wrong type differs from absent entry (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("Title", func(t *testing.T) {
		for name, title := range map[string]pdf.Object{
			"wrong type": pdf.Dict{},
			"dangling":   pdf.NewReference(99, 0),
		} {
			o := decodeOutlineItem(t, pdf.Dict{"Title": title})
			if len(o.Items) != 1 || o.Items[0].Title != (pdf.TextString{}) {
				t.Errorf("%s: got %v, want one item with empty title", name, o.Items)
			}
		}
	})
}

// TestOpenWithoutChildren checks that an item with a positive /Count but no
// children reads as closed, so that a read-write-read cycle is stable.
func TestOpenWithoutChildren(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	rootRef := w.Alloc()
	itemRef := w.Alloc()
	err := w.Put(itemRef, pdf.Dict{
		"Title":  pdf.Text("childless"),
		"Parent": rootRef,
		"Count":  pdf.Integer(5),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(rootRef, pdf.Dict{
		"Type":  pdf.Name("Outlines"),
		"First": itemRef,
		"Last":  itemRef,
		"Count": pdf.Integer(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	o1, err := pdf.CursorAt(pdf.NewExtractor(w), nil).Decode(rootRef, Decode)
	if err != nil {
		t.Fatal(err)
	}
	if o1.Items[0].Open {
		t.Error("item without children reads as open")
	}

	testRoundTrip(t, pdf.V2_0, o1)
}

// TestColorNonNumeric checks that a /C entry with a component which is not a
// number reads as absent, rather than with that component set to zero.
func TestColorNonNumeric(t *testing.T) {
	absent := decodeOutlineItem(t, nil)
	if absent.Items[0].Color != (color.DeviceRGB{}) {
		t.Fatalf("absent /C read as %v", absent.Items[0].Color)
	}

	for name, c := range map[string]pdf.Array{
		"name":     {pdf.Integer(1), pdf.Name("x"), pdf.Integer(0)},
		"null":     {pdf.Integer(1), nil, pdf.Integer(0)},
		"dangling": {pdf.Integer(1), pdf.NewReference(99, 0), pdf.Integer(0)},
		"string":   {pdf.String("1"), pdf.Integer(0), pdf.Integer(0)},
	} {
		got := decodeOutlineItem(t, pdf.Dict{"C": c})
		if diff := cmp.Diff(absent, got); diff != "" {
			t.Errorf("%s: differs from absent entry (-absent +got):\n%s", name, diff)
		}
	}

	got := decodeOutlineItem(t, pdf.Dict{"C": pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(0)}})
	if want := (color.DeviceRGB{1, 0, 0}); got.Items[0].Color != want {
		t.Errorf("valid colour %v, want %v", got.Items[0].Color, want)
	}
}
