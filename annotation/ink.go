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
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.13

// Ink represents an ink annotation that represents a freehand "scribble"
// composed of one or more disjoint paths. When opened, it displays a popup
// window containing the text of the associated note.
type Ink struct {
	Common
	Markup

	// InkList (required) is a sequence of stroked paths. Each inner slice
	// lists the points along one path, in default user space. When drawn,
	// consecutive points are connected by straight lines or curves in an
	// implementation-dependent way.
	InkList [][]vec.Vec2

	// BorderStyle (optional) is a border style dictionary specifying the line
	// width and dash pattern that is used in drawing the ink annotation.
	//
	// If the BorderStyle field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// Path (optional; PDF 2.0) is a sequence of path-building operators.
	// The first entry holds a single point (moveto); subsequent entries
	// hold one point (lineto) or three points — two control points
	// followed by the endpoint — (curveto).
	//
	// See https://github.com/pdf-association/pdf-issues/issues/730 .
	Path [][]vec.Vec2
}

var _ Annotation = (*Ink)(nil)

// AnnotationType returns "Ink".
// This implements the [Annotation] interface.
func (i *Ink) AnnotationType() pdf.Name {
	return "Ink"
}

func (i *Ink) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "ink annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	if i.BorderStyle != nil && i.Common.Border != nil {
		return nil, errors.New("Border and BorderStyle are mutually exclusive")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Ink"),
	}

	// Add common annotation fields
	if err := i.Common.fillDict(rm, dict, isMarkup(i), i.BorderStyle != nil); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := i.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// InkList (required)
	if len(i.InkList) > 0 {
		inkArray := make(pdf.Array, len(i.InkList))
		for k, path := range i.InkList {
			if len(path) > 0 {
				pathArray := make(pdf.Array, 2*len(path))
				for j, p := range path {
					pathArray[2*j] = pdf.Number(p.X)
					pathArray[2*j+1] = pdf.Number(p.Y)
				}
				inkArray[k] = pathArray
			} else {
				inkArray[k] = pdf.Array{}
			}
		}
		dict["InkList"] = inkArray
	} else {
		return nil, errors.New("ink annotation requires InkList")
	}

	// BS (optional)
	if i.BorderStyle != nil {
		bs, err := rm.Embed(i.BorderStyle)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	// Path (optional; PDF 2.0)
	if len(i.Path) > 0 {
		if err := pdf.CheckVersion(rm.Out, "ink annotation Path entry", pdf.V2_0); err != nil {
			return nil, err
		}
		pathArray, err := encodePath(i.Path)
		if err != nil {
			return nil, err
		}
		dict["Path"] = pathArray
	}

	return dict, nil
}
