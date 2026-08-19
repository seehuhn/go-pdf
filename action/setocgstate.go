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

package action

import (
	"errors"
	"fmt"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/oc"
)

// PDF 2.0 sections: 12.6.2 12.6.4.13

// OCGOperation specifies how the state of optional content groups is changed.
//
// An OCGOperation written to a PDF file must be one of [OCGOperationON],
// [OCGOperationOFF], or [OCGOperationToggle].
type OCGOperation pdf.Name

const (
	// OCGOperationON turns the groups on.
	OCGOperationON OCGOperation = "ON"

	// OCGOperationOFF turns the groups off.
	OCGOperationOFF OCGOperation = "OFF"

	// OCGOperationToggle reverses the state of the groups.
	OCGOperationToggle OCGOperation = "Toggle"
)

// OCGStateChange applies an operation to a set of optional content groups.
type OCGStateChange struct {
	// Op is the operation to apply to Groups.  It must be one of
	// [OCGOperationON], [OCGOperationOFF], or [OCGOperationToggle].
	Op OCGOperation

	// Groups lists the optional content groups the operation applies to.
	// At least one group must be given.
	Groups []*oc.Group
}

// SetOCGState represents a set-OCG-state action that sets the state of
// optional content groups.
type SetOCGState struct {
	// State lists the changes to perform, in order.  A group may appear more
	// than once, in which case the last change to it wins.
	State []OCGStateChange

	// IgnoreRBGroups, when true, causes radio-button state relationships
	// between optional content groups to be ignored.
	//
	// This corresponds to the PreserveRB entry in the PDF specification, but
	// with inverted meaning.
	IgnoreRBGroups bool

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "SetOCGState".
// This implements the [pdf.Action] interface.
func (a *SetOCGState) ActionType() pdf.Name  { return TypeSetOCGState }
func (a *SetOCGState) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

// Apply changes the states of optional content groups as the action
// prescribes.  The changes are applied in order, so that the last change to a
// group wins, and each is recorded as a manual change, so that a later usage
// application does not undo what the document asked for.
//
// A toggle reverses the state in force when it runs: for a group this action
// has already changed, the state just set; otherwise the state the effective
// snapshot shows.  The snapshot should be the visibility the user sees, as
// [oc.ViewState.Effective] derives it for the current view; passing nil makes
// a toggle read the stored state instead, which for a group hidden by a usage
// application inverts a state the user never saw.
//
// Radio-button relationships between groups are honoured unless
// IgnoreRBGroups is set: switching a group on switches off the other members
// of its radio-button collections, and the group the action names is the one
// which survives.
//
// An operation outside the set of operations the specification defines is
// ignored.
func (a *SetOCGState) Apply(state *oc.ViewState, effective *oc.GroupStates) {
	for _, change := range a.State {
		for _, group := range change.Groups {
			var on bool
			switch change.Op {
			case OCGOperationON:
				on = true
			case OCGOperationOFF:
				on = false
			case OCGOperationToggle:
				switch {
				case state.IsManual(group):
					on = !state.IsOn(group)
				case effective != nil:
					on = !effective.IsOn(group)
				default:
					on = !state.IsOn(group)
				}
			default:
				continue
			}
			if a.IgnoreRBGroups {
				state.SetManualState(group, on)
			} else {
				state.Switch(group, on)
			}
		}
	}
}

func (a *SetOCGState) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "SetOCGState action", pdf.V1_5); err != nil {
		return nil, err
	}

	state := pdf.Array{}
	for _, change := range a.State {
		switch change.Op {
		case OCGOperationON, OCGOperationOFF, OCGOperationToggle:
		default:
			return nil, fmt.Errorf("invalid SetOCGState operation %q", change.Op)
		}
		if len(change.Groups) == 0 {
			return nil, errors.New("SetOCGState operation must have groups")
		}

		state = append(state, pdf.Name(change.Op))
		for _, group := range change.Groups {
			if group == nil {
				return nil, errors.New("SetOCGState action has nil group")
			}
			ref, err := rm.Embed(group)
			if err != nil {
				return nil, err
			}
			state = append(state, ref)
		}
	}

	dict := pdf.Dict{
		"S":     pdf.Name(TypeSetOCGState),
		"State": state,
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	// only write PreserveRB when false (true is the PDF default)
	if a.IgnoreRBGroups {
		dict["PreserveRB"] = pdf.Boolean(false)
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeSetOCGState(c pdf.Cursor, dict pdf.Dict) (*SetOCGState, error) {
	stateArray, err := pdf.Optional(c.Array(dict["State"]))
	if err != nil {
		return nil, err
	}

	// Each sequence in the array starts with an operation name, followed by
	// the groups the operation applies to.  Entries which are neither a
	// non-empty name nor a group are skipped, as are groups appearing before
	// the first operation.  The set of operations is closed, so a name from
	// outside it is dropped together with its groups; it still ends the
	// preceding sequence, matching a viewer which applies each name only
	// until the next one.
	var state []OCGStateChange
	cur := -1
	for _, obj := range stateArray {
		op, err := pdf.Optional(c.Name(obj))
		if err != nil {
			return nil, err
		}
		if op != "" {
			switch OCGOperation(op) {
			case OCGOperationON, OCGOperationOFF, OCGOperationToggle:
				state = append(state, OCGStateChange{Op: OCGOperation(op)})
				cur = len(state) - 1
			default:
				cur = -1
			}
			continue
		}
		if cur < 0 {
			continue
		}

		group, err := pdf.DecodeOptional(c, obj, oc.ExtractGroup)
		if err != nil {
			return nil, err
		} else if group == nil {
			continue
		}
		state[cur].Groups = append(state[cur].Groups, group)
	}

	// drop operations which ended up without groups
	state = slices.DeleteFunc(state, func(change OCGStateChange) bool {
		return len(change.Groups) == 0
	})
	if len(state) == 0 {
		state = nil
	}

	ignoreRB := false // default: preserve radio-button groups
	if dict["PreserveRB"] != nil {
		rb, _ := pdf.Optional(c.Boolean(dict["PreserveRB"]))
		ignoreRB = !bool(rb)
	}

	next, err := pdf.Decode(c, dict["Next"], DecodeActionList)
	if err != nil {
		return nil, err
	}

	return &SetOCGState{
		State:          state,
		IgnoreRBGroups: ignoreRB,
		Next:           next,
	}, nil
}
