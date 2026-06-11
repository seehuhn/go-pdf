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
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
)

// withAnnotAA attaches an annotation additional-actions dictionary to a widget.
func withAnnotAA(w *annotation.Widget, aa *triggers.Annotation) *annotation.Widget {
	w.AA = aa
	return w
}

// fieldTestCases holds representative field-tree roots to round-trip.
func fieldTestCases() []struct {
	name string
	root acroform.TreeNode
} {
	tx := func(name string, setup ...func(*acroform.FieldTx)) *acroform.FieldTx {
		f := acroform.NewTextField(name)
		for _, s := range setup {
			s(f)
		}
		return f
	}
	btn := func(name string, setup ...func(*acroform.FieldBtn)) *acroform.FieldBtn {
		f := acroform.NewButtonField(name)
		for _, s := range setup {
			s(f)
		}
		return f
	}
	ch := func(name string, setup ...func(*acroform.FieldChoice)) *acroform.FieldChoice {
		f := acroform.NewChoiceField(name)
		for _, s := range setup {
			s(f)
		}
		return f
	}
	sig := func(name string, setup ...func(*acroform.FieldSig)) *acroform.FieldSig {
		f := acroform.NewSignatureField(name)
		for _, s := range setup {
			s(f)
		}
		return f
	}

	return []struct {
		name string
		root acroform.TreeNode
	}{
		{"minimal text", tx("name")},
		{"flags", tx("locked", func(f *acroform.FieldTx) {
			f.Ff = acroform.FieldReadOnly | acroform.FieldRequired | acroform.FieldNoExport
		})},
		{"direct values", tx("v", func(f *acroform.FieldTx) {
			f.V = pdf.String("hello")
			f.DV = pdf.String("world")
		})},
		{"reference value", tx("vr", func(f *acroform.FieldTx) { f.V = pdf.NewReference(100, 0) })},
		{"text variable text", tx("vt", func(f *acroform.FieldTx) {
			f.DefaultAppearance = "/Helv 12 Tf 0 g"
			f.Align = pdf.TextAlignCenter
			f.MaxLen = 24
		})},
		{"comb", tx("comb", func(f *acroform.FieldTx) { f.Ff = acroform.FieldComb; f.MaxLen = 6 })},
		{"alternate names", ch("choice", func(f *acroform.FieldChoice) { f.TU = "Choose one"; f.TM = "choice_map" })},
		{"choice options", ch("fonts", func(f *acroform.FieldChoice) {
			f.Ff = acroform.FieldCombo
			f.Opt = []acroform.ChoiceOption{{Export: "h", Display: "Helvetica"}, {Export: "Times", Display: "Times"}}
			f.Selected = []int{1}
			f.V = pdf.String("Times")
		})},
		{"checkbox", btn("agree", func(f *acroform.FieldBtn) { f.V = "Yes"; f.DV = "Off" })},
		{"radio with export values", btn("size", func(f *acroform.FieldBtn) {
			f.Ff = acroform.FieldRadio
			f.Opt = []string{"small", "large"}
			f.V = "small"
		})},
		{"push button", btn("submit", func(f *acroform.FieldBtn) { f.Ff = acroform.FieldPushbutton })},
		{"signature", sig("sig")},
		{"signature with lock all", sig("siglockall", func(f *acroform.FieldSig) {
			f.Lock = &acroform.SigFieldLock{Action: acroform.SigFieldLockAll}
		})},
		{"signature with lock include", sig("siglock", func(f *acroform.FieldSig) {
			f.Lock = &acroform.SigFieldLock{Action: acroform.SigFieldLockInclude, Fields: []string{"name", "address"}}
		})},
		{"signature with seed value", sig("sigsv", func(f *acroform.FieldSig) {
			f.SV = &acroform.SigSeedValue{
				Flags:            acroform.SigSeedFilter | acroform.SigSeedSubFilter | acroform.SigSeedReasons,
				Filter:           "Adobe.PPKLite",
				SubFilter:        []pdf.Name{"adbe.pkcs7.detached", "ETSI.CAdES.detached"},
				V:                2,
				Reasons:          []string{"I agree", "I approve"},
				TimeStamp:        &acroform.SigSeedValueTimeStamp{URL: "https://ts.example.com", Required: true},
				LegalAttestation: []string{"attestation"},
				AddRevInfo:       true,
			}
		})},
		{"additional actions", tx("calc", func(f *acroform.FieldTx) {
			f.AA = &triggers.Form{Calculate: &action.JavaScript{JS: pdf.String("event.value = 0;")}}
		})},
		{"group of sub-fields", &acroform.Group{Name: "address", Kids: []acroform.TreeNode{
			acroform.NewTextField("street"),
			acroform.NewTextField("zip"),
		}}},
		{"merged widget", func() acroform.TreeNode {
			f := btn("submitW")
			addWidget(f, 0, 0, 72, 24)
			return f
		}()},
		{"merged widget with mixed AA", func() acroform.TreeNode {
			f := btn("submitAA", func(f *acroform.FieldBtn) {
				f.AA = &triggers.Form{Calculate: &action.JavaScript{JS: pdf.String("calc();")}}
			})
			withAnnotAA(addWidget(f, 0, 0, 72, 24),
				&triggers.Annotation{Focus: &action.JavaScript{JS: pdf.String("focus();")}})
			return f
		}()},
		{"merged widget field-only AA", func() acroform.TreeNode {
			f := btn("calcOnly", func(f *acroform.FieldBtn) {
				f.AA = &triggers.Form{Calculate: &action.JavaScript{JS: pdf.String("calc();")}}
			})
			addWidget(f, 0, 0, 72, 24)
			return f
		}()},
		{"multiple widgets", func() acroform.TreeNode {
			f := btn("color", func(f *acroform.FieldBtn) {
				f.Ff = acroform.FieldRadio
				f.Opt = []string{"red", "green"}
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
				if diff := cmp.Diff(snapNodes([]acroform.TreeNode{tc.root}), snapNodes(got), fieldCmpOptions()...); diff != "" {
					t.Errorf("round trip failed (-want +got):\n%s", diff)
				}
			})
		}
	}
}

// decodeRootField decodes a single field-tree root reference, threading an empty
// inherited context, for tests that build a tree by hand.
func decodeRootField(x *pdf.Extractor, ref pdf.Reference) (acroform.TreeNode, error) {
	d := newFieldTreeDecoder()
	res, err := pdf.ExtractorGetOptional(x, nil, ref, d.nodeFunc(inherited{}))
	if err != nil || res == nil {
		return nil, err
	}
	return res.node, nil
}

func TestDecodeFieldNil(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	d := newFieldTreeDecoder()
	node, err := d.decodeNode(x, nil, nil, inherited{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node != nil {
		t.Errorf("expected nil node for nil object, got %v", node)
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

	node, err := decodeRootField(x, ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// the only kid is the self-reference; once it is dropped the node is a
	// childless terminal text field
	if _, ok := node.(*acroform.FieldTx); !ok {
		t.Fatalf("expected a *acroform.FieldTx, got %T", node)
	}
}

func TestDecodeFieldKidsMutualCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
	if len(g.Kids) != 1 {
		t.Fatalf("expected one kid, got %d", len(g.Kids))
	}
	if _, ok := g.Kids[0].(*acroform.FieldTx); !ok {
		t.Fatalf("expected B to be a terminal field, got %T", g.Kids[0])
	}
}

// a partial name read from a file must not contain a period, since it is the
// fully-qualified-name separator. The reader strips any so the field stays
// writable.
func TestDecodeFieldNameStripsPeriod(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.TextString("a.b.c")}); err != nil {
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

// the field type is inheritable: a sub-field without its own /FT is flattened to
// the inherited concrete type, so its type-specific entries are preserved.
func TestDecodeFieldInheritedType(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
	kid, ok := g.Kids[0].(*acroform.FieldTx)
	if !ok {
		t.Fatalf("expected a *acroform.FieldTx kid, got %T", g.Kids[0])
	}
	if v, ok := kid.V.(pdf.String); !ok || string(v) != "hello" {
		t.Errorf("kid V = %v, want \"hello\"", kid.V)
	}
}

// MaxLen is inheritable: a comb field whose MaxLen sits on an ancestor adopts
// the inherited value on decode, keeping the Comb flag valid.
func TestDecodeCombInheritedMaxLen(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
	kid := node.(*acroform.Group).Kids[0].(*acroform.FieldTx)
	if kid.Ff&acroform.FieldComb == 0 {
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
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
			tx := node.(*acroform.FieldTx)
			if got := tx.Ff&acroform.FieldComb != 0; got != tc.wantComb {
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
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	parentRef := w.Alloc()
	kidRef := w.Alloc()
	if err := w.Put(parentRef, pdf.Dict{
		"FT": pdf.Name("Tx"), "T": pdf.String("p"), "TU": pdf.TextString("dropped"),
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
// recursing one frame per level would exhaust the Go stack. The ExtractorGet
// depth cap truncates the over-deep tail; because every level here is a
// non-terminal field (a group), losing the deepest kid empties each group in
// turn, so the whole over-deep chain is dropped — the point is that decoding
// terminates within bounds and never crashes.
func TestDecodeFieldKidsDeepChainBounded(t *testing.T) {
	depth := limits.MaxExtractDepth + 10
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

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
		if !ok || len(g.Kids) == 0 {
			break
		}
		node = g.Kids[0]
	}
	if levels > limits.MaxExtractDepth {
		t.Errorf("chain depth = %d, want at most %d", levels, limits.MaxExtractDepth)
	}
}

// TestDecodeFieldKidsWide verifies that the depth cap bounds nesting depth, not
// sibling breadth: a single field with many kids is read in full.
func TestDecodeFieldKidsWide(t *testing.T) {
	n := 2*limits.MaxExtractDepth + 50
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

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
	if got := len(g.Kids); got != n {
		t.Errorf("kids = %d, want %d", got, n)
	}
}

// a choice field's /Opt may mix string entries with [export, display] pairs;
// entries that are neither are skipped rather than decoded as empty options.
func TestDecodeChoiceOptSkipsNonStrings(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
	ch, ok := node.(*acroform.FieldChoice)
	if !ok {
		t.Fatalf("expected *acroform.FieldChoice, got %T", node)
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
