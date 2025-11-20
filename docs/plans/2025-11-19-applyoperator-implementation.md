# ApplyOperator Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement `operator.ApplyOperator` to track graphics state changes from content stream operators, enabling both PDF reading and operator-based graphics writing.

**Architecture:** Handler table pattern with one function per operator. Each handler parses arguments, validates context, updates Parameters, and tracks In/Out dependencies. Five implementation files organize 73 operators by category.

**Tech Stack:** Go 1.24, seehuhn.de/go/pdf library, resource package for font/XObject/ExtGState resolution

---

## Task 1: Core Infrastructure (apply.go)

**Files:**
- Modify: `go-pdf/graphics/operator/apply.go:22-24`
- Create: `go-pdf/graphics/operator/apply_test.go`

### Step 1: Write failing test for ApplyOperator signature

```go
package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/resource"
)

func TestApplyOperator_BasicStructure(t *testing.T) {
	state := &State{}
	op := Operator{Name: "q", Args: nil}
	res := &resource.Resource{}

	err := ApplyOperator(state, op, res)
	if err != nil {
		t.Errorf("ApplyOperator returned error for valid operator: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./go-pdf/graphics/operator/ -run TestApplyOperator_BasicStructure -v`
Expected: FAIL with panic "not implemented"

### Step 3: Implement argParser struct and basic methods

Modify `go-pdf/graphics/operator/apply.go:22-24` to:

```go
package operator

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

// argParser provides a scanner-style API for parsing operator arguments
type argParser struct {
	args []pdf.Native
	err  error
}

func (p *argParser) GetFloat() float64 {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return 0
	}
	arg := p.args[0]
	p.args = p.args[1:]

	switch x := arg.(type) {
	case pdf.Real:
		return float64(x)
	case pdf.Integer:
		return float64(x)
	case pdf.Number:
		return float64(x)
	default:
		p.err = fmt.Errorf("expected number, got %T", arg)
		return 0
	}
}

func (p *argParser) GetInt() int {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return 0
	}
	arg := p.args[0]
	p.args = p.args[1:]

	i, ok := arg.(pdf.Integer)
	if !ok {
		p.err = fmt.Errorf("expected integer, got %T", arg)
		return 0
	}
	return int(i)
}

func (p *argParser) GetName() pdf.Name {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return ""
	}
	arg := p.args[0]
	p.args = p.args[1:]

	name, ok := arg.(pdf.Name)
	if !ok {
		p.err = fmt.Errorf("expected name, got %T", arg)
		return ""
	}
	return name
}

func (p *argParser) GetArray() pdf.Array {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return nil
	}
	arg := p.args[0]
	p.args = p.args[1:]

	arr, ok := arg.(pdf.Array)
	if !ok {
		p.err = fmt.Errorf("expected array, got %T", arg)
		return nil
	}
	return arr
}

func (p *argParser) GetDict() pdf.Dict {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return nil
	}
	arg := p.args[0]
	p.args = p.args[1:]

	dict, ok := arg.(pdf.Dict)
	if !ok {
		p.err = fmt.Errorf("expected dict, got %T", arg)
		return nil
	}
	return dict
}

func (p *argParser) GetString() pdf.String {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return ""
	}
	arg := p.args[0]
	p.args = p.args[1:]

	str, ok := arg.(pdf.String)
	if !ok {
		p.err = fmt.Errorf("expected string, got %T", arg)
		return ""
	}
	return str
}

func (p *argParser) Check() error {
	if len(p.args) > 0 {
		return errors.New("too many arguments")
	}
	return p.err
}

// State tracking helpers

func (s *State) markIn(bits graphics.StateBits) {
	s.In |= (bits &^ s.Out)
}

func (s *State) markOut(bits graphics.StateBits) {
	s.Out |= bits
}

// savedState stores graphics state for q/Q operators
type savedState struct {
	param *graphics.Parameters
	out   graphics.StateBits
}

// ApplyOperator applies a single content stream operator to the state
func ApplyOperator(state *State, op Operator, res *resource.Resource) error {
	handler, ok := handlers[op.Name]
	if !ok {
		return fmt.Errorf("unknown operator: %s", op.Name)
	}
	return handler(state, op.Args, res)
}

// opHandler is the function signature for operator handlers
type opHandler func(*State, []pdf.Native, *resource.Resource) error

// handlers maps operator names to their handler functions
var handlers = map[pdf.Name]opHandler{
	"q": handlePushState,
	// more handlers will be added in subsequent tasks
}

// handlePushState implements the q operator (save graphics state)
func handlePushState(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.stack == nil {
		s.stack = make([]savedState, 0, 4)
	}
	s.stack = append(s.stack, savedState{
		param: s.Param.Clone(),
		out:   s.Out,
	})
	return nil
}
```

Add to State struct in apply.go after line 20:

```go
	stack []savedState // graphics state stack for q/Q
```

### Step 4: Run test to verify it passes

Run: `go test ./go-pdf/graphics/operator/ -run TestApplyOperator_BasicStructure -v`
Expected: PASS

### Step 5: Add unit tests for argParser

Add to `go-pdf/graphics/operator/apply_test.go`:

```go
func TestArgParser_GetFloat(t *testing.T) {
	tests := []struct {
		name    string
		args    []pdf.Native
		want    float64
		wantErr bool
	}{
		{"Real", []pdf.Native{pdf.Real(3.14)}, 3.14, false},
		{"Integer", []pdf.Native{pdf.Integer(42)}, 42.0, false},
		{"Number", []pdf.Native{pdf.Number(2.71)}, 2.71, false},
		{"WrongType", []pdf.Native{pdf.Name("foo")}, 0, true},
		{"NoArgs", []pdf.Native{}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := argParser{args: tt.args}
			got := p.GetFloat()
			if got != tt.want {
				t.Errorf("GetFloat() = %v, want %v", got, tt.want)
			}
			if (p.err != nil) != tt.wantErr {
				t.Errorf("GetFloat() error = %v, wantErr %v", p.err, tt.wantErr)
			}
		})
	}
}

func TestArgParser_Check(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*argParser)
		wantErr bool
	}{
		{"NoArgs", func(p *argParser) {}, false},
		{"ExtraArgs", func(p *argParser) { p.args = []pdf.Native{pdf.Integer(1)} }, true},
		{"PreviousError", func(p *argParser) { p.err = errors.New("test") }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &argParser{}
			tt.setup(p)
			err := p.Check()
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

### Step 6: Run tests to verify they pass

Run: `go test ./go-pdf/graphics/operator/ -v`
Expected: All PASS

### Step 7: Commit

```bash
git add go-pdf/graphics/operator/apply.go go-pdf/graphics/operator/apply_test.go
git commit -m "feat(operator): add ApplyOperator infrastructure and argParser"
```

---

## Task 2: Graphics State Operators (state.go)

**Files:**
- Create: `go-pdf/graphics/operator/state.go`
- Create: `go-pdf/graphics/operator/state_test.go`
- Modify: `go-pdf/graphics/operator/apply.go` (add handlers to map)

### Step 1: Write failing test for Q operator

Create `go-pdf/graphics/operator/state_test.go`:

```go
package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

