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

// Group represents an optional content group dictionary that defines a collection
// of graphics that can be made visible or invisible dynamically by PDF processors.
// This corresponds to Table 96 in the PDF specification.
type Group struct {
	// Name specifies the name of the optional content group, suitable for
	// presentation in an interactive PDF processor's user interface.
	Name string

	// Intent (optional) represents the intended use of the graphics in the group.
	// Common values include "View" and "Design". Default is ["View"].
	Intent []pdf.Name

	// Usage (optional) describes the nature of the content controlled by the group.
	// It may be used by features that automatically control the group state.
	Usage *Usage
}

var _ pdf.Embedder = (*Group)(nil)

// ExtractGroup extracts an optional content group from a PDF object.
func ExtractGroup(x *pdf.Extractor, obj pdf.Object) (*Group, error) {
	dict, err := x.GetDictTyped(obj, "OCG")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing optional content group dictionary")
	}

	group := &Group{}

	// extract Name (required)
	if name, err := pdf.Optional(pdf.GetTextString(x.R, dict["Name"])); err != nil {
		return nil, err
	} else if name != "" {
		group.Name = string(name)
	}

	// Intent (optional) can be either a single name or an array of names.
	intent, err := x.Resolve(dict["Intent"])
	if err != nil {
		return nil, err
	}
	switch intent := intent.(type) {
	case pdf.Array:
		for _, o := range intent {
			if name, err := pdf.Optional(x.GetName(o)); err != nil {
				return nil, err
			} else if name != "" {
				group.Intent = append(group.Intent, name)
			}
		}
	case pdf.Name:
		if intent != "" {
			group.Intent = []pdf.Name{intent}
		}
	}
	if len(group.Intent) == 0 {
		group.Intent = []pdf.Name{"View"}
	}

	if usage, err := pdf.ExtractorGetOptional(x, dict["Usage"], ExtractUsage); err != nil {
		return nil, err
	} else {
		group.Usage = usage
	}

	return group, nil
}

// Embed adds the optional content group to a PDF file.
func (g *Group) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// validate required fields
	if g.Name == "" {
		return nil, errors.New("Group.Name is required")
	}

	dict := pdf.Dict{
		"Type": pdf.Name("OCG"),
		"Name": pdf.TextString(g.Name),
	}

	// embed Intent
	if len(g.Intent) == 0 {
		// use default
		dict["Intent"] = pdf.Name("View")
	} else if len(g.Intent) == 1 {
		// single name
		dict["Intent"] = g.Intent[0]
	} else {
		// array of names
		intentArray := make(pdf.Array, len(g.Intent))
		for i, intent := range g.Intent {
			intentArray[i] = intent
		}
		dict["Intent"] = intentArray
	}

	// embed Usage dictionary if present
	if g.Usage != nil {
		usageObj, err := rm.Embed(g.Usage)
		if err != nil {
			return nil, err
		}
		dict["Usage"] = usageObj
	}

	// always create indirect reference
	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// IsVisible returns whether the group is visible given a state map.
func (g *Group) IsVisible(states map[*Group]bool) bool {
	return states[g]
}
