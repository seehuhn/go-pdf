// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/text/language"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/content"
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
// Field points back to the field whose snapshot holds the widget, so it is
// ignored here.
func fieldCmpOptions() []cmp.Option {
	return []cmp.Option{
		cmpopts.EquateComparable(language.Tag{}),
		cmpopts.IgnoreFields(annotation.Widget{}, "Field"),
	}
}

// nodeSnap is a comparable snapshot of a field-tree node, capturing the public
// state of a [acroform.Group] or terminal [acroform.Field]. It sidesteps the
// unexported base shared by the field types, which cmp cannot traverse.
type nodeSnap struct {
	Kind     string
	Name     string
	TU       pdf.TextString
	TM       string
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
	ChV      []pdf.TextString
	ChDV     []pdf.TextString
	ChOpt    []acroform.ChoiceOption
	TopIndex int
	Selected []int
	Lock     *acroform.SigFieldLock
	SV       *acroform.SigSeedValue
	Widgets  []*annotation.Widget
	Kids     []nodeSnap
}

func snapNodes(t testing.TB, nodes []acroform.Node) []nodeSnap {
	t.Helper()
	out := make([]nodeSnap, len(nodes))
	for i, n := range nodes {
		out[i] = snapNode(t, n)
	}
	return out
}

func snapNode(t testing.TB, n acroform.Node) nodeSnap {
	t.Helper()
	s := nodeSnap{Name: n.PartialName()}
	switch f := n.(type) {
	case *acroform.Group:
		s.Kind = "Group"
		s.Kids = snapNodes(t, f.Children)
	case *acroform.TextField:
		s.Kind = "Tx"
		s.TU, s.TM, s.Flags, s.AA = f.AltName, f.ExportName, f.Flags, f.AA
		s.VT, s.V, s.DV, s.MaxLen = f.VariableText, f.Text, f.DefaultText, f.MaxLen
		s.Widgets = widgetsOf(t, f)
	case *acroform.ButtonField:
		s.Kind = "Btn"
		s.TU, s.TM, s.Flags, s.AA = f.AltName, f.ExportName, f.Flags, f.AA
		s.VT, s.BtnV, s.BtnDV, s.BtnOpt = f.VariableText, f.State, f.DefaultState, f.ExportValues
		s.Widgets = widgetsOf(t, f)
	case *acroform.ChoiceField:
		s.Kind = "Ch"
		s.TU, s.TM, s.Flags, s.AA = f.AltName, f.ExportName, f.Flags, f.AA
		s.VT, s.ChV, s.ChDV = f.VariableText, f.Value, f.DefaultValue
		s.ChOpt, s.TopIndex, s.Selected = f.Options, f.TopIndex, f.Selected
		s.Widgets = widgetsOf(t, f)
	case *acroform.SignatureField:
		s.Kind = "Sig"
		s.TU, s.TM, s.Flags, s.AA = f.AltName, f.ExportName, f.Flags, f.AA
		s.SigV, s.SigDV, s.Lock, s.SV = f.V, f.DV, f.Lock, f.SV
		s.Widgets = widgetsOf(t, f)
	}
	return s
}

func widgetsOf(t testing.TB, f acroform.Field) []*annotation.Widget {
	t.Helper()
	ws := f.GetCommon().Widgets
	if len(ws) == 0 {
		return nil
	}
	out := make([]*annotation.Widget, len(ws))
	for i, w := range ws {
		out[i] = w.(*annotation.Widget)
		if out[i].Field != f {
			t.Errorf("widget %d of field %q does not link back to it", i, f.PartialName())
		}
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
	// the pages come first: writing them reserves the widgets' references,
	// which the form fills in
	if err := storeWidgets(rm, want.Fields); err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("store widgets: %v", err)
	}
	ref, err := rm.Store(want)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("encode failed: %v", err)
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

	idx, err := DocumentForm(pdf.NewExtractor(r))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	} else if idx == nil {
		t.Fatal("form not found")
	}
	return idx.Form
}

