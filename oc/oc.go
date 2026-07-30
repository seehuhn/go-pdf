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

package oc

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/property"
)

// Conditional represents an optional content element that controls visibility
// based on conditions. This must be either a [Group] or a [Membership]
// object.
type Conditional interface {
	property.List
	IsVisible(*GroupStates) bool
}

var (
	_ Conditional = (*Group)(nil)
	_ Conditional = (*Membership)(nil)
)

// ExtractConditional extracts an optional content element from a PDF object.
// The object can be either a Group (OCG) or a Membership (OCMD) dictionary.
// A dictionary whose /Type entry is missing, or is not a name, is classified
// by its contents.  A /Type naming anything other than OCG or OCMD is an
// error: the entry then describes some other kind of object, and guessing at
// one would attach optional-content semantics to a dictionary that has none.
//
// An indirect OCG or OCMD yields the same Go value as extracting it directly
// with [ExtractGroup] or [ExtractMembership], so the result can be tested
// against a [GroupStates] built from the document's group list.
func ExtractConditional(c pdf.Cursor, obj pdf.Object, isDirect bool) (Conditional, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	tp, err := pdf.Optional(c.Name(dict["Type"]))
	if err != nil {
		return nil, err
	}
	if tp == "" {
		tp = inferConditionalType(dict)
	}

	// Decode under the reference this dictionary came from rather than the
	// resolved dictionary, so that one OCG in the file stays one Go value
	// however it is reached.  [GroupStates] is keyed by *Group identity, and a
	// second value for the same group would be beyond the reach of any state.
	c, obj = c.AtRef(obj, isDirect)

	switch tp {
	case "OCG":
		return pdf.Decode(c, obj, ExtractGroup)
	case "OCMD":
		return pdf.Decode(c, obj, ExtractMembership)
	default:
		return nil, pdf.Error("invalid optional content object")
	}
}

// inferConditionalType guesses the type of an optional content dictionary
// whose required /Type entry is unusable.  Of the two, only a membership
// dictionary carries /OCGs, /P or /VE, so their presence settles it; anything
// else reads as a group, which is both the commoner case and the one an /OC
// entry most often names.
func inferConditionalType(dict pdf.Dict) pdf.Name {
	for _, key := range []pdf.Name{"OCGs", "P", "VE"} {
		if _, ok := dict[key]; ok {
			return "OCMD"
		}
	}
	return "OCG"
}
