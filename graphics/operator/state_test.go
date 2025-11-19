package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

func TestStateOperators_PushPop(t *testing.T) {
	gState := graphics.NewState()
	state := &State{
		Param: *gState.Parameters,
		Out:   graphics.StateLineWidth,
	}
	res := &resource.Resource{}

	// Push state
	opQ := Operator{Name: "q", Args: nil}
	if err := ApplyOperator(state, opQ, res); err != nil {
		t.Fatalf("q operator failed: %v", err)
	}

	// Modify state
	state.Param.LineWidth = 5.0
	state.Out |= graphics.StateLineWidth

	// Pop state
	opPop := Operator{Name: "Q", Args: nil}
	if err := ApplyOperator(state, opPop, res); err != nil {
		t.Fatalf("Q operator failed: %v", err)
	}

	// Verify restoration
	if state.Out != graphics.StateLineWidth {
		t.Errorf("Out not restored: got %v", state.Out)
	}
}

func TestStateOperators_LineWidth(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	op := Operator{Name: "w", Args: []pdf.Native{pdf.Real(2.5)}}
	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("w operator failed: %v", err)
	}

	if state.Param.LineWidth != 2.5 {
		t.Errorf("LineWidth = %v, want 2.5", state.Param.LineWidth)
	}
	if state.Out&graphics.StateLineWidth == 0 {
		t.Error("StateLineWidth not marked in Out")
	}
}

func TestStateOperators_PopWithoutPush(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	op := Operator{Name: "Q", Args: nil}
	err := ApplyOperator(state, op, res)
	if err == nil {
		t.Error("expected error for Q without matching q")
	}
}

func TestStateOperators_LineDash(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	op := Operator{
		Name: "d",
		Args: []pdf.Native{
			pdf.Array{pdf.Integer(3), pdf.Integer(2)},
			pdf.Integer(0),
		},
	}

	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("d operator failed: %v", err)
	}

	if len(state.Param.DashPattern) != 2 {
		t.Errorf("DashPattern length = %d, want 2", len(state.Param.DashPattern))
	}
	if state.Param.DashPattern[0] != 3.0 || state.Param.DashPattern[1] != 2.0 {
		t.Errorf("DashPattern = %v, want [3 2]", state.Param.DashPattern)
	}
	if state.Out&graphics.StateLineDash == 0 {
		t.Error("StateLineDash not marked in Out")
	}
}

func TestStateOperators_ConcatMatrix(t *testing.T) {
	gState := graphics.NewState()
	state := &State{
		Param: *gState.Parameters,
	}
	res := &resource.Resource{}

	// apply cm operator: 2 0 0 2 10 20 cm (scale by 2 and translate by (10,20))
	op := Operator{
		Name: "cm",
		Args: []pdf.Native{
			pdf.Real(2), pdf.Real(0),
			pdf.Real(0), pdf.Real(2),
			pdf.Real(10), pdf.Real(20),
		},
	}

	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("cm operator failed: %v", err)
	}

	// verify CTM was modified
	if state.Param.CTM[0] != 2.0 || state.Param.CTM[3] != 2.0 {
		t.Errorf("CTM scaling = [%v, %v], want [2, 2]", state.Param.CTM[0], state.Param.CTM[3])
	}
	if state.Param.CTM[4] != 10.0 || state.Param.CTM[5] != 20.0 {
		t.Errorf("CTM translation = [%v, %v], want [10, 20]", state.Param.CTM[4], state.Param.CTM[5])
	}
}
