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

package annotation

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/text/language"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/optional"
)

// testWidget returns a minimal widget annotation suitable for use as a field
// child, with an appearance. Highlight is set to "I" to match the value
// substituted on decode.
func testWidget(llx, lly, urx, ury float64) *Widget {
	w := &Widget{
		Common:    Common{Rect: pdf.Rectangle{LLx: llx, LLy: lly, URx: urx, URy: ury}},
		Highlight: "I",
	}
	ensureWidgetAppearance(w)
	return w
}

// ensureWidgetAppearance gives a widget an appearance if it lacks one, as PDF
// 2.0 requires for widgets with a non-empty rectangle. The shared
// defaultAppearanceDict sets Normal, RollOver and Down to the same stream,
// keeping the round trip exact (an absent /R or /D defaults to /N on read).
func ensureWidgetAppearance(w *Widget) {
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
func ensureFieldAppearances(f Field) {
	c := f.GetFieldCommon()
	for _, kid := range c.Kids {
		switch k := kid.(type) {
		case Field:
			ensureFieldAppearances(k)
		case *Widget:
			ensureWidgetAppearance(k)
		}
	}
}

// withAA attaches an annotation additional-actions dictionary to a widget.
func withAA(w *Widget, aa *triggers.Annotation) *Widget {
	w.AA = aa
	return w
}

var fieldTestCases = []struct {
	name  string
	field Field
}{
	{
		name:  "minimal text",
		field: &FieldTx{FieldCommon: FieldCommon{T: "name"}},
	},
	{
		name:  "flags",
		field: &FieldTx{FieldCommon: FieldCommon{T: "locked", Ff: optional.NewUInt(uint(FieldReadOnly | FieldRequired | FieldNoExport))}},
	},
	{
		// an explicit Ff of zero must round-trip as present, not absent: it
		// blocks inheritance of an ancestor's flags
		name:  "explicit zero flags",
		field: &FieldTx{FieldCommon: FieldCommon{T: "unlocked", Ff: optional.NewUInt(0)}},
	},
	{
		name:  "direct values",
		field: &FieldTx{FieldCommon: FieldCommon{T: "v"}, V: pdf.String("hello"), DV: pdf.String("world")},
	},
	{
		name:  "reference value",
		field: &FieldTx{FieldCommon: FieldCommon{T: "vr"}, V: pdf.NewReference(100, 0)},
	},
	{
		name:  "text variable text",
		field: &FieldTx{FieldCommon: FieldCommon{T: "vt"}, VariableText: VariableText{DefaultAppearance: "/Helv 12 Tf 0 g", Align: pdf.TextAlignCenter}, MaxLen: 24},
	},
	{
		name:  "comb",
		field: &FieldTx{FieldCommon: FieldCommon{T: "comb", Ff: optional.NewUInt(uint(FieldComb))}, MaxLen: 6},
	},
	{
		name:  "alternate names",
		field: &FieldChoice{FieldCommon: FieldCommon{T: "choice", TU: "Choose one", TM: "choice_map"}},
	},
	{
		name: "choice options",
		field: &FieldChoice{
			FieldCommon: FieldCommon{T: "fonts", Ff: optional.NewUInt(uint(FieldCombo))},
			Opt:         []ChoiceOption{{Export: "h", Display: "Helvetica"}, {Export: "Times", Display: "Times"}},
			TopIndex:    0,
			Selected:    []int{1},
			V:           pdf.String("Times"),
		},
	},
	{
		name: "checkbox",
		field: &FieldBtn{
			FieldCommon: FieldCommon{T: "agree"},
			V:           "Yes",
			DV:          "Off",
		},
	},
	{
		name: "radio with export values",
		field: &FieldBtn{
			FieldCommon: FieldCommon{T: "size", Ff: optional.NewUInt(uint(FieldRadio))},
			Opt:         []string{"small", "large"},
			V:           "small",
		},
	},
	{
		name:  "push button",
		field: &FieldBtn{FieldCommon: FieldCommon{T: "submit", Ff: optional.NewUInt(uint(FieldPushbutton))}},
	},
	{
		name:  "signature",
		field: &FieldSig{FieldCommon: FieldCommon{T: "sig"}},
	},
	{
		name: "signature with lock all",
		field: &FieldSig{
			FieldCommon: FieldCommon{T: "siglockall"},
			Lock:        &SigFieldLock{Action: SigFieldLockAll},
		},
	},
	{
		name: "signature with lock include",
		field: &FieldSig{
			FieldCommon: FieldCommon{T: "siglock"},
			Lock: &SigFieldLock{
				Action: SigFieldLockInclude,
				Fields: []string{"name", "address"},
			},
		},
	},
	{
		name: "signature with seed value",
		field: &FieldSig{
			FieldCommon: FieldCommon{T: "sigsv"},
			SV: &SigSeedValue{
				Flags:            SigSeedFilter | SigSeedSubFilter | SigSeedReasons,
				Filter:           "Adobe.PPKLite",
				SubFilter:        []pdf.Name{"adbe.pkcs7.detached", "ETSI.CAdES.detached"},
				DigestMethod:     []pdf.Name{"SHA256", "SHA512"},
				V:                2,
				Reasons:          []string{"I agree", "I approve"},
				MDP:              optional.NewUInt(2),
				TimeStamp:        &SigSeedValueTimeStamp{URL: "https://ts.example.com", Required: true},
				LegalAttestation: []string{"attestation"},
				AddRevInfo:       true,
			},
		},
	},
	{
		name: "signature with cert seed value",
		field: &FieldSig{
			FieldCommon: FieldCommon{T: "sigcert"},
			SV: &SigSeedValue{
				Filter: "Adobe.PPKLite",
				Cert: &SigCertSeedValue{
					Flags:     SigCertSubject | SigCertKeyUsage,
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
		field: &FieldSig{
			FieldCommon: FieldCommon{T: "sigsv20"},
			Lock: &SigFieldLock{
				Action: SigFieldLockExclude,
				Fields: []string{"sig"},
				P:      2,
			},
			SV: &SigSeedValue{
				Flags:            SigSeedLockDocument | SigSeedAppearanceFilter,
				LockDocument:     "true",
				AppearanceFilter: "MyAppearance",
				Cert: &SigCertSeedValue{
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
		field: &FieldTx{
			FieldCommon: FieldCommon{
				T: "calc",
				AA: &triggers.Form{
					Calculate: &action.JavaScript{JS: pdf.String("event.value = 0;")},
				},
			},
		},
	},
	{
		name: "non-terminal tree",
		field: &FieldTx{
			FieldCommon: FieldCommon{
				T: "address",
				Kids: []Node{
					&FieldCommon{T: "street"},
					&FieldCommon{T: "zip"},
				},
			},
		},
	},
	{
		name: "merged widget",
		field: &FieldBtn{
			FieldCommon: FieldCommon{
				T:    "submit",
				Kids: []Node{testWidget(0, 0, 72, 24)},
			},
		},
	},
	{
		// the merged /AA holds a field-level trigger (C) on the field and an
		// annotation-level trigger (Fo) on the widget; both halves must
		// survive the split/merge round trip
		name: "merged widget with mixed AA",
		field: &FieldBtn{
			FieldCommon: FieldCommon{
				T: "submitAA",
				Kids: []Node{withAA(testWidget(0, 0, 72, 24),
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
		field: &FieldBtn{
			FieldCommon: FieldCommon{
				T:    "calcOnly",
				Kids: []Node{testWidget(0, 0, 72, 24)},
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
		field: &FieldBtn{
			FieldCommon: FieldCommon{
				T: "focusOnly",
				Kids: []Node{withAA(testWidget(0, 0, 72, 24),
					&triggers.Annotation{Focus: &action.JavaScript{JS: pdf.String("focus();")}})},
			},
		},
	},
	{
		name: "multiple widgets",
		field: &FieldBtn{
			FieldCommon: FieldCommon{
				T:    "color",
				Ff:   optional.NewUInt(uint(FieldRadio)),
				Kids: []Node{testWidget(0, 0, 20, 20), testWidget(30, 0, 50, 20)},
			},
			Opt: []string{"red", "green"},
		},
	},
}

func fieldCmpOptions() []cmp.Option {
	return []cmp.Option{
		cmpopts.IgnoreUnexported(FieldCommon{}),
		// Widget.Parent is transient encode-time state, set while a merged field
		// is written; a decoded tree-owned widget has it nil
		cmpopts.IgnoreFields(Widget{}, "Parent"),
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

// storeFieldTree writes a single field's subtree the way [InteractiveForm.Encode]
// does, then writes its widget annotations as their pages would, so a field can
// be round-tripped on its own. It returns the reference naming f.
func storeFieldTree(rm *pdf.ResourceManager, f Field) (pdf.Reference, error) {
	ref, err := fieldRef(rm, f)
	if err != nil {
		return 0, err
	}
	if err := storeFieldWidgets(rm, f); err != nil {
		return 0, err
	}
	return ref, nil
}

// storeFieldWidgets stores every widget annotation in f's subtree, standing in
// for the pages that would normally write them.
func storeFieldWidgets(rm *pdf.ResourceManager, f Field) error {
	for _, kid := range f.GetFieldCommon().Kids {
		switch k := kid.(type) {
		case *Widget:
			if _, err := rm.Store(k); err != nil {
				return err
			}
		case Field:
			if err := storeFieldWidgets(rm, k); err != nil {
				return err
			}
		}
	}
	return nil
}

func fieldRoundTripTest(t *testing.T, version pdf.Version, want Field) {
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
	parent := &FieldTx{
		FieldCommon: FieldCommon{Ff: optional.NewUInt(uint(FieldRequired))},
		V:           pdf.String("pv"),
		DV:          pdf.String("dv"),
	}
	child := &FieldCommon{T: "c"}
	child.parent = parent

	if got := ResolvedFT(child); got != "Tx" {
		t.Errorf("ResolvedFT = %q, want %q", got, "Tx")
	}
	if got := ResolvedFf(child); got != FieldRequired {
		t.Errorf("ResolvedFf = %d, want %d", got, FieldRequired)
	}
	if got, ok := ResolvedV(child).(pdf.String); !ok || string(got) != "pv" {
		t.Errorf("ResolvedV = %v, want %q", ResolvedV(child), "pv")
	}
	if got, ok := ResolvedDV(child).(pdf.String); !ok || string(got) != "dv" {
		t.Errorf("ResolvedDV = %v, want %q", ResolvedDV(child), "dv")
	}

	// a local value overrides the inherited one
	typedChild := &FieldBtn{FieldCommon: FieldCommon{T: "c"}}
	typedChild.parent = parent
	if got := ResolvedFT(typedChild); got != "Btn" {
		t.Errorf("ResolvedFT after override = %q, want %q", got, "Btn")
	}

	// an explicit zero blocks inheritance of the ancestor's flags
	child.Ff = optional.NewUInt(0)
	if got := ResolvedFf(child); got != 0 {
		t.Errorf("ResolvedFf with explicit zero = %d, want 0", got)
	}
}

func TestFieldBtnVariant(t *testing.T) {
	for _, tt := range []struct {
		name string
		ff   FieldFlags
		want ButtonVariant
	}{
		{"checkbox", 0, ButtonCheckbox},
		{"radio", FieldRadio, ButtonRadio},
		{"push", FieldPushbutton, ButtonPush},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &FieldBtn{}
			if tt.ff != 0 {
				f.Ff = optional.NewUInt(uint(tt.ff))
			}
			if got := f.Variant(); got != tt.want {
				t.Errorf("Variant() = %d, want %d", got, tt.want)
			}
		})
	}

	// the variant flag may be inherited: a button sub-field with no flags of its
	// own takes the Radio flag from its parent
	child := &FieldBtn{}
	child.parent = &FieldCommon{Ff: optional.NewUInt(uint(FieldRadio))}
	if got := child.Variant(); got != ButtonRadio {
		t.Errorf("inherited Variant() = %d, want ButtonRadio (%d)", got, ButtonRadio)
	}
}

// an explicit Ff of zero on a child must survive a write/read cycle so that it
// keeps blocking inheritance of the parent's flags. Before Ff tracked its
// presence, the zero value was indistinguishable from absent and was dropped
// on encode, causing the child to inherit FieldRequired after the round trip.
func TestFieldExplicitZeroFlagsRoundTrip(t *testing.T) {
	want := &FieldTx{
		FieldCommon: FieldCommon{
			T:  "parent",
			Ff: optional.NewUInt(uint(FieldRequired)),
			Kids: []Node{
				&FieldCommon{T: "child", Ff: optional.NewUInt(0)},
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
	got, err := pdf.ExtractorGet(x, nil, r.GetMeta().Trailer["Quir:E"], DecodeField)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	kids := got.GetFieldCommon().Kids
	if len(kids) != 1 {
		t.Fatalf("expected one kid, got %d", len(kids))
	}
	child, ok := kids[0].(*FieldCommon)
	if !ok {
		t.Fatalf("expected a *FieldCommon kid, got %T", kids[0])
	}
	if _, set := child.Ff.Get(); !set {
		t.Error("child Ff lost its explicit-present status after round trip")
	}
	if rff := ResolvedFf(child); rff != 0 {
		t.Errorf("child ResolvedFf = %d, want 0 (explicit zero blocks inheritance)", rff)
	}
}

func TestFieldFullyQualifiedName(t *testing.T) {
	root := &FieldCommon{T: "PersonalData"}
	mid := &FieldCommon{T: "Address"}
	mid.parent = root
	leaf := &FieldCommon{T: "ZipCode"}
	leaf.parent = mid

	if got := leaf.FullyQualifiedName(); got != "PersonalData.Address.ZipCode" {
		t.Errorf("FullyQualifiedName = %q, want %q", got, "PersonalData.Address.ZipCode")
	}

	// an ancestor without a partial name is skipped
	anon := &FieldCommon{}
	anon.parent = root
	leaf2 := &FieldCommon{T: "Phone"}
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

	field, err := pdf.ExtractorGet(x, nil, refA, DecodeField)
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
	b, ok := kids[0].(*FieldCommon)
	if !ok {
		t.Fatalf("expected a sub-field kid, got %T", kids[0])
	}
	if len(b.Kids) != 0 {
		t.Errorf("expected B to have no kids after cycle break, got %d", len(b.Kids))
	}
}

func TestEncodeFieldNameWithPeriod(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	field := &FieldTx{FieldCommon: FieldCommon{T: "a.b"}}
	if _, err := fieldEntries(rm, field); err == nil {
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
	if name := field.GetFieldCommon().T; name != "abc" {
		t.Errorf("partial name = %q, want %q", name, "abc")
	}

	// the snapped name must re-encode without error
	rm := pdf.NewResourceManager(w)
	if _, err := fieldEntries(rm, field); err != nil {
		t.Errorf("re-encode of snapped name failed: %v", err)
	}
}

func TestEncodeFieldVersionGating(t *testing.T) {
	tests := []struct {
		name    string
		version pdf.Version
		field   Field
	}{
		{"field requires 1.2", pdf.V1_1, &FieldTx{FieldCommon: FieldCommon{T: "x"}}},
		{"TU requires 1.3", pdf.V1_2, &FieldTx{FieldCommon: FieldCommon{T: "x", TU: "label"}}},
		{"TM requires 1.3", pdf.V1_2, &FieldTx{FieldCommon: FieldCommon{T: "x", TM: "map"}}},
		{"AA requires 1.3", pdf.V1_2, &FieldTx{
			FieldCommon: FieldCommon{
				T:  "x",
				AA: &triggers.Form{Calculate: &action.JavaScript{JS: pdf.String("0;")}},
			},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := fieldEntries(rm, tc.field); !pdf.IsWrongVersion(err) {
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
	field, err := pdf.ExtractorGet(x, nil, parent, DecodeField)
	if err != nil {
		t.Fatal(err)
	}
	kids := field.GetFieldCommon().Kids
	if len(kids) != 1 {
		t.Fatalf("expected the anonymous sub-field to be preserved, got %d kids", len(kids))
	}
	child, ok := kids[0].(*FieldCommon)
	if !ok {
		t.Fatalf("expected a *FieldCommon kid, got %T", kids[0])
	}
	if got, set := child.Ff.Get(); !set || FieldFlags(got) != FieldRequired {
		t.Errorf("sub-field Ff = %v (set %v), want %d", got, set, FieldRequired)
	}
	if ResolvedFT(child) != "Tx" {
		t.Errorf("sub-field ResolvedFT = %q, want Tx (inherited)", ResolvedFT(child))
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
	field, err := pdf.ExtractorGet(x, nil, refs[0], DecodeField)
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
		next, ok := fc.Kids[0].(*FieldCommon)
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
	field, err := pdf.ExtractorGet(x, nil, rootRef, DecodeField)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(field.GetFieldCommon().Kids); got != n {
		t.Errorf("kids = %d, want %d", got, n)
	}
}
