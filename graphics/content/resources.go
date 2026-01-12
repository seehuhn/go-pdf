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
	"fmt"
	"maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/property"
)

// Resources represents a PDF resource dictionary.
//
// A resource dictionary enumerates the named resources needed by operators
// in a content stream and the names by which they can be referred to.
// Operators in a content stream can only use direct objects as operands;
// when an operator needs to refer to an external object (such as a font
// dictionary or an image stream), it does so by name through this dictionary.
//
// The scope of resource names is local to a particular content stream.
// Different content streams may use different names for the same resource,
// or the same name for different resources.
type Resources struct {
	ExtGState  map[pdf.Name]*extgstate.ExtGState
	ColorSpace map[pdf.Name]color.Space
	Pattern    map[pdf.Name]color.Pattern
	Shading    map[pdf.Name]graphics.Shading
	XObject    map[pdf.Name]graphics.XObject
	Font       map[pdf.Name]font.Instance
	ProcSet    ProcSet
	Properties map[pdf.Name]property.List

	// SingleUse determines whether the resource dictionary is embedded
	// directly (true) or as an indirect object reference (false).
	SingleUse bool
}

type ProcSet struct {
	PDF    bool
	Text   bool
	ImageB bool
	ImageC bool
	ImageI bool
}

var _ pdf.Embedder = (*Resources)(nil)

func (r *Resources) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// validate PDF version constraints
	if len(r.Shading) > 0 {
		if err := pdf.CheckVersion(rm.Out(), "Shading resources", pdf.V1_3); err != nil {
			return nil, err
		}
	}
	if len(r.Properties) > 0 {
		if err := pdf.CheckVersion(rm.Out(), "Properties resources", pdf.V1_2); err != nil {
			return nil, err
		}
	}
	if r.ProcSet.PDF || r.ProcSet.Text || r.ProcSet.ImageB || r.ProcSet.ImageC || r.ProcSet.ImageI {
		v := pdf.GetVersion(rm.Out())
		if v >= pdf.V2_0 {
			return nil, fmt.Errorf("ProcSet is deprecated in PDF 2.0")
		}
	}

	// create result dictionary
	dict := pdf.Dict{}

	// embed ExtGState
	if len(r.ExtGState) > 0 {
		extGStateDict := pdf.Dict{}
		for name, gs := range r.ExtGState {
			ref, err := rm.Embed(gs)
			if err != nil {
				return nil, err
			}
			extGStateDict[name] = ref
		}
		dict["ExtGState"] = extGStateDict
	}

	// embed ColorSpace
	if len(r.ColorSpace) > 0 {
		colorSpaceDict := pdf.Dict{}
		for name, cs := range r.ColorSpace {
			ref, err := rm.Embed(cs)
			if err != nil {
				return nil, err
			}
			colorSpaceDict[name] = ref
		}
		dict["ColorSpace"] = colorSpaceDict
	}

	// embed Pattern
	if len(r.Pattern) > 0 {
		patternDict := pdf.Dict{}
		for name, pat := range r.Pattern {
			ref, err := rm.Embed(pat)
			if err != nil {
				return nil, err
			}
			patternDict[name] = ref
		}
		dict["Pattern"] = patternDict
	}

	// embed Shading
	if len(r.Shading) > 0 {
		shadingDict := pdf.Dict{}
		for name, sh := range r.Shading {
			ref, err := rm.Embed(sh)
			if err != nil {
				return nil, err
			}
			shadingDict[name] = ref
		}
		dict["Shading"] = shadingDict
	}

	// embed XObject
	if len(r.XObject) > 0 {
		xobjectDict := pdf.Dict{}
		for name, xobj := range r.XObject {
			ref, err := rm.Embed(xobj)
			if err != nil {
				return nil, err
			}
			xobjectDict[name] = ref
		}
		dict["XObject"] = xobjectDict
	}

	// embed Font
	if len(r.Font) > 0 {
		fontDict := pdf.Dict{}
		for name, f := range r.Font {
			ref, err := rm.Embed(f)
			if err != nil {
				return nil, err
			}
			fontDict[name] = ref
		}
		dict["Font"] = fontDict
	}

	// embed Properties
	if len(r.Properties) > 0 {
		propertiesDict := pdf.Dict{}
		for name, props := range r.Properties {
			ref, err := props.Embed(rm)
			if err != nil {
				return nil, err
			}
			propertiesDict[name] = ref
		}
		dict["Properties"] = propertiesDict
	}

	// embed ProcSet
	var procSetArray pdf.Array
	if r.ProcSet.PDF {
		procSetArray = append(procSetArray, pdf.Name("PDF"))
	}
	if r.ProcSet.Text {
		procSetArray = append(procSetArray, pdf.Name("Text"))
	}
	if r.ProcSet.ImageB {
		procSetArray = append(procSetArray, pdf.Name("ImageB"))
	}
	if r.ProcSet.ImageC {
		procSetArray = append(procSetArray, pdf.Name("ImageC"))
	}
	if r.ProcSet.ImageI {
		procSetArray = append(procSetArray, pdf.Name("ImageI"))
	}
	if len(procSetArray) > 0 {
		dict["ProcSet"] = procSetArray
	}

	// return based on SingleUse
	if r.SingleUse {
		return dict, nil
	}
	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// Equal compares two Resources for value equality.
//
// TODO(voss): XObjects are currently ignored in the comparison, as long as the
// same keys are present. Implement proper equality checks for these types.
func (r *Resources) Equal(other *Resources) bool {
	if r == nil || other == nil || r == other {
		return r == other
	}
	return r.SingleUse == other.SingleUse &&
		r.ProcSet == other.ProcSet &&
		maps.EqualFunc(r.ExtGState, other.ExtGState, (*extgstate.ExtGState).Equal) &&
		maps.EqualFunc(r.ColorSpace, other.ColorSpace, color.SpacesEqual) &&
		maps.EqualFunc(r.Pattern, other.Pattern, patternsEqual) &&
		maps.EqualFunc(r.Shading, other.Shading, shadingsEqual) &&
		haveSameKeys(r.XObject, other.XObject) &&
		maps.EqualFunc(r.Font, other.Font, font.InstancesEqual) &&
		maps.EqualFunc(r.Properties, other.Properties, property.ListsEqual)
}

func patternsEqual(a, b color.Pattern) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(b)
}

func shadingsEqual(a, b graphics.Shading) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(b)
}

func haveSameKeys[Val pdf.Embedder](a, b map[pdf.Name]Val) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
