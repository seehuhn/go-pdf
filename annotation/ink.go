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

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.13

// Ink represents an ink annotation that represents a freehand "scribble"
// composed of one or more disjoint paths. When opened, it displays a popup
// window containing the text of the associated note.
type Ink struct {
	Common
	Markup

	// InkList (required) is an array of n arrays, each representing a stroked
	// path. Each array is a series of alternating horizontal and vertical
	// coordinates in default user space, specifying points along the path.
	// When drawn, the points are connected by straight lines or curves in an
	// implementation-dependent way.
	InkList [][]float64

	// Path (optional; PDF 2.0) is an array of arrays, each supplying operands
	// for path building operators (m, l, or c).  Each array inner contains
	// pairs of values specifying points for path drawing operations. The first
	// array is of length 2 (moveto), subsequent arrays of length 2 specify
	// lineto operators, and arrays of length 6 specify curveto operators.
	Path [][]float64

	// BorderStyle (optional) is a border style dictionary specifying the line
	// width and dash pattern that is used in drawing the ink annotation.
	//
	// If the BorderStyle field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle
}

var _ Annotation = (*Ink)(nil)

// AnnotationType returns "Ink".
// This implements the [Annotation] interface.
func (i *Ink) AnnotationType() pdf.Name {
	return "Ink"
}

func decodeInk(x *pdf.Extractor, dict pdf.Dict) (*Ink, error) {
	r := x.R
	ink := &Ink{}

	// Extract common annotation fields
	if err := decodeCommon(x, &ink.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, dict, &ink.Markup); err != nil {
		return nil, err
	}

	// Extract ink-specific fields
	// InkList (required)
	if inkList, err := pdf.GetArray(r, dict["InkList"]); err == nil && len(inkList) > 0 {
		paths := make([][]float64, len(inkList))
		for i, pathEntry := range inkList {
			if coords, err := pdf.GetFloatArray(r, pathEntry); err == nil && len(coords) > 0 {
				paths[i] = coords
			} else {
				paths[i] = []float64{} // Default to empty slice if extraction fails
			}
		}
		ink.InkList = paths
	}

	// BS (optional)
	if bs, err := pdf.Optional(pdf.ExtractorGet(x, dict["BS"], ExtractBorderStyle)); err != nil {
		return nil, err
	} else {
		ink.BorderStyle = bs
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
			ink.Path = pathArrays
		}
	}

	return ink, nil
}

func (i *Ink) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "ink annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Ink"),
	}

	// Add common annotation fields
	if err := i.Common.fillDict(rm, dict, isMarkup(i)); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := i.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add ink-specific fields
	// InkList (required)
	if len(i.InkList) > 0 {
		inkArray := make(pdf.Array, len(i.InkList))
		for i, path := range i.InkList {
			if len(path) > 0 {
				pathArray := make(pdf.Array, len(path))
				for j, coord := range path {
					pathArray[j] = pdf.Number(coord)
				}
				inkArray[i] = pathArray
			} else {
				inkArray[i] = pdf.Array{} // Empty array for empty paths
			}
		}
		dict["InkList"] = inkArray
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
		pathArray := make(pdf.Array, 0, len(i.Path))
		for _, pathEntry := range i.Path {
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
