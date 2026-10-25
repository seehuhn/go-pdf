// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package decode

import (
	"errors"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// failGetter is a pdf.Getter which fails for one reference with an error
// that is not a malformed-file error.
type failGetter struct {
	pdf.Getter
	fail pdf.Reference
	err  error
}

func (g failGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	if ref == g.fail {
		return nil, g.err
	}
	return g.Getter.Get(ref, canObjStm)
}

var (
	errTestRead = errors.New("test read error")
	testFailRef = pdf.NewReference(77, 0)
)

func failingCursor() pdf.Cursor {
	return pdf.NewCursor(failGetter{Getter: mock.Getter, fail: testFailRef, err: errTestRead})
}

func testRect() pdf.Array {
	return pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)}
}

// TestOptionalBooleanEntries checks that a dangling reference, null and a
// value of the wrong type are read like an absent boolean entry, and that a
// valid value is effective.
func TestOptionalBooleanEntries(t *testing.T) {
	annot := func(subtype pdf.Name, extra pdf.Dict) func(pdf.Dict) (any, error) {
		return func(d pdf.Dict) (any, error) {
			d = maps.Clone(d)
			d["Subtype"] = subtype
			d["Rect"] = testRect()
			maps.Copy(d, extra)
			return Annotation(pdf.NewCursor(mock.Getter), d, false)
		}
	}

	cases := []struct {
		name   string
		key    pdf.Name
		decode func(pdf.Dict) (any, error)
	}{
		{"Line Cap", "Cap", annot("Line", pdf.Dict{"L": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(5), pdf.Integer(5)}})},
		{"Popup Open", "Open", annot("Popup", nil)},
		{"Redact Repeat", "Repeat", annot("Redact", nil)},
		{"Text Open", "Open", annot("Text", nil)},
		{"Form NeedAppearances", "NeedAppearances", func(d pdf.Dict) (any, error) {
			d = maps.Clone(d)
			d["Fields"] = pdf.Array{}
			return Form(pdf.NewCursor(mock.Getter), d, true)
		}},
		{"Sig AddRevInfo", "AddRevInfo", func(d pdf.Dict) (any, error) {
			return sigSeedValue(pdf.NewCursor(mock.Getter), d, true)
		}},
	}

	invalid := map[string]pdf.Object{
		"dangling": pdf.NewReference(99, 0),
		"null":     nil,
		"name":     pdf.Name("x"),
		"integer":  pdf.Integer(5),
		"string":   pdf.String("true"),
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			absent, err := tc.decode(pdf.Dict{})
			if err != nil {
				t.Fatal(err)
			}

			on, err := tc.decode(pdf.Dict{tc.key: pdf.Boolean(true)})
			if err != nil {
				t.Fatal(err)
			}
			if cmp.Diff(absent, on) == "" {
				t.Errorf("a true value has no effect")
			}

			for name, value := range invalid {
				got, err := tc.decode(pdf.Dict{tc.key: value})
				if err != nil {
					t.Errorf("%s: %v", name, err)
					continue
				}
				if diff := cmp.Diff(absent, got); diff != "" {
					t.Errorf("%s: read differently from absent entry (-absent +got):\n%s", name, diff)
				}
			}
		})
	}
}

