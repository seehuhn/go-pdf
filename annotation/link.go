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

// Link represents a hypertext link annotation.
type Link struct {
	Common

	// A (optional; PDF 1.1) is an action that is performed when the
	// link annotation is activated. Mutually exclusive with Dest.
	A pdf.Reference

	// Dest (optional) is a destination that is displayed when the
	// annotation is activated. Not permitted if A is present.
	Dest pdf.Object

	// H (optional; PDF 1.2) is the annotation's highlighting mode.
	// Valid values: "N" (None), "I" (Invert), "O" (Outline), "P" (Push).
	// Default value: "I".
	H pdf.Name

	// PA (optional; PDF 1.3) is a URI action formerly associated with this
	// annotation, used for reverting go-to actions back to URI actions.
	PA pdf.Reference

	// QuadPoints (optional; PDF 1.6) specifies the coordinates of
	// quadrilaterals that comprise the region where the link should be
	// activated. Array of 8×n numbers (x1 y1 x2 y2 x3 y3 x4 y4 for each quad).
	QuadPoints []float64

	// BS (optional; PDF 1.6) is a border style dictionary specifying
	// the line width and dash pattern for drawing the annotation's border.
	BS pdf.Reference
}

var _ pdf.Annotation = (*Link)(nil)

// AnnotationType returns "Link".
// This implements the [pdf.Annotation] interface.
func (l *Link) AnnotationType() pdf.Name {
	return "Link"
}

func extractLink(r pdf.Getter, obj pdf.Object) (*Link, error) {
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	}

	link := &Link{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &link.Common); err != nil {
		return nil, err
	}

	// Extract link-specific fields
	if a, ok := dict["A"].(pdf.Reference); ok {
		link.A = a
	}

	if dest := dict["Dest"]; dest != nil {
		link.Dest = dest
	}

	if h, err := pdf.GetName(r, dict["H"]); err == nil {
		link.H = h
	}

	if pa, ok := dict["PA"].(pdf.Reference); ok {
		link.PA = pa
	}

	if quadPoints, err := pdf.GetArray(r, dict["QuadPoints"]); err == nil && len(quadPoints) > 0 {
		coords := make([]float64, len(quadPoints))
		for i, coord := range quadPoints {
			if num, err := pdf.GetNumber(r, coord); err == nil {
				coords[i] = float64(num)
			}
		}
		link.QuadPoints = coords
	}

	if bs, ok := dict["BS"].(pdf.Reference); ok {
		link.BS = bs
	}

	return link, nil
}

func (l *Link) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Link"),
	}

	// Add common annotation fields
	if err := l.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add link-specific fields
	if l.A != 0 {
		if err := pdf.CheckVersion(rm.Out, "link annotation A entry", pdf.V1_1); err != nil {
			return nil, zero, err
		}
		dict["A"] = l.A
	} else if l.Dest != nil {
		dict["Dest"] = l.Dest
	}

	if l.H != "" {
		if err := pdf.CheckVersion(rm.Out, "link annotation H entry", pdf.V1_2); err != nil {
			return nil, zero, err
		}
		dict["H"] = l.H
	}

	if l.PA != 0 {
		if err := pdf.CheckVersion(rm.Out, "link annotation PA entry", pdf.V1_3); err != nil {
			return nil, zero, err
		}
		dict["PA"] = l.PA
	}

	if l.QuadPoints != nil {
		if err := pdf.CheckVersion(rm.Out, "link annotation QuadPoints entry", pdf.V1_6); err != nil {
			return nil, zero, err
		}
		quadArray := make(pdf.Array, len(l.QuadPoints))
		for i, v := range l.QuadPoints {
			quadArray[i] = pdf.Number(v)
		}
		dict["QuadPoints"] = quadArray
	}

	if l.BS != 0 {
		if err := pdf.CheckVersion(rm.Out, "link annotation BS entry", pdf.V1_6); err != nil {
			return nil, zero, err
		}
		dict["BS"] = l.BS
	}

	return dict, zero, nil
}