// decodeForm reads a form dictionary through [readForm], for tests which build
// the dictionary by hand.  It returns nil if obj is nil.
func decodeForm(c pdf.Cursor, obj pdf.Object) (*acroform.InteractiveForm, error) {
	idx, err := readForm(c, obj, true)
	if err != nil || idx == nil {
		return nil, err
	}
	return idx.Form, nil
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
	if diff := cmp.Diff(snapNodes(t, want.Fields), snapNodes(t, got.Fields), fieldCmpOptions()...); diff != "" {
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
	if err := w.Put(ref, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.Text("f0")}); err != nil {
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

	idx, err := readForm(pdf.CursorAt(x, nil), dict, true)
	if err != nil {
		t.Fatal(err)
	}
	form := idx.Form

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

	form, err := decodeForm(pdf.CursorAt(x, nil), nil)
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

		idx, err := DocumentForm(pdf.NewExtractor(r))
		if err != nil || idx == nil {
			t.Skip("no interactive form")
		}
		form1 := idx.Form

		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}
		form2 := roundTripForm(t, version, form1)
		if diff := cmp.Diff(snapNodes(t, form1.Fields), snapNodes(t, form2.Fields), fieldCmpOptions()...); diff != "" {
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
				if diff := cmp.Diff(snapNodes(t, got1.Fields), snapNodes(t, got2.Fields), fieldCmpOptions()...); diff != "" {
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

	form, err := decodeForm(pdf.CursorAt(x, nil), pdf.Dict{
		"Fields":   pdf.Array{},
		"SigFlags": pdf.Integer(-1),
		"XFA":      pdf.Integer(1),
	})
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

	form, err := decodeForm(pdf.CursorAt(pdf.NewExtractor(src), nil), pdf.Dict{
		"Fields": pdf.Array{},
		"XFA":    ref,
	})
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
	s, err := got.XFA.As(nil, func(c pdf.Cursor, obj pdf.Object, _ bool) ([]byte, error) {
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
// the same type takes the value of the first, with its rich text value, and
// one of a different type is dropped.
func TestDecodeDuplicateNames(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	dicts := []pdf.Dict{
		{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "V": pdf.String("first"), "RV": pdf.String("<p>first</p>")},
		{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "V": pdf.String("second"), "RV": pdf.String("<p>second</p>")},
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

	form, err := decodeForm(pdf.CursorAt(x, nil), pdf.Dict{"Fields": fields})
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
		if f.Text == nil || f.Text.Value.String() != "first" {
			t.Errorf("field %d: V = %v, want \"first\"", i, f.Text)
		}
		if f.RichText == nil || f.RichText.Value.String() != "<p>first</p>" {
			t.Errorf("field %d: RichText = %v, want the first field's", i, f.RichText)
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
			form, err := decodeForm(pdf.CursorAt(x, nil), pdf.Dict{"Fields": fields})
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
				if f.Flags&acroform.FieldPassword != 0 && f.Text != nil {
					t.Errorf("field %d: password field has value %v", i, f.Text)
				}
			}
			roundTripForm(t, pdf.V1_7, form)
		})
	}
}

// A single-selection choice field cannot hold an explicit empty or a
// multiple selection of a same-named multi-select field, and is dropped; one
// which can hold the value takes it, and its own /I goes unless it restates
// the value.
func TestDecodeDuplicateNamesChoice(t *testing.T) {
	multi := pdf.Integer(acroform.FieldMultiSelect)
	opt := pdf.Array{pdf.String("a"), pdf.String("b"), pdf.String("c")}
	cases := []struct {
		name   string
		first  pdf.Dict
		second pdf.Dict
		want   int
	}{
		{"multiple", pdf.Dict{"Ff": multi, "V": pdf.Array{pdf.String("a"), pdf.String("b")}},
			pdf.Dict{}, 1},
		{"explicit empty", pdf.Dict{"Ff": multi, "V": pdf.Array{}}, pdf.Dict{}, 1},
		{"multiple DV", pdf.Dict{"Ff": multi, "DV": pdf.Array{pdf.String("a"), pdf.String("b")}},
			pdf.Dict{}, 1},
		{"single", pdf.Dict{"V": pdf.String("b")}, pdf.Dict{"I": pdf.Array{pdf.Integer(0)}}, 2},
		{"option missing", pdf.Dict{"V": pdf.String("c")},
			pdf.Dict{"Opt": pdf.Array{pdf.String("a"), pdf.String("b")}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			objs := map[string]pdf.Dict{}
			for name, extra := range map[string]pdf.Dict{"first": tc.first, "second": tc.second} {
				d := pdf.Dict{"FT": pdf.Name("Ch"), "T": pdf.String("a"), "Opt": opt}
				maps.Copy(d, extra)
				objs[name] = d
			}
			r, _ := writeRawForm(t, objs, []string{"first", "second"}, func(refs map[string]pdf.Reference) pdf.Dict {
				return pdf.Dict{"Fields": pdf.Array{refs["first"], refs["second"]}}
			})
			idx, err := DocumentForm(pdf.NewExtractor(r))
			if err != nil {
				t.Fatal(err)
			}
			form := idx.Form
			if n := len(form.Fields); n != tc.want {
				t.Fatalf("got %d fields, want %d", n, tc.want)
			}
			first := form.Fields[0].(*acroform.ChoiceField)
			for i, node := range form.Fields[1:] {
				f := node.(*acroform.ChoiceField)
				if diff := cmp.Diff(first.Value, f.Value); diff != "" {
					t.Errorf("field %d: V differs (-want +got):\n%s", i+1, diff)
				}
				if !f.SelectionAgrees() && f.Selected != nil {
					t.Errorf("field %d: Selected %v disagrees with Value %v", i+1, f.Selected, f.Value)
				}
			}
			roundTripForm(t, pdf.V1_7, form)
		})
	}
}

// A check box takes the value of a same-named one and shows it in its
// widgets, rather than letting their states override it; one whose widgets
// have no such on-state keeps the value with all its widgets off.
func TestDecodeDuplicateNamesButton(t *testing.T) {
	cases := []struct {
		name   string
		on     pdf.Name
		wantAS pdf.Name
	}{
		{"same on-state", "Yes", "Yes"},
		{"other on-state", "On", "Off"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pw, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rect := pdf.Rectangle{URx: 10, URy: 10}
			ap := func(on pdf.Name) pdf.Dict {
				return pdf.Dict{"N": pdf.Dict{
					on:    appearanceStream(t, pw, rect),
					"Off": appearanceStream(t, pw, rect),
				}}
			}
			firstRef, secondRef, kidRef := pw.Alloc(), pw.Alloc(), pw.Alloc()
			objs := map[pdf.Reference]pdf.Dict{
				firstRef: rawWidget(pdf.Dict{
					"FT": pdf.Name("Btn"), "T": pdf.String("a"),
					"V": pdf.Name("Yes"), "AS": pdf.Name("Yes"), "AP": ap("Yes"),
				}),
				secondRef: {
					"FT": pdf.Name("Btn"), "T": pdf.String("a"),
					"V": pdf.Name("Off"), "Kids": pdf.Array{kidRef},
				},
				kidRef: rawWidget(pdf.Dict{
					"Parent": secondRef, "AS": pdf.Name("Off"), "AP": ap(tc.on),
				}),
			}
			for ref, d := range objs {
				if err := pw.Put(ref, d); err != nil {
					t.Fatal(err)
				}
			}

			x := pdf.NewExtractor(pw)
			c := pdf.CursorAt(x, nil)
			idx, err := readForm(c, pdf.Dict{"Fields": pdf.Array{firstRef, secondRef}}, true)
			if err != nil {
				t.Fatal(err)
			}
			form := idx.Form
			if n := len(form.Fields); n != 2 {
				t.Fatalf("got %d fields, want 2", n)
			}

			kid, err := c.Decode(kidRef, annotationBody)
			if err != nil {
				t.Fatal(err)
			}
			w := kid.(*annotation.Widget)
			_, listed := idx.WidgetRefs[w]
			f := form.Fields[1].(*acroform.ButtonField)
			if f.State != "Yes" || w.AppearanceState != tc.wantAS {
				t.Errorf("V=%q AS=%q, want Yes, %s", f.State, w.AppearanceState, tc.wantAS)
			}
			if w.Field != f || !listed {
				t.Errorf("widget not linked: Field=%v listed=%v", w.Field, listed)
			}
			roundTripForm(t, pdf.V1_7, form)
		})
	}
}

// writeRawForm writes the objects of a small form by hand and returns a
// reader together with the reference of each named object.  A string value
// "@name" in a dictionary, or in an array within one, is replaced by the
// reference of the object called name.  The form dictionary is written as
// the object called "form".
func writeRawForm(t *testing.T, objs map[string]pdf.Dict, order []string, form func(refs map[string]pdf.Reference) pdf.Dict) (*pdf.Reader, map[string]pdf.Reference) {
	t.Helper()
	return writeRawDoc(t, objs, order, form, false)
}

// writeRawDoc is like [writeRawForm], but if inline is set it stores the form
// dictionary directly in the document catalog.
func writeRawDoc(t *testing.T, objs map[string]pdf.Dict, order []string, form func(refs map[string]pdf.Reference) pdf.Dict, inline bool) (*pdf.Reader, map[string]pdf.Reference) {
	t.Helper()
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	refs := map[string]pdf.Reference{}
	for _, name := range order {
		refs[name] = w.Alloc()
	}
	if !inline {
		refs["form"] = w.Alloc()
	}
	for _, name := range order {
		d := objs[name]
		for k, v := range d {
			if s, ok := v.(pdf.String); ok && len(s) > 0 && s[0] == '@' {
				d[k] = refs[string(s[1:])]
			}
			if a, ok := v.(pdf.Array); ok {
				for i, el := range a {
					if s, ok := el.(pdf.String); ok && len(s) > 0 && s[0] == '@' {
						a[i] = refs[string(s[1:])]
					}
				}
			}
		}
		if err := w.Put(refs[name], d); err != nil {
			t.Fatal(err)
		}
	}
	if inline {
		w.GetMeta().Catalog.AcroForm = form(refs)
	} else {
		if err := w.Put(refs["form"], form(refs)); err != nil {
			t.Fatal(err)
		}
		w.GetMeta().Catalog.AcroForm = refs["form"]
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r, refs
}

func rawWidget(extra pdf.Dict) pdf.Dict {
	d := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Widget"),
		"Rect":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)},
	}
	maps.Copy(d, extra)
	return d
}

