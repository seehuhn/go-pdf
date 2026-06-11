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
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/text/language"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/formhooks"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/optional"
)

// testWidget returns a minimal widget annotation suitable for use as a field
// child, with an appearance. Highlight is set to "I" to match the value
// substituted on decode.
func testWidget(llx, lly, urx, ury float64) *annotation.Widget {
	w := &annotation.Widget{
		Common:    annotation.Common{Rect: pdf.Rectangle{LLx: llx, LLy: lly, URx: urx, URy: ury}},
		Highlight: "I",
	}
	ensureWidgetAppearance(w)
	return w
}

// ensureWidgetAppearance gives a widget an appearance if it lacks one, as PDF
// 2.0 requires for widgets with a non-empty rectangle. The shared
// defaultAppearanceDict sets Normal, RollOver and Down to the same stream,
// keeping the round trip exact (an absent /R or /D defaults to /N on read).
func ensureWidgetAppearance(w *annotation.Widget) {
	if w.Appearance != nil {
		return
	}
	if w.Rect.LLx == w.Rect.URx && w.Rect.LLy == w.Rect.URy {
		return // single-point widgets are exempt from the requirement
	}
	w.Appearance = defaultAppearanceDict
}

// ensureFieldAppearances supplies fallback appearances for every widget in a
// field tree, as required when writing to PDF 2.0.
func ensureFieldAppearances(f acroform.Field) {
	c := f.GetFieldCommon()
	for _, kid := range c.Kids {
		switch k := kid.(type) {
		case acroform.Field:
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
	field acroform.Field
}{
	{
		name:  "minimal text",
		field: &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "name"}},
	},
	{
		name:  "flags",
		field: &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "locked", Ff: optional.New(acroform.FieldReadOnly | acroform.FieldRequired | acroform.FieldNoExport)}},
	},
	{
		// an explicit Ff of zero must round-trip as present, not absent: it
		// blocks inheritance of an ancestor's flags
		name:  "explicit zero flags",
		field: &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "unlocked", Ff: optional.New(acroform.FieldFlags(0))}},
	},
	{
		name:  "direct values",
		field: &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "v"}, V: pdf.String("hello"), DV: pdf.String("world")},
	},
	{
		name:  "reference value",
		field: &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "vr"}, V: pdf.NewReference(100, 0)},
	},
	{
		name:  "text variable text",
		field: &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "vt"}, VariableText: acroform.VariableText{DefaultAppearance: "/Helv 12 Tf 0 g", Align: pdf.TextAlignCenter}, MaxLen: 24},
	},
	{
		name:  "comb",
		field: &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "comb", Ff: optional.New(acroform.FieldComb)}, MaxLen: 6},
	},
	{
		name:  "alternate names",
		field: &acroform.FieldChoice{FieldCommon: acroform.FieldCommon{T: "choice", TU: "Choose one", TM: "choice_map"}},
	},
	{
		name: "choice options",
		field: &acroform.FieldChoice{
			FieldCommon: acroform.FieldCommon{T: "fonts", Ff: optional.New(acroform.FieldCombo)},
			Opt:         []acroform.ChoiceOption{{Export: "h", Display: "Helvetica"}, {Export: "Times", Display: "Times"}},
			TopIndex:    0,
			Selected:    []int{1},
			V:           pdf.String("Times"),
		},
	},
	{
		name: "checkbox",
		field: &acroform.FieldBtn{
			FieldCommon: acroform.FieldCommon{T: "agree"},
			V:           "Yes",
			DV:          "Off",
		},
	},
	{
		name: "radio with export values",
		field: &acroform.FieldBtn{
			FieldCommon: acroform.FieldCommon{T: "size", Ff: optional.New(acroform.FieldRadio)},
			Opt:         []string{"small", "large"},
			V:           "small",
		},
	},
	{
		name:  "push button",
		field: &acroform.FieldBtn{FieldCommon: acroform.FieldCommon{T: "submit", Ff: optional.New(acroform.FieldPushbutton)}},
	},
	{
		name:  "signature",
		field: &acroform.FieldSig{FieldCommon: acroform.FieldCommon{T: "sig"}},
	},
	{
		name: "signature with lock all",
		field: &acroform.FieldSig{
			FieldCommon: acroform.FieldCommon{T: "siglockall"},
			Lock:        &acroform.SigFieldLock{Action: acroform.SigFieldLockAll},
		},
	},
	{
		name: "signature with lock include",
		field: &acroform.FieldSig{
			FieldCommon: acroform.FieldCommon{T: "siglock"},
			Lock: &acroform.SigFieldLock{
				Action: acroform.SigFieldLockInclude,
				Fields: []string{"name", "address"},
			},
		},
	},
	{
		name: "signature with seed value",
		field: &acroform.FieldSig{
			FieldCommon: acroform.FieldCommon{T: "sigsv"},
			SV: &acroform.SigSeedValue{
				Flags:            acroform.SigSeedFilter | acroform.SigSeedSubFilter | acroform.SigSeedReasons,
				Filter:           "Adobe.PPKLite",
				SubFilter:        []pdf.Name{"adbe.pkcs7.detached", "ETSI.CAdES.detached"},
				DigestMethod:     []pdf.Name{"SHA256", "SHA512"},
				V:                2,
				Reasons:          []string{"I agree", "I approve"},
				MDP:              optional.NewUInt(2),
				TimeStamp:        &acroform.SigSeedValueTimeStamp{URL: "https://ts.example.com", Required: true},
				LegalAttestation: []string{"attestation"},
				AddRevInfo:       true,
			},
		},
	},
	{
		name: "signature with cert seed value",
		field: &acroform.FieldSig{
			FieldCommon: acroform.FieldCommon{T: "sigcert"},
			SV: &acroform.SigSeedValue{
				Filter: "Adobe.PPKLite",
				Cert: &acroform.SigCertSeedValue{
					Flags:     acroform.SigCertSubject | acroform.SigCertKeyUsage,
					Subject:   [][]byte{{0x30, 0x82, 0x01}, {0x00, 0xff}},
					Issuer:    [][]byte{{0x30, 0x10}},
					OID:       [][]byte{[]byte("2.16.840.1.113733.1.7.1.1")},
					SubjectDN: []map[pdf.Name]string{{"cn": "Example", "o": "Example Org"}},
					KeyUsage:  []string{"1XXXXXXXX"},
					URL:       "https://ca.example.com",
					URLType:   "Browser",
				},
			},
		},
	},
	{
		name: "signature with pdf 2.0 seed value",
		field: &acroform.FieldSig{
			FieldCommon: acroform.FieldCommon{T: "sigsv20"},
			Lock: &acroform.SigFieldLock{
				Action: acroform.SigFieldLockExclude,
				Fields: []string{"sig"},
				P:      2,
			},
			SV: &acroform.SigSeedValue{
				Flags:            acroform.SigSeedLockDocument | acroform.SigSeedAppearanceFilter,
				LockDocument:     "true",
				AppearanceFilter: "MyAppearance",
				Cert: &acroform.SigCertSeedValue{
					SignaturePolicyOID:            "1.2.3.4",
					SignaturePolicyHashValue:      []byte{0xde, 0xad, 0xbe, 0xef},
					SignaturePolicyHashAlgorithm:  "SHA256",
					SignaturePolicyCommitmentType: []string{"1.2.840.113549.1.9.16.6.1"},
				},
			},
		},
	},
	{
		name: "additional actions",
		field: &acroform.FieldTx{
			FieldCommon: acroform.FieldCommon{
				T: "calc",
				AA: &triggers.Form{
					Calculate: &action.JavaScript{JS: pdf.String("event.value = 0;")},
				},
			},
		},
	},
	{
		name: "non-terminal tree",
		field: &acroform.FieldTx{
			FieldCommon: acroform.FieldCommon{
				T: "address",
				Kids: []acroform.Node{
					&acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "street"}},
					&acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "zip"}},
				},
			},
		},
	},
	{
		name: "merged widget",
		field: &acroform.FieldBtn{
			FieldCommon: acroform.FieldCommon{
				T:    "submit",
				Kids: []acroform.Node{testWidget(0, 0, 72, 24)},
			},
		},
	},
	{
		// the merged /AA holds a field-level trigger (C) on the field and an
		// annotation-level trigger (Fo) on the widget; both halves must
		// survive the split/merge round trip
		name: "merged widget with mixed AA",
		field: &acroform.FieldBtn{
			FieldCommon: acroform.FieldCommon{
				T: "submitAA",
				Kids: []acroform.Node{withAA(testWidget(0, 0, 72, 24),
					&triggers.Annotation{Focus: &action.JavaScript{JS: pdf.String("focus();")}})},
				AA: &triggers.Form{
					Calculate: &action.JavaScript{JS: pdf.String("calc();")},
				},
			},
		},
	},
	{
		// only a field-level trigger; the widget half of the shared /AA is
		// empty and must decode back to a nil widget AA
		name: "merged widget field-only AA",
		field: &acroform.FieldBtn{
			FieldCommon: acroform.FieldCommon{
				T:    "calcOnly",
				Kids: []acroform.Node{testWidget(0, 0, 72, 24)},
				AA: &triggers.Form{
					Calculate: &action.JavaScript{JS: pdf.String("calc();")},
				},
			},
		},
	},
	{
		// only an annotation-level trigger; the field half of the shared /AA
		// is empty and must decode back to a nil field AA
		name: "merged widget annotation-only AA",
		field: &acroform.FieldBtn{
			FieldCommon: acroform.FieldCommon{
				T: "focusOnly",
				Kids: []acroform.Node{withAA(testWidget(0, 0, 72, 24),
					&triggers.Annotation{Focus: &action.JavaScript{JS: pdf.String("focus();")}})},
			},
		},
	},
	{
		name: "multiple widgets",
		field: &acroform.FieldBtn{
			FieldCommon: acroform.FieldCommon{
				T:    "color",
				Ff:   optional.New(acroform.FieldRadio),
				Kids: []acroform.Node{testWidget(0, 0, 20, 20), testWidget(30, 0, 50, 20)},
			},
			Opt: []string{"red", "green"},
		},
	},
}

