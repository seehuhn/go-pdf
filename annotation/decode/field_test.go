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
	"fmt"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/opaque"
)

// testDA is a default appearance string for test fields.
const testDA = "/Helv 0 Tf 0 g"

// newTextField returns a new text field with a default appearance.
func newTextField(name string) *acroform.TextField {
	f := acroform.NewTextField(name)
	f.DefaultAppearance = testDA
	return f
}

// newChoiceField returns a new choice field with a default appearance.
func newChoiceField(name string) *acroform.ChoiceField {
	f := acroform.NewChoiceField(name)
	f.DefaultAppearance = testDA
	return f
}

// withAnnotAA attaches an annotation additional-actions dictionary to a widget.
func withAnnotAA(w *annotation.Widget, aa *triggers.Annotation) *annotation.Widget {
	w.AA = aa
	return w
}

// fieldTestCases holds representative field-tree roots to round-trip.
func fieldTestCases() []struct {
	name string
	root acroform.Node
} {
	tx := func(name string, setup ...func(*acroform.TextField)) *acroform.TextField {
		f := newTextField(name)
		for _, s := range setup {
			s(f)
		}
		return f
	}
	btn := func(name string, setup ...func(*acroform.ButtonField)) *acroform.ButtonField {
		f := acroform.NewButtonField(name, acroform.ButtonCheckbox)
		for _, s := range setup {
			s(f)
		}
		return f
	}
	ch := func(name string, setup ...func(*acroform.ChoiceField)) *acroform.ChoiceField {
		f := newChoiceField(name)
		for _, s := range setup {
			s(f)
		}
		return f
	}
	sig := func(name string, setup ...func(*acroform.SignatureField)) *acroform.SignatureField {
		f := acroform.NewSignatureField(name)
		for _, s := range setup {
			s(f)
		}
		return f
	}

	return []struct {
		name string
		root acroform.Node
	}{
		{"minimal text", tx("name")},
		{"flags", tx("locked", func(f *acroform.TextField) {
			f.Flags = acroform.FieldReadOnly | acroform.FieldRequired | acroform.FieldNoExport
		})},
		{"direct values", tx("v", func(f *acroform.TextField) {
			f.Text = &pdf.StringOrStream{Value: pdf.Text("hello")}
			f.DefaultText = &pdf.StringOrStream{Value: pdf.Text("world")}
		})},
		{"stream value", tx("vs", func(f *acroform.TextField) {
			f.Text = &pdf.StringOrStream{Value: pdf.Text("streamed value"), IsStream: true}
		})},
		{"text variable text", tx("vt", func(f *acroform.TextField) {
			f.DefaultAppearance = "/Helv 12 Tf 0 g"
			f.Align = pdf.TextAlignCenter
			f.MaxLen = 24
		})},
		{"comb", tx("comb", func(f *acroform.TextField) { f.Flags = acroform.FieldComb; f.MaxLen = 6 })},
		{"alternate names", ch("choice", func(f *acroform.ChoiceField) { f.AltName = pdf.Text("Choose one"); f.ExportName = "choice_map" })},
		{"choice options", ch("fonts", func(f *acroform.ChoiceField) {
			f.Flags = acroform.FieldCombo
			f.Options = []acroform.ChoiceOption{{Export: "h", Display: pdf.Text("Helvetica")}, {Export: "Times", Display: pdf.Text("Times")}}
			f.Selected = []int{1}
			f.Value = []pdf.TextString{pdf.Text("Times")}
		})},
		{"multi-select", ch("colors", func(f *acroform.ChoiceField) {
			f.Flags = acroform.FieldMultiSelect
			f.Options = []acroform.ChoiceOption{{Export: "r", Display: pdf.Text("Red")}, {Export: "g", Display: pdf.Text("Green")}, {Export: "b", Display: pdf.Text("Blue")}}
			f.Selected = []int{0, 2}
			f.Value = []pdf.TextString{pdf.Text("Red"), pdf.Text("Blue")}
		})},
		{"list box selection", ch("sizes", func(f *acroform.ChoiceField) {
			f.Options = []acroform.ChoiceOption{{Export: "S", Display: pdf.Text("Small")}, {Export: "L", Display: pdf.Text("Large")}}
			f.Selected = []int{1}
			f.Value = []pdf.TextString{pdf.Text("Large")}
		})},
		{"checkbox", btn("agree", func(f *acroform.ButtonField) { f.State = "Yes"; f.DefaultState = "Off" })},
		{"radio with export values", btn("size", func(f *acroform.ButtonField) {
			f.Flags = acroform.FieldRadio
			f.ExportValues = []string{"small", "large"}
			f.State, f.DefaultState = "0", "Off"
			addWidget(f, 0, 0, 20, 20)
			addWidget(f, 30, 0, 50, 20)
		})},
		{"push button", acroform.NewPushButtonField("submit")},
		{"signature", sig("sig")},
		{"signature with lock all", sig("siglockall", func(f *acroform.SignatureField) {
			f.Lock = &acroform.SigFieldLock{Action: acroform.SigFieldLockAll}
		})},
		{"signature with lock include", sig("siglock", func(f *acroform.SignatureField) {
			f.Lock = &acroform.SigFieldLock{Action: acroform.SigFieldLockInclude, Fields: []string{"name", "address"}}
		})},
		{"signature with seed value", sig("sigsv", func(f *acroform.SignatureField) {
			f.SV = &acroform.SigSeedValue{
				Flags:            acroform.SigSeedFilter | acroform.SigSeedSubFilter | acroform.SigSeedReasons,
				Filter:           "Adobe.PPKLite",
				SubFilter:        []pdf.Name{"adbe.pkcs7.detached", "ETSI.CAdES.detached"},
				V:                2,
				Reasons:          []pdf.TextString{pdf.Text("I agree"), pdf.Text("I approve")},
				TimeStamp:        &acroform.SigSeedValueTimeStamp{URL: "https://ts.example.com", Required: true},
				LegalAttestation: []pdf.TextString{pdf.Text("attestation")},
				AddRevInfo:       true,
			}
		})},
		{"additional actions", tx("calc", func(f *acroform.TextField) {
			f.AA = &triggers.Form{Calculate: &action.JavaScript{JS: &pdf.StringOrStream{Value: pdf.Text("event.value = 0;")}}}
		})},
		{"group of sub-fields", &acroform.Group{Name: "address", Children: []acroform.Node{
			newTextField("street"),
			newTextField("zip"),
		}}},
		{"merged widget", func() acroform.Node {
			f := btn("submitW")
			f.State, f.DefaultState = "Off", "Off"
			addWidget(f, 0, 0, 72, 24)
			return f
		}()},
		{"merged widget with mixed AA", func() acroform.Node {
			f := btn("submitAA", func(f *acroform.ButtonField) {
				f.State, f.DefaultState = "Off", "Off"
				f.AA = &triggers.Form{Calculate: &action.JavaScript{JS: &pdf.StringOrStream{Value: pdf.Text("calc();")}}}
			})
			withAnnotAA(addWidget(f, 0, 0, 72, 24),
				&triggers.Annotation{Focus: &action.JavaScript{JS: &pdf.StringOrStream{Value: pdf.Text("focus();")}}})
			return f
		}()},
		{"merged widget field-only AA", func() acroform.Node {
			f := btn("calcOnly", func(f *acroform.ButtonField) {
				f.State, f.DefaultState = "Off", "Off"
				f.AA = &triggers.Form{Calculate: &action.JavaScript{JS: &pdf.StringOrStream{Value: pdf.Text("calc();")}}}
			})
			addWidget(f, 0, 0, 72, 24)
			return f
		}()},
		{"multiple widgets", func() acroform.Node {
			f := btn("color", func(f *acroform.ButtonField) {
				f.Flags = acroform.FieldRadio
				f.ExportValues = []string{"red", "green"}
				f.State, f.DefaultState = "Off", "Off"
			})
			addWidget(f, 0, 0, 20, 20)
			addWidget(f, 30, 0, 50, 20)
			return f
		}()},
	}
}