// TestDocumentFormSharesValues checks that the widgets read from a page are
// those of the form, linked to the fields the index names.
func TestDocumentFormSharesValues(t *testing.T) {
	objs := map[string]pdf.Dict{
		"merged": rawWidget(pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("m")}),
		"parent": {"FT": pdf.Name("Tx"), "T": pdf.String("p"), "Kids": pdf.Array{pdf.String("@kid")}},
		"kid":    rawWidget(pdf.Dict{"Parent": pdf.String("@parent")}),
	}
	r, refs := writeRawForm(t, objs, []string{"merged", "parent", "kid"}, func(refs map[string]pdf.Reference) pdf.Dict {
		return pdf.Dict{"Fields": pdf.Array{refs["merged"], refs["parent"]}}
	})

	x := pdf.NewExtractor(r)
	c := pdf.CursorAt(x, nil)
	idx, err := DocumentForm(x)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.FieldRefs) != 2 {
		t.Fatalf("got %d field refs, want 2", len(idx.FieldRefs))
	}

	_, annots, err := PageAnnotations(c, pdf.Array{refs["merged"], refs["kid"]})
	if err != nil {
		t.Fatal(err)
	}
	for i, wantRef := range []pdf.Reference{refs["merged"], refs["parent"]} {
		w, ok := annots[i].(*annotation.Widget)
		if !ok {
			t.Fatalf("annotation %d is %T", i, annots[i])
		}
		if got := idx.FieldRefs[w.Field]; got != wantRef {
			t.Errorf("field %d: ref %v, want %v", i, got, wantRef)
		}
	}
}

