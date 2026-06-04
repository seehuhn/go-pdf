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

package acroform

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/text/language"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/optional"
)

// widgetStyle generates fallback appearances for the test widgets. Appearance
// streams are required on widgets in PDF 2.0.
var widgetStyle = fallback.NewStyle()

// testWidget returns a minimal widget annotation suitable for use as a field
// child, with a generated fallback appearance. Highlight is set to "I" to
// match the value substituted on decode.
func testWidget(llx, lly, urx, ury float64) *annotation.Widget {
	w := &annotation.Widget{
		Common:    annotation.Common{Rect: pdf.Rectangle{LLx: llx, LLy: lly, URx: urx, URy: ury}},
		Highlight: "I",
	}
	ensureWidgetAppearance(w)
	return w
}

// ensureWidgetAppearance gives a widget a generated fallback appearance if it
// lacks one, as PDF 2.0 requires for widgets with a non-empty rectangle. Normal
// is mirrored into RollOver and Down because an absent /R or /D appearance
// defaults to /N on read, keeping the round trip exact.
func ensureWidgetAppearance(w *annotation.Widget) {
	if w.Appearance != nil {
		return
	}
	if w.Rect.LLx == w.Rect.URx && w.Rect.LLy == w.Rect.URy {
		return // single-point widgets are exempt from the requirement
	}
	if err := widgetStyle.AddAppearance(w); err != nil {
		panic(err)
	}
	w.Appearance.RollOver = w.Appearance.Normal
	w.Appearance.Down = w.Appearance.Normal
}

// ensureFieldAppearances supplies fallback appearances for every widget in a
// field tree, as required when writing to PDF 2.0.
func ensureFieldAppearances(f *Field) {
	if f.Merged && f.Widget != nil {
		ensureWidgetAppearance(f.Widget)
	}
	for _, kid := range f.Kids {
		switch k := kid.(type) {
		case *Field:
			ensureFieldAppearances(k)
		case *annotation.Widget:
			ensureWidgetAppearance(k)
		}
	}
}

// withAA attaches an annotation additional-actions dictionary to a widget.
func withAA(w *annotation.Widget, aa *triggers.Annotation) *annotation.Widget {
	w.AA = aa
	return w
}

var fieldTestCases = []struct {
	name  string
	field *Field
}{
	{
		name:  "minimal text",
		field: &Field{FT: "Tx", T: "name"},
	},
	{
		name:  "flags",
		field: &Field{FT: "Tx", T: "locked", Ff: optional.NewUInt(uint(FieldReadOnly | FieldRequired | FieldNoExport))},
	},
	{
		// an explicit Ff of zero must round-trip as present, not absent: it
		// blocks inheritance of an ancestor's flags
		name:  "explicit zero flags",
		field: &Field{FT: "Tx", T: "unlocked", Ff: optional.NewUInt(0)},
	},
	{
		name:  "direct values",
		field: &Field{FT: "Tx", T: "v", V: pdf.String("hello"), DV: pdf.String("world")},
	},
	{
		name:  "reference value",
		field: &Field{FT: "Tx", T: "vr", V: pdf.NewReference(100, 0)},
	},
	{
		name:  "alternate names",
		field: &Field{FT: "Ch", T: "choice", TU: "Choose one", TM: "choice_map"},
	},
	{
		name:  "data passthrough",
		field: &Field{FT: "Tx", T: "d", Data: pdf.Dict{"MaxLen": pdf.Integer(20)}},
	},
	{
		name: "additional actions",
		field: &Field{
			FT: "Tx", T: "calc",
			AA: &triggers.Form{
				Calculate: &action.JavaScript{JS: pdf.String("event.value = 0;")},
			},
		},
	},
	{
		name: "non-terminal tree",
		field: &Field{
			FT: "Tx", T: "address",
			Kids: []Node{
				&Field{T: "street"},
				&Field{T: "zip"},
			},
		},
	},
	{
		name: "merged widget",
		field: &Field{
			FT: "Btn", T: "submit",
			Merged: true,
			Widget: testWidget(0, 0, 72, 24),
		},
	},
	{
		// the merged /AA holds a field-level trigger (C) on the field and an
		// annotation-level trigger (Fo) on the widget; both halves must
		// survive the split/merge round trip
		name: "merged widget with mixed AA",
		field: &Field{
			FT: "Btn", T: "submitAA",
			Merged: true,
			Widget: withAA(testWidget(0, 0, 72, 24),
				&triggers.Annotation{Focus: &action.JavaScript{JS: pdf.String("focus();")}}),
			AA: &triggers.Form{
				Calculate: &action.JavaScript{JS: pdf.String("calc();")},
			},
		},
	},
	{
		// only a field-level trigger; the widget half of the shared /AA is
		// empty and must decode back to a nil widget AA
		name: "merged widget field-only AA",
		field: &Field{
			FT: "Btn", T: "calcOnly",
			Merged: true,
			Widget: testWidget(0, 0, 72, 24),
			AA: &triggers.Form{
				Calculate: &action.JavaScript{JS: pdf.String("calc();")},
			},
		},
	},
	{
		// only an annotation-level trigger; the field half of the shared /AA
		// is empty and must decode back to a nil field AA
		name: "merged widget annotation-only AA",
		field: &Field{
			FT: "Btn", T: "focusOnly",
			Merged: true,
			Widget: withAA(testWidget(0, 0, 72, 24),
				&triggers.Annotation{Focus: &action.JavaScript{JS: pdf.String("focus();")}}),
		},
	},
	{
		name: "multiple widgets",
		field: &Field{
			FT: "Btn", T: "color",
			Kids: []Node{
				testWidget(0, 0, 20, 20),
				testWidget(30, 0, 50, 20),
			},
		},
	},
}