func fieldCmpOptions() []cmp.Option {
	return []cmp.Option{
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

// storeFieldTree writes a single field's subtree through the public interactive
// form encode path (which names each root field the same way the form would),
// then writes its widget annotations as their pages would, so a field can be
// round-tripped on its own. It returns the reference naming f.
// linkTree wires the parent links of a hand-assembled test fixture,
// recursively, standing in for the wiring the builder functions do.
func linkTree(f acroform.Field) {
	for _, kid := range f.GetFieldCommon().Kids {
		switch k := kid.(type) {
		case acroform.Field:
			k.GetFieldCommon().Parent = f
			linkTree(k)
		case *annotation.Widget:
			k.Parent = f
		}
	}
}

func storeFieldTree(rm *pdf.ResourceManager, f acroform.Field) (pdf.Reference, error) {
	linkTree(f)
	form := &acroform.InteractiveForm{Fields: []acroform.Field{f}}
	obj, err := form.Encode(rm)
	if err != nil {
		return 0, err
	}
	dict, ok := obj.(pdf.Dict)
	if !ok {
		return 0, fmt.Errorf("form did not encode to a dictionary")
	}
	fields, ok := dict["Fields"].(pdf.Array)
	if !ok || len(fields) != 1 {
		return 0, fmt.Errorf("unexpected form Fields entry")
	}
	ref, ok := fields[0].(pdf.Reference)
	if !ok {
		return 0, fmt.Errorf("field reference is not indirect")
	}
	if err := storeFieldWidgets(rm, f); err != nil {
		return 0, err
	}
	return ref, nil
}

// storeFieldWidgets stores every widget annotation in f's subtree, standing in
// for the pages that would normally write them.
func storeFieldWidgets(rm *pdf.ResourceManager, f acroform.Field) error {
	for _, kid := range f.GetFieldCommon().Kids {
		switch k := kid.(type) {
		case *annotation.Widget:
			if _, err := rm.Store(k); err != nil {
				return err
			}
		case acroform.Field:
			if err := storeFieldWidgets(rm, k); err != nil {
				return err
			}
		}
	}
	return nil
}

func fieldRoundTripTest(t *testing.T, version pdf.Version, want acroform.Field) {
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
	ref, err := storeFieldTree(rm, want)
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
	got, err := pdf.ExtractorGet(x, nil, r.GetMeta().Trailer["Quir:E"], field)
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
			ref, err := storeFieldTree(rm, tc.field)
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
		field, err := pdf.ExtractorGet(x, nil, r.GetMeta().Trailer["Quir:E"], field)
		if err != nil {
			t.Skip("malformed field")
		}
		if field == nil {
			t.Skip("no field")
		}

		fieldRoundTripTest(t, pdf.GetVersion(r), field)
	})
}

