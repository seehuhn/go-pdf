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

	"seehuhn.de/go/pdf/graphics"
)

// State tracks graphics state during content stream building and writing.
type State struct {
	// Param contains current parameter values.
	// Only parameters with Known bit set are valid.
	Param graphics.Parameters

	// Set indicates which graphics parameters have usable values.
	// For the corresponding bit in Known is also set, the value can
	// be found in Param.  Otherwise, the value is inherited from
	// the surrounding context at the time the content stream is used.
	//
	// Commands list SetLineWidth update this field.
	Set graphics.StateBits

	// Known indicates values in Param are valid.
	// This must be a subset of Set.
	//
	// Commands list SetLineWidth update this field.
	Known graphics.StateBits

	// FromContext tracks which graphics parameters were used from inherited
	// context. This is updated, every time a graphics operator uses a
	// parameter which is not listed in Known.
	//
	// Commands like Stroke and Fill update this field.
	FromContext graphics.StateBits

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
	Param *graphics.Parameters
	Set   graphics.StateBits
	Known graphics.StateBits
}

// NewState creates a State initialized for the given content type.
func NewState(ct Type) *State {
	s := &State{
		Param:         *graphics.NewState().Parameters,
		CurrentObject: ObjPage,
	}

	switch ct {
	case Page:
		// PDF-defined defaults are Set and Known, except font
		s.Set = initializedStateBits
		s.Known = initializedStateBits
	case Form, Pattern:
		// All parameters inherited (Set but not Known)
		s.Set = graphics.AllStateBits
		s.Known = 0
	case Glyph:
		// All parameters inherited (Set but not Known)
		s.Set = graphics.AllStateBits
		s.Known = 0
		s.CurrentObject = ObjType3Start
	}

	return s
}

// initializedStateBits lists parameters where PDF defines initial values
// at the start of a page content stream.  This includes all parameters except
// the text font and font size.
const initializedStateBits = graphics.AllStateBits & ^graphics.StateTextFont

// IsSet returns true if all specified parameters are Set.
func (s *State) IsSet(bits graphics.StateBits) bool {
	return s.Set&bits == bits
}

// IsKnown returns true if all specified parameters are Known.
func (s *State) IsKnown(bits graphics.StateBits) bool {
	return s.Known&bits == bits
}

// MarkAsSet records that parameters were set by a graphics operator.
func (s *State) MarkAsSet(bits graphics.StateBits) {
	s.Set |= bits
	s.Known |= bits
}

// MarkAsUsed records that parameters were used by a graphics operator.
func (s *State) MarkAsUsed(bits graphics.StateBits) {
	setUnknown := s.Set &^ s.Known
	s.FromContext |= bits & setUnknown
}

// Push saves the current graphics state (for the q operator).
func (s *State) Push() error {
	s.stack = append(s.stack, savedState{
		Param: s.Param.Clone(),
		Set:   s.Set,
		Known: s.Known,
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

	s.Param = *saved.Param
	s.Set = saved.Set
	s.Known = saved.Known
	// Note: UsedUnknown is NOT restored (cumulative across stream)

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

// GlyphColored handles the d0 operator state transition.
// In colored glyphs, color operators are allowed.
func (s *State) GlyphColored() error {
	if s.CurrentObject != ObjType3Start {
		return errors.New("d0: must be first operator in a Type 3 glyph")
	}
	s.CurrentObject = ObjPage
	return nil
}

// GlyphUncolored handles the d1 operator state transition.
// In uncolored glyphs, color operators are forbidden.
func (s *State) GlyphUncolored() error {
	if s.CurrentObject != ObjType3Start {
		return errors.New("d1: must be first operator in a Type 3 glyph")
	}
	s.CurrentObject = ObjPage
	s.ColorOpsForbidden = true
	return nil
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

// ApplyTransition updates CurrentObject based on the operator's state transition rule.
func (s *State) ApplyTransition(name OpName) {
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