// TestReadErrorsPropagate checks that read errors which are not malformed-file
// errors are returned, rather than being treated as an absent entry.
func TestReadErrorsPropagate(t *testing.T) {
	f := testFailRef
	num := func(n ...int) pdf.Array {
		a := make(pdf.Array, len(n))
		for i, x := range n {
			a[i] = pdf.Integer(x)
		}
		return a
	}

	cases := []struct {
		name    string
		subtype pdf.Name
		extra   pdf.Dict
	}{
		{"Circle RD", "Circle", pdf.Dict{"RD": f}},
		{"Square RD", "Square", pdf.Dict{"RD": f}},
		{"Caret RD", "Caret", pdf.Dict{"RD": f}},
		{"Caret Sy", "Caret", pdf.Dict{"Sy": f}},
		{"Line L", "Line", pdf.Dict{"L": f}},
		{"Line L element", "Line", pdf.Dict{"L": pdf.Array{f, pdf.Integer(0), pdf.Integer(1), pdf.Integer(1)}}},
		{"Line LE", "Line", pdf.Dict{"L": num(0, 0, 1, 1), "LE": f}},
		{"Line LE element", "Line", pdf.Dict{"L": num(0, 0, 1, 1), "LE": pdf.Array{f}}},
		{"Line LE second element", "Line", pdf.Dict{"L": num(0, 0, 1, 1), "LE": pdf.Array{pdf.Name("None"), f}}},
		{"Line CP", "Line", pdf.Dict{"L": num(0, 0, 1, 1), "Cap": pdf.Boolean(true), "CP": f}},
		{"Link QuadPoints", "Link", pdf.Dict{"QuadPoints": f}},
		{"Redact QuadPoints", "Redact", pdf.Dict{"QuadPoints": f}},
		{"Redact OverlayText", "Redact", pdf.Dict{"OverlayText": f}},
		{"Redact DA", "Redact", pdf.Dict{"OverlayText": pdf.String("x"), "DA": f}},
		{"FreeText CL", "FreeText", pdf.Dict{"IT": pdf.Name("FreeTextCallout"), "CL": pdf.Array{f, f, f, f}}},
		{"FreeText RD", "FreeText", pdf.Dict{"RD": pdf.Array{f, f, f, f}}},
		{"PolyLine LE", "PolyLine", pdf.Dict{"Vertices": num(0, 0, 1, 1), "LE": f}},
		{"PolyLine LE element", "PolyLine", pdf.Dict{"Vertices": num(0, 0, 1, 1), "LE": pdf.Array{f}}},
		{"PolyLine LE second element", "PolyLine", pdf.Dict{"Vertices": num(0, 0, 1, 1), "LE": pdf.Array{pdf.Name("None"), f}}},
		{"Movie A", "Movie", pdf.Dict{"Movie": pdf.Dict{"F": pdf.String("a.mov")}, "A": f}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dict := pdf.Dict{
				"Subtype": tc.subtype,
				"Rect":    testRect(),
			}
			maps.Copy(dict, tc.extra)
			_, err := Annotation(failingCursor(), dict, false)
			if !errors.Is(err, errTestRead) {
				t.Errorf("got %v, want the read error", err)
			}
		})
	}
}

// TestFieldContextReadErrors checks that read errors are returned from the
// inheritable field attributes, both for a field's own entries and for the
// /Parent chain.
func TestFieldContextReadErrors(t *testing.T) {
	c := failingCursor()
	for _, key := range []pdf.Name{"FT", "Ff", "V", "DV", "DA", "Q", "MaxLen", "Opt"} {
		_, err := applyOwnContext(inherited{}, c, pdf.Dict{key: testFailRef})
		if !errors.Is(err, errTestRead) {
			t.Errorf("%s: got %v, want the read error", key, err)
		}
	}

	// a read error in an ancestor is returned
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	parent := w.Alloc()
	if err := w.Put(parent, pdf.Dict{"Q": testFailRef}); err != nil {
		t.Fatal(err)
	}
	g := failGetter{Getter: w, fail: testFailRef, err: errTestRead}
	_, err := inheritedFromChain(pdf.NewCursor(g), pdf.Dict{"Parent": parent})
	if !errors.Is(err, errTestRead) {
		t.Errorf("parent chain: got %v, want the read error", err)
	}
}