func fieldCmpOptions() []cmp.Option {
	return []cmp.Option{
		cmpopts.IgnoreUnexported(Field{}),
		cmp.AllowUnexported(language.Tag{}),
		cmpopts.EquateComparable(language.Tag{}),
		// form.Equal handles nil-vs-empty and resource differences
		cmp.Comparer(func(a, b *form.Form) bool {
			if a == nil || b == nil {
				return a == b
			}
			return a.Equal(b)
		}),
	}
}

func fieldRoundTripTest(t *testing.T, version pdf.Version, want *Field) {
	t.Helper()

	// PDF 2.0 requires widgets to carry an appearance stream; supply fallbacks
	// for any that lack one (e.g. a fuzz input with the /AP entry stripped)
	if version >= pdf.V2_0 {
		ensureFieldAppearances(want)
	}

	w, buf := memfile.NewPDFWriter(version, nil)
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatalf("add blank page: %v", err)
	}

	rm := pdf.NewResourceManager(w)
	ref, err := want.Encode(rm)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("encode failed: %v", err)
	}
	w.GetMeta().Trailer["Quir:E"] = ref

	if err := rm.Close(); err != nil {
		t.Fatalf("resource manager close failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer close failed: %v", err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatalf("open document: %v", err)
	}
	defer r.Close()

	x := pdf.NewExtractor(r)
	got, err := pdf.ExtractorGet(x, nil, r.GetMeta().Trailer["Quir:E"], DecodeField)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(want, got, fieldCmpOptions()...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestFieldRoundTrip(t *testing.T) {
	for _, version := range testVersions {
		for _, tc := range fieldTestCases {
			t.Run(tc.name+"-"+version.String(), func(t *testing.T) {
				fieldRoundTripTest(t, version, tc.field)
			})
		}
	}
}

func FuzzFieldRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, version := range testVersions {
		for _, tc := range fieldTestCases {
			w, buf := memfile.NewPDFWriter(version, opt)
			if err := memfile.AddBlankPage(w); err != nil {
				continue
			}

			rm := pdf.NewResourceManager(w)
			ref, err := tc.field.Encode(rm)
			if err != nil {
				continue
			}
			w.GetMeta().Trailer["Quir:E"] = ref

			if err := rm.Close(); err != nil {
				continue
			}
			if err := w.Close(); err != nil {
				continue
			}

			f.Add(buf.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		defer r.Close()

		x := pdf.NewExtractor(r)
		field, err := pdf.ExtractorGet(x, nil, r.GetMeta().Trailer["Quir:E"], DecodeField)
		if err != nil {
			t.Skip("malformed field")
		}
		if field == nil {
			t.Skip("no field")
		}

		fieldRoundTripTest(t, pdf.GetVersion(r), field)
	})
}

func TestFieldResolvedAttributes(t *testing.T) {
	parent := &Field{FT: "Tx", Ff: optional.NewUInt(uint(FieldRequired)), V: pdf.String("pv"), DV: pdf.Name("Off")}
	child := &Field{T: "c"}
	child.parent = parent

	if got := child.ResolvedFT(); got != "Tx" {
		t.Errorf("ResolvedFT = %q, want %q", got, "Tx")
	}
	if got := child.ResolvedFf(); got != FieldRequired {
		t.Errorf("ResolvedFf = %d, want %d", got, FieldRequired)
	}
	if got, ok := child.ResolvedV().(pdf.String); !ok || string(got) != "pv" {
		t.Errorf("ResolvedV = %v, want %q", child.ResolvedV(), "pv")
	}
	if got, ok := child.ResolvedDV().(pdf.Name); !ok || got != "Off" {
		t.Errorf("ResolvedDV = %v, want %q", child.ResolvedDV(), "Off")
	}

	// a local value overrides the inherited one
	child.FT = "Btn"
	if got := child.ResolvedFT(); got != "Btn" {
		t.Errorf("ResolvedFT after override = %q, want %q", got, "Btn")
	}

	// an explicit zero blocks inheritance of the ancestor's flags
	child.Ff = optional.NewUInt(0)
	if got := child.ResolvedFf(); got != 0 {
		t.Errorf("ResolvedFf with explicit zero = %d, want 0", got)
	}
}

// an explicit Ff of zero on a child must survive a write/read cycle so that it
// keeps blocking inheritance of the parent's flags. Before Ff tracked its
// presence, the zero value was indistinguishable from absent and was dropped
// on encode, causing the child to inherit FieldRequired after the round trip.
func TestFieldExplicitZeroFlagsRoundTrip(t *testing.T) {
	want := &Field{
		FT: "Tx", T: "parent", Ff: optional.NewUInt(uint(FieldRequired)),
		Kids: []Node{
			&Field{T: "child", Ff: optional.NewUInt(0)},
		},
	}

	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatal(err)
	}
	rm := pdf.NewResourceManager(w)
	ref, err := want.Encode(rm)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	w.GetMeta().Trailer["Quir:E"] = ref
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	x := pdf.NewExtractor(r)
	got, err := pdf.ExtractorGet(x, nil, r.GetMeta().Trailer["Quir:E"], DecodeField)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(got.Kids) != 1 {
		t.Fatalf("expected one kid, got %d", len(got.Kids))
	}
	child, ok := got.Kids[0].(*Field)
	if !ok {
		t.Fatalf("expected a *Field kid, got %T", got.Kids[0])
	}
	if _, set := child.Ff.Get(); !set {
		t.Error("child Ff lost its explicit-present status after round trip")
	}
	if rff := child.ResolvedFf(); rff != 0 {
		t.Errorf("child ResolvedFf = %d, want 0 (explicit zero blocks inheritance)", rff)
	}
}

func TestFieldFullyQualifiedName(t *testing.T) {
	root := &Field{T: "PersonalData"}
	mid := &Field{T: "Address"}
	mid.parent = root
	leaf := &Field{T: "ZipCode"}
	leaf.parent = mid

	if got := leaf.FullyQualifiedName(); got != "PersonalData.Address.ZipCode" {
		t.Errorf("FullyQualifiedName = %q, want %q", got, "PersonalData.Address.ZipCode")
	}

	// an ancestor without a partial name is skipped
	anon := &Field{}
	anon.parent = root
	leaf2 := &Field{T: "Phone"}
	leaf2.parent = anon
	if got := leaf2.FullyQualifiedName(); got != "PersonalData.Phone" {
		t.Errorf("FullyQualifiedName with anonymous ancestor = %q, want %q", got, "PersonalData.Phone")
	}
}

func TestDecodeFieldNil(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	field, err := DecodeField(x, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field != nil {
		t.Errorf("expected nil field for nil object, got %v", field)
	}
}

func TestDecodeFieldKidsSelfCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	dict := pdf.Dict{
		"FT":   pdf.Name("Tx"),
		"T":    pdf.String("loop"),
		"Kids": pdf.Array{ref}, // refers to itself
	}
	if err := w.Put(ref, dict); err != nil {
		t.Fatal(err)
	}

	field, err := pdf.ExtractorGet(x, nil, ref, DecodeField)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field == nil {
		t.Fatal("expected a field")
	}
	if len(field.Kids) != 0 {
		t.Errorf("expected no kids (self-reference dropped), got %d", len(field.Kids))
	}
}

func TestDecodeFieldKidsMutualCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	refA := w.Alloc()
	refB := w.Alloc()
	if err := w.Put(refA, pdf.Dict{"T": pdf.String("a"), "Kids": pdf.Array{refB}}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(refB, pdf.Dict{"T": pdf.String("b"), "Kids": pdf.Array{refA}}); err != nil {
		t.Fatal(err)
	}

	field, err := pdf.ExtractorGet(x, nil, refA, DecodeField)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field == nil {
		t.Fatal("expected a field")
	}
	// A has one child B; B's back-reference to A is broken by cycle detection.
	if len(field.Kids) != 1 {
		t.Fatalf("expected one kid, got %d", len(field.Kids))
	}
	b, ok := field.Kids[0].(*Field)
	if !ok {
		t.Fatalf("expected a sub-field kid, got %T", field.Kids[0])
	}
	if len(b.Kids) != 0 {
		t.Errorf("expected B to have no kids after cycle break, got %d", len(b.Kids))
	}
}

func TestEncodeFieldInvalidType(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	field := &Field{FT: "Bogus", T: "x"}
	if _, err := field.Encode(rm); err == nil {
		t.Error("expected error for invalid field type, got nil")
	}
}

func TestEncodeFieldDataCollision(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	field := &Field{FT: "Tx", T: "x", Data: pdf.Dict{"V": pdf.String("clash")}}
	if _, err := field.Encode(rm); err == nil {
		t.Error("expected error for Data colliding with a modeled key, got nil")
	}
}

func TestEncodeFieldNameWithPeriod(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	field := &Field{FT: "Tx", T: "a.b"}
	if _, err := field.Encode(rm); err == nil {
		t.Error("expected error for partial name containing a period, got nil")
	}
}

// a partial name read from a file must not contain a period, since it is the
// fully-qualified-name separator. The reader strips any so the field stays
// writable, keeping the read-write-read cycle intact.
func TestDecodeFieldNameStripsPeriod(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatal(err)
	}

	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.TextString("a.b.c")}); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Trailer["Quir:E"] = ref
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	x := pdf.NewExtractor(r)
	field, err := pdf.ExtractorGet(x, nil, ref, DecodeField)
	if err != nil {
		t.Fatal(err)
	}
	if field.T != "abc" {
		t.Errorf("partial name = %q, want %q", field.T, "abc")
	}

	// the snapped name must re-encode without error
	rm := pdf.NewResourceManager(w)
	if _, err := field.Encode(rm); err != nil {
		t.Errorf("re-encode of snapped name failed: %v", err)
	}
}