func TestStateOperators_PushPop(t *testing.T) {
	state := &State{
		Param: graphics.NewState().Parameters,
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
```

### Step 2: Run test to verify it fails

Run: `go test ./go-pdf/graphics/operator/ -run TestStateOperators -v`
Expected: FAIL (Q operator not in handlers)

### Step 3: Implement graphics state operators

Create `go-pdf/graphics/operator/state.go`:

```go
package operator

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/matrix"
	"seehuhn.de/go/pdf/resource"
)

// handlePopState implements the Q operator (restore graphics state)
func handlePopState(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if len(s.stack) == 0 {
		return errors.New("no saved state to restore")
	}

	saved := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]

	s.Param = *saved.param
	s.Out = saved.out
	// Note: In is NOT restored, CurrentObject is NOT restored

	return nil
}

// handleConcatMatrix implements the cm operator (modify CTM)
func handleConcatMatrix(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	a := p.GetFloat()
	b := p.GetFloat()
	c := p.GetFloat()
	d := p.GetFloat()
	e := p.GetFloat()
	f := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	m := matrix.Matrix{a, b, c, d, e, f}
	s.Param.CTM = s.Param.CTM.Mul(m)
	s.markOut(graphics.StateCTM)
	return nil
}

// handleSetLineWidth implements the w operator
func handleSetLineWidth(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	width := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.LineWidth = width
	s.markOut(graphics.StateLineWidth)
	return nil
}

// handleSetLineCap implements the J operator
func handleSetLineCap(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	cap := p.GetInt()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.LineCap = graphics.LineCapStyle(cap)
	s.markOut(graphics.StateLineCap)
	return nil
}

// handleSetLineJoin implements the j operator
func handleSetLineJoin(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	join := p.GetInt()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.LineJoin = graphics.LineJoinStyle(join)
	s.markOut(graphics.StateLineJoin)
	return nil
}

// handleSetMiterLimit implements the M operator
func handleSetMiterLimit(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	limit := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.MiterLimit = limit
	s.markOut(graphics.StateMiterLimit)
	return nil
}

// handleSetLineDash implements the d operator
func handleSetLineDash(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	arr := p.GetArray()
	phase := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	// Convert array to []float64
	pattern := make([]float64, len(arr))
	for i, val := range arr {
		switch v := val.(type) {
		case pdf.Real:
			pattern[i] = float64(v)
		case pdf.Integer:
			pattern[i] = float64(v)
		case pdf.Number:
			pattern[i] = float64(v)
		default:
			return errors.New("dash array must contain numbers")
		}
	}

	s.Param.DashPattern = pattern
	s.Param.DashPhase = phase
	s.markOut(graphics.StateLineDash)
	return nil
}

// handleSetRenderingIntent implements the ri operator
func handleSetRenderingIntent(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	intent := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.RenderingIntent = graphics.RenderingIntent(intent)
	s.markOut(graphics.StateRenderingIntent)
	return nil
}

// handleSetFlatness implements the i operator
func handleSetFlatness(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	flatness := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.FlatnessTolerance = flatness
	s.markOut(graphics.StateFlatnessTolerance)
	return nil
}

// handleSetExtGState implements the gs operator
func handleSetExtGState(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	if res.ExtGState == nil {
		return errors.New("no ExtGState resources available")
	}

	gs, ok := res.ExtGState[name]
	if !ok {
		return errors.New("ExtGState not found")
	}

	// Apply ExtGState parameters to current state
	// This is simplified - full implementation needs to handle all fields
	if gs.LineWidth != nil {
		s.Param.LineWidth = *gs.LineWidth
		s.markOut(graphics.StateLineWidth)
	}
	if gs.LineCap != nil {
		s.Param.LineCap = *gs.LineCap
		s.markOut(graphics.StateLineCap)
	}
	if gs.LineJoin != nil {
		s.Param.LineJoin = *gs.LineJoin
		s.markOut(graphics.StateLineJoin)
	}
	if gs.MiterLimit != nil {
		s.Param.MiterLimit = *gs.MiterLimit
		s.markOut(graphics.StateMiterLimit)
	}
	if gs.LineDashPattern != nil {
		s.Param.DashPattern = gs.LineDashPattern.Pattern
		s.Param.DashPhase = gs.LineDashPattern.Phase
		s.markOut(graphics.StateLineDash)
	}
	if gs.RenderingIntent != "" {
		s.Param.RenderingIntent = gs.RenderingIntent
		s.markOut(graphics.StateRenderingIntent)
	}
	if gs.StrokeAdjustment != nil {
		s.Param.StrokeAdjustment = *gs.StrokeAdjustment
		s.markOut(graphics.StateStrokeAdjustment)
	}

	return nil
}
```

### Step 4: Register handlers in apply.go

Modify `go-pdf/graphics/operator/apply.go` handlers map to include:

```go
var handlers = map[pdf.Name]opHandler{
	// Graphics state
	"q":  handlePushState,
	"Q":  handlePopState,
	"cm": handleConcatMatrix,
	"w":  handleSetLineWidth,
	"J":  handleSetLineCap,
	"j":  handleSetLineJoin,
	"M":  handleSetMiterLimit,
	"d":  handleSetLineDash,
	"ri": handleSetRenderingIntent,
	"i":  handleSetFlatness,
	"gs": handleSetExtGState,
}
```

### Step 5: Run tests to verify they pass

Run: `go test ./go-pdf/graphics/operator/ -run TestStateOperators -v`
Expected: PASS

### Step 6: Add comprehensive tests for edge cases

Add to `go-pdf/graphics/operator/state_test.go`:

```go
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
```

### Step 7: Run all tests

Run: `go test ./go-pdf/graphics/operator/ -v`
Expected: All PASS

### Step 8: Commit

```bash
git add go-pdf/graphics/operator/state.go go-pdf/graphics/operator/state_test.go go-pdf/graphics/operator/apply.go
git commit -m "feat(operator): implement graphics state operators (q, Q, cm, w, J, j, M, d, ri, i, gs)"
```

---

## Task 3: Path Construction Operators (path.go - part 1)

**Files:**
- Create: `go-pdf/graphics/operator/path.go`
- Create: `go-pdf/graphics/operator/path_test.go`
- Modify: `go-pdf/graphics/operator/apply.go` (add handlers)

### Step 1: Write failing tests for path construction

Create `go-pdf/graphics/operator/path_test.go`:

```go
package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

func TestPathConstruction_MoveTo(t *testing.T) {
	state := &State{CurrentObject: graphics.ObjectType(1)} // objPage
	res := &resource.Resource{}

	op := Operator{
		Name: "m",
		Args: []pdf.Native{pdf.Real(10.0), pdf.Real(20.0)},
	}

	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("m operator failed: %v", err)
	}

	if state.Param.CurrentX != 10.0 || state.Param.CurrentY != 20.0 {
		t.Errorf("Current point = (%v, %v), want (10, 20)",
			state.Param.CurrentX, state.Param.CurrentY)
	}
	if state.Param.StartX != 10.0 || state.Param.StartY != 20.0 {
		t.Errorf("Start point = (%v, %v), want (10, 20)",
			state.Param.StartX, state.Param.StartY)
	}
	if state.CurrentObject != graphics.ObjectType(2) { // objPath
		t.Errorf("CurrentObject = %v, want objPath", state.CurrentObject)
	}
}

