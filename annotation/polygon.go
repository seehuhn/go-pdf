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

// Polygon represents a polygon annotation that displays a closed polygon on
// the page. When opened, the annotation displays a pop-up window containing
// the text of an associated note.
//
//   - The polygon shape is defined by the Vertices field.
//     Vertices are connected by straight lines, with the first and last vertex
//     implicitly connected to close the shape.
//   - The border line color is specified by the Common.Color field.
//     If this is nil, no border is drawn.
//   - The fill color is specified by the FillColor field.
//     If this is nil, no fill is applied.
//   - The border line style is specified by the BorderStyle field.
//     If this is nil, the Common.Border field is used instead.
//     If both are nil, a solid border with width 1 is used.
type Polygon struct {
	Common
	Markup

	// Vertices (required) is an array of numbers specifying the alternating
	// horizontal and vertical coordinates of each vertex in default user
	// space coordinates.
	Vertices []float64

	// BorderStyle (optional) is a border style dictionary specifying the width
	// and dash pattern used in drawing the polygon.
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// BorderEffect (optional) is a border effect dictionary describing an
	// effect applied to the border described by the BS entry.
	//
	// This corresponds to the /BE entry in the PDF annotation dictionary.
	BorderEffect *BorderEffect

	// FillColor (optional) is the colour used to fill the polygon.
	//
	// Only certain color types are allowed:
	//  - colors in the [color.DeviceGray] color space
	//  - colors in the [color.DeviceRGB] color space
	//  - colors in the [color.DeviceCMYK] color space
	//
	// This corresponds to the /IC entry in the PDF annotation dictionary.
	FillColor color.Color

	// Measure (optional) is a measure dictionary that specifies the scale and
	// units that apply to the annotation.
	Measure measure.Measure

	// Path (optional; PDF 2.0) is an array of arrays, each supplying operands
	// for path building operators (m, l, or c).  Each array contains pairs of
	// values specifying points for path drawing operations.  The first array is
	// of length 2 (moveto), subsequent arrays of length 2 specify lineto
	// operators, and arrays of length 6 specify curveto operators.
	//
	// See https://github.com/pdf-association/pdf-issues/issues/730 .
	Path [][]float64
}

var _ Annotation = (*Polygon)(nil)

// AnnotationType returns "Polygon".
// This implements the [Annotation] interface.
func (p *Polygon) AnnotationType() pdf.Name {
	return "Polygon"
}

func decodePolygon(x *pdf.Extractor, dict pdf.Dict) (*Polygon, error) {
	polygon := &Polygon{}

	// Extract common annotation fields
	if err := decodeCommon(x, &polygon.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, dict, &polygon.Markup); err != nil {
		return nil, err
	}

	// Extract polygon-specific fields
	// Vertices (required)
	if vertices, err := pdf.GetFloatArray(x.R, dict["Vertices"]); err == nil && len(vertices) >= 4 {
		polygon.Vertices = vertices[:len(vertices)&^1]
	} else {
		return nil, errors.New("polygon annotation requires Vertices")
	}

	// BS (optional)
	if bs, err := pdf.ExtractorGetOptional(x, dict["BS"], ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		polygon.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			polygon.Common.Border = nil
		}
	}

	// IC (optional)
	if ic, err := pdf.Optional(extractColor(x.R, dict["IC"])); err != nil {
		return nil, err
	} else {
		polygon.FillColor = ic
	}

	// BE (optional)
	if be, err := pdf.ExtractorGetOptional(x, dict["BE"], ExtractBorderEffect); err != nil {
		return nil, err
	} else {
		polygon.BorderEffect = be
	}

	// Measure (optional)
	if m, err := pdf.Optional(measure.Extract(x, dict["Measure"])); err != nil {
		return nil, err
	} else {
		polygon.Measure = m
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
			polygon.Path = pathArrays
		}
	}

	return polygon, nil
}

func (p *Polygon) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "polygon annotations", pdf.V1_5); err != nil {
		return nil, err
	}

	if p.BorderStyle != nil && p.Common.Border != nil {
		return nil, errors.New("Border and BorderStyle are mutually exclusive")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Polygon"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict, isMarkup(p), p.BorderStyle != nil); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := p.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	if len(p.Vertices) > 0 {
		if len(p.Vertices)%2 != 0 {
			return nil, errors.New("Vertices must have an even number of elements")
		}
		verticesArray := make(pdf.Array, len(p.Vertices))
		for i, vertex := range p.Vertices {
			verticesArray[i] = pdf.Number(vertex)
		}
		dict["Vertices"] = verticesArray
	} else {
		return nil, errors.New("polygon annotation requires Vertices")
	}

	if len(p.Path) > 0 {
		if err := pdf.CheckVersion(rm.Out, "polygon annotation Path entry", pdf.V2_0); err != nil {
			return nil, err
		}
		pathArray, err := encodePath(p.Path)
		if err != nil {
			return nil, err
		}
		dict["Path"] = pathArray
	}

	// BS (optional)
	if p.BorderStyle != nil {
		bs, err := rm.Embed(p.BorderStyle)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	// IC (optional)
	if p.FillColor != nil {
		if icArray, err := encodeColor(p.FillColor); err != nil {
			return nil, err
		} else if icArray != nil {
			dict["IC"] = icArray
		}
	}

	// BE (optional)
	if p.BorderEffect != nil {
		be, err := rm.Embed(p.BorderEffect)
		if err != nil {
			return nil, err
		}
		dict["BE"] = be
	}

	// Measure (optional)
	if p.Measure != nil {
		if err := pdf.CheckVersion(rm.Out, "polygon annotation Measure entry", pdf.V1_7); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(p.Measure)
		if err != nil {
			return nil, err
		}
		dict["Measure"] = embedded
	}

	return dict, nil
}