func TestEncodeFieldVersionGating(t *testing.T) {
	tests := []struct {
		name    string
		version pdf.Version
		field   *Field
	}{
		{"field requires 1.2", pdf.V1_1, &Field{FT: "Tx", T: "x"}},
		{"TU requires 1.3", pdf.V1_2, &Field{FT: "Tx", T: "x", TU: "label"}},
		{"TM requires 1.3", pdf.V1_2, &Field{FT: "Tx", T: "x", TM: "map"}},
		{"AA requires 1.3", pdf.V1_2, &Field{
			FT: "Tx", T: "x",
			AA: &triggers.Form{Calculate: &action.JavaScript{JS: pdf.String("0;")}},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := tc.field.Encode(rm); !pdf.IsWrongVersion(err) {
				t.Errorf("expected version error, got %v", err)
			}
		})
	}
}

func TestIsWidgetKid(t *testing.T) {
	cases := []struct {
		name string
		dict pdf.Dict
		want bool
	}{
		{"plain widget", pdf.Dict{"Subtype": pdf.Name("Widget")}, true},
		{"merged terminal field", pdf.Dict{"Subtype": pdf.Name("Widget"), "FT": pdf.Name("Tx")}, false},
		{"named is a field", pdf.Dict{"Subtype": pdf.Name("Widget"), "T": pdf.String("x")}, false},
		{"has kids is a field", pdf.Dict{"Subtype": pdf.Name("Widget"), "Kids": pdf.Array{}}, false},
		{"no subtype is a field", pdf.Dict{"V": pdf.String("x")}, false},
		{"other subtype is a field", pdf.Dict{"Subtype": pdf.Name("Link")}, false},
	}
	for _, tc := range cases {
		if got := isWidgetKid(tc.dict); got != tc.want {
			t.Errorf("%s: isWidgetKid = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// a kid that is a sub-field without FT, T, or Kids (and without a Widget
// subtype) must be decoded as a field, not misclassified as a widget and
// silently dropped.
func TestDecodeFieldAnonymousSubField(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatal(err)
	}

	parent := w.Alloc()
	kid := w.Alloc()
	if err := w.Put(kid, pdf.Dict{"V": pdf.String("x"), "Ff": pdf.Integer(2)}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(parent, pdf.Dict{
		"FT":   pdf.Name("Tx"),
		"T":    pdf.String("p"),
		"Kids": pdf.Array{kid},
	}); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Trailer["Quir:E"] = parent
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	x := pdf.NewExtractor(r)
	field, err := pdf.ExtractorGet(x, nil, parent, DecodeField)
	if err != nil {
		t.Fatal(err)
	}
	if len(field.Kids) != 1 {
		t.Fatalf("expected the anonymous sub-field to be preserved, got %d kids", len(field.Kids))
	}
	child, ok := field.Kids[0].(*Field)
	if !ok {
		t.Fatalf("expected a *Field kid, got %T", field.Kids[0])
	}
	if got, ok := child.V.(pdf.String); !ok || string(got) != "x" {
		t.Errorf("sub-field value = %v, want %q", child.V, "x")
	}
	if child.ResolvedFT() != "Tx" {
		t.Errorf("sub-field ResolvedFT = %q, want Tx (inherited)", child.ResolvedFT())
	}
}
