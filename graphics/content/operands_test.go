// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

func TestOperands(t *testing.T) {
	type N = pdf.Name
	type I = pdf.Integer
	type S = pdf.String
	res := &Resources{
		Font:       map[pdf.Name]font.Instance{"F1": &stubFont{name: "F1"}},
		ColorSpace: map[pdf.Name]color.Space{"CS1": color.SpaceDeviceRGB},
	}
	cases := []struct {
		name     string
		fill     color.Color // fill colour, if non-nil
		op       OpName
		args     []pdf.Object
		wantOp   OpName
		wantArgs []pdf.Object
		wantOK   bool
	}{
		{"no operands", nil, OpTextBegin, []pdf.Object{I(1)}, OpTextBegin, []pdf.Object{}, true},
		{"number", nil, OpSetLineWidth, []pdf.Object{pdf.Real(2.5)}, OpSetLineWidth, []pdf.Object{pdf.Real(2.5)}, true},
		{"surplus numbers", nil, OpMoveTo, []pdf.Object{I(1), I(2), I(3)}, OpMoveTo, []pdf.Object{I(1), I(2)}, true},
		{"missing number", nil, OpLineTo, []pdf.Object{I(1)}, OpLineTo, nil, false},
		{"wrong kind", nil, OpSetLineWidth, []pdf.Object{N("x")}, OpSetLineWidth, nil, false},
		{"name", nil, OpXObject, []pdf.Object{N("Im1"), N("Im2")}, OpXObject, []pdf.Object{N("Im1")}, true},
		{"string", nil, OpTextShow, []pdf.Object{S("a"), S("b")}, OpTextShow, []pdf.Object{S("a")}, true},
		{"quote", nil, OpTextShowMoveNextLineSetSpacing, []pdf.Object{I(1), I(2), S("a")}, OpTextShowMoveNextLineSetSpacing, []pdf.Object{I(1), I(2), S("a")}, true},
		{"quote without string", nil, OpTextShowMoveNextLineSetSpacing, []pdf.Object{I(1), I(2)}, OpTextShowMoveNextLineSetSpacing, nil, false},

		{"known font", nil, OpTextSetFont, []pdf.Object{N("F1"), I(12), I(3)}, OpTextSetFont, []pdf.Object{N("F1"), I(12)}, true},
		{"unknown font", nil, OpTextSetFont, []pdf.Object{N("F2"), I(12)}, OpTextSetFont, nil, false},
		{"device colour space", nil, OpSetFillColorSpace, []pdf.Object{N("DeviceCMYK")}, OpSetFillColorSpace, []pdf.Object{N("DeviceCMYK")}, true},
		{"resource colour space", nil, OpSetStrokeColorSpace, []pdf.Object{N("CS1")}, OpSetStrokeColorSpace, []pdf.Object{N("CS1")}, true},
		{"unknown colour space", nil, OpSetFillColorSpace, []pdf.Object{N("CS2")}, OpSetFillColorSpace, nil, false},

		{"dash", nil, OpSetLineDash, []pdf.Object{pdf.Array{I(3), I(2)}, I(0)}, OpSetLineDash, []pdf.Object{pdf.Array{I(3), I(2)}, I(0)}, true},
		{"dash with name", nil, OpSetLineDash, []pdf.Object{pdf.Array{I(3), N("x")}, I(0)}, OpSetLineDash, nil, false},

		{"colour surplus numbers", color.DeviceRGB{}, OpSetFillColor, []pdf.Object{I(1), I(0), I(0), I(1)}, OpSetFillColor, []pdf.Object{I(1), I(0), I(0)}, true},
		{"colour without name", color.DeviceGray(0), OpSetFillColor, []pdf.Object{pdf.Real(0.5), N("P1")}, OpSetFillColor, []pdf.Object{pdf.Real(0.5)}, true},
		{"colour last name", color.DeviceGray(0), OpSetFillColorN, []pdf.Object{N("P1"), pdf.Real(0.5), N("P2")}, OpSetFillColorN, []pdf.Object{pdf.Real(0.5), N("P2")}, true},
		{"colour name moved", color.DeviceGray(0), OpSetFillColorN, []pdf.Object{N("P1"), pdf.Real(0.5)}, OpSetFillColorN, []pdf.Object{pdf.Real(0.5), N("P1")}, true},
		{"pattern", color.SpacePatternColored.Default(), OpSetFillColorN, []pdf.Object{I(1), N("P1")}, OpSetFillColorN, []pdf.Object{N("P1")}, true},
		{"uncoloured pattern", color.SpacePatternUncolored{Base: color.SpaceDeviceRGB}.Default(), OpSetFillColorN, []pdf.Object{N("P1"), I(1), I(0), I(1), I(0)}, OpSetFillColorN, []pdf.Object{I(1), I(0), I(1), N("P1")}, true},

		{"TJ", nil, OpTextShowArray, []pdf.Object{pdf.Array{S("a"), I(-10), S("b")}}, OpTextShowArray, []pdf.Object{pdf.Array{S("a"), I(-10), S("b")}}, true},
		{"TJ with junk", nil, OpTextShowArray, []pdf.Object{pdf.Array{S("a"), pdf.Array{S("x")}, N("n"), S("b")}}, OpTextShowArray, []pdf.Object{pdf.Array{S("a"), S("b")}}, true},
		{"TJ without array", nil, OpTextShowArray, []pdf.Object{S("a")}, OpTextShowArray, nil, false},

		{"BDC dict", nil, OpBeginMarkedContentWithProperties, []pdf.Object{N("Span"), pdf.Dict{"MCID": I(0)}, I(1)}, OpBeginMarkedContentWithProperties, []pdf.Object{N("Span"), pdf.Dict{"MCID": I(0)}}, true},
		{"BDC name", nil, OpBeginMarkedContentWithProperties, []pdf.Object{N("Span"), N("P0")}, OpBeginMarkedContentWithProperties, []pdf.Object{N("Span"), N("P0")}, true},
		{"BDC as BMC", nil, OpBeginMarkedContentWithProperties, []pdf.Object{N("Span"), I(7)}, OpBeginMarkedContent, []pdf.Object{N("Span")}, true},
		{"DP as MP", nil, OpMarkedContentPointWithProperties, []pdf.Object{N("Tag")}, OpMarkedContentPoint, []pdf.Object{N("Tag")}, true},
		{"BDC without tag", nil, OpBeginMarkedContentWithProperties, []pdf.Object{pdf.Dict{}}, OpBeginMarkedContent, nil, false},

		{"unknown operator", nil, "foo", []pdf.Object{I(1), N("x")}, "foo", []pdf.Object{I(1), N("x")}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NewState(Page, res)
			if c.fill != nil {
				s.GState.FillColor = c.fill
			}
			op, used, _, ok := s.Operands(c.op, c.args)
			if ok != c.wantOK {
				t.Fatalf("got ok=%t, want %t", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if op != c.wantOp {
				t.Errorf("got operator %q, want %q", op, c.wantOp)
			}
			if diff := cmp.Diff(c.wantArgs, used); diff != "" {
				t.Errorf("wrong operands (-want +got):\n%s", diff)
			}

			// the operands which Operands returns are kept as they are
			op2, used2, same2, ok2 := s.Operands(op, used)
			if !ok2 || !same2 || op2 != op || len(used2) != len(used) {
				t.Error("operands are not stable")
			}
			if diff := cmp.Diff(used, used2); diff != "" {
				t.Errorf("operands changed (-want +got):\n%s", diff)
			}
		})
	}
}

// TestOperandsSame checks the result same of [State.Operands].
func TestOperandsSame(t *testing.T) {
	s := NewState(Page, nil)
	s.GState.FillColor = color.DeviceGray(0)

	args := []pdf.Object{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)}
	op, used, same, ok := s.Operands(OpMoveTo, args)
	if !ok || !same || op != OpMoveTo {
		t.Fatalf("got op=%q same=%t ok=%t", op, same, ok)
	}
	if diff := cmp.Diff(args[:2], used); diff != "" {
		t.Errorf("wrong operands (-want +got):\n%s", diff)
	}

	// operands which are rearranged are reported as changed
	args = []pdf.Object{pdf.Name("P1"), pdf.Real(0.5)}
	_, used, same, _ = s.Operands(OpSetFillColorN, args)
	if same {
		t.Error("rearranged operands reported as same")
	}
	want := []pdf.Object{pdf.Real(0.5), pdf.Name("P1")}
	if diff := cmp.Diff(want, used); diff != "" {
		t.Errorf("wrong operands (-want +got):\n%s", diff)
	}
	used[0] = pdf.Integer(0)
	if args[0] != pdf.Name("P1") {
		t.Error("rearranged operands share storage with args")
	}

	// an operator which is read as another one is reported as changed
	_, _, same, _ = s.Operands(OpBeginMarkedContentWithProperties,
		[]pdf.Object{pdf.Name("Span"), pdf.Integer(7)})
	if same {
		t.Error("renamed operator reported as same")
	}
}