// TestParentCycleEndsChain checks that a /Parent chain which loops back on
// itself, or which leads to a non-dictionary, ends at the form defaults.
func TestParentCycleEndsChain(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	a, b, c := w.Alloc(), w.Alloc(), w.Alloc()
	for ref, dict := range map[pdf.Reference]pdf.Dict{
		a: {"Parent": b, "Q": pdf.Integer(1)},
		b: {"Parent": a, "Q": pdf.Integer(2)},
	} {
		if err := w.Put(ref, dict); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Put(c, pdf.Integer(3)); err != nil {
		t.Fatal(err)
	}

	cur := pdf.NewCursor(w)
	want := inherited{q: pdf.TextAlignLeft}
	for _, parent := range []pdf.Reference{a, c} {
		got, err := inheritedFromChain(cur, pdf.Dict{"Parent": parent})
		if err != nil {
			t.Fatal(err)
		}
		if parent == c {
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(inherited{})); diff != "" {
				t.Errorf("non-dictionary parent (-want +got):\n%s", diff)
			}
		} else if got.q != pdf.TextAlignCenter {
			t.Errorf("cycle: q = %v, want the parent's own value", got.q)
		}
	}
}

// TestNonNumericColor checks that a colour array with a component which is
// not a number is read as absent, rather than with that component set to 0.
func TestNonNumericColor(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	for name, arr := range map[string]pdf.Array{
		"gray":  {pdf.Name("x")},
		"rgb":   {pdf.Integer(1), pdf.Name("x"), pdf.Integer(0)},
		"cmyk":  {pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), nil},
		"dangl": {pdf.Integer(1), pdf.NewReference(99, 0), pdf.Integer(0)},
	} {
		col, err := pdf.Optional(colorenc.Extract(c, arr))
		if err != nil {
			t.Fatal(err)
		}
		if col != nil {
			t.Errorf("%s: got %v, want no colour", name, col)
		}
	}
	rgb := pdf.Array{pdf.Integer(1), pdf.Name("x"), pdf.Integer(0)}
	if col, err := pdf.Optional(colorenc.ExtractRGB(c, rgb)); err != nil || col != nil {
		t.Errorf("ExtractRGB: got %v, %v, want no colour", col, err)
	}

	// the same through an annotation entry
	dict := pdf.Dict{"Subtype": pdf.Name("Square"), "Rect": testRect(), "IC": rgb}
	a, err := Annotation(c, dict, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.(*annotation.Square).FillColor; got != nil {
		t.Errorf("fill colour %v, want none", got)
	}
}