// an explicit Ff of zero on a child must survive a write/read cycle so that it
// keeps blocking inheritance of the parent's flags. Before Ff tracked its
// presence, the zero value was indistinguishable from absent and was dropped
// on encode, causing the child to inherit FieldRequired after the round trip.
func TestFieldExplicitZeroFlagsRoundTrip(t *testing.T) {
	want := &acroform.FieldTx{
		FieldCommon: acroform.FieldCommon{
			T:  "parent",
			Ff: optional.New(acroform.FieldRequired),
			Kids: []acroform.Node{
				&acroform.FieldCommon{T: "child", Ff: optional.New(acroform.FieldFlags(0))},
			},
		},
	}

	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatal(err)
	}
	rm := pdf.NewResourceManager(w)
	ref, err := storeFieldTree(rm, want)
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
	got, err := pdf.ExtractorGet(x, nil, r.GetMeta().Trailer["Quir:E"], field)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	kids := got.GetFieldCommon().Kids
	if len(kids) != 1 {
		t.Fatalf("expected one kid, got %d", len(kids))
	}
	child, ok := kids[0].(acroform.Field)
	if !ok {
		t.Fatalf("expected a field kid, got %T", kids[0])
	}
	if _, set := child.GetFieldCommon().Ff.Get(); !set {
		t.Error("child Ff lost its explicit-present status after round trip")
	}
	if rff := acroform.ResolvedFf(child); rff != 0 {
		t.Errorf("child ResolvedFf = %d, want 0 (explicit zero blocks inheritance)", rff)
	}
}

