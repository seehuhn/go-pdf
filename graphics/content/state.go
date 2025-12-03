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

	"seehuhn.de/go/pdf/graphics"
)

// State tracks graphics state during content stream building and writing.
type State struct {
	// Param contains current parameter values.
	// Only parameters with Known bit set have reliable values.
	Param graphics.Parameters

	// Set indicates which parameters have values (either known or inherited).
	Set graphics.StateBits

	// Known indicates which parameters have known values (subset of Set).
	// Known ⊆ Set always holds.
	Known graphics.StateBits

	// UsedUnknown tracks which Set-Unknown parameters were used.
	// This indicates inherited dependencies for forms/patterns.
	UsedUnknown graphics.StateBits

	// Stack holds saved states for q/Q operators.
	Stack []SavedState

	// CurrentObject is the current graphics object context.
	CurrentObject Object

	// MaxStackDepth tracks the highest nesting depth reached.
	MaxStackDepth int

	// Nesting tracks paired operators (q/Q, BT/ET, BMC/EMC, BX/EX).
	Nesting []PairType

	// CompatibilityDepth counts BX/EX nesting for fast "inside compatibility" checks.
	CompatibilityDepth int

	// Type3Mode tracks d0/d1 for Type 3 content streams.
	Type3Mode Type3Mode
}

// PairType identifies a type of paired operator.
type PairType byte

const (
	PairQ   PairType = iota + 1 // q ... Q
	PairBT                      // BT ... ET
	PairBMC                     // BMC/BDC ... EMC
	PairBX                      // BX ... EX
)

// SavedState holds state saved by the q operator.
type SavedState struct {
	Param *graphics.Parameters
	Set   graphics.StateBits
	Known graphics.StateBits
}

// NewStateForContent creates a State initialized for the given content type.
func NewStateForContent(ct Type) *State {
	s := &State{
		Param:         *graphics.NewState().Parameters,
		CurrentObject: ObjPage,
	}

	switch ct {
	case PageContent:
		// PDF-defined defaults are Set and Known, except font
		s.Set = initializedStateBits
		s.Known = initializedStateBits
	case FormContent, PatternContent:
		// All parameters inherited (Set but not Known)
		s.Set = graphics.AllStateBits
		s.Known = 0
	case Type3Content:
		// All parameters inherited (Set but not Known)
		s.Set = graphics.AllStateBits
		s.Known = 0
		s.CurrentObject = ObjType3Start
	}

	return s
}

// initializedStateBits lists parameters with PDF-defined defaults.
// This equals AllStateBits & ^StateTextFont (font has no default).
const initializedStateBits = graphics.StateStrokeColor | graphics.StateFillColor |
	graphics.StateTextCharacterSpacing | graphics.StateTextWordSpacing |
	graphics.StateTextHorizontalScaling | graphics.StateTextLeading |
	graphics.StateTextRenderingMode | graphics.StateTextRise |
	graphics.StateTextKnockout | graphics.StateLineWidth | graphics.StateLineCap |
	graphics.StateLineJoin | graphics.StateMiterLimit | graphics.StateLineDash |
	graphics.StateRenderingIntent | graphics.StateStrokeAdjustment |
	graphics.StateBlendMode | graphics.StateSoftMask | graphics.StateStrokeAlpha |
	graphics.StateFillAlpha | graphics.StateAlphaSourceFlag |
	graphics.StateBlackPointCompensation | graphics.StateOverprint |
	graphics.StateOverprintMode | graphics.StateFlatnessTolerance

// IsSet returns true if all specified parameters are Set.
func (s *State) IsSet(bits graphics.StateBits) bool {
	return s.Set&bits == bits
}

// IsKnown returns true if all specified parameters are Known.
func (s *State) IsKnown(bits graphics.StateBits) bool {
	return s.Known&bits == bits
}

// MarkKnown marks parameters as Known (implies Set).
func (s *State) MarkKnown(bits graphics.StateBits) {
	s.Set |= bits
	s.Known |= bits
}

// MarkUsedIfUnknown records usage of Set-Unknown parameters.
// This tracks inherited dependencies for forms/patterns.
func (s *State) MarkUsedIfUnknown(bits graphics.StateBits) {
	// Only track params that are Set but not Known
	setUnknown := s.Set &^ s.Known
	s.UsedUnknown |= bits & setUnknown
}

// Push saves the current graphics state (implements q operator).
func (s *State) Push() error {
	s.Stack = append(s.Stack, SavedState{
		Param: s.Param.Clone(),
		Set:   s.Set,
		Known: s.Known,
	})
	if len(s.Stack) > s.MaxStackDepth {
		s.MaxStackDepth = len(s.Stack)
	}
	s.Nesting = append(s.Nesting, PairQ)
	return nil
}

