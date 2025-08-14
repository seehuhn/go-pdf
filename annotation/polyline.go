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

package annotation

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/measure"
)

// Polyline represents a polyline annotation that displays open polygons on the page.
// Polylines are similar to polygons, except that the first and last vertex are not
// implicitly connected.
type Polyline struct {
	Common
	Markup

	// Vertices (required unless Path is present) is an array of numbers specifying
	// the alternating horizontal and vertical coordinates of each vertex in default
	// user space.
	Vertices []float64

	// LE (optional; meaningful only for polyline annotations) is an array of two
	// names that specify the line ending styles for the endpoints defined by the
	// first and last pairs of coordinates in the Vertices array.
	// Default value: [/None /None]
	LE []pdf.Name

	// BorderStyle (optional) is a border style dictionary specifying the width
	// and dash pattern used in drawing the polyline.
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// IC (optional) is an array of numbers in the range 0.0 to 1.0 specifying
	// the interior color with which to fill the annotation's line endings.
	// The number of array elements determines the colour space:
	// 0 - No colour; transparent
	// 1 - DeviceGray
	// 3 - DeviceRGB
	// 4 - DeviceCMYK
	IC []float64

	// Measure (optional; PDF 1.7) is a measure dictionary that specifies the
	// scale and units that apply to the annotation.
	Measure measure.Measure

	// Path (optional; PDF 2.0) is an array of n arrays, each supplying operands
	// for path building operators (m, l, or c). If present, Vertices is not present.
	// Each array contains pairs of values specifying points for path drawing operations.
	Path [][]float64
}

var _ Annotation = (*Polyline)(nil)

// AnnotationType returns "PolyLine".
// This implements the [Annotation] interface.
func (p *Polyline) AnnotationType() pdf.Name {
	return "PolyLine"
}

func extractPolyline(r pdf.Getter, dict pdf.Dict) (*Polyline, error) {
	polyline := &Polyline{}

	// Extract common annotation fields
	if err := decodeCommon(r, &polyline.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(r, dict, &polyline.Markup); err != nil {
		return nil, err
	}

	// Extract polyline-specific fields
	// Vertices (required unless Path is present)
	if vertices, err := pdf.GetFloatArray(r, dict["Vertices"]); err == nil && len(vertices) > 0 {
		polyline.Vertices = vertices
	}

	// LE (optional)
	if le, err := pdf.GetArray(r, dict["LE"]); err == nil && len(le) == 2 {
		endings := make([]pdf.Name, 2)
		for i, ending := range le {
			if name, err := pdf.GetName(r, ending); err == nil {
				endings[i] = name
			}
		}
		polyline.LE = endings
	}

	// BS (optional)
	if bs, err := pdf.Optional(ExtractBorderStyle(r, dict["BS"])); err != nil {
		return nil, err
	} else {
		polyline.BorderStyle = bs
	}

	// IC (optional)
	if ic, err := pdf.GetFloatArray(r, dict["IC"]); err == nil && len(ic) > 0 {
		polyline.IC = ic
	}

	// Measure (optional)
	if dict["Measure"] != nil {
		if m, err := pdf.Optional(measure.Extract(r, dict["Measure"])); err != nil {
			return nil, err
		} else {
			polyline.Measure = m
		}
	}

	// Path (optional; PDF 2.0)
	if path, err := pdf.GetArray(r, dict["Path"]); err == nil && len(path) > 0 {
		pathArrays := make([][]float64, 0, len(path))
		for _, pathEntry := range path {
			if coords, err := pdf.GetFloatArray(r, pathEntry); err == nil {
				pathArrays = append(pathArrays, coords)
			}
		}
		if len(pathArrays) > 0 {
			polyline.Path = pathArrays
		}
	}

	return polyline, nil
}

func (p *Polyline) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
	dict := pdf.Dict{
		"Subtype": pdf.Name("PolyLine"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict, isMarkup(p)); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := p.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add polyline-specific fields
	// Vertices (required unless Path is present)
	if p.Path == nil && p.Vertices != nil && len(p.Vertices) > 0 {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation", pdf.V1_5); err != nil {
			return nil, err
		}
		verticesArray := make(pdf.Array, len(p.Vertices))
		for i, vertex := range p.Vertices {
			verticesArray[i] = pdf.Number(vertex)
		}
		dict["Vertices"] = verticesArray
	}

	// LE (optional)
	if len(p.LE) == 2 {
		leArray := pdf.Array{p.LE[0], p.LE[1]}
		dict["LE"] = leArray
	}

	// BS (optional)
	if p.BorderStyle != nil {
		bs, _, err := p.BorderStyle.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	// IC (optional)
	if p.IC != nil {
		icArray := make(pdf.Array, len(p.IC))
		for i, color := range p.IC {
			icArray[i] = pdf.Number(color)
		}
		dict["IC"] = icArray
	}

	// Measure (optional)
	if p.Measure != nil {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation Measure entry", pdf.V1_7); err != nil {
			return nil, err
		}
		embedded, _, err := p.Measure.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["Measure"] = embedded
	}

	// Path (optional; PDF 2.0)
	if len(p.Path) > 0 {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation Path entry", pdf.V2_0); err != nil {
			return nil, err
		}
		pathArray := make(pdf.Array, 0, len(p.Path))
		for _, pathEntry := range p.Path {
			if pathEntry != nil {
				entryArray := make(pdf.Array, len(pathEntry))
				for j, coord := range pathEntry {
					entryArray[j] = pdf.Number(coord)
				}
				pathArray = append(pathArray, entryArray)
			}
		}
		if len(pathArray) > 0 {
			dict["Path"] = pathArray
		}
	}

	return dict, nil
}