func TestDecodeFieldNil(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	field, err := field(x, nil, nil, false)
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

	field, err := pdf.ExtractorGet(x, nil, ref, field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field == nil {
		t.Fatal("expected a field")
	}
	if kids := field.GetFieldCommon().Kids; len(kids) != 0 {
		t.Errorf("expected no kids (self-reference dropped), got %d", len(kids))
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

	field, err := pdf.ExtractorGet(x, nil, refA, field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field == nil {
		t.Fatal("expected a field")
	}
	// A has one child B; B's back-reference to A is broken by cycle detection.
	kids := field.GetFieldCommon().Kids
	if len(kids) != 1 {
		t.Fatalf("expected one kid, got %d", len(kids))
	}
	b, ok := kids[0].(*acroform.FieldCommon)
	if !ok {
		t.Fatalf("expected a sub-field kid, got %T", kids[0])
	}
	if len(b.Kids) != 0 {
		t.Errorf("expected B to have no kids after cycle break, got %d", len(b.Kids))
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
	field, err := pdf.ExtractorGet(x, nil, ref, field)
	if err != nil {
		t.Fatal(err)
	}
	if name := field.GetFieldCommon().T; name != "abc" {
		t.Errorf("partial name = %q, want %q", name, "abc")
	}

	// the snapped name must re-encode without error
	rm := pdf.NewResourceManager(w)
	if _, err := formhooks.FieldEntries(rm, field); err != nil {
		t.Errorf("re-encode of snapped name failed: %v", err)
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
// silently dropped. Its common attributes (here Ff) survive, and it inherits
// its type from the parent.
func TestDecodeFieldAnonymousSubField(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatal(err)
	}

	parent := w.Alloc()
	kid := w.Alloc()
	if err := w.Put(kid, pdf.Dict{"Ff": pdf.Integer(2)}); err != nil {
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
	field, err := pdf.ExtractorGet(x, nil, parent, field)
	if err != nil {
		t.Fatal(err)
	}
	kids := field.GetFieldCommon().Kids
	if len(kids) != 1 {
		t.Fatalf("expected the anonymous sub-field to be preserved, got %d kids", len(kids))
	}
	child, ok := kids[0].(*acroform.FieldCommon)
	if !ok {
		t.Fatalf("expected a *acroform.FieldCommon kid, got %T", kids[0])
	}
	if got, set := child.Ff.Get(); !set || got != acroform.FieldRequired {
		t.Errorf("sub-field Ff = %v (set %v), want %d", got, set, acroform.FieldRequired)
	}
	if acroform.ResolvedFT(child) != "Tx" {
		t.Errorf("sub-field ResolvedFT = %q, want Tx (inherited)", acroform.ResolvedFT(child))
	}
}

// TestDecodeFieldKidsDeepChainBounded guards against a stack-overflow DoS: a
// /Kids chain of distinct fields is acyclic, so the cycle guard never trips,
// yet recursing one frame per level would exhaust the Go stack. The
// ExtractorGet depth cap makes decodeKids (which decodes each kid via
// pdf.Optional) silently truncate the over-deep tail, keeping the fields
// above the cap.
func TestDecodeFieldKidsDeepChainBounded(t *testing.T) {
	depth := limits.MaxExtractDepth + 10
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	refs := make([]pdf.Reference, depth)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i, ref := range refs {
		d := pdf.Dict{"T": pdf.String(fmt.Sprintf("f%d", i))}
		if i+1 < depth {
			d["Kids"] = pdf.Array{refs[i+1]}
		}
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}

	x := pdf.NewExtractor(w)
	field, err := pdf.ExtractorGet(x, nil, refs[0], field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if field == nil {
		t.Fatal("expected a field")
	}

	// the chain must be truncated at the cap, not read in full
	levels := 1
	fc := field.GetFieldCommon()
	for len(fc.Kids) > 0 {
		next, ok := fc.Kids[0].(*acroform.FieldCommon)
		if !ok {
			break
		}
		levels++
		fc = next
	}
	if levels > limits.MaxExtractDepth {
		t.Errorf("chain depth = %d, want at most %d", levels, limits.MaxExtractDepth)
	}
}

// TestDecodeFieldKidsWide verifies that the depth cap bounds nesting depth,
// not sibling breadth: a single field with many kids is read in full.
func TestDecodeFieldKidsWide(t *testing.T) {
	n := 2*limits.MaxExtractDepth + 50
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	kids := make(pdf.Array, n)
	for i := range kids {
		ref := w.Alloc()
		if err := w.Put(ref, pdf.Dict{"T": pdf.String(fmt.Sprintf("k%d", i))}); err != nil {
			t.Fatal(err)
		}
		kids[i] = ref
	}
	rootRef := w.Alloc()
	if err := w.Put(rootRef, pdf.Dict{"T": pdf.String("root"), "Kids": kids}); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	field, err := pdf.ExtractorGet(x, nil, rootRef, field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(field.GetFieldCommon().Kids); got != n {
		t.Errorf("kids = %d, want %d", got, n)
	}
}

// a choice field's /Opt may mix string entries with [export, display] pairs;
// entries that are neither (e.g. a number, or a pair with a non-string member)
// are skipped rather than decoded as empty options.
func TestDecodeChoiceOptSkipsNonStrings(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	dict := pdf.Dict{
		"FT": pdf.Name("Ch"),
		"T":  pdf.String("choice"),
		"Opt": pdf.Array{
			pdf.String("plain"), // string option
			pdf.Array{pdf.String("exp"), pdf.String("disp")}, // [export, display]
			pdf.Integer(42), // not a string: skipped
			pdf.Array{pdf.Integer(1), pdf.String("x")}, // bad pair: skipped
			pdf.String(""), // empty string: kept
		},
	}
	if err := w.Put(ref, dict); err != nil {
		t.Fatal(err)
	}

	field, err := pdf.ExtractorGet(x, nil, ref, field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ch, ok := field.(*acroform.FieldChoice)
	if !ok {
		t.Fatalf("expected *acroform.FieldChoice, got %T", field)
	}
	want := []acroform.ChoiceOption{
		{Export: "plain", Display: "plain"},
		{Export: "exp", Display: "disp"},
		{Export: "", Display: ""},
	}
	if diff := cmp.Diff(want, ch.Opt); diff != "" {
		t.Errorf("Opt mismatch (-want +got):\n%s", diff)
	}
}

// the Comb flag may be set only with a MaxLen and with the Multiline, Password
// and FileSelect flags clear; decoding clears an invalid Comb flag so that the
// field can always be written back.
func TestDecodeCombSnap(t *testing.T) {
	tests := []struct {
		name       string
		dict       pdf.Dict
		wantComb   bool
		wantMaxLen int
	}{
		{
			name: "valid comb",
			dict: pdf.Dict{
				"FT":     pdf.Name("Tx"),
				"T":      pdf.String("x"),
				"Ff":     pdf.Integer(acroform.FieldComb),
				"MaxLen": pdf.Integer(6),
			},
			wantComb:   true,
			wantMaxLen: 6,
		},
		{
			name: "zero MaxLen",
			dict: pdf.Dict{
				"FT":     pdf.Name("Tx"),
				"T":      pdf.String("x"),
				"Ff":     pdf.Integer(acroform.FieldComb),
				"MaxLen": pdf.Integer(0),
			},
			wantComb: false,
		},
		{
			name: "missing MaxLen",
			dict: pdf.Dict{
				"FT": pdf.Name("Tx"),
				"T":  pdf.String("x"),
				"Ff": pdf.Integer(acroform.FieldComb),
			},
			wantComb: false,
		},
		{
			name: "conflicting Multiline",
			dict: pdf.Dict{
				"FT":     pdf.Name("Tx"),
				"T":      pdf.String("x"),
				"Ff":     pdf.Integer(acroform.FieldComb | acroform.FieldMultiline),
				"MaxLen": pdf.Integer(6),
			},
			wantComb:   false,
			wantMaxLen: 6,
		},
		{
			name: "conflicting Password",
			dict: pdf.Dict{
				"FT":     pdf.Name("Tx"),
				"T":      pdf.String("x"),
				"Ff":     pdf.Integer(acroform.FieldComb | acroform.FieldPassword),
				"MaxLen": pdf.Integer(6),
			},
			wantComb:   false,
			wantMaxLen: 6,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			x := pdf.NewExtractor(w)
			ref := w.Alloc()
			if err := w.Put(ref, tc.dict); err != nil {
				t.Fatal(err)
			}

			field, err := pdf.ExtractorGet(x, nil, ref, field)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			tx, ok := field.(*acroform.FieldTx)
			if !ok {
				t.Fatalf("expected *acroform.FieldTx, got %T", field)
			}

			if got := acroform.ResolvedFf(tx)&acroform.FieldComb != 0; got != tc.wantComb {
				t.Errorf("Comb flag = %t, want %t", got, tc.wantComb)
			}
			if tx.MaxLen != tc.wantMaxLen {
				t.Errorf("MaxLen = %d, want %d", tx.MaxLen, tc.wantMaxLen)
			}
			if origFf, _ := pdf.GetInteger(w, tc.dict["Ff"]); acroform.ResolvedFf(tx)&^acroform.FieldComb != acroform.FieldFlags(origFf)&^acroform.FieldComb {
				t.Error("flags other than Comb were changed")
			}

			// every decoded field must be writable
			rm := pdf.NewResourceManager(w)
			if _, err := formhooks.FieldEntries(rm, tx); err != nil {
				t.Errorf("decoded field cannot be written back: %v", err)
			}
		})
	}
}

// MaxLen is inheritable: a comb field whose MaxLen sits on an ancestor adopts
// the inherited value on decode, keeping the field self-contained.
func TestDecodeCombInheritedMaxLen(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	parentRef := w.Alloc()
	kidRef := w.Alloc()
	err := w.Put(parentRef, pdf.Dict{
		"FT":     pdf.Name("Tx"),
		"T":      pdf.String("p"),
		"MaxLen": pdf.Integer(8),
		"Kids":   pdf.Array{kidRef},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(kidRef, pdf.Dict{
		"T":      pdf.String("c"),
		"Ff":     pdf.Integer(acroform.FieldComb),
		"Parent": parentRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	field, err := pdf.ExtractorGet(x, nil, parentRef, field)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	kids := field.GetFieldCommon().Kids
	if len(kids) != 1 {
		t.Fatalf("expected one kid, got %d", len(kids))
	}
	kid, ok := kids[0].(*acroform.FieldTx)
	if !ok {
		t.Fatalf("expected a *acroform.FieldTx kid, got %T", kids[0])
	}
	if acroform.ResolvedFf(kid)&acroform.FieldComb == 0 {
		t.Error("Comb flag was cleared despite inherited MaxLen")
	}
	if kid.MaxLen != 8 {
		t.Errorf("kid MaxLen = %d, want 8 (inherited)", kid.MaxLen)
	}
}

// the field type is inheritable: a sub-field without its own /FT decodes as
// the inherited concrete type, so its type-specific entries are preserved.
func TestDecodeFieldInheritedType(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	parentRef := w.Alloc()
	kidRef := w.Alloc()
	err := w.Put(parentRef, pdf.Dict{
		"FT":   pdf.Name("Tx"),
		"T":    pdf.String("p"),
		"Kids": pdf.Array{kidRef},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(kidRef, pdf.Dict{
		"T":      pdf.String("c"),
		"V":      pdf.String("hello"),
		"Parent": parentRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	field, err := pdf.ExtractorGet(x, nil, parentRef, field)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	kids := field.GetFieldCommon().Kids
	if len(kids) != 1 {
		t.Fatalf("expected one kid, got %d", len(kids))
	}
	kid, ok := kids[0].(*acroform.FieldTx)
	if !ok {
		t.Fatalf("expected a *acroform.FieldTx kid, got %T", kids[0])
	}
	if v, ok := kid.V.(pdf.String); !ok || string(v) != "hello" {
		t.Errorf("kid V = %v, want \"hello\"", kid.V)
	}
}