func TestDocumentFormParentMismatch(t *testing.T) {
	objs := map[string]pdf.Dict{
		"holder": {"FT": pdf.Name("Tx"), "T": pdf.String("h"), "Kids": pdf.Array{pdf.String("@kid")}},
		"other":  {"FT": pdf.Name("Tx"), "T": pdf.String("o")},
		"kid":    rawWidget(pdf.Dict{"Parent": pdf.String("@other")}),
	}
	r, refs := writeRawForm(t, objs, []string{"holder", "other", "kid"}, func(refs map[string]pdf.Reference) pdf.Dict {
		return pdf.Dict{"Fields": pdf.Array{refs["holder"]}}
	})

	idx, err := DocumentForm(pdf.NewExtractor(r))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.FieldRefs) != 1 {
		t.Fatalf("got %d field refs, want 1", len(idx.FieldRefs))
	}
	for _, ref := range idx.FieldRefs {
		if ref != refs["holder"] {
			t.Errorf("ref %v, want %v", ref, refs["holder"])
		}
	}
}

func TestReadFormDirect(t *testing.T) {
	r, _ := writeRawForm(t, nil, nil, func(map[string]pdf.Reference) pdf.Dict { return pdf.Dict{} })
	dict := pdf.Dict{"Fields": pdf.Array{}}
	c := pdf.CursorAt(pdf.NewExtractor(r), nil)
	idx, err := readForm(c, dict, true)
	if err != nil || idx == nil || idx.Form == nil {
		t.Fatalf("direct index: %v, %v", idx, err)
	}
}

