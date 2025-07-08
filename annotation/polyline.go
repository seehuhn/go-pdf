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

import "seehuhn.de/go/pdf"

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

	// BS (optional) is a border style dictionary specifying the width and dash
	// pattern used in drawing the polyline.
	BS pdf.Reference

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
	Measure pdf.Reference

	// Path (optional; PDF 2.0) is an array of n arrays, each supplying operands
	// for path building operators (m, l, or c). If present, Vertices is not present.
	// Each array contains pairs of values specifying points for path drawing operations.
	Path [][]float64
}

var _ pdf.Annotation = (*Polyline)(nil)

// AnnotationType returns "PolyLine".
// This implements the [pdf.Annotation] interface.
func (p *Polyline) AnnotationType() pdf.Name {
	return "PolyLine"
}

func extractPolyline(r pdf.Getter, dict pdf.Dict) (*Polyline, error) {
	polyline := &Polyline{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &polyline.Common); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &polyline.Markup); err != nil {
		return nil, err
	}

	// Extract polyline-specific fields
	// Vertices (required unless Path is present)
	if vertices, err := pdf.GetArray(r, dict["Vertices"]); err == nil && len(vertices) > 0 {
		coords := make([]float64, len(vertices))
		for i, vertex := range vertices {
			if num, err := pdf.GetNumber(r, vertex); err == nil {
				coords[i] = float64(num)
			}
		}
		polyline.Vertices = coords
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
	if bs, ok := dict["BS"].(pdf.Reference); ok {
		polyline.BS = bs
	}

	// IC (optional)
	if ic, err := pdf.GetArray(r, dict["IC"]); err == nil && len(ic) > 0 {
		colors := make([]float64, len(ic))
		for i, color := range ic {
			if num, err := pdf.GetNumber(r, color); err == nil {
				colors[i] = float64(num)
			}
		}
		polyline.IC = colors
	}

	// Measure (optional)
	if measure, ok := dict["Measure"].(pdf.Reference); ok {
		polyline.Measure = measure
	}

	// Path (optional; PDF 2.0)
	if path, err := pdf.GetArray(r, dict["Path"]); err == nil && len(path) > 0 {
		pathArrays := make([][]float64, 0, len(path))
		for _, pathEntry := range path {
			if pathArray, err := pdf.GetArray(r, pathEntry); err == nil {
				coords := make([]float64, len(pathArray))
				for j, coord := range pathArray {
					if num, err := pdf.GetNumber(r, coord); err == nil {
						coords[j] = float64(num)
					}
				}
				pathArrays = append(pathArrays, coords)
			}
		}
		if len(pathArrays) > 0 {
			polyline.Path = pathArrays
		}
	}

	return polyline, nil
}

func (p *Polyline) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("PolyLine"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add markup annotation fields
	if err := p.Markup.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add polyline-specific fields
	// Vertices (required unless Path is present)
	if p.Path == nil && p.Vertices != nil && len(p.Vertices) > 0 {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation", pdf.V1_5); err != nil {
			return nil, zero, err
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
	if p.BS != 0 {
		dict["BS"] = p.BS
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
	if p.Measure != 0 {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation Measure entry", pdf.V1_7); err != nil {
			return nil, zero, err
		}
		dict["Measure"] = p.Measure
	}

	// Path (optional; PDF 2.0)
	if len(p.Path) > 0 {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation Path entry", pdf.V2_0); err != nil {
			return nil, zero, err
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

	return dict, zero, nil
}
