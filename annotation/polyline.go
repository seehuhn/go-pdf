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
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/measure"
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.9

// PolyLine represents a polyline annotation that displays open polygons on the page.
// Polylines are similar to polygons, except that the first and last vertex are not
// implicitly connected.
type PolyLine struct {
	Common
	Markup

	// Vertices (required unless Path is present) is an array of numbers
	// specifying the alternating horizontal and vertical coordinates of each
	// vertex in default user space.
	Vertices []float64

	// Path (optional; PDF 2.0) is an array of arrays, each supplying operands
	// for path building operators (m, l, or c).  Each array inner contains
	// pairs of values specifying points for path drawing operations. The first
	// array is of length 2 (moveto), subsequent arrays of length 2 specify
	// lineto operators, and arrays of length 6 specify curveto operators.
	Path [][]float64

	// BorderStyle (optional) is a border style dictionary specifying the width
	// and dash pattern used in drawing the polyline.
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// LineEndingStyle (optional) is an array of two names specifying the line
	// ending styles for the start and end points respectively.
	//
	// When writing annotations empty names may be used as a shorthand for
	// [LineEndingStyleNone].
	//
	// This corresponds to the /LE entry in the PDF annotation dictionary.
	LineEndingStyle [2]LineEndingStyle

	// FillColor (optional; PDF 1.4) is the colour used to fill the
	// polyline's line endings, if applicable.
	//
	// Only certain color types are allowed:
	//  - colors in the [color.DeviceGray] color space
	//  - colors in the [color.DeviceRGB] color space
	//  - colors in the [color.DeviceCMYK] color space
	//  - the [Transparent] color
	//
	// This corresponds to the /IC entry in the PDF annotation dictionary.
	FillColor color.Color

	// Measure (optional; PDF 1.7) is a measure dictionary that specifies the
	// scale and units that apply to the annotation.
	Measure measure.Measure
}

var _ Annotation = (*PolyLine)(nil)

// AnnotationType returns "PolyLine".
// This implements the [Annotation] interface.
func (p *PolyLine) AnnotationType() pdf.Name {
	return "PolyLine"
}

func decodePolyline(x *pdf.Extractor, dict pdf.Dict) (*PolyLine, error) {
	polyline := &PolyLine{}

	// Extract common annotation fields
	if err := decodeCommon(x, &polyline.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, dict, &polyline.Markup); err != nil {
		return nil, err
	}

	// Extract polyline-specific fields
	// Vertices (required unless Path is present)
	if vertices, err := pdf.GetFloatArray(x.R, dict["Vertices"]); err == nil && len(vertices) > 0 {
		polyline.Vertices = vertices
	}

	// LE (optional; PDF 1.4) - default is [None, None]
	polyline.LineEndingStyle = [2]LineEndingStyle{LineEndingStyleNone, LineEndingStyleNone}
	if le, err := pdf.Optional(x.GetArray(dict["LE"])); err != nil {
		return nil, err
	} else if len(le) >= 1 {
		if name, err := x.GetName(le[0]); err == nil {
			polyline.LineEndingStyle[0] = LineEndingStyle(name)
		}
		if len(le) >= 2 {
			if name, err := x.GetName(le[1]); err == nil {
				polyline.LineEndingStyle[1] = LineEndingStyle(name)
			}
		} else {
			// if only one element, copy first element to second
			polyline.LineEndingStyle[1] = polyline.LineEndingStyle[0]
		}
	}

	// BS (optional)
	if bs, err := pdf.Optional(pdf.ExtractorGet(x, dict["BS"], ExtractBorderStyle)); err != nil {
		return nil, err
	} else {
		polyline.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			polyline.Common.Border = nil
		}
	}

	// IC (optional; PDF 1.4)
	if ic, err := pdf.Optional(extractColor(x.R, dict["IC"])); err != nil {
		return nil, err
	} else {
		polyline.FillColor = ic
	}

	// Measure (optional)
	if dict["Measure"] != nil {
		if m, err := pdf.Optional(measure.Extract(x, dict["Measure"])); err != nil {
			return nil, err
		} else {
			polyline.Measure = m
		}
	}

	// Path (optional; PDF 2.0)
	if path, err := x.GetArray(dict["Path"]); err == nil && len(path) > 0 {
		pathArrays := make([][]float64, 0, len(path))
		for _, pathEntry := range path {
			if coords, err := pdf.GetFloatArray(x.R, pathEntry); err == nil {
				pathArrays = append(pathArrays, coords)
			}
		}
		if len(pathArrays) > 0 {
			polyline.Path = pathArrays
		}
	}

	return polyline, nil
}

func (p *PolyLine) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if p.BorderStyle != nil && p.Common.Border != nil {
		return nil, errors.New("Border and BorderStyle are mutually exclusive")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("PolyLine"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict, isMarkup(p), p.BorderStyle != nil); err != nil {
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

	// LE (optional; PDF 1.4) - only write if not default [None, None]
	// normalize empty strings to None as documented
	normalized := p.LineEndingStyle
	if normalized[0] == "" {
		normalized[0] = LineEndingStyleNone
	}
	if normalized[1] == "" {
		normalized[1] = LineEndingStyleNone
	}

	if normalized != [2]LineEndingStyle{LineEndingStyleNone, LineEndingStyleNone} {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation LE entry", pdf.V1_4); err != nil {
			return nil, err
		}
		leArray := make(pdf.Array, 2)
		leArray[0] = pdf.Name(normalized[0])
		leArray[1] = pdf.Name(normalized[1])
		dict["LE"] = leArray
	}

	// BS (optional)
	if p.BorderStyle != nil {
		bs, err := rm.Embed(p.BorderStyle)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	// IC (optional; PDF 1.4)
	if p.FillColor != nil {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation IC entry", pdf.V1_4); err != nil {
			return nil, err
		}
		if icArray, err := encodeColor(p.FillColor); err != nil {
			return nil, err
		} else if icArray != nil {
			dict["IC"] = icArray
		}
	}

	// Measure (optional)
	if p.Measure != nil {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation Measure entry", pdf.V1_7); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(p.Measure)
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
