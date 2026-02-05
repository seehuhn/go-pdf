// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package content

import (
	"errors"
	"fmt"
	"slices"

	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// State tracks graphics state during content stream building and writing.
type State struct {
	// Usable indicates which graphics parameters have usable values.
	// If the corresponding GState.Set bit is also set, a concrete value is available.
	// Otherwise, the value is inherited from the surrounding context.
	Usable graphics.Bits

	// ColorOpsForbidden is set for uncolored Type 3 glyphs (d1) and
	// uncolored tiling patterns (PaintType 2).
	ColorOpsForbidden bool

	// CurrentObject is the current graphics object context.
	CurrentObject Object

	// MaxStackDepth tracks the highest nesting depth reached.
	MaxStackDepth int

	// stack holds saved states for q/Q operators.
	stack []savedState

	// nesting tracks paired operators (q/Q, BT/ET, BMC/EMC, BX/EX).
	nesting []pairType

	// compatibilityDepth counts BX/EX nesting for fast "inside compatibility" checks.
	compatibilityDepth int

	// GState holds the full graphics state parameters.
	GState *graphics.State

	// Resources is the resource dictionary for resolving named resources.
	Resources *Resources

	// StartX, StartY is the starting point of the current subpath, in user space.
	// Not saved/restored by q/Q.
	StartX, StartY float64

	// CurrentX, CurrentY is the current point, in user space.
	// Not saved/restored by q/Q.
	CurrentX, CurrentY float64

	// currentPath accumulates path data from construction operators.
	// Not saved/restored by q/Q (the current path is not part of the graphics state).
	currentPath path.Data

	// paintedPath holds the path consumed by the most recent paint operator.
	paintedPath path.Data

	// pendingClip is true when W or W* has been issued but the paint operator
	// has not yet fired.
	pendingClip bool

	// pendingClipEvenOdd records the fill rule for the pending clip.
	pendingClipEvenOdd bool
}

// pairType identifies a type of paired operator.
type pairType byte

const (
	pairQ   pairType = iota + 1 // q ... Q
	pairBT                      // BT ... ET
	pairBMC                     // BMC/BDC ... EMC
	pairBX                      // BX ... EX
)

// savedState holds state saved by the q operator.
type savedState struct {
	Usable graphics.Bits
	GState *graphics.State
}

// NewState creates a State initialized for the given content type.
// The res argument is mandatory and must not be nil.
func NewState(ct Type, res *Resources) *State {
	if res == nil {
		panic("NewState: res must not be nil")
	}

	gstate := graphics.NewState()
	s := &State{
		CurrentObject: ObjPage,
		GState:        &gstate,
		Resources:     res,
	}

	switch ct {
	case Page:
		// PDF-defined defaults are Set and Known, except font
		s.Usable = initializedStateBits
		// GState.Set already has initializedStateBits from graphics.NewState()
	case Form, PatternColored:
		// All parameters inherited (Usable but not Set)
		s.Usable = graphics.AllBits
		s.GState.Set = 0
	case TransparencyGroup:
		// Inherit all state, but reset blend mode, alpha constants, and soft mask
		// to initial values (PDF 32000-1:2008, section 11.6.6).
		s.Usable = graphics.AllBits
		s.GState.Set = graphics.StateBlendMode | graphics.StateStrokeAlpha | graphics.StateFillAlpha | graphics.StateSoftMask
	case PatternUncolored:
		// All parameters inherited (Usable but not Set)
		s.Usable = graphics.AllBits
		s.GState.Set = 0
		s.ColorOpsForbidden = true
	case Glyph:
		// All parameters inherited (Usable but not Set)
		s.Usable = graphics.AllBits
		s.GState.Set = 0
		s.CurrentObject = ObjType3Start
	}

	return s
}

// initializedStateBits lists parameters where PDF defines initial values
// at the start of a page content stream.  This includes all parameters except
// the text font and font size.
const initializedStateBits = graphics.AllBits & ^graphics.StateTextFont

// IsUsable returns true if all specified parameters are known to have usable values.
func (s *State) IsUsable(bits graphics.Bits) bool {
	return s.Usable&bits == bits
}

// IsSet returns true if all specified parameters are set to known values.
func (s *State) IsSet(bits graphics.Bits) bool {
	return s.GState.Set&bits == bits
}

// Push saves the current graphics state (for the q operator).
func (s *State) Push() error {
	s.stack = append(s.stack, savedState{
		Usable: s.Usable,
		GState: s.GState.Clone(),
	})
	if len(s.stack) > s.MaxStackDepth {
		s.MaxStackDepth = len(s.stack)
	}
	s.nesting = append(s.nesting, pairQ)
	return nil
}

// Pop restores the previous graphics state (for the Q operator).
func (s *State) Pop() error {
	if err := s.expectNesting(pairQ, "Q"); err != nil {
		return err
	}
	saved := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]

	s.Usable = saved.Usable
	s.GState = saved.GState

	return nil
}