// inlineFormOrder names the objects of [inlineFormObjects], in the order they
// are written.
var inlineFormOrder = []string{"f", "kid", "orphan1", "orphan2", "cb", "on", "off"}

// inlineFormObjects returns the objects of a form with a field "f", whose
// widget "kid" is meant for a page, two orphan widgets, and a check box "cb"
// with a widget "on" meant for a page and a widget "off" for none.
func inlineFormObjects() map[string]pdf.Dict {
	return map[string]pdf.Dict{
		"f":       {"FT": pdf.Name("Tx"), "T": pdf.String("f"), "Kids": pdf.Array{pdf.String("@kid")}},
		"kid":     rawWidget(pdf.Dict{"Parent": pdf.String("@f")}),
		"orphan1": rawWidget(nil),
		"orphan2": rawWidget(nil),
		"cb":      {"FT": pdf.Name("Btn"), "T": pdf.String("cb"), "Kids": pdf.Array{pdf.String("@on"), pdf.String("@off")}},
		"on":      rawWidget(pdf.Dict{"Parent": pdf.String("@cb")}),
		"off":     rawWidget(pdf.Dict{"Parent": pdf.String("@cb")}),
	}
}

// inlineFormDict returns the form dictionary for [inlineFormObjects].
func inlineFormDict(refs map[string]pdf.Reference) pdf.Dict {
	return pdf.Dict{
		"Fields": pdf.Array{refs["f"], refs["cb"]},
		"CO":     pdf.Array{refs["f"]},
	}
}

// TestDocumentFormInline checks that a form dictionary stored directly in the
// catalog is read once per extractor, and that the widgets decoded from the
// page share their fields with it.
func TestDocumentFormInline(t *testing.T) {
	r, refs := writeRawDoc(t, inlineFormObjects(), inlineFormOrder, inlineFormDict, true)

	x := pdf.NewExtractor(r)
	c := pdf.CursorAt(x, nil)
	annots := pdf.Array{refs["kid"], refs["orphan1"], refs["orphan2"], refs["on"]}
	_, page, err := PageAnnotations(c, annots)
	if err != nil {
		t.Fatal(err)
	}

	idx, err := DocumentForm(x)
	if err != nil || idx == nil {
		t.Fatalf("document form: %v, %v", idx, err)
	}
	again, err := DocumentForm(x)
	if err != nil || again != idx {
		t.Fatalf("second read gave a different form: %v", err)
	}

	kid := page[0].(*annotation.Widget)
	if kid.Field == nil || idx.FieldRefs[kid.Field] != refs["f"] {
		t.Error("the page widget's field is not the form's")
	}
	if len(idx.Form.CalculationOrder) != 1 || idx.Form.CalculationOrder[0] != kid.Field {
		t.Error("calculation order lost")
	}
	for i := 1; i <= 2; i++ {
		if w := page[i].(*annotation.Widget); w.Field != nil {
			t.Errorf("orphan %d has a field", i)
		}
	}
}

// TestDocumentFormWidgetRefs checks that the index names every widget of the
// form, including one which is on no page.
func TestDocumentFormWidgetRefs(t *testing.T) {
	objs := inlineFormObjects()
	objs["merged"] = rawWidget(pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("m")})
	order := append(slices.Clone(inlineFormOrder), "merged")
	r, refs := writeRawForm(t, objs, order, func(refs map[string]pdf.Reference) pdf.Dict {
		return pdf.Dict{"Fields": pdf.Array{refs["f"], refs["cb"], refs["merged"]}}
	})

	idx, err := DocumentForm(pdf.NewExtractor(r))
	if err != nil {
		t.Fatal(err)
	}
	got := map[pdf.Reference]bool{}
	for f := range idx.FieldRefs {
		for _, w := range f.GetCommon().Widgets {
			ref, ok := idx.WidgetRefs[w]
			if !ok {
				t.Errorf("widget of %s has no reference", f.PartialName())
			}
			got[ref] = true
		}
	}
	want := map[pdf.Reference]bool{refs["kid"]: true, refs["on"]: true, refs["off"]: true, refs["merged"]: true}
	if !maps.Equal(got, want) || len(idx.WidgetRefs) != len(want) {
		t.Errorf("widget refs %v, want %v", got, want)
	}
}