// TestApplyStateChangesRejected checks that an operator which [State.Operands]
// rejects leaves the state unchanged.
func TestApplyStateChangesRejected(t *testing.T) {
	cases := []struct {
		op   OpName
		args []pdf.Object
		bit  graphics.Bits
	}{
		{OpSetLineWidth, []pdf.Object{pdf.Name("x")}, graphics.StateLineWidth},
		{OpSetFillColorSpace, []pdf.Object{pdf.Name("Nope")}, graphics.StateFillColor},
		{OpSetLineDash, []pdf.Object{pdf.Array{pdf.Name("x")}, pdf.Integer(0)}, graphics.StateLineDash},
	}
	for _, c := range cases {
		s := NewState(Form, nil)
		if err := s.ApplyStateChanges(c.op, c.args); err != nil {
			t.Fatal(err)
		}
		if s.IsSet(c.bit) {
			t.Errorf("%s %v sets %v", c.op, c.args, c.bit)
		}
	}

	s := NewState(Page, nil)
	if err := s.ApplyStateChanges(OpMoveTo, []pdf.Object{pdf.Integer(1)}); err != nil {
		t.Fatal(err)
	}
	if s.CurrentObject != ObjPage {
		t.Errorf("malformed m moves to %s context", s.CurrentObject)
	}
}

