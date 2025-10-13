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

import "seehuhn.de/go/pdf"

// Conditional represents an optional content element that controls visibility
// based on conditions. This must be be either a [Group] or a [Membership]
// object.
type Conditional interface {
	pdf.Embedder
	IsVisible(map[*Group]bool) bool
}

var (
	_ Conditional = (*Group)(nil)
	_ Conditional = (*Membership)(nil)
)

// ExtractConditional extracts an optional content element from a PDF object.
// The object can be either a Group (OCG) or a Membership (OCMD) dictionary.
func ExtractConditional(x *pdf.Extractor, obj pdf.Object) (Conditional, error) {
	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}
	tp, err := x.GetName(dict["Type"])
	if err != nil {
		return nil, err
	}
	switch tp {
	case "OCG":
		return pdf.ExtractorGet(x, obj, ExtractGroup)
	case "OCMD":
		return pdf.ExtractorGet(x, obj, ExtractMembership)
	default:
		return nil, pdf.Error("invalid optional content object")
	}
}
