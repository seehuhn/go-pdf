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
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 8.11.2

// Membership represents an optional content membership dictionary that
// expresses complex visibility policies for content based on optional content
// groups. This corresponds to Table 97 in the PDF specification.
type Membership struct {
	// OCGs (optional) specifies the optional content groups whose states
	// determine the visibility of content controlled by this membership dictionary.
	//
	// If VE is present, this entry is ignored.
	OCGs []*Group

	// Policy specifies the visibility policy for content belonging to this
	// membership dictionary. Valid values are [PolicyAllOn], [PolicyAnyOn],
	// [PolicyAnyOff], [PolicyAllOff].
	//
	// If VE is present, this entry is ignored.
	Policy Policy

	// VE (optional) specifies a visibility expression for computing
	// visibility based on a set of optional content groups.
	//
	// If present, this takes precedence over OCGs and Policy.
	VE VisibilityExpression

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder[pdf.Unused] = (*Membership)(nil)

// ExtractMembership extracts an optional content membership dictionary from a PDF object.
func ExtractMembership(x *pdf.Extractor, obj pdf.Object) (*Membership, error) {
	_, isIndirect := obj.(pdf.Reference)

	dict, err := pdf.GetDictTyped(x.R, obj, "OCMD")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing optional content membership dictionary")
	}

	m := &Membership{}

	ocgsObj, err := pdf.Resolve(x.R, dict["OCGs"])
	if err != nil {
		return nil, err
	}
	switch arr := ocgsObj.(type) {
	case pdf.Array:
		m.OCGs = make([]*Group, 0, len(arr))
		for _, item := range arr {
			if group, err := pdf.ExtractorGetOptional(x, item, ExtractGroup); err != nil {
				return nil, err
			} else if group != nil {
				m.OCGs = append(m.OCGs, group)
			}
		}
	default:
		if group, err := pdf.ExtractorGetOptional(x, ocgsObj, ExtractGroup); err != nil {
			return nil, err
		} else if group != nil {
			m.OCGs = []*Group{group}
		}
	}

	if pName, err := pdf.Optional(pdf.GetName(x.R, dict["P"])); err != nil {
		return nil, err
	} else {
		switch Policy(pName) {
		case PolicyAllOn, PolicyAnyOn, PolicyAnyOff, PolicyAllOff:
			m.Policy = Policy(pName)
		default:
			m.Policy = PolicyAnyOn
		}
	}

	if ve, err := pdf.ExtractorGetOptional(x, dict["VE"], ExtractVisibilityExpression); err != nil {
		return nil, err
	} else {
		m.VE = ve
	}

	m.SingleUse = !isIndirect

	return m, nil
}

// Embed converts the Membership to a PDF object.
func (m *Membership) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"Type": pdf.Name("OCMD"),
	}

	switch len(m.OCGs) {
	case 0:
		if m.VE == nil {
			return nil, zero, errors.New("membership dictionary must have either OCGs or VE")
		}
	case 1:
		ocgObj, _, err := pdf.EmbedHelperEmbed(rm, m.OCGs[0])
		if err != nil {
			return nil, zero, err
		}
		dict["OCGs"] = ocgObj
	default:
		ocgArray := make(pdf.Array, len(m.OCGs))
		for i, group := range m.OCGs {
			ocgObj, _, err := pdf.EmbedHelperEmbed(rm, group)
			if err != nil {
				return nil, zero, err
			}
			ocgArray[i] = ocgObj
		}
		dict["OCGs"] = ocgArray
	}

	if m.Policy != PolicyAnyOn && m.Policy != "" {
		switch m.Policy {
		case PolicyAllOn, PolicyAnyOn, PolicyAnyOff, PolicyAllOff:
			dict["P"] = pdf.Name(m.Policy)
		default:
			return nil, zero, errors.New("invalid Policy value")
		}
	}

	if m.VE != nil {
		if err := pdf.CheckVersion(rm.Out(), "visibility expressions", pdf.V1_6); err != nil {
			return nil, zero, err
		}
		veObj, _, err := pdf.EmbedHelperEmbed(rm, m.VE)
		if err != nil {
			return nil, zero, err
		}
		dict["VE"] = veObj
	}

	if m.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}

// IsVisible evaluates the visibility of content controlled by this membership
// dictionary based on the current state of optional content groups.
func (m *Membership) IsVisible(states map[*Group]bool) bool {
	if m.VE != nil {
		return m.VE.isVisible(states)
	}

	if len(m.OCGs) == 0 {
		return true
	}

	switch m.Policy {
	case PolicyAllOn:
		for _, g := range m.OCGs {
			if !g.IsVisible(states) {
				return false
			}
		}
		return true
	case PolicyAnyOff:
		for _, g := range m.OCGs {
			if !g.IsVisible(states) {
				return true
			}
		}
		return false
	case PolicyAllOff:
		for _, g := range m.OCGs {
			if g.IsVisible(states) {
				return false
			}
		}
		return true
	default: // PolicyAnyOn
		for _, g := range m.OCGs {
			if g.IsVisible(states) {
				return true
			}
		}
		return false
	}
}

// Policy represents the visibility policy for an optional content membership dictionary.
type Policy pdf.Name

const (
	// PolicyAllOn means visible only if all OCGs are ON.
	PolicyAllOn Policy = "AllOn"

	// PolicyAnyOn means visible if any of the OCGs are ON (default).
	PolicyAnyOn Policy = "AnyOn"

	// PolicyAnyOff means visible if any of the OCGs are OFF.
	PolicyAnyOff Policy = "AnyOff"

	// PolicyAllOff means visible only if all OCGs are OFF.
	PolicyAllOff Policy = "AllOff"
)
