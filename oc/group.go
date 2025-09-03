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

var _ pdf.Embedder[pdf.Unused] = (*Group)(nil)

// ExtractGroup extracts an optional content group from a PDF object.
func ExtractGroup(r pdf.Getter, obj pdf.Object) (*Group, error) {
	dict, err := pdf.GetDictTyped(r, obj, "OCG")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing optional content group dictionary")
	}

	group := &Group{}

	// extract Name (required)
	if name, err := pdf.Optional(pdf.GetTextString(r, dict["Name"])); err != nil {
		return nil, err
	} else if name != "" {
		group.Name = string(name)
	}

	// extract Intent (optional)
	intentObj := dict["Intent"]
	if intentObj != nil {
		// Intent can be either a single name or an array of names
		if arr, err := pdf.GetArray(r, intentObj); err == nil && arr != nil {
			// array of names
			for _, item := range arr {
				if name, err := pdf.Optional(pdf.GetName(r, item)); err != nil {
					return nil, err
				} else if name != "" {
					group.Intent = append(group.Intent, name)
				}
			}
		} else if name, err := pdf.Optional(pdf.GetName(r, intentObj)); err != nil {
			return nil, err
		} else if name != "" {
			// single name
			group.Intent = []pdf.Name{name}
		}
	}

	// apply default Intent if none specified
	if len(group.Intent) == 0 {
		group.Intent = []pdf.Name{"View"}
	}

	// extract Usage dictionary (optional)
	if usageObj := dict["Usage"]; usageObj != nil {
		if usage, err := ExtractUsage(r, usageObj); err != nil {
			return nil, err
		} else {
			group.Usage = usage
		}
	}

	return group, nil
}

// Embed converts the Group to a PDF object, always as an indirect reference.
func (g *Group) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	// validate required fields
	if g.Name == "" {
		return nil, zero, errors.New("Group.Name is required")
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
		usageObj, _, err := g.Usage.Embed(rm)
		if err != nil {
			return nil, zero, err
		}
		dict["Usage"] = usageObj
	}

	// always create indirect reference
	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}