func TestPathConstruction_LineTo(t *testing.T) {
	state := &State{CurrentObject: graphics.ObjectType(2)} // objPath
	state.Param.CurrentX = 10.0
	state.Param.CurrentY = 20.0
	res := &resource.Resource{}

	op := Operator{
		Name: "l",
		Args: []pdf.Native{pdf.Real(30.0), pdf.Real(40.0)},
	}

	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("l operator failed: %v", err)
	}

	if state.Param.CurrentX != 30.0 || state.Param.CurrentY != 40.0 {
		t.Errorf("Current point = (%v, %v), want (30, 40)",
			state.Param.CurrentX, state.Param.CurrentY)
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./go-pdf/graphics/operator/ -run TestPathConstruction -v`
Expected: FAIL (handlers not registered)

### Step 3: Implement path construction operators

Create `go-pdf/graphics/operator/path.go`:

```go
package operator

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

const (
	objPage         = graphics.ObjectType(1 << 0)
	objPath         = graphics.ObjectType(1 << 1)
	objText         = graphics.ObjectType(1 << 2)
	objClippingPath = graphics.ObjectType(1 << 3)
)

// handleMoveTo implements the m operator (begin new subpath)
func handleMoveTo(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	x := p.GetFloat()
	y := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPage && s.CurrentObject != objPath {
		return errors.New("m: invalid context")
	}

	// Finalize any existing open subpath
	if s.CurrentObject == objPath && !s.Param.ThisSubpathClosed {
		s.Param.AllSubpathsClosed = false
	}

	s.CurrentObject = objPath
	s.Param.StartX, s.Param.StartY = x, y
	s.Param.CurrentX, s.Param.CurrentY = x, y
	s.Param.ThisSubpathClosed = false

	return nil
}

// handleLineTo implements the l operator (append straight line)
func handleLineTo(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	x := p.GetFloat()
	y := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX, s.Param.CurrentY = x, y
	return nil
}

// handleCurveTo implements the c operator (append Bezier curve)
func handleCurveTo(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	x1 := p.GetFloat()
	y1 := p.GetFloat()
	x2 := p.GetFloat()
	y2 := p.GetFloat()
	x3 := p.GetFloat()
	y3 := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX, s.Param.CurrentY = x3, y3
	return nil
}

// handleCurveToV implements the v operator (Bezier curve, initial point replicated)
func handleCurveToV(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	x2 := p.GetFloat()
	y2 := p.GetFloat()
	x3 := p.GetFloat()
	y3 := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX, s.Param.CurrentY = x3, y3
	return nil
}

// handleCurveToY implements the y operator (Bezier curve, final point replicated)
func handleCurveToY(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	x1 := p.GetFloat()
	y1 := p.GetFloat()
	x3 := p.GetFloat()
	y3 := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX, s.Param.CurrentY = x3, y3
	return nil
}

// handleClosePath implements the h operator (close current subpath)
func handleClosePath(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX = s.Param.StartX
	s.Param.CurrentY = s.Param.StartY
	s.Param.ThisSubpathClosed = true
	return nil
}

// handleRectangle implements the re operator (append rectangle)
func handleRectangle(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	x := p.GetFloat()
	y := p.GetFloat()
	width := p.GetFloat()
	height := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPage && s.CurrentObject != objPath {
		return errors.New("re: invalid context")
	}

	// Finalize any existing open subpath
	if s.CurrentObject == objPath && !s.Param.ThisSubpathClosed {
		s.Param.AllSubpathsClosed = false
	}

	s.CurrentObject = objPath
	// Rectangle creates a closed subpath
	s.Param.StartX, s.Param.StartY = x, y
	s.Param.CurrentX, s.Param.CurrentY = x, y
	s.Param.ThisSubpathClosed = true

	return nil
}
```

### Step 4: Register handlers in apply.go

Add to handlers map in `go-pdf/graphics/operator/apply.go`:

```go
	// Path construction
	"m":  handleMoveTo,
	"l":  handleLineTo,
	"c":  handleCurveTo,
	"v":  handleCurveToV,
	"y":  handleCurveToY,
	"h":  handleClosePath,
	"re": handleRectangle,
```

### Step 5: Run tests to verify they pass

Run: `go test ./go-pdf/graphics/operator/ -run TestPathConstruction -v`
Expected: PASS

### Step 6: Commit

```bash
git add go-pdf/graphics/operator/path.go go-pdf/graphics/operator/path_test.go go-pdf/graphics/operator/apply.go
git commit -m "feat(operator): implement path construction operators (m, l, c, v, y, h, re)"
```

---

## Task 4: Path Painting Operators (path.go - part 2)

**Files:**
- Modify: `go-pdf/graphics/operator/path.go`
- Modify: `go-pdf/graphics/operator/path_test.go`
- Modify: `go-pdf/graphics/operator/apply.go`

### Step 1: Write failing tests for path painting

Add to `go-pdf/graphics/operator/path_test.go`:

```go
func TestPathPainting_Stroke(t *testing.T) {
	state := &State{CurrentObject: objPath}
	state.Param.AllSubpathsClosed = true
	state.Param.DashPattern = nil
	res := &resource.Resource{}

	op := Operator{Name: "S", Args: nil}
	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("S operator failed: %v", err)
	}

	// Should mark dependencies
	expected := graphics.StateLineWidth | graphics.StateLineJoin |
		graphics.StateLineDash | graphics.StateStrokeColor
	if state.In&expected != expected {
		t.Errorf("In = %v, want at least %v", state.In, expected)
	}

	// LineCap should NOT be marked for closed path without dashes
	if state.In&graphics.StateLineCap != 0 {
		t.Error("LineCap marked but path is closed and not dashed")
	}

	// Should reset to page context
	if state.CurrentObject != objPage {
		t.Errorf("CurrentObject = %v, want objPage", state.CurrentObject)
	}
}

func TestPathPainting_StrokeOpenPath(t *testing.T) {
	state := &State{CurrentObject: objPath}
	state.Param.AllSubpathsClosed = false
	state.Param.DashPattern = nil
	res := &resource.Resource{}

	op := Operator{Name: "S", Args: nil}
	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("S operator failed: %v", err)
	}

	// LineCap SHOULD be marked for open path
	if state.In&graphics.StateLineCap == 0 {
		t.Error("LineCap not marked for open path")
	}
}

