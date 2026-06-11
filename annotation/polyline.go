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

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/measure"
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.9

// PolyLine represents a polygonal line annotation on the page. When opened,
// the annotation displays a pop-up window containing the text of an associated
// note.
//
//   - The polyline shape is defined by the Vertices field.
//     Vertices are connected by straight lines in the order specified.
//   - The line color is specified by the Common.Color field.
//     If this is nil, no line is drawn.
//   - The line style is specified by the BorderStyle field.
//     If this is nil, the Common.Border field is used instead.
//     If both are nil, a solid line with width 1 is used.
//   - The line ending styles for the start and end points are specified by the
//     LineEndingStyle field.  If this is not set, both default to None.
//   - The color used to fill the line endings (if applicable) is specified by
//     the FillColor field.  If this is nil, no fill is applied to the line endings.
type PolyLine struct {
	Common
	Markup

	// Vertices (required) is an array of numbers specifying the alternating
	// horizontal and vertical coordinates of each vertex in default user
	// space coordinates.
	Vertices []float64

	// BorderStyle (optional) is a border style dictionary specifying the width
	// and dash pattern used in drawing the polygonal line.
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// LineEndingStyle (optional) specifies the line ending styles for the
	// start and end points respectively.
	//
	// When writing annotations zero values may be used as a shorthand for
	// [LineEndingStyleNone].
	//
	// This corresponds to the /LE entry in the PDF annotation dictionary.
	LineEndingStyle [2]LineEndingStyle

	// FillColor (optional) is the colour used to fill the line endings, if
	// applicable.
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

	// Path (optional; PDF 2.0) is a sequence of path-building operators.
	// The first entry holds a single point (moveto); subsequent entries
	// hold one point (lineto) or three points — two control points
	// followed by the endpoint — (curveto).
	//
	// See https://github.com/pdf-association/pdf-issues/issues/730 .
	Path [][]vec.Vec2
}

var _ Annotation = (*PolyLine)(nil)

// AnnotationType returns "PolyLine".
// This implements the [Annotation] interface.
func (p *PolyLine) AnnotationType() pdf.Name {
	return "PolyLine"
}

func (p *PolyLine) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "polyline annotations", pdf.V1_5); err != nil {
		return nil, err
	}

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
		return nil, errors.New("polyline annotation requires Vertices")
	}

	if len(p.Path) > 0 {
		if err := pdf.CheckVersion(rm.Out, "polyline annotation Path entry", pdf.V2_0); err != nil {
			return nil, err
		}
		pathArray, err := encodePath(p.Path)
		if err != nil {
			return nil, err
		}
		dict["Path"] = pathArray
	}

	// LE (optional) - only write if not default [None, None]
	// normalize empty strings to None as documented
	normalized := p.LineEndingStyle
	if normalized[0] == "" {
		normalized[0] = LineEndingStyleNone
	}
	if normalized[1] == "" {
		normalized[1] = LineEndingStyleNone
	}

	if normalized != [2]LineEndingStyle{LineEndingStyleNone, LineEndingStyleNone} {
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

	// IC (optional)
	if p.FillColor != nil {
		if icArray, err := colorenc.Encode(p.FillColor); err != nil {
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

	return dict, nil
}