// TestApplyStateChangesBDCAsBMC checks that a BDC operator without a usable
// property list opens a marked-content sequence.
func TestApplyStateChangesBDCAsBMC(t *testing.T) {
	s := NewState(Page, nil)
	err := s.ApplyStateChanges(OpBeginMarkedContentWithProperties, []pdf.Object{pdf.Name("Span"), pdf.Integer(1)})
	if err != nil {
		t.Fatal(err)
	}
	if s.MarkedContentDepth() != 1 {
		t.Errorf("got depth %d, want 1", s.MarkedContentDepth())
	}
}

// TestApplyStateChangesDroppedSequence checks that a marked-content sequence
// whose opening operator has no usable tag is left out as a whole: the EMC
// which closes it is rejected, and the enclosing sequence stays open.
func TestApplyStateChangesDroppedSequence(t *testing.T) {
	openers := []struct {
		name string
		op   OpName
		args []pdf.Object
	}{
		{"BMC", OpBeginMarkedContent, nil},
		{"BDC", OpBeginMarkedContentWithProperties, []pdf.Object{pdf.Dict{}}},
	}
	for _, c := range openers {
		t.Run(c.name, func(t *testing.T) {
			s := NewState(Page, nil)
			apply := func(op OpName, args ...pdf.Object) {
				t.Helper()
				if err := s.ApplyStateChanges(op, args); err != nil {
					t.Fatal(err)
				}
			}
			emcOK := func() bool {
				_, _, _, ok := s.Operands(OpEndMarkedContent, nil)
				return ok
			}

			apply(OpBeginMarkedContent, pdf.Name("A"))
			apply(c.op, c.args...)
			if d := s.MarkedContentDepth(); d != 1 {
				t.Errorf("got depth %d inside the dropped sequence, want 1", d)
			}

			// an operator pair nested across the end of the sequence
			apply(OpPushGraphicsState)
			if emcOK() {
				t.Error("EMC of the dropped sequence is accepted")
			}
			apply(OpEndMarkedContent)
			apply(OpPopGraphicsState)
			if d := s.MarkedContentDepth(); d != 1 {
				t.Errorf("got depth %d after the dropped sequence, want 1", d)
			}

			if !emcOK() {
				t.Error("EMC of the enclosing sequence is rejected")
			}
			apply(OpEndMarkedContent)
			if err := s.CanClose(); err != nil {
				t.Errorf("stream is not balanced: %v", err)
			}
		})
	}
}