// TestDocumentFormConcurrentWidget checks that a widget decoded while the
// form is being read is returned linked to its field.  The form's walk must
// not publish a widget before linking it, since a decoder which finds the
// widget in the extractor's cache does not wait for the form.
func TestDocumentFormConcurrentWidget(t *testing.T) {
	objs := map[string]pdf.Dict{}
	order := []string{"f"}
	var kids pdf.Array
	for i := range 100 {
		name := fmt.Sprintf("kid%d", i)
		objs[name] = rawWidget(pdf.Dict{"Parent": pdf.String("@f")})
		order = append(order, name)
		kids = append(kids, pdf.String("@"+name))
	}
	objs["f"] = pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("f"), "Kids": kids}
	r, refs := writeRawForm(t, objs, order, func(refs map[string]pdf.Reference) pdf.Dict {
		return pdf.Dict{"Fields": pdf.Array{refs["f"]}}
	})

	for i := range 100 {
		x := pdf.NewExtractor(r)
		var wg sync.WaitGroup
		wg.Go(func() {
			if _, err := DocumentForm(x); err != nil {
				t.Error(err)
			}
		})
		wg.Go(func() {
			// stagger the start, so that some runs meet the walk midway
			for range i * 100 {
				runtime.Gosched()
			}
			a, err := pdf.CursorAt(x, nil).Decode(refs["kid0"], Annotation)
			if err != nil {
				t.Error(err)
				return
			}
			if a.(*annotation.Widget).Field == nil {
				t.Error("widget returned without its field")
			}
		})
		wg.Wait()
	}
}

// TestDocumentFormKeepsFailure checks that a failed read of the form is not
// repeated: later calls return the same error, even where a new read would
// succeed.
func TestDocumentFormKeepsFailure(t *testing.T) {
	r, refs := writeRawForm(t, inlineFormObjects(), inlineFormOrder, inlineFormDict)
	g := &failingGetter{Getter: r, bad: refs["f"]}
	x := pdf.NewExtractor(g)

	_, first := DocumentForm(x)
	if !pdf.IsReadError(first) {
		t.Fatalf("first read: got %v, want a read error", first)
	}
	g.bad = 0
	if idx, err := DocumentForm(x); !errors.Is(err, first) || idx != nil {
		t.Errorf("second read: got %v, %v, want the kept error", idx, err)
	}
	if idx, err := DocumentForm(pdf.NewExtractor(g)); err != nil || idx == nil {
		t.Errorf("new extractor: got %v, %v, want the form", idx, err)
	}
}

// TestUnclaimedMergedWidget checks that a widget merged with a field which
// the form does not keep is read without a field and can be written back.
// The form drops a field of the same name but another type as an earlier
// one, and does not see a field outside /Fields.
func TestUnclaimedMergedWidget(t *testing.T) {
	objs := map[string]pdf.Dict{
		"first":    rawWidget(pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a")}),
		"namesake": rawWidget(pdf.Dict{"FT": pdf.Name("Ch"), "T": pdf.String("a")}),
		"outside":  rawWidget(pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("o")}),
	}
	r, refs := writeRawForm(t, objs, []string{"first", "namesake", "outside"}, func(refs map[string]pdf.Reference) pdf.Dict {
		return pdf.Dict{"Fields": pdf.Array{refs["first"], refs["namesake"]}}
	})
	x := pdf.NewExtractor(r)

	for _, name := range []string{"namesake", "outside"} {
		a, err := pdf.CursorAt(x, nil).Decode(refs[name], Annotation)
		if err != nil {
			t.Fatal(err)
		}
		w := a.(*annotation.Widget)
		if w.Field != nil {
			t.Errorf("%s: widget has field %T, want none", name, w.Field)
		}

		out, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		rm := pdf.NewResourceManager(out)
		if _, err := rm.Store(w); err != nil {
			t.Errorf("%s: store failed: %v", name, err)
		}
		if err := rm.Close(); err != nil {
			t.Errorf("%s: close failed: %v", name, err)
		}
	}
}