// TextBegin transitions to text object context (for the BT operator).
func (s *State) TextBegin() error {
	if s.CurrentObject != ObjPage {
		return errors.New("BT: expected page context, got " + s.CurrentObject.String())
	}
	s.CurrentObject = ObjText
	s.nesting = append(s.nesting, pairBT)
	return nil
}

// TextEnd transitions from text object context (for the ET operator).
func (s *State) TextEnd() error {
	if err := s.expectNesting(pairBT, "ET"); err != nil {
		return err
	}
	s.CurrentObject = ObjPage
	// The text matrix does not persist between text objects (ISO 32000-2:2020, 9.4.1)
	s.Usable &^= graphics.StateTextMatrix
	s.GState.Set &^= graphics.StateTextMatrix
	return nil
}

// MarkedContentBegin starts a marked content section (for the BMC/BDC operators).
func (s *State) MarkedContentBegin() {
	s.nesting = append(s.nesting, pairBMC)
}

// MarkedContentEnd ends a marked content section (for the EMC operator).
func (s *State) MarkedContentEnd() error {
	return s.expectNesting(pairBMC, "EMC")
}

// CompatibilityBegin starts a compatibility section (for the BX operator).
func (s *State) CompatibilityBegin() {
	s.nesting = append(s.nesting, pairBX)
	s.compatibilityDepth++
}

// CompatibilityEnd ends a compatibility section (for the EX operator).
func (s *State) CompatibilityEnd() error {
	if err := s.expectNesting(pairBX, "EX"); err != nil {
		return err
	}
	s.compatibilityDepth--
	return nil
}

// InCompatibilitySection returns true if we are inside a BX/EX compatibility section.
func (s *State) InCompatibilitySection() bool {
	return s.compatibilityDepth > 0
}

// CheckOperatorAllowed verifies that the given operator can be used in the current state.
func (s *State) CheckOperatorAllowed(name OpName) error {
	info, ok := operators[name]
	if !ok {
		return nil // unknown operator (allowed in BX/EX sections)
	}

	if s.CurrentObject&info.Allowed == 0 {
		return fmt.Errorf("%s: not allowed in %s context", name, s.CurrentObject)
	}
	return nil
}

// ApplyOperator validates and applies all state changes for an operator.
// It checks if the operator is allowed, validates required state,
// applies state-modifying changes (q/Q, BT/ET, etc.), and updates state bits.
func (s *State) ApplyOperator(name OpName, args []pdf.Object) error {
	if err := s.CheckOperatorAllowed(name); err != nil {
		return err
	}

	info := operators[name]
	if info != nil {
		requires := info.Requires

		// Conditional LineCap relaxation: not needed for closed paths without dashes
		if isStrokeOp(name) && s.allSubpathsClosed() && len(s.GState.DashPattern) == 0 {
			requires &^= graphics.StateLineCap
		}

		// Validate requirements
		if requires != 0 {
			missing := requires &^ s.Usable
			if missing != 0 {
				return fmt.Errorf("%s: required state not set: %v", name, missing)
			}
		}

		// Update state bits
		if info.Sets != 0 {
			s.Usable |= info.Sets
			s.GState.Set |= info.Sets
		}
	}

	return s.ApplyStateChanges(name, args)
}