// TestNonNumericGeometry checks that arrays of coordinates with a component
// which is not a number are treated as absent.
func TestNonNumericGeometry(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	bad := pdf.Array{pdf.Integer(1), pdf.Name("x"), pdf.Integer(3), pdf.Integer(4)}

	line, err := Annotation(c, pdf.Dict{"Subtype": pdf.Name("Line"), "Rect": testRect(), "L": bad}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := line.(*annotation.Line); got.Coords != [4]float64{} {
		t.Errorf("L = %v, want all zero", got.Coords)
	}

	ft, err := Annotation(c, pdf.Dict{
		"Subtype": pdf.Name("FreeText"),
		"Rect":    testRect(),
		"IT":      pdf.Name("FreeTextCallout"),
		"CL":      pdf.Array{pdf.Integer(1), pdf.Name("x"), pdf.Integer(3), pdf.Integer(4)},
		"RD":      pdf.Array{pdf.Integer(1), pdf.Name("x"), pdf.Integer(1), pdf.Integer(1)},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := ft.(*annotation.FreeText); got.CalloutLine != nil || got.Margin != nil {
		t.Errorf("CL = %v, RD = %v, want both absent", got.CalloutLine, got.Margin)
	}
}

// TestFieldReadErrors checks that read errors are returned from the entries of
// a terminal field and from the checks which classify a dictionary as a
// field or a widget.
func TestFieldReadErrors(t *testing.T) {
	f := testFailRef
	cases := []struct {
		name string
		dict pdf.Dict
	}{
		{"Ff", pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "Ff": f}},
		{"V", pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "V": f}},
		{"DA", pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "DA": f}},
		{"Q", pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "Q": f}},
		{"MaxLen", pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "MaxLen": f}},
		{"TU", pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "TU": f}},
		{"TM", pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "TM": f}},
		{"DS", pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "DS": f}},
		{"T", pdf.Dict{"FT": pdf.Name("Tx"), "T": f}},
		{"Opt", pdf.Dict{"FT": pdf.Name("Ch"), "T": pdf.String("a"), "Opt": f}},
		{"Opt element", pdf.Dict{"FT": pdf.Name("Ch"), "T": pdf.String("a"), "Opt": pdf.Array{f}}},
		{"Opt pair", pdf.Dict{"FT": pdf.Name("Ch"), "T": pdf.String("a"), "Opt": pdf.Array{pdf.Array{pdf.String("a"), f}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTerminalField(failingCursor(), tc.dict, inherited{})
			if !errors.Is(err, errTestRead) {
				t.Errorf("got %v, want the read error", err)
			}
		})
	}

	c := failingCursor()
	if _, err := isWidgetSubtype(c, pdf.Dict{"Subtype": f}); !errors.Is(err, errTestRead) {
		t.Errorf("isWidgetSubtype: got %v, want the read error", err)
	}
	for _, key := range []pdf.Name{"T", "Kids"} {
		d := pdf.Dict{"Subtype": pdf.Name("Widget"), key: f}
		if _, err := isWidgetKid(c, d); !errors.Is(err, errTestRead) {
			t.Errorf("isWidgetKid %s: got %v, want the read error", key, err)
		}
	}
	for _, key := range []pdf.Name{"Kids", "FT", "T", "Ff", "V", "Opt"} {
		d := pdf.Dict{"Subtype": pdf.Name("Widget"), key: f}
		if _, err := isMergedFieldDict(c, d); !errors.Is(err, errTestRead) {
			t.Errorf("isMergedFieldDict %s: got %v, want the read error", key, err)
		}
	}
}

// TestAssociatedFilesWritable checks that associated files which could not be
// written back are dropped on read, so that the decoded annotation can be
// written out again.
func TestAssociatedFilesWritable(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	stream := func(dict pdf.Dict) pdf.Reference {
		ref := w.Alloc()
		stm, err := w.OpenStream(ref, dict)
		if err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		return ref
	}
	spec := func(dict pdf.Dict) pdf.Reference {
		dict["Type"] = pdf.Name("Filespec")
		ref := w.Alloc()
		if err := w.Put(ref, dict); err != nil {
			t.Fatal(err)
		}
		return ref
	}
	typed := pdf.Dict{"Subtype": pdf.Name("text/plain")}
	ef := func(dict pdf.Dict) pdf.Dict {
		return pdf.Dict{"F": stream(dict)}
	}

	good := spec(pdf.Dict{"F": pdf.String("a.txt")})
	embedded := spec(pdf.Dict{"F": pdf.String("b.txt"), "EF": ef(typed)})
	noName := spec(pdf.Dict{"EF": ef(typed)})
	noMIME := spec(pdf.Dict{"F": pdf.String("c.txt"), "EF": ef(pdf.Dict{})})

	dict := pdf.Dict{
		"Subtype": pdf.Name("Text"),
		"Rect":    testRect(),
		"AF":      pdf.Array{good, embedded, noName, noMIME},
	}
	a, err := Annotation(pdf.NewCursor(w), dict, false)
	if err != nil {
		t.Fatal(err)
	}
	afs := a.GetCommon().AssociatedFiles
	if len(afs) != 3 {
		t.Fatalf("got %d associated files, want 3", len(afs))
	}
	if afs[0].FileName != "a.txt" || afs[1].FileName != "b.txt" || afs[2].FileName != "" {
		t.Errorf("wrong files kept: %q, %q, %q", afs[0].FileName, afs[1].FileName, afs[2].FileName)
	}

	roundTripFile(t, pdf.V2_0, a)
}
