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
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/opaque"
)

// testVersions lists the PDF versions exercised by the round-trip unit tests and
// the fuzz seed corpora.
var testVersions = []pdf.Version{pdf.V1_7, pdf.V2_0}

// ensureWidgetAppearance gives a widget an appearance if it lacks one, as PDF
// 2.0 requires for widgets with a non-empty rectangle.
func ensureWidgetAppearance(w *annotation.Widget) {
	if w.Appearance != nil {
		return
	}
	if w.Rect.LLx == w.Rect.URx && w.Rect.LLy == w.Rect.URy {
		return // single-point widgets are exempt from the requirement
	}
	w.Appearance = defaultAppearanceDict
}

// addWidget attaches a widget annotation to a field, giving it the default
// fallback appearance so the round trip is valid at PDF 2.0.
func addWidget(f acroform.Field, llx, lly, urx, ury float64) *annotation.Widget {
	w := annotation.AddWidget(f, pdf.Rectangle{LLx: llx, LLy: lly, URx: urx, URy: ury})
	ensureWidgetAppearance(w)
	return w
}

// fieldCmpOptions configures cmp for the field-tree snapshots. A widget's
// /Parent is a back-reference into the tree and is compared structurally
// elsewhere, so it is ignored here.
func fieldCmpOptions() []cmp.Option {
	return []cmp.Option{
		cmp.AllowUnexported(language.Tag{}),
		cmpopts.EquateComparable(language.Tag{}),
		cmpopts.IgnoreFields(annotation.Widget{}, "Field"),
		cmp.Comparer(func(a, b *form.Form) bool {
			if a == nil || b == nil {
				return a == b
			}
			return a.Equal(b)
		}),
	}
}

// nodeSnap is a comparable snapshot of a field-tree node, capturing the public
// state of a [acroform.Group] or terminal [acroform.Field]. It sidesteps the
// unexported base shared by the field types, which cmp cannot traverse.
type nodeSnap struct {
	Kind     string
	Name     string
	TU, TM   string
	Flags    acroform.FieldFlags
	AA       *triggers.Form
	VT       acroform.VariableText
	V, DV    *pdf.StringOrStream
	SigV     *opaque.Object
	SigDV    *opaque.Object
	MaxLen   int
	BtnV     pdf.Name
	BtnDV    pdf.Name
	BtnOpt   []string
	ChV      []string
	ChDV     []string
	ChOpt    []acroform.ChoiceOption
	TopIndex int
	Selected []int
	Lock     *acroform.SigFieldLock
	SV       *acroform.SigSeedValue
	Widgets  []*annotation.Widget
	Kids     []nodeSnap
}

func snapNodes(nodes []acroform.Node) []nodeSnap {
	out := make([]nodeSnap, len(nodes))
	for i, n := range nodes {
		out[i] = snapNode(n)
	}
	return out
}

func snapNode(n acroform.Node) nodeSnap {
	s := nodeSnap{Name: n.PartialName()}
	switch f := n.(type) {
	case *acroform.Group:
		s.Kind = "Group"
		s.Kids = snapNodes(f.Children)
	case *acroform.TextField:
		s.Kind = "Tx"
		s.TU, s.TM, s.Flags, s.AA = f.AltName, f.ExportName, f.Flags, f.AA
		s.VT, s.V, s.DV, s.MaxLen = f.VariableText, f.V, f.DV, f.MaxLen
		s.Widgets = widgetsOf(f)
	case *acroform.ButtonField:
		s.Kind = "Btn"
		s.TU, s.TM, s.Flags, s.AA = f.AltName, f.ExportName, f.Flags, f.AA
		s.VT, s.BtnV, s.BtnDV, s.BtnOpt = f.VariableText, f.V, f.DV, f.Opt
		s.Widgets = widgetsOf(f)
	case *acroform.ChoiceField:
		s.Kind = "Ch"
		s.TU, s.TM, s.Flags, s.AA = f.AltName, f.ExportName, f.Flags, f.AA
		s.VT, s.ChV, s.ChDV = f.VariableText, f.V, f.DV
		s.ChOpt, s.TopIndex, s.Selected = f.Opt, f.TopIndex, f.Selected
		s.Widgets = widgetsOf(f)
	case *acroform.SignatureField:
		s.Kind = "Sig"
		s.TU, s.TM, s.Flags, s.AA = f.AltName, f.ExportName, f.Flags, f.AA
		s.SigV, s.SigDV, s.Lock, s.SV = f.V, f.DV, f.Lock, f.SV
		s.Widgets = widgetsOf(f)
	}
	return s
}