func TestFieldRoundTrip(t *testing.T) {
	for _, version := range testVersions {
		for _, tc := range fieldTestCases() {
			t.Run(tc.name+"-"+version.String(), func(t *testing.T) {
				got := roundTripRoots(t, version, tc.root)
				if diff := cmp.Diff(snapNodes(t, []acroform.Node{tc.root}), snapNodes(t, got), fieldCmpOptions()...); diff != "" {
					t.Errorf("round trip failed (-want +got):\n%s", diff)
				}
			})
		}
	}
}

// decodeRootField decodes a single field-tree root reference, threading an empty
// inherited context, for tests that build a tree by hand.  Like [readForm], it
// publishes the widgets it links.
func decodeRootField(x *pdf.Extractor, ref pdf.Reference) (acroform.Node, error) {
	d := newFieldTreeDecoder()
	node, err := d.decodeOne(pdf.CursorAt(x, nil), ref, inherited{})
	if err != nil {
		return nil, err
	}
	d.publishWidgets(x)
	return node, nil
}

func TestDecodeFieldNil(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	d := newFieldTreeDecoder()
	node, err := d.decodeNode(pdf.CursorAt(x, nil), nil, inherited{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node != nil {
		t.Errorf("expected nil node for nil object, got %v", node)
	}
}

func TestDecodeFieldKidsSelfCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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

	node, err := decodeRootField(x, ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// the only kid is the self-reference; once it is dropped the node is a
	// childless terminal text field
	if _, ok := node.(*acroform.TextField); !ok {
		t.Fatalf("expected a *acroform.TextField, got %T", node)
	}
}

func TestDecodeFieldKidsMutualCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	refA := w.Alloc()
	refB := w.Alloc()
	if err := w.Put(refA, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "Kids": pdf.Array{refB}}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(refB, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("b"), "Kids": pdf.Array{refA}}); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, refA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A is a group containing B; B's back-reference to A is broken by cycle
	// detection, so B is a childless terminal
	g, ok := node.(*acroform.Group)
	if !ok {
		t.Fatalf("expected a *acroform.Group, got %T", node)
	}
	if len(g.Children) != 1 {
		t.Fatalf("expected one kid, got %d", len(g.Children))
	}
	if _, ok := g.Children[0].(*acroform.TextField); !ok {
		t.Fatalf("expected B to be a terminal field, got %T", g.Children[0])
	}
}

// a partial name read from a file must not contain a period, since it is the
// fully-qualified-name separator. The reader strips any so the field stays
// writable.
func TestDecodeFieldNameStripsPeriod(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.Text("a.b.c")}); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, ref)
	if err != nil {
		t.Fatal(err)
	}
	if name := node.PartialName(); name != "abc" {
		t.Errorf("partial name = %q, want %q", name, "abc")
	}
}