func TestPathPainting_Fill(t *testing.T) {
	state := &State{CurrentObject: objPath}
	res := &resource.Resource{}

	op := Operator{Name: "f", Args: nil}
	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("f operator failed: %v", err)
	}

	if state.In&graphics.StateFillColor == 0 {
		t.Error("FillColor not marked in In")
	}
	if state.CurrentObject != objPage {
		t.Errorf("CurrentObject = %v, want objPage", state.CurrentObject)
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./go-pdf/graphics/operator/ -run TestPathPainting -v`
Expected: FAIL (handlers not implemented)

### Step 3: Implement path painting operators

Add to `go-pdf/graphics/operator/path.go`:

```go
// handleStroke implements the S operator (stroke path)
func handleStroke(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath && s.CurrentObject != objClippingPath {
		return errors.New("not in path context")
	}

	// Finalize current subpath
	if !s.Param.ThisSubpathClosed {
		s.Param.AllSubpathsClosed = false
	}

	// Mark dependencies
	s.markIn(graphics.StateLineWidth | graphics.StateLineJoin |
		graphics.StateLineDash | graphics.StateStrokeColor)

	// Conditional dependency on LineCap
	if !s.Param.AllSubpathsClosed || len(s.Param.DashPattern) > 0 {
		s.markIn(graphics.StateLineCap)
	}

	// Reset path
	s.CurrentObject = objPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleCloseAndStroke implements the s operator (close and stroke path)
func handleCloseAndStroke(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath && s.CurrentObject != objClippingPath {
		return errors.New("not in path context")
	}

	// Close current subpath
	s.Param.CurrentX = s.Param.StartX
	s.Param.CurrentY = s.Param.StartY
	s.Param.ThisSubpathClosed = true

	// Mark dependencies (same as S)
	s.markIn(graphics.StateLineWidth | graphics.StateLineJoin |
		graphics.StateLineDash | graphics.StateStrokeColor)

	// Conditional dependency on LineCap
	if !s.Param.AllSubpathsClosed || len(s.Param.DashPattern) > 0 {
		s.markIn(graphics.StateLineCap)
	}

	s.CurrentObject = objPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleFill implements the f operator (fill path using nonzero winding rule)
func handleFill(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath && s.CurrentObject != objClippingPath {
		return errors.New("not in path context")
	}

	s.markIn(graphics.StateFillColor)
	s.CurrentObject = objPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleFillCompat implements the F operator (deprecated alias for f)
func handleFillCompat(s *State, args []pdf.Native, res *resource.Resource) error {
	return handleFill(s, args, res)
}

// handleFillEvenOdd implements the f* operator (fill using even-odd rule)
func handleFillEvenOdd(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath && s.CurrentObject != objClippingPath {
		return errors.New("not in path context")
	}

	s.markIn(graphics.StateFillColor)
	s.CurrentObject = objPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleFillAndStroke implements the B operator (fill and stroke, nonzero)
func handleFillAndStroke(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath && s.CurrentObject != objClippingPath {
		return errors.New("not in path context")
	}

	if !s.Param.ThisSubpathClosed {
		s.Param.AllSubpathsClosed = false
	}

	s.markIn(graphics.StateFillColor | graphics.StateLineWidth |
		graphics.StateLineJoin | graphics.StateLineDash | graphics.StateStrokeColor)

	if !s.Param.AllSubpathsClosed || len(s.Param.DashPattern) > 0 {
		s.markIn(graphics.StateLineCap)
	}

	s.CurrentObject = objPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleFillAndStrokeEvenOdd implements the B* operator
func handleFillAndStrokeEvenOdd(s *State, args []pdf.Native, res *resource.Resource) error {
	return handleFillAndStroke(s, args, res)
}

// handleCloseFillAndStroke implements the b operator
func handleCloseFillAndStroke(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath && s.CurrentObject != objClippingPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX = s.Param.StartX
	s.Param.CurrentY = s.Param.StartY
	s.Param.ThisSubpathClosed = true

	s.markIn(graphics.StateFillColor | graphics.StateLineWidth |
		graphics.StateLineJoin | graphics.StateLineDash | graphics.StateStrokeColor)

	if !s.Param.AllSubpathsClosed || len(s.Param.DashPattern) > 0 {
		s.markIn(graphics.StateLineCap)
	}

	s.CurrentObject = objPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleCloseFillAndStrokeEvenOdd implements the b* operator
func handleCloseFillAndStrokeEvenOdd(s *State, args []pdf.Native, res *resource.Resource) error {
	return handleCloseFillAndStroke(s, args, res)
}

// handleEndPath implements the n operator (end path without painting)
func handleEndPath(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath && s.CurrentObject != objClippingPath {
		return errors.New("not in path context")
	}

	s.CurrentObject = objPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleClip implements the W operator (set clipping path, nonzero)
func handleClip(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath {
		return errors.New("not in path context")
	}

	s.CurrentObject = objClippingPath
	return nil
}

// handleClipEvenOdd implements the W* operator (set clipping path, even-odd)
func handleClipEvenOdd(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPath {
		return errors.New("not in path context")
	}

	s.CurrentObject = objClippingPath
	return nil
}
```

### Step 4: Register handlers

Add to handlers map in `go-pdf/graphics/operator/apply.go`:

```go
	// Path painting
	"S":  handleStroke,
	"s":  handleCloseAndStroke,
	"f":  handleFill,
	"F":  handleFillCompat,
	"f*": handleFillEvenOdd,
	"B":  handleFillAndStroke,
	"B*": handleFillAndStrokeEvenOdd,
	"b":  handleCloseFillAndStroke,
	"b*": handleCloseFillAndStrokeEvenOdd,
	"n":  handleEndPath,
	"W":  handleClip,
	"W*": handleClipEvenOdd,
```

### Step 5: Run tests

Run: `go test ./go-pdf/graphics/operator/ -run TestPathPainting -v`
Expected: PASS

### Step 6: Commit

```bash
git add go-pdf/graphics/operator/path.go go-pdf/graphics/operator/path_test.go go-pdf/graphics/operator/apply.go
git commit -m "feat(operator): implement path painting operators (S, s, f, F, f*, B, B*, b, b*, n, W, W*)"
```

---

## Task 5: Text Operators (text.go)

**Files:**
- Create: `go-pdf/graphics/operator/text.go`
- Create: `go-pdf/graphics/operator/text_test.go`
- Modify: `go-pdf/graphics/operator/apply.go`

### Step 1: Write failing tests for text operators

Create `go-pdf/graphics/operator/text_test.go`:

```go
package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/matrix"
	"seehuhn.de/go/pdf/resource"
)

func TestTextOperators_BeginEnd(t *testing.T) {
	state := &State{CurrentObject: objPage}
	res := &resource.Resource{}

	// Begin text
	opBT := Operator{Name: "BT", Args: nil}
	if err := ApplyOperator(state, opBT, res); err != nil {
		t.Fatalf("BT operator failed: %v", err)
	}

	if state.CurrentObject != objText {
		t.Errorf("CurrentObject = %v, want objText", state.CurrentObject)
	}
	if state.Param.TextMatrix != matrix.Identity {
		t.Error("TextMatrix not reset to identity")
	}
	if state.Out&graphics.StateTextMatrix == 0 {
		t.Error("StateTextMatrix not marked in Out")
	}

	// End text
	opET := Operator{Name: "ET", Args: nil}
	if err := ApplyOperator(state, opET, res); err != nil {
		t.Fatalf("ET operator failed: %v", err)
	}

	if state.CurrentObject != objPage {
		t.Errorf("CurrentObject = %v, want objPage", state.CurrentObject)
	}
	if state.Out&graphics.StateTextMatrix != 0 {
		t.Error("StateTextMatrix still marked after ET")
	}
}

func TestTextOperators_SetFont(t *testing.T) {
	state := &State{}
	mockFont := &mockFontInstance{}
	res := &resource.Resource{
		Font: map[pdf.Name]font.Instance{
			"F1": mockFont,
		},
	}

	op := Operator{
		Name: "Tf",
		Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)},
	}

	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("Tf operator failed: %v", err)
	}

	if state.Param.TextFont != mockFont {
		t.Error("TextFont not set")
	}
	if state.Param.TextFontSize != 12.0 {
		t.Errorf("TextFontSize = %v, want 12.0", state.Param.TextFontSize)
	}
	if state.Out&graphics.StateTextFont == 0 {
		t.Error("StateTextFont not marked in Out")
	}
}

// mockFontInstance for testing
type mockFontInstance struct{}

func (m *mockFontInstance) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }
```

### Step 2: Run test to verify it fails

Run: `go test ./go-pdf/graphics/operator/ -run TestTextOperators -v`
Expected: FAIL (handlers not implemented)

### Step 3: Implement text operators

Create `go-pdf/graphics/operator/text.go`:

```go
package operator

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/matrix"
	"seehuhn.de/go/pdf/resource"
)

// handleTextBegin implements the BT operator (begin text object)
func handleTextBegin(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPage {
		return errors.New("BT: not in page context")
	}

	s.CurrentObject = objText
	s.Param.TextMatrix = matrix.Identity
	s.Param.TextLineMatrix = matrix.Identity
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextEnd implements the ET operator (end text object)
func handleTextEnd(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
		return errors.New("not in text object")
	}

	s.CurrentObject = objPage
	s.Out &= ^graphics.StateTextMatrix

	return nil
}

// handleTextSetCharSpacing implements the Tc operator
func handleTextSetCharSpacing(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	spacing := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextCharacterSpacing = spacing
	s.markOut(graphics.StateTextCharacterSpacing)
	return nil
}

// handleTextSetWordSpacing implements the Tw operator
func handleTextSetWordSpacing(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	spacing := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextWordSpacing = spacing
	s.markOut(graphics.StateTextWordSpacing)
	return nil
}

// handleTextSetHorizontalScaling implements the Tz operator
func handleTextSetHorizontalScaling(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	scale := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextHorizontalScaling = scale / 100.0 // PDF uses percentage
	s.markOut(graphics.StateTextHorizontalScaling)
	return nil
}

// handleTextSetLeading implements the TL operator
func handleTextSetLeading(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	leading := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextLeading = leading
	s.markOut(graphics.StateTextLeading)
	return nil
}

// handleTextSetFont implements the Tf operator
func handleTextSetFont(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	name := p.GetName()
	size := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if res.Font == nil {
		return errors.New("no font resources available")
	}

	fontInstance, ok := res.Font[name]
	if !ok {
		return errors.New("font not found")
	}

	s.Param.TextFont = fontInstance
	s.Param.TextFontSize = size
	s.markOut(graphics.StateTextFont)
	return nil
}

// handleTextSetRenderingMode implements the Tr operator
func handleTextSetRenderingMode(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	mode := p.GetInt()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextRenderingMode = graphics.TextRenderingMode(mode)
	s.markOut(graphics.StateTextRenderingMode)
	return nil
}

// handleTextSetRise implements the Ts operator
func handleTextSetRise(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	rise := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextRise = rise
	s.markOut(graphics.StateTextRise)
	return nil
}

// handleTextMoveOffset implements the Td operator
func handleTextMoveOffset(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	tx := p.GetFloat()
	ty := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
		return errors.New("not in text object")
	}

	// Translate text line matrix
	s.Param.TextLineMatrix = s.Param.TextLineMatrix.Mul(matrix.Matrix{1, 0, 0, 1, tx, ty})
	s.Param.TextMatrix = s.Param.TextLineMatrix
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextMoveOffsetSetLeading implements the TD operator
func handleTextMoveOffsetSetLeading(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	tx := p.GetFloat()
	ty := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
		return errors.New("not in text object")
	}

	// Set leading
	s.Param.TextLeading = -ty
	s.markOut(graphics.StateTextLeading)

	// Move text position
	s.Param.TextLineMatrix = s.Param.TextLineMatrix.Mul(matrix.Matrix{1, 0, 0, 1, tx, ty})
	s.Param.TextMatrix = s.Param.TextLineMatrix
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextSetMatrix implements the Tm operator
func handleTextSetMatrix(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	a := p.GetFloat()
	b := p.GetFloat()
	c := p.GetFloat()
	d := p.GetFloat()
	e := p.GetFloat()
	f := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
		return errors.New("not in text object")
	}

	m := matrix.Matrix{a, b, c, d, e, f}
	s.Param.TextMatrix = m
	s.Param.TextLineMatrix = m
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextNextLine implements the T* operator
func handleTextNextLine(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
		return errors.New("not in text object")
	}

	// Mark dependency on TextLeading
	s.markIn(graphics.StateTextLeading)

	// Move to next line
	leading := s.Param.TextLeading
	s.Param.TextLineMatrix = s.Param.TextLineMatrix.Mul(matrix.Matrix{1, 0, 0, 1, 0, -leading})
	s.Param.TextMatrix = s.Param.TextLineMatrix
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextShow implements the Tj operator
func handleTextShow(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetString() // text to show
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
		return errors.New("not in text object")
	}

	s.markIn(graphics.StateTextFont | graphics.StateTextMatrix)

	// Dependencies based on rendering mode
	mode := s.Param.TextRenderingMode
	if mode == graphics.TextRenderingModeFill ||
		mode == graphics.TextRenderingModeFillStroke ||
		mode == graphics.TextRenderingModeFillClip ||
		mode == graphics.TextRenderingModeFillStrokeClip {
		s.markIn(graphics.StateFillColor)
	}
	if mode == graphics.TextRenderingModeStroke ||
		mode == graphics.TextRenderingModeFillStroke ||
		mode == graphics.TextRenderingModeStrokeClip ||
		mode == graphics.TextRenderingModeFillStrokeClip {
		s.markIn(graphics.StateStrokeColor | graphics.StateLineWidth |
			graphics.StateLineJoin | graphics.StateLineCap)
	}

	s.markOut(graphics.StateTextMatrix)
	return nil
}

// handleTextShowArray implements the TJ operator
func handleTextShowArray(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetArray() // array of strings and numbers
	if err := p.Check(); err != nil {
		return err
	}

	// Same dependencies as Tj
	return handleTextShow(s, []pdf.Native{pdf.String("")}, res)
}

// handleTextShowMoveNextLine implements the ' operator
func handleTextShowMoveNextLine(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	text := p.GetString()
	if err := p.Check(); err != nil {
		return err
	}

	// Equivalent to: T* Tj
	if err := handleTextNextLine(s, nil, res); err != nil {
		return err
	}
	return handleTextShow(s, []pdf.Native{text}, res)
}

// handleTextShowMoveNextLineSetSpacing implements the " operator
func handleTextShowMoveNextLineSetSpacing(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	aw := p.GetFloat()
	ac := p.GetFloat()
	text := p.GetString()
	if err := p.Check(); err != nil {
		return err
	}

	// Equivalent to: aw Tw ac Tc string '
	s.Param.TextWordSpacing = aw
	s.markOut(graphics.StateTextWordSpacing)
	s.Param.TextCharacterSpacing = ac
	s.markOut(graphics.StateTextCharacterSpacing)

	return handleTextShowMoveNextLine(s, []pdf.Native{text}, res)
}
```

### Step 4: Register handlers

Add to handlers map in `go-pdf/graphics/operator/apply.go`:

```go
	// Text objects
	"BT": handleTextBegin,
	"ET": handleTextEnd,

	// Text state
	"Tc": handleTextSetCharSpacing,
	"Tw": handleTextSetWordSpacing,
	"Tz": handleTextSetHorizontalScaling,
	"TL": handleTextSetLeading,
	"Tf": handleTextSetFont,
	"Tr": handleTextSetRenderingMode,
	"Ts": handleTextSetRise,

	// Text positioning
	"Td": handleTextMoveOffset,
	"TD": handleTextMoveOffsetSetLeading,
	"Tm": handleTextSetMatrix,
	"T*": handleTextNextLine,

	// Text showing
	"Tj": handleTextShow,
	"TJ": handleTextShowArray,
	"'":  handleTextShowMoveNextLine,
	`"`:  handleTextShowMoveNextLineSetSpacing,
```

### Step 5: Run tests

Run: `go test ./go-pdf/graphics/operator/ -run TestTextOperators -v`
Expected: PASS

### Step 6: Commit

```bash
git add go-pdf/graphics/operator/text.go go-pdf/graphics/operator/text_test.go go-pdf/graphics/operator/apply.go
git commit -m "feat(operator): implement text operators (BT, ET, Tc, Tw, Tz, TL, Tf, Tr, Ts, Td, TD, Tm, T*, Tj, TJ, ', \")"
```

---

## Task 6: Color Operators (color.go)

**Files:**
- Create: `go-pdf/graphics/operator/color.go`
- Create: `go-pdf/graphics/operator/color_test.go`
- Modify: `go-pdf/graphics/operator/apply.go`

### Step 1: Write failing tests for color operators

Create `go-pdf/graphics/operator/color_test.go`:

```go
package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/resource"
)