// encodeRawForm writes form, with its widgets, to an in-memory file and
// returns the writer together with the dictionaries named by /Fields.
func encodeRawForm(t *testing.T, form *acroform.InteractiveForm) (*pdf.Writer, []pdf.Dict) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	if err := storeWidgets(rm, form.Fields); err != nil {
		t.Fatalf("store widgets: %v", err)
	}
	native, err := form.Encode(rm)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("resource manager close: %v", err)
	}
	return w, rawDicts(t, w, native.(pdf.Dict)["Fields"])
}

// rawDicts reads the dictionaries named by an array of references.
func rawDicts(t *testing.T, w *pdf.Writer, obj pdf.Object) []pdf.Dict {
	t.Helper()
	arr, ok := obj.(pdf.Array)
	if !ok {
		t.Fatalf("not an array: %v", obj)
	}
	dicts := make([]pdf.Dict, len(arr))
	for i, ref := range arr {
		d, err := pdf.NewCursor(w).Dict(ref)
		if err != nil {
			t.Fatalf("read dict %d: %v", i, err)
		}
		dicts[i] = d
	}
	return dicts
}

// radioWithExports returns a radio button field with one widget per export
// value.
func radioWithExports(name string, exports ...string) *acroform.ButtonField {
	f := acroform.NewButtonField(name, acroform.ButtonRadio)
	f.ExportValues = exports
	for i := range exports {
		x := float64(30 * i)
		addWidget(f, x, 0, x+20, 20)
	}
	return f
}

// export values shared by all children are hoisted into their group
func TestEncodeHoistButtonExportValues(t *testing.T) {
	form := &acroform.InteractiveForm{Fields: []acroform.Node{
		&acroform.Group{Name: "g", Children: []acroform.Node{
			radioWithExports("a", "Yes", "No"),
			radioWithExports("b", "Yes", "No"),
		}},
	}}
	w, roots := encodeRawForm(t, form)
	group := roots[0]

	if _, ok := group["Opt"].(pdf.Array); !ok {
		t.Errorf("group /Opt = %v, want hoisted array", group["Opt"])
	}
	for i, kid := range rawDicts(t, w, group["Kids"]) {
		if _, ok := kid["Opt"]; ok {
			t.Errorf("kid %d still carries /Opt", i)
		}
	}
}

// a choice field's /Opt is not inheritable, so a choice field among the
// children prevents hoisting, even when its /Opt array is identical
func TestEncodeChoiceOptNotHoisted(t *testing.T) {
	ch := newChoiceField("b")
	ch.Options = []acroform.ChoiceOption{
		{Export: "Yes", Display: pdf.Text("Yes")},
		{Export: "No", Display: pdf.Text("No")},
	}
	form := &acroform.InteractiveForm{Fields: []acroform.Node{
		&acroform.Group{Name: "g", Children: []acroform.Node{
			radioWithExports("a", "Yes", "No"), ch,
		}},
	}}
	w, roots := encodeRawForm(t, form)
	group := roots[0]

	if _, ok := group["Opt"]; ok {
		t.Error("/Opt was hoisted into the group")
	}
	for i, kid := range rawDicts(t, w, group["Kids"]) {
		if _, ok := kid["Opt"]; !ok {
			t.Errorf("kid %d lost its /Opt", i)
		}
	}
}

// a button field whose export values do not match its widgets cannot be
// written
func TestEncodeExportValuesLength(t *testing.T) {
	f := radioWithExports("r", "Yes", "No")
	f.ExportValues = f.ExportValues[:1]
	form := &acroform.InteractiveForm{Fields: []acroform.Node{f}}

	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	if err := storeWidgets(rm, form.Fields); err != nil {
		t.Fatalf("store widgets: %v", err)
	}
	if _, err := form.Encode(rm); err == nil {
		t.Error("expected an error for mismatched export values")
	}
}