func TestIsWidgetKid(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	subtypeRef := w.Alloc()
	if err := w.Put(subtypeRef, pdf.Name("Widget")); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		dict pdf.Dict
		want bool
	}{
		{"plain widget", pdf.Dict{"Subtype": pdf.Name("Widget")}, true},
		{"unnamed with field type is a widget", pdf.Dict{"Subtype": pdf.Name("Widget"), "FT": pdf.Name("Tx")}, true},
		{"named is a field", pdf.Dict{"Subtype": pdf.Name("Widget"), "T": pdf.String("x")}, false},
		{"has kids is a field", pdf.Dict{"Subtype": pdf.Name("Widget"), "Kids": pdf.Array{}}, false},
		{"no subtype is a field", pdf.Dict{"V": pdf.String("x")}, false},
		{"other subtype is a field", pdf.Dict{"Subtype": pdf.Name("Link")}, false},
		{"indirect subtype is resolved", pdf.Dict{"Subtype": subtypeRef}, true},
		{"null name is absent", pdf.Dict{"Subtype": pdf.Name("Widget"), "T": nil}, true},
	}
	for _, tc := range cases {
		got, err := isWidgetKid(c, tc.dict)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%s: isWidgetKid = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// the field type is inheritable: a sub-field without its own /FT is flattened to
// the inherited concrete type, so its type-specific entries are preserved.
func TestDecodeFieldInheritedType(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	parentRef := w.Alloc()
	kidRef := w.Alloc()
	if err := w.Put(parentRef, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("p"), "Kids": pdf.Array{kidRef}}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(kidRef, pdf.Dict{"T": pdf.String("c"), "V": pdf.String("hello"), "Parent": parentRef}); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, parentRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	g, ok := node.(*acroform.Group)
	if !ok {
		t.Fatalf("expected a *acroform.Group, got %T", node)
	}
	kid, ok := g.Children[0].(*acroform.TextField)
	if !ok {
		t.Fatalf("expected a *acroform.TextField kid, got %T", g.Children[0])
	}
	if kid.Text == nil || kid.Text.Value.String() != "hello" {
		t.Errorf("kid V = %v, want \"hello\"", kid.Text)
	}
}

// /V, /DV and /Opt do not apply to push buttons: a push button reads as a
// PushButtonField, whatever own or inherited entries of this kind it has
func TestDecodePushButtonDropsOpt(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	opt := pdf.Array{pdf.String("Yes")}
	push := pdf.Integer(acroform.FieldPushbutton)
	parentRef := w.Alloc()
	inheritRef := w.Alloc()
	ownRef := w.Alloc()
	if err := w.Put(parentRef, pdf.Dict{
		"FT": pdf.Name("Btn"), "T": pdf.String("p"), "Opt": opt,
		"V": pdf.Name("Yes"), "DV": pdf.Name("Yes"),
		"Kids": pdf.Array{inheritRef, ownRef},
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(inheritRef, pdf.Dict{"T": pdf.String("a"), "Ff": push, "Parent": parentRef}); err != nil {
		t.Fatal(err)
	}
	own := pdf.Dict{
		"T": pdf.String("b"), "Ff": push, "Opt": opt, "Parent": parentRef,
		"V": pdf.Name("On"), "DV": pdf.Name("On"),
	}
	if err := w.Put(ownRef, own); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, parentRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	g, ok := node.(*acroform.Group)
	if !ok {
		t.Fatalf("expected a *acroform.Group, got %T", node)
	}
	for i, child := range g.Children {
		if _, ok := child.(*acroform.PushButtonField); !ok {
			t.Errorf("kid %d: expected a *acroform.PushButtonField, got %T", i, child)
		}
	}
}

// MaxLen is inheritable: a comb field whose MaxLen sits on an ancestor adopts
// the inherited value on decode, keeping the Comb flag valid.
func TestDecodeCombInheritedMaxLen(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	parentRef := w.Alloc()
	kidRef := w.Alloc()
	if err := w.Put(parentRef, pdf.Dict{
		"FT": pdf.Name("Tx"), "T": pdf.String("p"), "MaxLen": pdf.Integer(8), "Kids": pdf.Array{kidRef},
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(kidRef, pdf.Dict{
		"T": pdf.String("c"), "Ff": pdf.Integer(acroform.FieldComb), "Parent": parentRef,
	}); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, parentRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	kid := node.(*acroform.Group).Children[0].(*acroform.TextField)
	if kid.Flags&acroform.FieldComb == 0 {
		t.Error("Comb flag was cleared despite inherited MaxLen")
	}
	if kid.MaxLen != 8 {
		t.Errorf("kid MaxLen = %d, want 8 (inherited)", kid.MaxLen)
	}
}

// the Comb flag may be set only with a MaxLen and with the Multiline, Password
// and FileSelect flags clear; decoding clears an invalid Comb flag.
func TestDecodeCombSnap(t *testing.T) {
	tests := []struct {
		name       string
		ff         acroform.FieldFlags
		maxLen     int
		wantComb   bool
		wantMaxLen int
	}{
		{"valid comb", acroform.FieldComb, 6, true, 6},
		{"zero MaxLen", acroform.FieldComb, 0, false, 0},
		{"conflicting Multiline", acroform.FieldComb | acroform.FieldMultiline, 6, false, 6},
		{"conflicting Password", acroform.FieldComb | acroform.FieldPassword, 6, false, 6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			x := pdf.NewExtractor(w)
			ref := w.Alloc()
			dict := pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("x"), "Ff": pdf.Integer(tc.ff)}
			if tc.maxLen > 0 {
				dict["MaxLen"] = pdf.Integer(tc.maxLen)
			}
			if err := w.Put(ref, dict); err != nil {
				t.Fatal(err)
			}

			node, err := decodeRootField(x, ref)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			tx := node.(*acroform.TextField)
			if got := tx.Flags&acroform.FieldComb != 0; got != tc.wantComb {
				t.Errorf("Comb flag = %t, want %t", got, tc.wantComb)
			}
			if tx.MaxLen != tc.wantMaxLen {
				t.Errorf("MaxLen = %d, want %d", tx.MaxLen, tc.wantMaxLen)
			}
		})
	}
}

// a terminal field whose effective type is unknown is dropped from the tree.
func TestDecodeUnknownTypeDropped(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"T": pdf.String("typeless"), "V": pdf.String("x")}); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, ref)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if node != nil {
		t.Errorf("expected the typeless field to be dropped, got %T", node)
	}
}