// allSubpathsClosed reports whether every subpath in the current path is closed.
// A subpath with no drawing segments (bare MoveTo) counts as closed.
func (s *State) allSubpathsClosed() bool {
	hasSegments := false
	for _, cmd := range s.currentPath.Cmds {
		switch cmd {
		case path.CmdMoveTo:
			if hasSegments {
				return false
			}
		case path.CmdLineTo, path.CmdQuadTo, path.CmdCubeTo:
			hasSegments = true
		case path.CmdClose:
			hasSegments = false
		}
	}
	return !hasSegments
}

// PaintedPath returns the path consumed by the most recent paint operator.
// The returned pointer is valid until the next call to ApplyStateChanges
// with a paint operator.
func (s *State) PaintedPath() *path.Data {
	return &s.paintedPath
}

// needsClose returns true if the operator implicitly closes the path.
func needsClose(name OpName) bool {
	switch name {
	case OpCloseAndStroke, OpCloseFillAndStroke, OpCloseFillAndStrokeEvenOdd:
		return true
	}
	return false
}

// isStrokeOp returns true if the operator is a stroking operation.
func isStrokeOp(name OpName) bool {
	switch name {
	case OpStroke, OpCloseAndStroke, OpFillAndStroke, OpFillAndStrokeEvenOdd,
		OpCloseFillAndStroke, OpCloseFillAndStrokeEvenOdd:
		return true
	}
	return false
}

// ApplyStateChanges applies state-modifying changes for an operator.
// This handles q/Q, BT/ET, BMC/EMC, BX/EX, d1, path state, and dash pattern tracking.
//
// Call this after the operator has been validated with CheckOperatorAllowed.
func (s *State) ApplyStateChanges(name OpName, args []pdf.Object) error {
	var err error
	switch name {
	case OpPushGraphicsState:
		err = s.Push()
	case OpPopGraphicsState:
		err = s.Pop()
	case OpTextBegin:
		err = s.TextBegin()
	case OpTextEnd:
		err = s.TextEnd()
	case OpBeginMarkedContent, OpBeginMarkedContentWithProperties:
		s.MarkedContentBegin()
	case OpEndMarkedContent:
		err = s.MarkedContentEnd()
	case OpBeginCompatibility:
		s.CompatibilityBegin()
	case OpEndCompatibility:
		err = s.CompatibilityEnd()
	case OpType3UncoloredGlyph:
		s.ColorOpsForbidden = true

	// Path construction operators
	case OpMoveTo:
		if p, ok := getVec2(args, 0); ok {
			s.currentPath.MoveTo(p)
			s.StartX, s.StartY = p.X, p.Y
			s.CurrentX, s.CurrentY = p.X, p.Y
		}
	case OpLineTo:
		if p, ok := getVec2(args, 0); ok {
			s.currentPath.LineTo(p)
			s.CurrentX, s.CurrentY = p.X, p.Y
		}
	case OpCurveTo:
		if p1, ok := getVec2(args, 0); ok {
			if p2, ok := getVec2(args, 2); ok {
				if p3, ok := getVec2(args, 4); ok {
					s.currentPath.CubeTo(p1, p2, p3)
					s.CurrentX, s.CurrentY = p3.X, p3.Y
				}
			}
		}
	case OpCurveToV:
		if p2, ok := getVec2(args, 0); ok {
			if p3, ok := getVec2(args, 2); ok {
				p1 := vec.Vec2{X: s.CurrentX, Y: s.CurrentY}
				s.currentPath.CubeTo(p1, p2, p3)
				s.CurrentX, s.CurrentY = p3.X, p3.Y
			}
		}
	case OpCurveToY:
		if p1, ok := getVec2(args, 0); ok {
			if p3, ok := getVec2(args, 2); ok {
				s.currentPath.CubeTo(p1, p3, p3)
				s.CurrentX, s.CurrentY = p3.X, p3.Y
			}
		}
	case OpClosePath:
		s.currentPath.Close()
		s.CurrentX, s.CurrentY = s.StartX, s.StartY
	case OpRectangle:
		if p, ok := getVec2(args, 0); ok {
			if sz, ok := getVec2(args, 2); ok {
				s.currentPath.MoveTo(p)
				s.currentPath.LineTo(vec.Vec2{X: p.X + sz.X, Y: p.Y})
				s.currentPath.LineTo(vec.Vec2{X: p.X + sz.X, Y: p.Y + sz.Y})
				s.currentPath.LineTo(vec.Vec2{X: p.X, Y: p.Y + sz.Y})
				s.currentPath.Close()
				s.StartX, s.StartY = p.X, p.Y
				s.CurrentX, s.CurrentY = p.X, p.Y
			}
		}

	// clipping path operators
	case OpClipNonZero:
		s.pendingClip = true
		s.pendingClipEvenOdd = false
	case OpClipEvenOdd:
		s.pendingClip = true
		s.pendingClipEvenOdd = true

	// path painting operators reset path state
	case OpStroke, OpCloseAndStroke, OpFill, OpFillCompat, OpFillEvenOdd,
		OpFillAndStroke, OpFillAndStrokeEvenOdd, OpCloseFillAndStroke,
		OpCloseFillAndStrokeEvenOdd, OpEndPath:
		// close path for close-first operators (s, b, b*)
		if needsClose(name) {
			n := len(s.currentPath.Cmds)
			if n > 0 && s.currentPath.Cmds[n-1] != path.CmdClose {
				s.currentPath.Close()
				s.CurrentX, s.CurrentY = s.StartX, s.StartY
			}
		}

		// capture clip path if pending
		if s.pendingClip {
			clipData := &path.Data{
				Cmds:   slices.Clone(s.currentPath.Cmds),
				Coords: slices.Clone(s.currentPath.Coords),
			}
			s.GState.ClipPaths = append(s.GState.ClipPaths, graphics.ClipPath{
				Path:    clipData,
				EvenOdd: s.pendingClipEvenOdd,
				CTM:     s.GState.CTM,
			})
			s.pendingClip = false
		}

		// move path to paintedPath; swap reuses backing arrays
		s.paintedPath, s.currentPath = s.currentPath, s.paintedPath
		s.currentPath.Cmds = s.currentPath.Cmds[:0]
		s.currentPath.Coords = s.currentPath.Coords[:0]
	}
	if err != nil {
		return err
	}
	s.applyTransition(name)
	s.applyOperatorToParams(name, args)
	return nil
}