// an explicitly empty ExportValues is valid only for a field without widgets
func TestEncodeEmptyExportValues(t *testing.T) {
	withWidget := radioWithExports("r", "Yes")
	withWidget.ExportValues = []string{}

	noWidgets := acroform.NewButtonField("n", acroform.ButtonRadio)
	noWidgets.ExportValues = []string{}

	tests := []struct {
		name    string
		field   *acroform.ButtonField
		wantErr bool
	}{
		{"with widget", withWidget, true},
		{"no widgets", noWidgets, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			form := &acroform.InteractiveForm{Fields: []acroform.Node{tc.field}}
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			if err := storeWidgets(rm, form.Fields); err != nil {
				t.Fatalf("store widgets: %v", err)
			}
			native, err := form.Encode(rm)
			if (err != nil) != tc.wantErr {
				t.Fatalf("got error %v, want error %t", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if err := rm.Close(); err != nil {
				t.Fatalf("resource manager close: %v", err)
			}
			for _, d := range rawDicts(t, w, native.(pdf.Dict)["Fields"]) {
				if _, ok := d["Opt"]; ok {
					t.Error("empty export values were written as /Opt")
				}
			}
		})
	}
}

// a button field which is off is written without /V and /DV, and one which
// is on with them
func TestEncodeButtonStateOff(t *testing.T) {
	tests := []struct {
		state pdf.Name
		want  pdf.Object
	}{
		{"", nil},
		{"Off", nil},
		{"Yes", pdf.Name("Yes")},
	}
	for _, tc := range tests {
		t.Run(string(tc.state), func(t *testing.T) {
			f := acroform.NewButtonField("c", acroform.ButtonCheckbox)
			f.State, f.DefaultState = tc.state, tc.state
			_, roots := encodeRawForm(t, &acroform.InteractiveForm{Fields: []acroform.Node{f}})
			if got := roots[0]["V"]; got != tc.want {
				t.Errorf("/V = %v, want %v", got, tc.want)
			}
			if got := roots[0]["DV"]; got != tc.want {
				t.Errorf("/DV = %v, want %v", got, tc.want)
			}
		})
	}
}

// a push button and a check box of the same name cannot share a value
func TestShareValuePushButton(t *testing.T) {
	push := acroform.NewPushButtonField("b")
	check := acroform.NewButtonField("b", acroform.ButtonCheckbox)
	check.State = "Yes"

	if shareValue(push, check) {
		t.Error("push button took the value of a check box")
	}
	if shareValue(check, push) {
		t.Error("check box took the value of a push button")
	}
	if !shareValue(push, acroform.NewPushButtonField("b")) {
		t.Error("push buttons of the same name were not shared")
	}
}

// A single-line text field holds no line breaks: each is read as a space,
// in the value and the default value.  A value shared with a single-line
// field loses its line breaks in all fields of the name.
func TestDecodeSingleLineText(t *testing.T) {
	multiline := pdf.Integer(acroform.FieldMultiline)
	objs := map[string]pdf.Dict{
		"one": {"FT": pdf.Name("Tx"), "T": pdf.String("one"),
			"V": pdf.String("a\r\nb\nc\rd"), "DV": pdf.String("x\ny")},
		"multi": {"FT": pdf.Name("Tx"), "T": pdf.String("multi"), "Ff": multiline,
			"V": pdf.String("a\nb")},
		"s1": {"FT": pdf.Name("Tx"), "T": pdf.String("s"), "Ff": multiline,
			"V": pdf.String("p\nq")},
		"s2": {"FT": pdf.Name("Tx"), "T": pdf.String("s")},
		"s3": {"FT": pdf.Name("Tx"), "T": pdf.String("s"), "Ff": multiline},
		"s4": {"FT": pdf.Name("Tx"), "T": pdf.String("s")},
	}
	order := []string{"one", "multi", "s1", "s2", "s3", "s4"}
	r, _ := writeRawForm(t, objs, order, func(refs map[string]pdf.Reference) pdf.Dict {
		var fields pdf.Array
		for _, name := range order {
			fields = append(fields, refs[name])
		}
		return pdf.Dict{"Fields": fields}
	})
	idx, err := DocumentForm(pdf.NewExtractor(r))
	if err != nil {
		t.Fatal(err)
	}
	form := idx.Form

	text := func(v *pdf.StringOrStream) string {
		if v == nil {
			return "<nil>"
		}
		return v.Value.String()
	}
	want := [][2]string{
		{"a b c d", "x y"},
		{"a\nb", "<nil>"},
		{"p q", "<nil>"},
		{"p q", "<nil>"},
		{"p q", "<nil>"},
		{"p q", "<nil>"},
	}
	if len(form.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d", len(form.Fields), len(want))
	}
	for i, node := range form.Fields {
		f := node.(*acroform.TextField)
		if got := [2]string{text(f.Text), text(f.DefaultText)}; got != want[i] {
			t.Errorf("field %d: got %q, want %q", i, got, want[i])
		}
	}
	roundTripForm(t, pdf.V1_7, form)
}
