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

package extract

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/property"
)

// Resources extracts a resource dictionary from a PDF file.
func Resources(x *pdf.Extractor, obj pdf.Object) (*content.Resources, error) {
	// check if original object was indirect before resolving
	_, wasIndirect := obj.(pdf.Reference)

	// resolve object
	obj, err := x.Resolve(obj)
	if err != nil {
		return nil, err
	}

	// handle nil - return empty resource
	if obj == nil {
		return &content.Resources{SingleUse: true}, nil
	}

	// must be a dictionary
	dict, ok := obj.(pdf.Dict)
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("resource must be dictionary, got %T", obj),
		}
	}

	// create result with SingleUse based on indirectness
	res := &content.Resources{
		SingleUse: !wasIndirect,
	}

	// extract ExtGState subdictionary
	if extGStateDict, ok := dict["ExtGState"].(pdf.Dict); ok {
		for name, obj := range extGStateDict {
			gs, err := ExtGState(x, obj)
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
			cs, err := ColorSpace(x, obj)
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
			pat, err := Pattern(x, obj)
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
			sh, err := Shading(x, obj)
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
			xobj, err := XObject(x, obj)
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
			f, err := Font(x, obj)
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
