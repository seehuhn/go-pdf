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
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/property"
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
	Policy MembershipPolicy

	// VE (optional) specifies a visibility expression for computing
	// visibility based on a set of optional content groups.
	//
	// If present, this takes precedence over OCGs and Policy.
	VE VisibilityExpression

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var (
	_ pdf.Embedder  = (*Membership)(nil)
	_ property.List = (*Membership)(nil)
)

// ExtractMembership extracts an optional content membership dictionary from a PDF object.
func ExtractMembership(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*Membership, error) {

	dict, err := x.GetDictTyped(path, obj, "OCMD")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing optional content membership dictionary")
	}

	m := &Membership{}

	ocgsObj, err := x.Resolve(path, dict["OCGs"])
	if err != nil {
		return nil, err
	}
	switch arr := ocgsObj.(type) {
	case pdf.Array:
		for _, item := range arr {
			if group, err := pdf.ExtractorGetOptional(x, path, item, ExtractGroup); err != nil {
				return nil, err
			} else if group != nil {
				m.OCGs = append(m.OCGs, group)
			}
		}
	default:
		if group, err := pdf.ExtractorGetOptional(x, path, ocgsObj, ExtractGroup); err != nil {
			return nil, err
		} else if group != nil {
			m.OCGs = []*Group{group}
		}
	}

	if pName, err := pdf.Optional(x.GetName(path, dict["P"])); err != nil {
		return nil, err
	} else {
		switch MembershipPolicy(pName) {
		case PolicyAllOn, PolicyAnyOn, PolicyAnyOff, PolicyAllOff:
			m.Policy = MembershipPolicy(pName)
		default:
			m.Policy = PolicyAnyOn
		}
	}

	if ve, err := pdf.ExtractorGetOptional(x, path, dict["VE"], ExtractVisibilityExpression); err != nil {
		return nil, err
	} else {
		m.VE = ve
	}

	if len(m.OCGs) == 0 && m.VE == nil {
		return nil, pdf.Error("membership dictionary must have either OCGs or VE")
	}

	m.SingleUse = isDirect

	return m, nil
}

// Embed converts the Membership to a PDF object.
func (m *Membership) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	dict := pdf.Dict{
		"Type": pdf.Name("OCMD"),
	}

	switch len(m.OCGs) {
	case 0:
		if m.VE == nil {
			return nil, errors.New("membership dictionary must have either OCGs or VE")
		}
	case 1:
		ocgObj, err := rm.Embed(m.OCGs[0])
		if err != nil {
			return nil, err
		}
		dict["OCGs"] = ocgObj
	default:
		ocgArray := make(pdf.Array, len(m.OCGs))
		for i, group := range m.OCGs {
			ocgObj, err := rm.Embed(group)
			if err != nil {
				return nil, err
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
			return nil, errors.New("invalid Policy value")
		}
	}

	if m.VE != nil {
		if err := pdf.CheckVersion(rm.Out(), "visibility expressions", pdf.V1_6); err != nil {
			return nil, err
		}
		veObj, err := rm.Embed(m.VE)
		if err != nil {
			return nil, err
		}
		dict["VE"] = veObj
	}

	if m.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// IsVisible evaluates the visibility of content controlled by this membership
// dictionary based on the current state of optional content groups.
// Groups that do not participate in the state are filtered out before
// evaluating the policy; if no groups remain, the content is visible.
func (m *Membership) IsVisible(s *GroupStates) bool {
	if m.VE != nil {
		vis, ok := m.VE.isVisible(s)
		return !ok || vis
	}

	// filter to participating groups
	var ocgs []*Group
	for _, g := range m.OCGs {
		if s.Participates(g) {
			ocgs = append(ocgs, g)
		}
	}
	if len(ocgs) == 0 {
		return true
	}

	switch m.Policy {
	case PolicyAllOn:
		for _, g := range ocgs {
			if !s.IsOn(g) {
				return false
			}
		}
		return true
	case PolicyAnyOff:
		for _, g := range ocgs {
			if !s.IsOn(g) {
				return true
			}
		}
		return false
	case PolicyAllOff:
		return !slices.ContainsFunc(ocgs, s.IsOn)
	default: // PolicyAnyOn
		return slices.ContainsFunc(ocgs, s.IsOn)
	}
}

// AsDirectDict returns nil since membership dictionaries are always indirect.
func (m *Membership) AsDirectDict() pdf.Dict { return nil }

// Equal reports whether two property lists are semantically equal.
func (m *Membership) Equal(other property.List) bool {
	n, ok := other.(*Membership)
	if !ok {
		return false
	}
	if m.Policy != n.Policy || m.SingleUse != n.SingleUse {
		return false
	}
	if len(m.OCGs) != len(n.OCGs) {
		return false
	}
	for i := range m.OCGs {
		if !m.OCGs[i].Equal(n.OCGs[i]) {
			return false
		}
	}
	return veEqual(m.VE, n.VE)
}

// MembershipPolicy represents the visibility policy for an optional content
// membership dictionary.
type MembershipPolicy pdf.Name

const (
	// PolicyAllOn means visible only if all OCGs are ON.
	PolicyAllOn MembershipPolicy = "AllOn"

	// PolicyAnyOn means visible if any of the OCGs are ON (default).
	PolicyAnyOn MembershipPolicy = "AnyOn"

	// PolicyAnyOff means visible if any of the OCGs are OFF.
	PolicyAnyOff MembershipPolicy = "AnyOff"

	// PolicyAllOff means visible only if all OCGs are OFF.
	PolicyAllOff MembershipPolicy = "AllOff"
)
