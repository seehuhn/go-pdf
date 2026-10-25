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

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.6.2 12.6.4.11

// Hide represents a hide action that shows or hides annotations.
type Hide struct {
	// T specifies the annotation(s) to hide or show.
	// This is a [pdf.String] (the fully qualified name of a form field), a
	// [pdf.Dict] or [pdf.Reference] (an annotation dictionary), or a
	// [pdf.Array] of these.
	T pdf.Object

	// H indicates whether to hide (true) or show (false) the annotations.
	// Default is true.
	H bool

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Hide".
// This implements the [pdf.Action] interface.
func (a *Hide) ActionType() pdf.Name  { return TypeHide }
func (a *Hide) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *Hide) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Hide action", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.T == nil {
		return nil, errors.New("Hide action must specify target annotations")
	}
	if !validHideTarget(a.T, true) {
		return nil, errors.New("invalid Hide action T entry")
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeHide),
		"T": a.T,
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	// only write H if false (true is default)
	if !a.H {
		dict["H"] = pdf.Boolean(false)
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

// validHideTarget reports whether t is a value allowed for the T entry of a
// Hide action.  Arrays are only allowed at the top level.
func validHideTarget(t pdf.Object, top bool) bool {
	switch t := t.AsPDF(0).(type) {
	case pdf.String, pdf.Dict, pdf.Reference:
		return true
	case pdf.Array:
		if !top || len(t) == 0 {
			return false
		}
		for _, e := range t {
			if e == nil || !validHideTarget(e, false) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// readHideTarget reads the T entry of a Hide action.  Invalid elements of an
// array are dropped.  The result is nil if there is no usable target.
//
// The entry is resolved without the cursor path, since it may name the
// annotation or field whose action is being read.  Annotation dictionaries
// are kept as they stand, so that a reference keeps denoting the annotation.
func readHideTarget(c pdf.Cursor, obj pdf.Object) (pdf.Object, error) {
	resolved, err := pdf.Optional(pdf.Resolve(c.Getter(), obj))
	if err != nil {
		return nil, err
	}
	switch v := resolved.(type) {
	case pdf.Dict:
		return obj, nil
	case pdf.String:
		return v, nil
	case pdf.Array:
		var res pdf.Array
		for _, elem := range v {
			resolved, err := pdf.Optional(pdf.Resolve(c.Getter(), elem))
			if err != nil {
				return nil, err
			}
			switch r := resolved.(type) {
			case pdf.Dict:
				res = append(res, elem)
			case pdf.String:
				res = append(res, r)
			}
		}
		if len(res) == 0 {
			return nil, nil
		}
		return res, nil
	}
	return nil, nil
}

func decodeHide(c pdf.Cursor, dict pdf.Dict) (*Hide, error) {
	t, err := readHideTarget(c, dict["T"])
	if err != nil {
		return nil, err
	} else if t == nil {
		return nil, pdf.Error("Hide action without valid T entry")
	}

	h := true // default value
	if hVal, ok, err := c.OptionalBoolean(dict["H"]); err != nil {
		return nil, err
	} else if ok {
		h = hVal
	}

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Hide{
		T:    t,
		H:    h,
		Next: next,
	}, nil
}