// applyTransition updates CurrentObject based on the operator's state transition rule.
func (s *State) applyTransition(name OpName) {
	if info, ok := operators[name]; ok && info.Transition != 0 {
		s.CurrentObject = info.Transition
	}
}

// expectNesting checks that the top of the nesting stack matches the expected type.
func (s *State) expectNesting(expected pairType, opName string) error {
	if len(s.nesting) == 0 {
		return errors.New(opName + ": no matching opening operator")
	}
	top := s.nesting[len(s.nesting)-1]
	if top != expected {
		return errors.New(opName + ": improper nesting, expected " + expected.String() + " but got " + top.String())
	}
	s.nesting = s.nesting[:len(s.nesting)-1]
	return nil
}

// CanClose returns an error if paired operators are unbalanced.
func (s *State) CanClose() error {
	if len(s.nesting) > 0 {
		return errors.New("unclosed operators: " + s.nesting[len(s.nesting)-1].String())
	}
	if s.CurrentObject != ObjPage {
		return errors.New("invalid end state: " + s.CurrentObject.String())
	}
	return nil
}

// ClosingOperators returns the operator names needed to close any open contexts.
// The operators are returned in the order they should be emitted.
func (s *State) ClosingOperators() []OpName {
	var ops []OpName

	// close path if open
	if s.CurrentObject == ObjPath || s.CurrentObject == ObjClippingPath {
		ops = append(ops, OpEndPath)
	}

	// close paired operators in reverse order
	for i := len(s.nesting) - 1; i >= 0; i-- {
		switch s.nesting[i] {
		case pairQ:
			ops = append(ops, OpPopGraphicsState)
		case pairBT:
			ops = append(ops, OpTextEnd)
		case pairBMC:
			ops = append(ops, OpEndMarkedContent)
		case pairBX:
			ops = append(ops, OpEndCompatibility)
		}
	}
	return ops
}

func (p pairType) String() string {
	switch p {
	case pairQ:
		return "q/Q"
	case pairBT:
		return "BT/ET"
	case pairBMC:
		return "BMC/EMC"
	case pairBX:
		return "BX/EX"
	default:
		return "unknown"
	}
}
