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

package resource

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	fontdict "seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/pattern"
	"seehuhn.de/go/pdf/graphics/shading"
	"seehuhn.de/go/pdf/graphics/xobject"
	"seehuhn.de/go/pdf/property"
)

// PDF 2.0 sections: 14.2 7.8

type Resource struct {
	ExtGState  map[pdf.Name]*graphics.ExtGState
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

var _ pdf.Embedder = (*Resource)(nil)

func Extract(x *pdf.Extractor, obj pdf.Object) (*Resource, error) {
	// check if original object was indirect before resolving
	_, wasIndirect := obj.(pdf.Reference)

	// resolve object
	obj, err := x.Resolve(obj)
	if err != nil {
		return nil, err
	}

	// handle nil - return empty resource
	if obj == nil {
		return &Resource{SingleUse: true}, nil
	}

	// must be a dictionary
	dict, ok := obj.(pdf.Dict)
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("resource must be dictionary, got %T", obj),
		}
	}

	// create result with SingleUse based on indirectness
	res := &Resource{
		SingleUse: !wasIndirect,
	}

	// extract ExtGState subdictionary
	if extGStateDict, ok := dict["ExtGState"].(pdf.Dict); ok {
		for name, obj := range extGStateDict {
			gs, err := graphics.ExtractExtGState(x, obj)
			if err != nil {
				continue // permissive
			}
			if res.ExtGState == nil {
				res.ExtGState = make(map[pdf.Name]*graphics.ExtGState)
			}
			res.ExtGState[name] = gs
		}
	}

	// extract ColorSpace subdictionary
	if colorSpaceDict, ok := dict["ColorSpace"].(pdf.Dict); ok {
		for name, obj := range colorSpaceDict {
			cs, err := color.ExtractSpace(x, obj)
			if err != nil {
				continue // permissive
			}
			if res.ColorSpace == nil {
				res.ColorSpace = make(map[pdf.Name]color.Space)
			}
			res.ColorSpace[name] = cs
		}
	}

	// extract Pattern subdictionary
	if patternDict, ok := dict["Pattern"].(pdf.Dict); ok {
		for name, obj := range patternDict {
			pat, err := pattern.Extract(x, obj)
			if err != nil {
				continue // permissive
			}
			if res.Pattern == nil {
				res.Pattern = make(map[pdf.Name]color.Pattern)
			}
			res.Pattern[name] = pat
		}
	}

	// extract Shading subdictionary
	if shadingDict, ok := dict["Shading"].(pdf.Dict); ok {
		for name, obj := range shadingDict {
			sh, err := shading.Extract(x, obj)
			if err != nil {
				continue // permissive
			}
			if res.Shading == nil {
				res.Shading = make(map[pdf.Name]graphics.Shading)
			}
			res.Shading[name] = sh
		}
	}

	// extract XObject subdictionary
	if xobjectDict, ok := dict["XObject"].(pdf.Dict); ok {
		for name, obj := range xobjectDict {
			xobj, err := xobject.Extract(x, obj)
			if err != nil {
				continue // permissive
			}
			if res.XObject == nil {
				res.XObject = make(map[pdf.Name]graphics.XObject)
			}
			res.XObject[name] = xobj
		}
	}

	// extract Font subdictionary
	if fontDict, ok := dict["Font"].(pdf.Dict); ok {
		for name, obj := range fontDict {
			f, err := fontdict.ExtractFont(x, obj)
			if err != nil {
				continue // permissive
			}
			if res.Font == nil {
				res.Font = make(map[pdf.Name]font.Instance)
			}
			res.Font[name] = f
		}
	}

	// extract Properties subdictionary
	if propertiesDict, ok := dict["Properties"].(pdf.Dict); ok {
		for name, obj := range propertiesDict {
			props, err := property.ExtractList(x, obj)
			if err != nil {
				continue // permissive
			}
			if res.Properties == nil {
				res.Properties = make(map[pdf.Name]property.List)
			}
			res.Properties[name] = props
		}
	}

	// extract ProcSet
	if procSetArray, ok := dict["ProcSet"].(pdf.Array); ok {
		for _, obj := range procSetArray {
			name, ok := obj.(pdf.Name)
			if !ok {
				continue // permissive
			}
			switch name {
			case "PDF":
				res.ProcSet.PDF = true
			case "Text":
				res.ProcSet.Text = true
			case "ImageB":
				res.ProcSet.ImageB = true
			case "ImageC":
				res.ProcSet.ImageC = true
			case "ImageI":
				res.ProcSet.ImageI = true
				// unknown names are ignored
			}
		}
	}

	return res, nil
}

func (r *Resource) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
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
		v := rm.Out().GetMeta().Version
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
			ref, err := gs.Embed(rm)
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
			ref, err := cs.Embed(rm)
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
			ref, err := pat.Embed(rm)
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
			ref, err := sh.Embed(rm)
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
			ref, err := xobj.Embed(rm)
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
			ref, err := f.Embed(rm)
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