func widgetsOf(f acroform.Field) []*annotation.Widget {
	ws := f.GetCommon().Widgets
	if len(ws) == 0 {
		return nil
	}
	out := make([]*annotation.Widget, len(ws))
	for i, w := range ws {
		out[i] = w.(*annotation.Widget)
	}
	return out
}

// roundTripRoots encodes the given field-tree roots as an interactive form,
// writes their widgets as their pages would, then decodes the form and returns
// its fields.
func roundTripRoots(t *testing.T, version pdf.Version, roots ...acroform.Node) []acroform.Node {
	t.Helper()
	form := &acroform.InteractiveForm{Fields: roots}
	return roundTripForm(t, version, form).Fields
}

// roundTripForm encodes and decodes a whole interactive form.
func roundTripForm(t *testing.T, version pdf.Version, want *acroform.InteractiveForm) *acroform.InteractiveForm {
	t.Helper()

	w, buf := memfile.NewPDFWriter(t, version, nil)

	rm := pdf.NewResourceManager(w)
	ref, err := rm.Store(want)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("encode failed: %v", err)
	}
	if err := storeWidgets(rm, want.Fields); err != nil {
		t.Fatalf("store widgets: %v", err)
	}
	w.GetMeta().Catalog.AcroForm = ref

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
	t.Cleanup(func() { r.Close() })

	x := pdf.NewExtractor(r)
	got, err := pdf.Decode(pdf.CursorAt(x, nil), r.GetMeta().Catalog.AcroForm, Form)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	return got
}

