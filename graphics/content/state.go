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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/state"
)

// State tracks graphics state during content stream building and writing.
type State struct {
	// Usable indicates which graphics parameters have usable values.
	// If the corresponding GState.Set bit is also set, a concrete value is available.
	// Otherwise, the value is inherited from the surrounding context.
	Usable state.Bits

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
	Usable state.Bits
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
		s.Usable = state.AllBits
		s.GState.Set = 0
	case PatternUncolored:
		// All parameters inherited (Usable but not Set)
		s.Usable = state.AllBits
		s.GState.Set = 0
		s.ColorOpsForbidden = true
	case Glyph:
		// All parameters inherited (Usable but not Set)
		s.Usable = state.AllBits
		s.GState.Set = 0
		s.CurrentObject = ObjType3Start
	}

	return s
}

// initializedStateBits lists parameters where PDF defines initial values
// at the start of a page content stream.  This includes all parameters except
// the text font and font size.
const initializedStateBits = state.AllBits & ^state.TextFont

// IsUsable returns true if all specified parameters are known to have usable values.
func (s *State) IsUsable(bits state.Bits) bool {
	return s.Usable&bits == bits
}

// IsSet returns true if all specified parameters are set to known values.
func (s *State) IsSet(bits state.Bits) bool {
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
	s.Usable &^= state.TextMatrix
	s.GState.Set &^= state.TextMatrix
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
		if isStrokeOp(name) && s.GState.AllSubpathsClosed && len(s.GState.DashPattern) == 0 {
			requires &^= state.LineCap
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
		// starting new subpath while current is open
		if !s.GState.ThisSubpathClosed {
			s.GState.AllSubpathsClosed = false
		}
		s.GState.ThisSubpathClosed = true // positioned but no segments yet
	case OpRectangle:
		// rectangle is always a closed subpath
		if !s.GState.ThisSubpathClosed {
			s.GState.AllSubpathsClosed = false
		}
		s.GState.ThisSubpathClosed = true
	case OpLineTo, OpCurveTo, OpCurveToV, OpCurveToY:
		s.GState.ThisSubpathClosed = false
	case OpClosePath:
		s.GState.ThisSubpathClosed = true

	// Path painting operators reset path state
	case OpStroke, OpCloseAndStroke, OpFill, OpFillCompat, OpFillEvenOdd,
		OpFillAndStroke, OpFillAndStrokeEvenOdd, OpCloseFillAndStroke,
		OpCloseFillAndStrokeEvenOdd, OpEndPath:
		// finalize current subpath
		if !s.GState.ThisSubpathClosed {
			s.GState.AllSubpathsClosed = false
		}
		// reset for next path
		s.GState.AllSubpathsClosed = true
		s.GState.ThisSubpathClosed = true

		// Note: DashPattern is updated in GState.DashPattern by applyOperatorToParams
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