// TU/TM and other own entries on a non-terminal field are dropped; only the
// name and the inheritable context survive.
func TestDecodeGroupDropsOwnEntries(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	parentRef := w.Alloc()
	kidRef := w.Alloc()
	if err := w.Put(parentRef, pdf.Dict{
		"FT": pdf.Name("Tx"), "T": pdf.String("p"), "TU": pdf.Text("dropped"),
		"Kids": pdf.Array{kidRef},
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(kidRef, pdf.Dict{"T": pdf.String("c"), "Parent": parentRef}); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, parentRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	g, ok := node.(*acroform.Group)
	if !ok {
		t.Fatalf("expected a *acroform.Group, got %T", node)
	}
	if g.Name != "p" {
		t.Errorf("group name = %q, want p", g.Name)
	}
}

// TestDecodeFieldKidsDeepChainBounded guards against a stack-overflow DoS: a
// /Kids chain of distinct fields is acyclic, so the cycle guard never trips, yet
// recursing one frame per level would exhaust the Go stack. The pdf.Cursor.Decode
// depth cap truncates the over-deep tail; because every level here is a
// non-terminal field (a group), losing the deepest kid empties each group in
// turn, so the whole over-deep chain is dropped — the point is that decoding
// terminates within bounds and never crashes.
func TestDecodeFieldKidsDeepChainBounded(t *testing.T) {
	depth := limits.MaxExtractDepth + 10
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	refs := make([]pdf.Reference, depth)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i, ref := range refs {
		d := pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String(fmt.Sprintf("f%d", i))}
		if i+1 < depth {
			d["Kids"] = pdf.Array{refs[i+1]}
		}
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}

	x := pdf.NewExtractor(w)
	node, err := decodeRootField(x, refs[0])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// the decoded tree, if any, must be depth-bounded
	levels := 0
	for node != nil {
		levels++
		g, ok := node.(*acroform.Group)
		if !ok || len(g.Children) == 0 {
			break
		}
		node = g.Children[0]
	}
	if levels > limits.MaxExtractDepth {
		t.Errorf("chain depth = %d, want at most %d", levels, limits.MaxExtractDepth)
	}
}

// TestDecodeFieldKidsWide verifies that the depth cap bounds nesting depth, not
// sibling breadth: a single field with many kids is read in full.
func TestDecodeFieldKidsWide(t *testing.T) {
	n := 2*limits.MaxExtractDepth + 50
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	kids := make(pdf.Array, n)
	for i := range kids {
		ref := w.Alloc()
		if err := w.Put(ref, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String(fmt.Sprintf("k%d", i))}); err != nil {
			t.Fatal(err)
		}
		kids[i] = ref
	}
	rootRef := w.Alloc()
	if err := w.Put(rootRef, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("root"), "Kids": kids}); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	node, err := decodeRootField(x, rootRef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g, ok := node.(*acroform.Group)
	if !ok {
		t.Fatalf("expected a *acroform.Group, got %T", node)
	}
	if got := len(g.Children); got != n {
		t.Errorf("kids = %d, want %d", got, n)
	}
}

// a choice field's /Opt may mix string entries with [export, display] pairs;
// entries that are neither are skipped rather than decoded as empty options.
func TestDecodeChoiceOptSkipsNonStrings(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	dict := pdf.Dict{
		"FT": pdf.Name("Ch"),
		"T":  pdf.String("choice"),
		"Opt": pdf.Array{
			pdf.String("plain"),
			pdf.Array{pdf.String("exp"), pdf.String("disp")},
			pdf.Integer(42),
			pdf.Array{pdf.Integer(1), pdf.String("x")},
			pdf.String(""),
		},
	}
	if err := w.Put(ref, dict); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ch, ok := node.(*acroform.ChoiceField)
	if !ok {
		t.Fatalf("expected *acroform.ChoiceField, got %T", node)
	}
	want := []acroform.ChoiceOption{
		{Export: "plain", Display: pdf.Text("plain")},
		{Export: "exp", Display: pdf.Text("disp")},
		{Export: "", Display: pdf.Text("")},
	}
	if diff := cmp.Diff(want, ch.Options); diff != "" {
		t.Errorf("Opt mismatch (-want +got):\n%s", diff)
	}
}