// storeWidgets writes every widget annotation in the field tree, standing in for
// the pages that would normally write them.
func storeWidgets(rm *pdf.ResourceManager, nodes []acroform.Node) error {
	for _, n := range nodes {
		switch f := n.(type) {
		case *acroform.Group:
			if err := storeWidgets(rm, f.Children); err != nil {
				return err
			}
		case acroform.Field:
			for _, wg := range f.GetCommon().Widgets {
				if _, err := rm.Store(wg); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

var formTestCases = []struct {
	name string
	form *acroform.InteractiveForm
}{
	{
		name: "minimal",
		form: &acroform.InteractiveForm{
			Fields: []acroform.Node{newTextField("f0")},
		},
	},
	{
		name: "flags and defaults",
		form: &acroform.InteractiveForm{
			Fields:          []acroform.Node{newTextField("f0"), newTextField("f1")},
			NeedAppearances: true,
			SigFlags:        acroform.SignaturesExist | acroform.AppendOnly,
		},
	},
	{
		name: "calculation order",
		form: func() *acroform.InteractiveForm {
			f0 := newTextField("calc0")
			f1 := newTextField("calc1")
			return &acroform.InteractiveForm{
				Fields:           []acroform.Node{f0, f1},
				CalculationOrder: []acroform.Field{f1, f0},
			}
		}(),
	},
	{
		name: "xfa",
		form: &acroform.InteractiveForm{
			Fields: []acroform.Node{newTextField("f0")},
			XFA:    opaque.Direct(pdf.Array{pdf.String("template"), pdf.String("<xdp/>")}),
		},
	},
	{
		name: "default resources",
		form: &acroform.InteractiveForm{
			Fields:           []acroform.Node{newTextField("f0")},
			DefaultResources: &content.Resources{SingleUse: true},
		},
	},
}

func compareForms(t *testing.T, want, got *acroform.InteractiveForm) {
	t.Helper()
	if want.NeedAppearances != got.NeedAppearances {
		t.Errorf("NeedAppearances = %v, want %v", got.NeedAppearances, want.NeedAppearances)
	}
	if !want.XFA.Equal(got.XFA) {
		t.Errorf("XFA = %v, want %v", got.XFA, want.XFA)
	}
	if want.SigFlags != got.SigFlags {
		t.Errorf("SigFlags = %d, want %d", got.SigFlags, want.SigFlags)
	}
	if len(want.CalculationOrder) != len(got.CalculationOrder) {
		t.Errorf("CalculationOrder length = %d, want %d", len(got.CalculationOrder), len(want.CalculationOrder))
	}
	if diff := cmp.Diff(snapNodes(want.Fields), snapNodes(got.Fields), fieldCmpOptions()...); diff != "" {
		t.Errorf("fields round trip failed (-want +got):\n%s", diff)
	}
}

func TestFormRoundTrip(t *testing.T) {
	for _, version := range testVersions {
		for _, tc := range formTestCases {
			t.Run(tc.name+"-"+version.String(), func(t *testing.T) {
				got := roundTripForm(t, version, tc.form)
				compareForms(t, tc.form, got)
			})
		}
	}
}

func TestDecodeValues(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.TextString("f0")}); err != nil {
		t.Fatal(err)
	}
	dict := pdf.Dict{
		// the second element is not a reference and must be skipped
		"Fields":          pdf.Array{ref, pdf.Integer(7)},
		"NeedAppearances": pdf.Boolean(true),
		"SigFlags":        pdf.Integer(3),
		"DA":              pdf.String("/Helv 0 Tf"),
		"Q":               pdf.Integer(99), // out of range, must be ignored
	}

	form, err := Form(pdf.CursorAt(x, nil), dict, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(form.Fields) != 1 || form.Fields[0].PartialName() != "f0" {
		t.Errorf("Fields = %v, want one field named f0", form.Fields)
	}
	if !form.NeedAppearances {
		t.Error("NeedAppearances = false, want true")
	}
	if form.SigFlags != acroform.SignaturesExist|acroform.AppendOnly {
		t.Errorf("SigFlags = %d, want %d", form.SigFlags, acroform.SignaturesExist|acroform.AppendOnly)
	}
}

func TestDecodeNil(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	form, err := Form(pdf.CursorAt(x, nil), nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if form != nil {
		t.Errorf("expected nil form for nil object, got %v", form)
	}
}

// FuzzFormRoundTrip checks that the reader survives malformed input and that any
// form it decodes re-encodes and decodes back to the same fields.
func FuzzFormRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}
	for _, version := range testVersions {
		for _, tc := range formTestCases {
			if data, ok := encodeFormBytes(f, version, opt, tc.form); ok {
				f.Add(data)
			}
		}
		for _, tc := range fieldTestCases() {
			form := &acroform.InteractiveForm{Fields: []acroform.Node{tc.root}}
			if data, ok := encodeFormBytes(f, version, opt, form); ok {
				f.Add(data)
			}
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		defer r.Close()

		x := pdf.NewExtractor(r)
		form1, err := pdf.Decode(pdf.CursorAt(x, nil), r.GetMeta().Catalog.AcroForm, Form)
		if err != nil || form1 == nil {
			t.Skip("no interactive form")
		}

		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}
		form2 := roundTripForm(t, version, form1)
		if diff := cmp.Diff(snapNodes(form1.Fields), snapNodes(form2.Fields), fieldCmpOptions()...); diff != "" {
			t.Errorf("not a fixed point (-first +second):\n%s", diff)
		}
	})
}

// encodeFormBytes writes a form (and its widgets) to a self-contained PDF,
// returning false if it cannot be encoded at the given version.
func encodeFormBytes(tb testing.TB, version pdf.Version, opt *pdf.WriterOptions, form *acroform.InteractiveForm) ([]byte, bool) {
	w, buf := memfile.NewPDFWriter(tb, version, opt)
	rm := pdf.NewResourceManager(w)
	// the widgets reserve their references (as a page would); the form is
	// encoded at Close and fills them in
	if err := storeWidgets(rm, form.Fields); err != nil {
		return nil, false
	}
	w.GetMeta().Catalog.AcroForm = rm.StoreDeferred(form)
	if err := rm.Close(); err != nil {
		return nil, false
	}
	if err := w.Close(); err != nil {
		return nil, false
	}
	return buf.Data, true
}

// flatten-on-read and hoist-on-write are a fixed point: decoding a form,
// re-encoding it, and decoding again yields the same fields. This exercises the
// inheritance round trip — values hoisted on the first write are flattened back
// on the second read.
func TestFormFixedPoint(t *testing.T) {
	build := func() []*acroform.InteractiveForm {
		forms := make([]*acroform.InteractiveForm, 0)
		for _, tc := range formTestCases {
			forms = append(forms, tc.form)
		}
		for _, tc := range fieldTestCases() {
			forms = append(forms, &acroform.InteractiveForm{Fields: []acroform.Node{tc.root}})
		}
		// a deeper tree with shared inheritable attributes to hoist
		mk := func(name, da string) *acroform.TextField {
			f := newTextField(name)
			f.DefaultAppearance = da
			f.Align = pdf.TextAlignCenter
			return f
		}
		forms = append(forms, &acroform.InteractiveForm{
			Fields: []acroform.Node{
				&acroform.Group{Name: "g", Children: []acroform.Node{
					mk("a", "/Helv 12 Tf"), mk("b", "/Helv 12 Tf"), mk("c", "/Helv 12 Tf"),
				}},
				mk("d", "/Helv 12 Tf"),
			},
		})
		return forms
	}

	for _, version := range testVersions {
		for i, form := range build() {
			t.Run(fmt.Sprintf("%d-%s", i, version), func(t *testing.T) {
				got1 := roundTripForm(t, version, form)
				got2 := roundTripForm(t, version, got1)
				if diff := cmp.Diff(snapNodes(got1.Fields), snapNodes(got2.Fields), fieldCmpOptions()...); diff != "" {
					t.Errorf("not a fixed point (-first +second):\n%s", diff)
				}
			})
		}
	}
}

// Undefined SigFlags bits are cleared, and an XFA entry which is neither a
// stream nor an array is dropped.
func TestDecodeFormRepairs(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	form, err := Form(pdf.CursorAt(x, nil), pdf.Dict{
		"Fields":   pdf.Array{},
		"SigFlags": pdf.Integer(-1),
		"XFA":      pdf.Integer(1),
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := acroform.SignaturesExist | acroform.AppendOnly; form.SigFlags != want {
		t.Errorf("SigFlags = %d, want %d", form.SigFlags, want)
	}
	if form.XFA != nil {
		t.Errorf("XFA = %v, want nil", form.XFA)
	}
}

// An XFA stream is copied into the output file.
func TestFormXFAStreamRoundTrip(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	ref := src.Alloc()
	stm, err := src.OpenStream(ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write([]byte("<xdp/>")); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}

	form, err := Form(pdf.CursorAt(pdf.NewExtractor(src), nil), pdf.Dict{
		"Fields": pdf.Array{},
		"XFA":    ref,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if form.XFA == nil {
		t.Fatal("XFA stream dropped")
	}
	form.Fields = []acroform.Node{newTextField("f")}

	got := roundTripForm(t, pdf.V1_7, form)
	if got.XFA == nil {
		t.Fatal("XFA lost in round trip")
	}
	s, err := opaque.ObjectAs(got.XFA, nil, func(c pdf.Cursor, obj pdf.Object, _ bool) ([]byte, error) {
		return c.ReadAll(obj, 1<<20)
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(s) != "<xdp/>" {
		t.Errorf("XFA stream = %q, want \"<xdp/>\"", s)
	}
}

// Fields sharing a fully qualified name are made to agree: a later field of
// the same type takes the value of the first, one of a different type is
// dropped.
func TestDecodeDuplicateNames(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	dicts := []pdf.Dict{
		{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "V": pdf.String("first")},
		{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "V": pdf.String("second")},
		{"FT": pdf.Name("Ch"), "T": pdf.String("a")},
	}
	var fields pdf.Array
	for _, d := range dicts {
		ref := w.Alloc()
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
		fields = append(fields, ref)
	}

	form, err := Form(pdf.CursorAt(x, nil), pdf.Dict{"Fields": fields}, true)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(form.Fields); n != 2 {
		t.Fatalf("got %d fields, want 2", n)
	}
	for i, node := range form.Fields {
		f, ok := node.(*acroform.TextField)
		if !ok {
			t.Fatalf("field %d is %T, want *acroform.TextField", i, node)
		}
		if f.V == nil || f.V.Value != "first" {
			t.Errorf("field %d: V = %v, want \"first\"", i, f.V)
		}
	}

	// the repaired form can be written
	roundTripForm(t, pdf.V1_7, form)
}

// A password field never stores a value, so it cannot agree with a valued
// field of the same name and is dropped; a valued field following a password
// field takes the empty value.
func TestDecodeDuplicateNamesPassword(t *testing.T) {
	password := pdf.Integer(acroform.FieldPassword)
	cases := []struct {
		name  string
		dicts []pdf.Dict
		want  int
	}{
		{"valued first", []pdf.Dict{
			{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "V": pdf.String("first")},
			{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "Ff": password},
		}, 1},
		{"password first", []pdf.Dict{
			{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "Ff": password},
			{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "V": pdf.String("second")},
		}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			x := pdf.NewExtractor(w)

			var fields pdf.Array
			for _, d := range tc.dicts {
				ref := w.Alloc()
				if err := w.Put(ref, d); err != nil {
					t.Fatal(err)
				}
				fields = append(fields, ref)
			}
			form, err := Form(pdf.CursorAt(x, nil), pdf.Dict{"Fields": fields}, true)
			if err != nil {
				t.Fatal(err)
			}
			if n := len(form.Fields); n != tc.want {
				t.Fatalf("got %d fields, want %d", n, tc.want)
			}
			for i, node := range form.Fields {
				f, ok := node.(*acroform.TextField)
				if !ok {
					t.Fatalf("field %d is %T, want *acroform.TextField", i, node)
				}
				if f.Flags&acroform.FieldPassword != 0 && f.V != nil {
					t.Errorf("field %d: password field has value %v", i, f.V)
				}
			}
			roundTripForm(t, pdf.V1_7, form)
		})
	}
}