// TestClosingOperatorsDroppedSequence checks that the closing operators,
// applied to the state, close every sequence which is still open when one of
// them was dropped, and that exactly the EMC of the dropped one is rejected.
func TestClosingOperatorsDroppedSequence(t *testing.T) {
	s := NewState(Page, nil)
	for _, args := range [][]pdf.Object{{pdf.Name("A")}, nil, {pdf.Name("B")}} {
		if err := s.ApplyStateChanges(OpBeginMarkedContent, args); err != nil {
			t.Fatal(err)
		}
	}
	if d := s.MarkedContentDepth(); d != 2 {
		t.Fatalf("got depth %d, want 2", d)
	}

	accepted := 0
	for _, op := range s.ClosingOperators() {
		if _, _, _, ok := s.Operands(op, nil); ok {
			accepted++
		}
		if err := s.ApplyStateChanges(op, nil); err != nil {
			t.Fatal(err)
		}
	}
	if accepted != 2 {
		t.Errorf("got %d accepted closing operators, want 2", accepted)
	}
	if err := s.CanClose(); err != nil {
		t.Errorf("stream is not balanced: %v", err)
	}
}

// TestApplyStateChangesColorOperands checks that a colour operator uses the
// operands which [State.Operands] keeps.
func TestApplyStateChangesColorOperands(t *testing.T) {
	s := NewState(Page, nil)
	err := s.ApplyStateChanges(OpSetFillColor, []pdf.Object{pdf.Name("X"), pdf.Real(0.25), pdf.Real(0.75)})
	if err != nil {
		t.Fatal(err)
	}
	if s.GState.FillColor != color.DeviceGray(0.25) {
		t.Errorf("got fill colour %v, want gray 0.25", s.GState.FillColor)
	}
}

// TestApplyOperatorOperands checks that ApplyOperator reports operands which
// [State.Operands] rejects, and accepts the operator otherwise.
func TestApplyOperatorOperands(t *testing.T) {
	num := func(xs ...int) []pdf.Object {
		res := make([]pdf.Object, len(xs))
		for i, x := range xs {
			res[i] = pdf.Integer(x)
		}
		return res
	}
	rgb := []pdf.Object{pdf.Name("DeviceRGB")}

	s := NewState(Page, nil)
	if err := s.ApplyOperator(OpMoveTo, num(1)); err == nil {
		t.Error("m with one operand: got no error")
	}
	if err := s.ApplyOperator(OpMoveTo, num(1, 2, 3)); err == nil {
		t.Error("m with three operands: got no error")
	}
	if s.CurrentObject != ObjPage {
		t.Errorf("rejected m moves to %s context", s.CurrentObject)
	}
	if err := s.ApplyOperator(OpMoveTo, num(1, 2)); err != nil {
		t.Errorf("m with two operands: %v", err)
	}

	s = NewState(Page, nil)
	bdc := []pdf.Object{pdf.Name("Span"), pdf.Integer(7)}
	if err := s.ApplyOperator(OpBeginMarkedContentWithProperties, bdc); err == nil {
		t.Error("BDC with a number for the properties: got no error")
	}

	s = NewState(Page, nil)
	if err := s.ApplyOperator(OpSetFillColorSpace, rgb); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyOperator(OpSetFillColor, num(1)); err == nil {
		t.Error("sc with one of three components: got no error")
	}
	if err := s.ApplyOperator(OpSetFillColor, num(1, 0, 0)); err != nil {
		t.Errorf("sc with three components: %v", err)
	}
}