func TestDecodeChoiceValueNormalises(t *testing.T) {
	cases := []struct {
		name string
		v    pdf.Object
		want []pdf.TextString
	}{
		{"absent", nil, nil},
		{"single string", pdf.String("one"), []pdf.TextString{pdf.Text("one")}},
		{"length-one array", pdf.Array{pdf.String("one")}, []pdf.TextString{pdf.Text("one")}},
		{"multi array", pdf.Array{pdf.String("a"), pdf.String("b")}, []pdf.TextString{pdf.Text("a"), pdf.Text("b")}},
		{"array skips non-strings", pdf.Array{pdf.String("a"), pdf.Integer(7), pdf.String("b")}, []pdf.TextString{pdf.Text("a"), pdf.Text("b")}},
		{"non-string scalar dropped", pdf.Integer(7), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			x := pdf.NewExtractor(w)

			ref := w.Alloc()
			dict := pdf.Dict{
				"FT":  pdf.Name("Ch"),
				"T":   pdf.String("choice"),
				"Ff":  pdf.Integer(acroform.FieldMultiSelect),
				"Opt": pdf.Array{pdf.String("one"), pdf.String("a"), pdf.String("b")},
			}
			if tc.v != nil {
				dict["V"] = tc.v
			}
			if err := w.Put(ref, dict); err != nil {
				t.Fatal(err)
			}

			node, err := decodeRootField(x, ref)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			ch, ok := node.(*acroform.ChoiceField)
			if !ok {
				t.Fatalf("expected *acroform.ChoiceField, got %T", node)
			}
			if diff := cmp.Diff(tc.want, ch.Value); diff != "" {
				t.Errorf("Value mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// An inheritable entry which is null or malformed reads as though it were
// absent, so the value inherited from the parent stays in effect.
func TestDecodeFieldInheritanceIgnoresInvalid(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	parentRef := w.Alloc()
	kidRef := w.Alloc()
	nullRef := w.Alloc() // never written, so it resolves to null
	parent := pdf.Dict{
		"FT":   pdf.Name("Tx"),
		"T":    pdf.String("p"),
		"Ff":   pdf.Integer(acroform.FieldRequired),
		"DA":   pdf.String("/Helv 12 Tf 0 g"),
		"V":    pdf.String("inherited"),
		"Q":    pdf.Integer(pdf.TextAlignRight),
		"Kids": pdf.Array{kidRef},
	}
	kid := pdf.Dict{
		"T":      pdf.String("c"),
		"Ff":     pdf.Name("bad"),
		"DA":     pdf.Integer(1),
		"V":      nullRef,
		"Q":      nullRef,
		"Parent": parentRef,
	}
	if err := w.Put(parentRef, parent); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(kidRef, kid); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, parentRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	f := node.(*acroform.Group).Children[0].(*acroform.TextField)
	if f.Flags != acroform.FieldRequired {
		t.Errorf("Flags = %v, want %v", f.Flags, acroform.FieldRequired)
	}
	if f.DefaultAppearance != "/Helv 12 Tf 0 g" {
		t.Errorf("DA = %q, want the inherited value", f.DefaultAppearance)
	}
	if f.Text == nil || f.Text.Value.String() != "inherited" {
		t.Errorf("V = %v, want \"inherited\"", f.Text)
	}
	if f.Align != pdf.TextAlignRight {
		t.Errorf("Align = %v, want the inherited value", f.Align)
	}
}

// A text field value which is neither a string nor a stream is dropped,
// rather than read as an empty string.
func TestDecodeTextValueInvalid(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	nullRef := w.Alloc() // never written, so it resolves to null
	dict := pdf.Dict{
		"FT": pdf.Name("Tx"),
		"T":  pdf.String("f"),
		"V":  pdf.Integer(42),
		"DV": nullRef,
		"RV": pdf.Name("x"),
	}
	if err := w.Put(ref, dict); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, ref)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	f := node.(*acroform.TextField)
	if f.Text != nil || f.DefaultText != nil || f.RichText != nil {
		t.Errorf("V, DV, RV = %v, %v, %v, want nil", f.Text, f.DefaultText, f.RichText)
	}
}

// The value of a signature field is kept as an opaque signature dictionary,
// which survives a round trip; any other value is dropped.
func TestDecodeSignatureValue(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	sigRef := w.Alloc()
	sigDict := pdf.Dict{
		"Type":   pdf.Name("Sig"),
		"Filter": pdf.Name("Adobe.PPKLite"),
		"Name":   pdf.Text("A. Signer"),
	}
	if err := w.Put(sigRef, sigDict); err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	dict := pdf.Dict{
		"FT": pdf.Name("Sig"),
		"T":  pdf.String("s"),
		"V":  sigRef,
		"DV": pdf.Integer(1),
	}
	if err := w.Put(ref, dict); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, ref)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	f := node.(*acroform.SignatureField)
	if f.DV != nil {
		t.Errorf("DV = %v, want nil", f.DV)
	}
	if !f.V.Equal(opaque.Direct(sigDict)) {
		t.Errorf("V = %v, want the signature dictionary", f.V)
	}

	// the signature dictionary must be copied into a different output file
	got := roundTripRoots(t, pdf.V1_7, f)[0].(*acroform.SignatureField)
	if !got.V.Equal(opaque.Direct(sigDict)) {
		t.Errorf("round trip V = %v, want the signature dictionary", got.V)
	}
}

// A dictionary without a partial name is not a field: at the root it is
// dropped, and as a kid with the Widget subtype it is a widget of its parent.
func TestDecodeUnnamedField(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	rootRef := w.Alloc()
	if err := w.Put(rootRef, pdf.Dict{"FT": pdf.Name("Tx"), "V": pdf.String("x")}); err != nil {
		t.Fatal(err)
	}
	node, err := decodeRootField(x, rootRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if node != nil {
		t.Errorf("unnamed root field decoded as %T, want nil", node)
	}

	parentRef := w.Alloc()
	kidRef := w.Alloc()
	parent := pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("p"), "Kids": pdf.Array{kidRef}}
	kid := pdf.Dict{
		"Subtype": pdf.Name("Widget"),
		"FT":      pdf.Name("Tx"),
		"Rect":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)},
		"Parent":  parentRef,
	}
	if err := w.Put(parentRef, parent); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(kidRef, kid); err != nil {
		t.Fatal(err)
	}
	node, err = decodeRootField(x, parentRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	f, ok := node.(*acroform.TextField)
	if !ok {
		t.Fatalf("got %T, want *acroform.TextField", node)
	}
	if n := len(f.Widgets); n != 1 {
		t.Errorf("got %d widgets, want 1", n)
	}
}

// A text or choice field without a default appearance gets one, and the value,
// rich text value and default value of a password field are dropped.
func TestDecodeFieldRepairs(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	txRef := w.Alloc()
	tx := pdf.Dict{
		"FT": pdf.Name("Tx"),
		"T":  pdf.String("pw"),
		"Ff": pdf.Integer(acroform.FieldPassword),
		"V":  pdf.String("secret"),
		"DV": pdf.String("default"),
		"RV": pdf.String("<p>secret</p>"),
	}
	chRef := w.Alloc()
	ch := pdf.Dict{"FT": pdf.Name("Ch"), "T": pdf.String("ch")}
	if err := w.Put(txRef, tx); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(chRef, ch); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, txRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	f := node.(*acroform.TextField)
	if f.Text != nil {
		t.Errorf("password V = %v, want nil", f.Text)
	}
	if f.DefaultText != nil {
		t.Errorf("password DV = %v, want nil", f.DefaultText)
	}
	if f.RichText != nil {
		t.Errorf("password RV = %v, want nil", f.RichText)
	}
	if f.DefaultAppearance == "" {
		t.Error("text field without DA was not repaired")
	}

	node, err = decodeRootField(x, chRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if da := node.(*acroform.ChoiceField).DefaultAppearance; da == "" {
		t.Error("choice field without DA was not repaired")
	}
}

// flag combinations the spec forbids are snapped to valid ones on read, so
// that the field stays writable.
func TestDecodeFlagSnap(t *testing.T) {
	tests := []struct {
		name string
		ft   pdf.Name
		ff   acroform.FieldFlags
		want acroform.FieldFlags
	}{
		{"radio and pushbutton", "Btn", acroform.FieldRadio | acroform.FieldPushbutton, acroform.FieldPushbutton},
		{"text flag on button", "Btn", acroform.FieldRadio | acroform.FieldMultiline, acroform.FieldRadio},
		{"edit without combo", "Ch", acroform.FieldEdit | acroform.FieldMultiSelect, acroform.FieldMultiSelect},
		{"reserved bit on text", "Tx", acroform.FieldMultiline | 1<<31, acroform.FieldMultiline},
		{"button flag on signature", "Sig", acroform.FieldReadOnly | acroform.FieldPushbutton, acroform.FieldReadOnly},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			x := pdf.NewExtractor(w)
			ref := w.Alloc()
			dict := pdf.Dict{"FT": tc.ft, "T": pdf.String("x"), "Ff": pdf.Integer(tc.ff), "DA": pdf.String(testDA)}
			if err := w.Put(ref, dict); err != nil {
				t.Fatal(err)
			}

			node, err := decodeRootField(x, ref)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if got := node.(acroform.Field).GetCommon().Flags; got != tc.want {
				t.Errorf("Flags = %#x, want %#x", uint32(got), uint32(tc.want))
			}
		})
	}
}

// selection indices and the top index refer to positions in the /Opt array;
// when an unusable /Opt element is dropped, the indices move with the options
// that remain.
func TestDecodeChoiceIndexRemap(t *testing.T) {
	// elements 1 and 3 are unusable, leaving A, B, C at indices 0, 1, 2
	opt := pdf.Array{
		pdf.String("A"),
		pdf.Integer(42),
		pdf.String("B"),
		pdf.Array{pdf.Integer(1), pdf.String("x")},
		pdf.String("C"),
	}
	tests := []struct {
		name         string
		ff           acroform.FieldFlags
		i            pdf.Array
		ti           pdf.Object
		v            pdf.Object
		wantSelected []int
		wantTopIndex int
	}{
		{"shifted indices", acroform.FieldMultiSelect, pdf.Array{pdf.Integer(2), pdf.Integer(4)}, pdf.Integer(4), nil, []int{1, 2}, 2},
		{"dropped option", acroform.FieldMultiSelect, pdf.Array{pdf.Integer(1), pdf.Integer(2)}, pdf.Integer(1), nil, []int{1}, 0},
		{"out of range", acroform.FieldMultiSelect, pdf.Array{pdf.Integer(0), pdf.Integer(7)}, pdf.Integer(7), nil, []int{0}, 0},
		{"unsorted duplicates", acroform.FieldMultiSelect, pdf.Array{pdf.Integer(4), pdf.Integer(0), pdf.Integer(4)}, nil, nil, []int{0, 2}, 0},
		{"single select drops several", 0, pdf.Array{pdf.Integer(0), pdf.Integer(2)}, nil, nil, nil, 0},
		{"single select keeps duplicates of one", 0, pdf.Array{pdf.Integer(2), pdf.Integer(2)}, nil, nil, []int{1}, 0},
		{"V wins over I", 0, pdf.Array{pdf.Integer(0)}, nil, pdf.String("B"), nil, 0},
		{"V agrees with I", 0, pdf.Array{pdf.Integer(2)}, nil, pdf.String("B"), []int{1}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			x := pdf.NewExtractor(w)
			ref := w.Alloc()
			dict := pdf.Dict{
				"FT":  pdf.Name("Ch"),
				"T":   pdf.String("c"),
				"DA":  pdf.String(testDA),
				"Ff":  pdf.Integer(tc.ff),
				"Opt": opt,
				"I":   tc.i,
			}
			if tc.ti != nil {
				dict["TI"] = tc.ti
			}
			if tc.v != nil {
				dict["V"] = tc.v
			}
			if err := w.Put(ref, dict); err != nil {
				t.Fatal(err)
			}

			node, err := decodeRootField(x, ref)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			ch := node.(*acroform.ChoiceField)
			if diff := cmp.Diff(tc.wantSelected, ch.Selected); diff != "" {
				t.Errorf("Selected mismatch (-want +got):\n%s", diff)
			}
			if ch.TopIndex != tc.wantTopIndex {
				t.Errorf("TopIndex = %d, want %d", ch.TopIndex, tc.wantTopIndex)
			}

			// the repaired field must survive a write-read cycle unchanged
			got := roundTripRoots(t, pdf.V1_7, node)
			if diff := cmp.Diff(snapNodes(t, []acroform.Node{node}), snapNodes(t, got), fieldCmpOptions()...); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

// a single-selection field with several values has no usable value, and
// its /I goes with it; the same holds for several default values
func TestDecodeChoiceSingleSelectValue(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)
	ref := w.Alloc()
	dict := pdf.Dict{
		"FT":  pdf.Name("Ch"),
		"T":   pdf.String("c"),
		"DA":  pdf.String(testDA),
		"Opt": pdf.Array{pdf.String("A"), pdf.String("B")},
		"V":   pdf.Array{pdf.String("B"), pdf.String("A")},
		"I":   pdf.Array{pdf.Integer(1)},
		"DV":  pdf.Array{pdf.String("A"), pdf.String("B")},
	}
	if err := w.Put(ref, dict); err != nil {
		t.Fatal(err)
	}
	node, err := decodeRootField(x, ref)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	ch := node.(*acroform.ChoiceField)
	if ch.Value != nil || ch.Selected != nil {
		t.Errorf("V, Selected = %q, %v, want nil, nil", ch.Value, ch.Selected)
	}
	if ch.DefaultValue != nil {
		t.Errorf("DV = %q, want nil", ch.DefaultValue)
	}
}

// /V need not list the selections in the order of /I
func TestDecodeChoiceValueOrder(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)
	ref := w.Alloc()
	dict := pdf.Dict{
		"FT":  pdf.Name("Ch"),
		"Ff":  pdf.Integer(acroform.FieldMultiSelect),
		"T":   pdf.String("c"),
		"DA":  pdf.String(testDA),
		"Opt": pdf.Array{pdf.String("A"), pdf.String("B")},
		"V":   pdf.Array{pdf.String("B"), pdf.String("A")},
		"I":   pdf.Array{pdf.Integer(0), pdf.Integer(1)},
	}
	if err := w.Put(ref, dict); err != nil {
		t.Fatal(err)
	}
	node, err := decodeRootField(x, ref)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	ch := node.(*acroform.ChoiceField)
	if diff := cmp.Diff([]pdf.TextString{pdf.Text("B"), pdf.Text("A")}, ch.Value); diff != "" {
		t.Errorf("V mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]int{0, 1}, ch.Selected); diff != "" {
		t.Errorf("Selected mismatch (-want +got):\n%s", diff)
	}
}

// countingGetter counts the indirect objects loaded through it.
type countingGetter struct {
	pdf.Getter
	loads int
}

func (g *countingGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	g.loads++
	return g.Getter.Get(ref, canObjStm)
}

// parentChain writes a chain of depth check box field dictionaries linked by
// /Parent and returns their references, the root first.  The root holds the
// value /Yes, which the widgets below the chain inherit.
func parentChain(t *testing.T, w *pdf.Writer, depth int) []pdf.Reference {
	t.Helper()
	refs := make([]pdf.Reference, depth)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i, ref := range refs {
		d := pdf.Dict{"FT": pdf.Name("Btn"), "T": pdf.String(fmt.Sprintf("p%d", i))}
		if i == 0 {
			d["V"] = pdf.Name("Yes")
		} else {
			d["Parent"] = refs[i-1]
		}
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}
	return refs
}

// checkBoxAppearance writes an appearance dictionary with the states /Yes
// and /Off.
func checkBoxAppearance(t *testing.T, w *pdf.Writer) pdf.Reference {
	t.Helper()
	rect := pdf.Rectangle{URx: 10, URy: 10}
	ref := w.Alloc()
	err := w.Put(ref, pdf.Dict{"N": pdf.Dict{
		"Yes": appearanceStream(t, w, rect),
		"Off": appearanceStream(t, w, rect),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// mergedWidget returns a check box widget merged with a field below parent,
// which names no appearance state, so that decoding it takes the state from
// the inherited value.
func mergedWidget(parent, ap pdf.Reference, name string) pdf.Dict {
	return pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
		"Rect": &pdf.Rectangle{URx: 10, URy: 10},
		"T":    pdf.String(name), "Parent": parent, "AP": ap,
	}
}

// Many widgets read without their field and sharing one /Parent chain load
// each ancestor once, not once per widget.
func TestDecodeParentChainShared(t *testing.T) {
	const depth, numWidgets = 50, 50
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	chain := parentChain(t, w, depth)
	ap := checkBoxAppearance(t, w)
	widgets := make([]pdf.Reference, numWidgets)
	for i := range widgets {
		widgets[i] = w.Alloc()
		if err := w.Put(widgets[i], mergedWidget(chain[depth-1], ap, fmt.Sprintf("w%d", i))); err != nil {
			t.Fatal(err)
		}
	}

	g := &countingGetter{Getter: w}
	x := pdf.NewExtractor(g)
	for _, ref := range widgets {
		a, err := pdf.CursorAt(x, nil).Decode(ref, Annotation)
		if err != nil {
			t.Fatal(err)
		}
		if as := a.GetCommon().AppearanceState; as != "Yes" {
			t.Errorf("appearance state = %q, want %q", as, "Yes")
		}
	}
	// a few loads per widget and per ancestor, far below the product
	if limit := 4 * (depth + numWidgets); g.loads > limit {
		t.Errorf("loaded %d objects, want at most %d", g.loads, limit)
	}
}

// A /Parent chain deeper than the extract limit is cut off rather than
// followed to the end, so the value at its root is not seen.
func TestDecodeParentChainDeepBounded(t *testing.T) {
	depth := limits.MaxExtractDepth + 10
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	chain := parentChain(t, w, depth)
	ref := w.Alloc()
	if err := w.Put(ref, mergedWidget(chain[depth-1], checkBoxAppearance(t, w), "w")); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	a, err := pdf.CursorAt(x, nil).Decode(ref, Annotation)
	if err != nil {
		t.Fatal(err)
	}
	if as := a.GetCommon().AppearanceState; as != "Off" {
		t.Errorf("appearance state = %q, want %q", as, "Off")
	}
}

// A /Parent cycle terminates, and the value of the field below it is seen.
func TestDecodeParentChainCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	a, b := w.Alloc(), w.Alloc()
	if err := w.Put(a, pdf.Dict{"FT": pdf.Name("Btn"), "T": pdf.String("a"), "V": pdf.Name("Yes"), "Parent": b}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(b, pdf.Dict{"T": pdf.String("b"), "Parent": a}); err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, mergedWidget(a, checkBoxAppearance(t, w), "w")); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	an, err := pdf.CursorAt(x, nil).Decode(ref, Annotation)
	if err != nil {
		t.Fatal(err)
	}
	if as := an.GetCommon().AppearanceState; as != "Yes" {
		t.Errorf("appearance state = %q, want %q", as, "Yes")
	}
}

// TestApplyOwnContextDanglingOpt checks that a reference to a nonexistent
// object in /Opt leaves the inherited value in effect.
func TestApplyOwnContextDanglingOpt(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	parent := inherited{opt: pdf.Array{pdf.String("a"), pdf.String("b")}}

	want, err := applyOwnContext(parent, c, pdf.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := applyOwnContext(parent, c, pdf.Dict{"Opt": pdf.NewReference(99, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want.opt, got.opt); diff != "" {
		t.Errorf("dangling reference read differently from absent entry (-absent +dangling):\n%s", diff)
	}
}

func TestDecodeChoiceExplicitEmpty(t *testing.T) {
	multi := pdf.Integer(acroform.FieldMultiSelect)
	opt := pdf.Array{pdf.String("A"), pdf.String("B")}
	cases := []struct {
		name     string
		ff       pdf.Integer
		v, dv, i pdf.Object
		wantV    []pdf.TextString
		wantDV   []pdf.TextString
		wantSel  []int
	}{
		{"multi empty V", multi, pdf.Array{}, nil, nil, []pdf.TextString{}, nil, nil},
		{"multi empty DV", multi, nil, pdf.Array{}, nil, nil, []pdf.TextString{}, nil},
		{"multi junk array", multi, pdf.Array{pdf.Integer(1)}, pdf.Array{pdf.Integer(1)}, nil, nil, nil, nil},
		{"multi empty V drops I", multi, pdf.Array{}, nil, pdf.Array{pdf.Integer(1)}, []pdf.TextString{}, nil, nil},
		{"multi absent V uses I", multi, nil, nil, pdf.Array{pdf.Integer(1)}, []pdf.TextString{pdf.Text("B")}, nil, []int{1}},
		{"single empty V", 0, pdf.Array{}, pdf.Array{}, nil, nil, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			x := pdf.NewExtractor(w)
			ref := w.Alloc()
			dict := pdf.Dict{
				"FT": pdf.Name("Ch"), "T": pdf.String("c"), "DA": pdf.String(testDA),
				"Ff": tc.ff, "Opt": opt,
			}
			for k, v := range map[pdf.Name]pdf.Object{"V": tc.v, "DV": tc.dv, "I": tc.i} {
				if v != nil {
					dict[k] = v
				}
			}
			if err := w.Put(ref, dict); err != nil {
				t.Fatal(err)
			}
			node, err := decodeRootField(x, ref)
			if err != nil {
				t.Fatal(err)
			}
			ch := node.(*acroform.ChoiceField)
			if diff := cmp.Diff(tc.wantV, ch.Value); diff != "" {
				t.Errorf("V differs (-want +got):\n%s", diff)
			}
			if (ch.Value == nil) != (tc.wantV == nil) {
				t.Errorf("V nil-ness: got %#v, want %#v", ch.Value, tc.wantV)
			}
			if diff := cmp.Diff(tc.wantDV, ch.DefaultValue); diff != "" {
				t.Errorf("DV differs (-want +got):\n%s", diff)
			}
			if (ch.DefaultValue == nil) != (tc.wantDV == nil) {
				t.Errorf("DV nil-ness: got %#v, want %#v", ch.DefaultValue, tc.wantDV)
			}
			if diff := cmp.Diff(tc.wantSel, ch.Selected); diff != "" {
				t.Errorf("Selected differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChoiceExplicitEmptyDVRoundTrip(t *testing.T) {
	f := newChoiceField("c")
	f.Flags |= acroform.FieldMultiSelect
	f.Value = []pdf.TextString{}
	f.DefaultValue = []pdf.TextString{}
	first := roundTripForm(t, pdf.V1_7, &acroform.InteractiveForm{Fields: []acroform.Node{f}})
	got := first.Fields[0].(*acroform.ChoiceField)
	if got.Value == nil || got.DefaultValue == nil {
		t.Errorf("V, DV = %#v, %#v, want non-nil empty", got.Value, got.DefaultValue)
	}
}

// A choice value must name options, except in an editable combo box.  A
// multiple selection keeps the entries which do; otherwise the value is
// dropped, and /I, if present, takes its place.
func TestDecodeChoiceValueNamesOptions(t *testing.T) {
	str := func(ss ...string) pdf.Array {
		a := pdf.Array{}
		for _, s := range ss {
			a = append(a, pdf.String(s))
		}
		return a
	}
	text := func(ss ...string) []pdf.TextString {
		var v []pdf.TextString
		for _, s := range ss {
			v = append(v, pdf.Text(s))
		}
		return v
	}
	multi := pdf.Integer(acroform.FieldMultiSelect)
	editable := pdf.Integer(acroform.FieldCombo | acroform.FieldEdit)
	cases := []struct {
		name          string
		extra         pdf.Dict
		value, defVal []pdf.TextString
	}{
		{"single, no option", pdf.Dict{"V": pdf.String("Z"), "DV": pdf.String("Y")}, nil, nil},
		{"single, /I restores", pdf.Dict{"V": pdf.String("Z"), "I": pdf.Array{pdf.Integer(1)}}, text("B"), nil},
		{"multi, keeps options", pdf.Dict{"Ff": multi, "V": str("A", "Z", "C"), "DV": str("Y", "B")},
			text("A", "C"), text("B")},
		{"multi, none left", pdf.Dict{"Ff": multi, "V": str("Z", "Y")}, nil, nil},
		{"multi, explicit empty", pdf.Dict{"Ff": multi, "V": pdf.Array{}}, []pdf.TextString{}, nil},
		{"editable combo", pdf.Dict{"Ff": editable, "V": pdf.String("Z"), "DV": pdf.String("Y")},
			text("Z"), text("Y")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			ref := w.Alloc()
			dict := pdf.Dict{"FT": pdf.Name("Ch"), "T": pdf.String("c"), "Opt": str("A", "B", "C")}
			maps.Copy(dict, tc.extra)
			if err := w.Put(ref, dict); err != nil {
				t.Fatal(err)
			}
			node, err := decodeRootField(pdf.NewExtractor(w), ref)
			if err != nil {
				t.Fatal(err)
			}
			ch := node.(*acroform.ChoiceField)
			if diff := cmp.Diff(tc.value, ch.Value); diff != "" {
				t.Errorf("Value mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.defVal, ch.DefaultValue); diff != "" {
				t.Errorf("DefaultValue mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