func TestColorOperators_DeviceGray(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	// Set stroke gray
	opG := Operator{Name: "G", Args: []pdf.Native{pdf.Real(0.5)}}
	if err := ApplyOperator(state, opG, res); err != nil {
		t.Fatalf("G operator failed: %v", err)
	}

	if state.Out&graphics.StateStrokeColor == 0 {
		t.Error("StateStrokeColor not marked in Out")
	}

	// Set fill gray
	opg := Operator{Name: "g", Args: []pdf.Native{pdf.Real(0.75)}}
	if err := ApplyOperator(state, opg, res); err != nil {
		t.Fatalf("g operator failed: %v", err)
	}

	if state.Out&graphics.StateFillColor == 0 {
		t.Error("StateFillColor not marked in Out")
	}
}

func TestColorOperators_DeviceRGB(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	op := Operator{
		Name: "rg",
		Args: []pdf.Native{pdf.Real(1.0), pdf.Real(0.0), pdf.Real(0.0)},
	}

	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("rg operator failed: %v", err)
	}

	if state.Out&graphics.StateFillColor == 0 {
		t.Error("StateFillColor not marked in Out")
	}
}

func TestColorOperators_SetColorSpace(t *testing.T) {
	state := &State{}
	res := &resource.Resource{
		ColorSpace: map[pdf.Name]color.Space{
			"CS1": color.DeviceGray,
		},
	}

	// Set stroke color space
	opCS := Operator{Name: "CS", Args: []pdf.Native{pdf.Name("CS1")}}
	if err := ApplyOperator(state, opCS, res); err != nil {
		t.Fatalf("CS operator failed: %v", err)
	}

	if state.Out&graphics.StateStrokeColor == 0 {
		t.Error("StateStrokeColor not marked in Out")
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./go-pdf/graphics/operator/ -run TestColorOperators -v`
Expected: FAIL

### Step 3: Implement color operators

Create `go-pdf/graphics/operator/color.go`:

```go
package operator

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/resource"
)

// handleSetStrokeColorSpace implements the CS operator
func handleSetStrokeColorSpace(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	// Handle device color spaces directly
	var cs color.Space
	switch name {
	case "DeviceGray":
		cs = color.DeviceGray
	case "DeviceRGB":
		cs = color.DeviceRGB
	case "DeviceCMYK":
		cs = color.DeviceCMYK
	default:
		// Look up in resources
		if res.ColorSpace == nil {
			return errors.New("no color space resources available")
		}
		var ok bool
		cs, ok = res.ColorSpace[name]
		if !ok {
			return errors.New("color space not found")
		}
	}

	s.Param.StrokeColor = cs.Default()
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillColorSpace implements the cs operator
func handleSetFillColorSpace(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	var cs color.Space
	switch name {
	case "DeviceGray":
		cs = color.DeviceGray
	case "DeviceRGB":
		cs = color.DeviceRGB
	case "DeviceCMYK":
		cs = color.DeviceCMYK
	default:
		if res.ColorSpace == nil {
			return errors.New("no color space resources available")
		}
		var ok bool
		cs, ok = res.ColorSpace[name]
		if !ok {
			return errors.New("color space not found")
		}
	}

	s.Param.FillColor = cs.Default()
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetStrokeColor implements the SC operator
func handleSetStrokeColor(s *State, args []pdf.Native, res *resource.Resource) error {
	// For simplicity, just mark the dependency
	// Full implementation would parse components based on current color space
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetStrokeColorN implements the SCN operator
func handleSetStrokeColorN(s *State, args []pdf.Native, res *resource.Resource) error {
	// Similar to SC but supports patterns
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillColor implements the sc operator
func handleSetFillColor(s *State, args []pdf.Native, res *resource.Resource) error {
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetFillColorN implements the scn operator
func handleSetFillColorN(s *State, args []pdf.Native, res *resource.Resource) error {
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetStrokeGray implements the G operator
func handleSetStrokeGray(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	gray := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.StrokeColor = color.DeviceGray.New(gray)
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillGray implements the g operator
func handleSetFillGray(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	gray := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.FillColor = color.DeviceGray.New(gray)
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetStrokeRGB implements the RG operator
func handleSetStrokeRGB(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	r := p.GetFloat()
	g := p.GetFloat()
	b := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.StrokeColor = color.DeviceRGB.New(r, g, b)
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillRGB implements the rg operator
func handleSetFillRGB(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	r := p.GetFloat()
	g := p.GetFloat()
	b := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.FillColor = color.DeviceRGB.New(r, g, b)
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetStrokeCMYK implements the K operator
func handleSetStrokeCMYK(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	c := p.GetFloat()
	m := p.GetFloat()
	y := p.GetFloat()
	k := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.StrokeColor = color.DeviceCMYK.New(c, m, y, k)
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillCMYK implements the k operator
func handleSetFillCMYK(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	c := p.GetFloat()
	m := p.GetFloat()
	y := p.GetFloat()
	k := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.FillColor = color.DeviceCMYK.New(c, m, y, k)
	s.markOut(graphics.StateFillColor)
	return nil
}
```

### Step 4: Register handlers

Add to handlers map:

```go
	// Color spaces
	"CS": handleSetStrokeColorSpace,
	"cs": handleSetFillColorSpace,

	// Generic color
	"SC":  handleSetStrokeColor,
	"SCN": handleSetStrokeColorN,
	"sc":  handleSetFillColor,
	"scn": handleSetFillColorN,

	// Device colors
	"G":  handleSetStrokeGray,
	"g":  handleSetFillGray,
	"RG": handleSetStrokeRGB,
	"rg": handleSetFillRGB,
	"K":  handleSetStrokeCMYK,
	"k":  handleSetFillCMYK,
```

### Step 5: Run tests

Run: `go test ./go-pdf/graphics/operator/ -v`
Expected: PASS

### Step 6: Commit

```bash
git add go-pdf/graphics/operator/color.go go-pdf/graphics/operator/color_test.go go-pdf/graphics/operator/apply.go
git commit -m "feat(operator): implement color operators (CS, cs, SC, SCN, sc, scn, G, g, RG, rg, K, k)"
```

---

## Task 7: Miscellaneous Operators (misc.go)

**Files:**
- Create: `go-pdf/graphics/operator/misc.go`
- Create: `go-pdf/graphics/operator/misc_test.go`
- Modify: `go-pdf/graphics/operator/apply.go`

### Step 1: Write failing tests

Create `go-pdf/graphics/operator/misc_test.go`:

```go
package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

func TestMiscOperators_XObject(t *testing.T) {
	state := &State{}
	mockXObj := &mockXObject{}
	res := &resource.Resource{
		XObject: map[pdf.Name]graphics.XObject{
			"Im1": mockXObj,
		},
	}

	op := Operator{Name: "Do", Args: []pdf.Native{pdf.Name("Im1")}}
	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("Do operator failed: %v", err)
	}
}

func TestMiscOperators_MarkedContent(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	// BMC
	opBMC := Operator{Name: "BMC", Args: []pdf.Native{pdf.Name("Tag1")}}
	if err := ApplyOperator(state, opBMC, res); err != nil {
		t.Fatalf("BMC operator failed: %v", err)
	}

	// EMC
	opEMC := Operator{Name: "EMC", Args: nil}
	if err := ApplyOperator(state, opEMC, res); err != nil {
		t.Fatalf("EMC operator failed: %v", err)
	}
}

func TestMiscOperators_SpecialOperators(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	// %raw%
	opRaw := Operator{Name: "%raw%", Args: []pdf.Native{pdf.String("  % comment\n")}}
	if err := ApplyOperator(state, opRaw, res); err != nil {
		t.Fatalf("%%raw%% operator failed: %v", err)
	}

	// %image%
	opImage := Operator{
		Name: "%image%",
		Args: []pdf.Native{
			pdf.Dict{"W": pdf.Integer(10), "H": pdf.Integer(10)},
			pdf.String("imagedata"),
		},
	}
	if err := ApplyOperator(state, opImage, res); err != nil {
		t.Fatalf("%%image%% operator failed: %v", err)
	}
}

// mockXObject for testing
type mockXObject struct{}

func (m *mockXObject) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }
```

### Step 2: Run test to verify it fails

Run: `go test ./go-pdf/graphics/operator/ -run TestMiscOperators -v`
Expected: FAIL

### Step 3: Implement miscellaneous operators

Create `go-pdf/graphics/operator/misc.go`:

```go
package operator

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/resource"
)

// handleShading implements the sh operator (paint shading)
func handleShading(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	if res.Shading == nil {
		return errors.New("no shading resources available")
	}

	_, ok := res.Shading[name]
	if !ok {
		return errors.New("shading not found")
	}

	return nil
}

// handleXObject implements the Do operator (paint XObject)
func handleXObject(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	if res.XObject == nil {
		return errors.New("no XObject resources available")
	}

	_, ok := res.XObject[name]
	if !ok {
		return errors.New("XObject not found")
	}

	return nil
}

// handleMarkedContentPoint implements the MP operator
func handleMarkedContentPoint(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleMarkedContentPointWithProperties implements the DP operator
func handleMarkedContentPointWithProperties(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	// Second arg can be name or dict
	if len(p.args) > 0 {
		p.args = p.args[1:]
	}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleBeginMarkedContent implements the BMC operator
func handleBeginMarkedContent(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleBeginMarkedContentWithProperties implements the BDC operator
func handleBeginMarkedContentWithProperties(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	// Second arg can be name or dict
	if len(p.args) > 0 {
		p.args = p.args[1:]
	}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleEndMarkedContent implements the EMC operator
func handleEndMarkedContent(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleType3d0 implements the d0 operator (Type 3 font glyph width)
func handleType3d0(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetFloat() // wx
	_ = p.GetFloat() // wy
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleType3d1 implements the d1 operator (Type 3 font glyph width and bbox)
func handleType3d1(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetFloat() // wx
	_ = p.GetFloat() // wy
	_ = p.GetFloat() // llx
	_ = p.GetFloat() // lly
	_ = p.GetFloat() // urx
	_ = p.GetFloat() // ury
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleBeginCompatibility implements the BX operator
func handleBeginCompatibility(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleEndCompatibility implements the EX operator
func handleEndCompatibility(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleRawContent implements the %raw% special operator
func handleRawContent(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetString() // raw content
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleInlineImage implements the %image% special operator
func handleInlineImage(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetDict()   // image parameters
	_ = p.GetString() // image data
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}
```

### Step 4: Register handlers

Add to handlers map:

```go
	// Shading
	"sh": handleShading,

	// XObjects
	"Do": handleXObject,

	// Marked content
	"MP":  handleMarkedContentPoint,
	"DP":  handleMarkedContentPointWithProperties,
	"BMC": handleBeginMarkedContent,
	"BDC": handleBeginMarkedContentWithProperties,
	"EMC": handleEndMarkedContent,

	// Type 3 fonts
	"d0": handleType3d0,
	"d1": handleType3d1,

	// Compatibility
	"BX": handleBeginCompatibility,
	"EX": handleEndCompatibility,

	// Special operators
	"%raw%":   handleRawContent,
	"%image%": handleInlineImage,
```

### Step 5: Run tests

Run: `go test ./go-pdf/graphics/operator/ -v`
Expected: PASS

### Step 6: Commit

```bash
git add go-pdf/graphics/operator/misc.go go-pdf/graphics/operator/misc_test.go go-pdf/graphics/operator/apply.go
git commit -m "feat(operator): implement misc operators (sh, Do, MP, DP, BMC, BDC, EMC, d0, d1, BX, EX, %raw%, %image%)"
```

---

## Task 8: Integration Tests and Documentation

**Files:**
- Create: `go-pdf/graphics/operator/integration_test.go`
- Modify: `go-pdf/graphics/operator/apply.go` (add package documentation)

### Step 1: Write integration tests

Create `go-pdf/graphics/operator/integration_test.go`:

```go
package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

func TestIntegration_PathWithStroke(t *testing.T) {
	state := &State{CurrentObject: objPage}
	res := &resource.Resource{}

	ops := []Operator{
		{Name: "w", Args: []pdf.Native{pdf.Real(2.0)}},
		{Name: "m", Args: []pdf.Native{pdf.Real(10.0), pdf.Real(10.0)}},
		{Name: "l", Args: []pdf.Native{pdf.Real(100.0), pdf.Real(10.0)}},
		{Name: "l", Args: []pdf.Native{pdf.Real(100.0), pdf.Real(100.0)}},
		{Name: "h", Args: nil},
		{Name: "S", Args: nil},
	}

	for i, op := range ops {
		if err := ApplyOperator(state, op, res); err != nil {
			t.Fatalf("operator %d (%s) failed: %v", i, op.Name, err)
		}
	}

	// Verify dependencies
	expected := graphics.StateLineWidth | graphics.StateLineJoin |
		graphics.StateLineDash | graphics.StateStrokeColor
	if state.In&expected != expected {
		t.Errorf("missing expected dependencies, In = %v", state.In)
	}

	// LineCap should not be needed (closed path, no dashes)
	if state.In&graphics.StateLineCap != 0 {
		t.Error("LineCap marked but not needed")
	}

	// Verify outputs
	if state.Out&graphics.StateLineWidth == 0 {
		t.Error("LineWidth not in Out")
	}
}

func TestIntegration_TextRenderingDependencies(t *testing.T) {
	state := &State{CurrentObject: objPage}
	mockFont := &mockFontInstance{}
	res := &resource.Resource{
		Font: map[pdf.Name]font.Instance{
			"F1": mockFont,
		},
	}

	ops := []Operator{
		{Name: "BT", Args: nil},
		{Name: "Tf", Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)}},
		{Name: "Tr", Args: []pdf.Native{pdf.Integer(1)}}, // Stroke mode
		{Name: "Tj", Args: []pdf.Native{pdf.String("Hello")}},
		{Name: "ET", Args: nil},
	}

	for i, op := range ops {
		if err := ApplyOperator(state, op, res); err != nil {
			t.Fatalf("operator %d (%s) failed: %v", i, op.Name, err)
		}
	}

	// Text stroke mode should require stroke color and line parameters
	if state.In&graphics.StateStrokeColor == 0 {
		t.Error("StrokeColor not marked for stroke rendering mode")
	}
	if state.In&graphics.StateLineWidth == 0 {
		t.Error("LineWidth not marked for stroke rendering mode")
	}
}

func TestIntegration_GraphicsStateStack(t *testing.T) {
	state := &State{CurrentObject: objPage}
	res := &resource.Resource{}

	// Set line width
	op1 := Operator{Name: "w", Args: []pdf.Native{pdf.Real(2.0)}}
	if err := ApplyOperator(state, op1, res); err != nil {
		t.Fatalf("w failed: %v", err)
	}

	// Push state
	opQ := Operator{Name: "q", Args: nil}
	if err := ApplyOperator(state, opQ, res); err != nil {
		t.Fatalf("q failed: %v", err)
	}

	savedOut := state.Out

	// Modify state
	op2 := Operator{Name: "w", Args: []pdf.Native{pdf.Real(5.0)}}
	if err := ApplyOperator(state, op2, res); err != nil {
		t.Fatalf("second w failed: %v", err)
	}

	// Pop state
	opPop := Operator{Name: "Q", Args: nil}
	if err := ApplyOperator(state, opPop, res); err != nil {
		t.Fatalf("Q failed: %v", err)
	}

	// Verify Out was restored
	if state.Out != savedOut {
		t.Errorf("Out not restored: got %v, want %v", state.Out, savedOut)
	}

	// Verify LineWidth was restored
	if state.Param.LineWidth != 2.0 {
		t.Errorf("LineWidth = %v, want 2.0", state.Param.LineWidth)
	}
}
```

### Step 2: Run integration tests

Run: `go test ./go-pdf/graphics/operator/ -run TestIntegration -v`
Expected: PASS

### Step 3: Add package documentation

Add to top of `go-pdf/graphics/operator/apply.go` after package declaration:

```go
// Package operator provides content stream operator handling.
//
// The ApplyOperator function analyzes PDF content stream operators and tracks
// how they modify graphics state. This supports both reading existing PDF files
// and implementing operator-based graphics writing.
//
// State tracking uses In/Out bit masks:
//   - In: External dependencies (accumulates, never restored by Q)
//   - Out: Modified parameters (saved/restored by q/Q)
//
// Once a parameter is in Out, subsequent operators reading it don't add it to In.
```

### Step 4: Run all tests with coverage

Run: `go test ./go-pdf/graphics/operator/ -cover -v`
Expected: High coverage (>80%)

### Step 5: Commit

```bash
git add go-pdf/graphics/operator/integration_test.go go-pdf/graphics/operator/apply.go
git commit -m "test(operator): add integration tests and package documentation"
```

---

## Task 9: Final Validation and Build Check

**Files:**
- All operator package files

### Step 1: Run full test suite

Run: `go test ./go-pdf/graphics/operator/... -v`
Expected: All PASS

### Step 2: Check for compilation errors in dependent packages

Run: `go test ./go-pdf/... -run NONE`
Expected: No build errors

### Step 3: Run goimports to fix imports

Run: `goimports -w ./go-pdf/graphics/operator/`
Expected: Clean formatting

### Step 4: Verify against design document

Check that all 73 operators from design are implemented:
- Graphics state (11): q, Q, cm, w, J, j, M, d, ri, i, gs ✓
- Path construction (7): m, l, c, v, y, h, re ✓
- Path painting (12): S, s, f, F, f*, B, B*, b, b*, n, W, W* ✓
- Text objects (2): BT, ET ✓
- Text state (7): Tc, Tw, Tz, TL, Tf, Tr, Ts ✓
- Text positioning (4): Td, TD, Tm, T* ✓
- Text showing (4): Tj, TJ, ', " ✓
- Color spaces (2): CS, cs ✓
- Generic color (4): SC, SCN, sc, scn ✓
- Device colors (6): G, g, RG, rg, K, k ✓
- Misc (14): sh, Do, MP, DP, BMC, BDC, EMC, d0, d1, BX, EX, %raw%, %image% ✓

Total: 73 operators ✓

### Step 5: Final commit

```bash
git add -A
git commit -m "feat(operator): complete ApplyOperator implementation

Implements all 73 PDF content stream operators with state tracking:
- Graphics state operators (q/Q, cm, line parameters, ExtGState)
- Path construction and painting (m, l, c, S, f, B, etc.)
- Text objects and operators (BT/ET, Tf, Tj, positioning)
- Color operators (device colors, color spaces)
- Miscellaneous (XObjects, shading, marked content, special ops)

Includes comprehensive unit and integration tests.
See docs/plans/2025-11-19-applyoperator-design.md for design."
```

---

## Success Criteria

- [ ] All 73 operators implemented with handlers
- [ ] argParser provides clean argument extraction
- [ ] State tracking (In/Out) correctly handles dependencies
- [ ] Graphics state stack (q/Q) saves and restores state
- [ ] CurrentObject state machine enforces context validation
- [ ] Conditional dependencies (LineCap, text rendering mode) work correctly
- [ ] All unit tests pass
- [ ] Integration tests verify realistic operator sequences
- [ ] No compilation errors in dependent packages
- [ ] Code formatted with goimports

## Notes for Implementation

- Use `go doc ./go-pdf/graphics/ Parameters` to see the full Parameters struct
- Use `go doc ./go-pdf/graphics/ StateBits` to see all state bit constants
- Check existing operator files for examples of validation and error handling
- Resource resolution errors should be descriptive but not prefixed with operator names
- Follow CLAUDE.md conventions: lowercase error messages, no operator prefixes
- Path state fields (CurrentX, StartX, etc.) are NOT saved by q/Q per PDF spec
- TextMatrix is reset at BT and cleared at ET per PDF spec section 9.4.1
