// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package group

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 8.10 11.6.6

// TransparencyAttributes represents a transparency group attributes dictionary
// (Tables 94 and 145 in the PDF spec).
//
// Transparency groups can be associated with pages (via the Group entry in the
// page dictionary) or with form XObjects (via the Group entry in the form
// dictionary).
type TransparencyAttributes struct {
	// CS is the group color space, used for compositing within the group.
	// For isolated groups, this specifies the color space for compositing.
	// For non-isolated groups, this is inherited from the parent if not set.
	//
	// The color space must be a device or CIE-based color space that treats
	// its components as independent additive or subtractive values.
	// Special color spaces (Pattern, Indexed, Separation, DeviceN) and
	// Lab color spaces are not allowed.
	CS color.Space

	// Isolated specifies whether the transparency group is isolated.
	// If true, objects within the group are composited against a fully
	// transparent initial backdrop. If false, they are composited against
	// the group's backdrop.
	//
	// This corresponds to the /I entry in the PDF transparency group dictionary.
	Isolated bool

	// Knockout specifies whether the transparency group is a knockout group.
	// If true, objects within the group are composited with the group's
	// initial backdrop and overwrite earlier overlapping objects.
	// If false, later objects are composited with earlier ones.
	//
	// This corresponds to the /K entry in the PDF transparency group dictionary.
	Knockout bool

	// SingleUse indicates that this object is used only once in the PDF file.
	// This allows the embedding code to return the dictionary directly
	// instead of creating an indirect reference.
	SingleUse bool
}

// Equal reports whether two TransparencyAttributes are equal.
func (a *TransparencyAttributes) Equal(other *TransparencyAttributes) bool {
	if a == nil || other == nil {
		return a == nil && other == nil
	}
	return color.SpacesEqual(a.CS, other.CS) &&
		a.Isolated == other.Isolated &&
		a.Knockout == other.Knockout &&
		a.SingleUse == other.SingleUse
}

// Embed adds the transparency group attributes dictionary to a PDF file.
//
// This implements the [pdf.Embedder] interface.
func (a *TransparencyAttributes) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "transparency groups", pdf.V1_4); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name("Transparency"),
	}

	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Group")
	}

	if a.CS != nil {
		csObj, err := rm.Embed(a.CS)
		if err != nil {
			return nil, err
		}
		dict["CS"] = csObj
	}

	if a.Isolated {
		dict["I"] = pdf.Boolean(true)
	}

	if a.Knockout {
		dict["K"] = pdf.Boolean(true)
	}

	if a.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	if err := rm.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// ExtractTransparencyAttributes reads a transparency group attributes
// dictionary from a PDF file.
func ExtractTransparencyAttributes(x *pdf.Extractor, obj pdf.Object) (*TransparencyAttributes, error) {
	_, isIndirect := obj.(pdf.Reference)

	dict, err := x.GetDictTyped(obj, "Group")
	if err != nil {
		return nil, err
	}

	// verify subtype
	s, err := x.GetName(dict["S"])
	if err != nil {
		return nil, err
	}
	if s != "Transparency" {
		return nil, pdf.Errorf("expected group subtype Transparency, got %q", s)
	}

	a := &TransparencyAttributes{}

	// extract color space (optional)
	if csObj, ok := dict["CS"]; ok {
		cs, err := color.ExtractSpace(x, csObj)
		if err != nil {
			return nil, pdf.Wrap(err, "group color space")
		}
		a.CS = cs
	}

	// extract isolated flag (default false)
	if iObj, ok := dict["I"]; ok {
		isolated, err := x.GetBoolean(iObj)
		if err != nil {
			return nil, err
		}
		a.Isolated = bool(isolated)
	}

	// extract knockout flag (default false)
	if kObj, ok := dict["K"]; ok {
		knockout, err := x.GetBoolean(kObj)
		if err != nil {
			return nil, err
		}
		a.Knockout = bool(knockout)
	}

	a.SingleUse = !isIndirect

	return a, nil
}