// Pop restores the previous graphics state (implements Q operator).
func (s *State) Pop() error {
	if err := s.expectNesting(PairQ, "Q"); err != nil {
		return err
	}
	saved := s.Stack[len(s.Stack)-1]
	s.Stack = s.Stack[:len(s.Stack)-1]

	s.Param = *saved.Param
	s.Set = saved.Set
	s.Known = saved.Known
	// Note: UsedUnknown is NOT restored (cumulative across stream)

	return nil
}

// TextBegin transitions to text object context (implements BT operator).
func (s *State) TextBegin() error {
	if s.CurrentObject != ObjPage {
		return errors.New("BT: expected page context, got " + s.CurrentObject.String())
	}
	s.CurrentObject = ObjText
	s.Nesting = append(s.Nesting, PairBT)
	return nil
}

// TextEnd transitions from text object context (implements ET operator).
func (s *State) TextEnd() error {
	if err := s.expectNesting(PairBT, "ET"); err != nil {
		return err
	}
	s.CurrentObject = ObjPage
	return nil
}

// MarkedContentBegin starts a marked content section (implements BMC/BDC operators).
func (s *State) MarkedContentBegin() {
	s.Nesting = append(s.Nesting, PairBMC)
}

// MarkedContentEnd ends a marked content section (implements EMC operator).
func (s *State) MarkedContentEnd() error {
	return s.expectNesting(PairBMC, "EMC")
}

// CompatibilityBegin starts a compatibility section (implements BX operator).
func (s *State) CompatibilityBegin() {
	s.Nesting = append(s.Nesting, PairBX)
	s.CompatibilityDepth++
}

// CompatibilityEnd ends a compatibility section (implements EX operator).
func (s *State) CompatibilityEnd() error {
	if err := s.expectNesting(PairBX, "EX"); err != nil {
		return err
	}
	s.CompatibilityDepth--
	return nil
}

// InCompatibilitySection returns true if inside a BX/EX compatibility section.
func (s *State) InCompatibilitySection() bool {
	return s.CompatibilityDepth > 0
}

// Type3ColoredGlyph handles the d0 operator state transition.
// In colored glyphs, color operators are allowed.
func (s *State) Type3ColoredGlyph() error {
	if s.CurrentObject != ObjType3Start {
		return errors.New("d0: must be first operator in Type 3 glyph")
	}
	s.CurrentObject = ObjPage
	s.Type3Mode = Type3ModeD0
	return nil
}

// Type3UncoloredGlyph handles the d1 operator state transition.
// In uncolored glyphs, color operators are forbidden.
func (s *State) Type3UncoloredGlyph() error {
	if s.CurrentObject != ObjType3Start {
		return errors.New("d1: must be first operator in Type 3 glyph")
	}
	s.CurrentObject = ObjPage
	s.Type3Mode = Type3ModeD1
	return nil
}

// CheckOperatorAllowed verifies that the given operator can be used in the current state.
func (s *State) CheckOperatorAllowed(name OpName) error {
	if s.CurrentObject == ObjType3Start {
		switch name {
		case OpType3ColoredGlyph, OpType3UncoloredGlyph:
			// allowed, will transition the state
		default:
			return errors.New("Type 3 glyph must start with d0 or d1")
		}
	}
	return nil
}

// expectNesting checks that the top of the nesting stack matches the expected type.
func (s *State) expectNesting(expected PairType, opName string) error {
	if len(s.Nesting) == 0 {
		return errors.New(opName + ": no matching opening operator")
	}
	top := s.Nesting[len(s.Nesting)-1]
	if top != expected {
		return errors.New(opName + ": improper nesting, expected " + expected.String() + " but got " + top.String())
	}
	s.Nesting = s.Nesting[:len(s.Nesting)-1]
	return nil
}

// CanClose checks end-of-stream validity.
func (s *State) CanClose() error {
	if len(s.Nesting) > 0 {
		return errors.New("unclosed operators: " + s.Nesting[len(s.Nesting)-1].String())
	}
	if s.CurrentObject != ObjPage {
		return errors.New("invalid end state: " + s.CurrentObject.String())
	}
	return nil
}

func (p PairType) String() string {
	switch p {
	case PairQ:
		return "q/Q"
	case PairBT:
		return "BT/ET"
	case PairBMC:
		return "BMC/EMC"
	case PairBX:
		return "BX/EX"
	default:
		return "unknown"
	}
}
