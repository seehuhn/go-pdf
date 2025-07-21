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

	// BS is a border style dictionary specifying the line width and dash
	// pattern for drawing the annotation's border.
	BS *Border
}

var _ pdf.Annotation = (*Link)(nil)

// AnnotationType returns "Link".
// This implements the [pdf.Annotation] interface.
func (l *Link) AnnotationType() pdf.Name {
	return "Link"
}

func extractLink(r pdf.Getter, dict pdf.Dict, singleUse bool) (*Link, error) {
	link := &Link{}

	// Extract common annotation fields
	if err := extractCommon(r, &link.Common, dict, singleUse); err != nil {
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

	// BS (optional)
	if border, err := pdf.GetArray(r, dict["BS"]); err == nil && border != nil {
		if len(border) >= 3 {
			b := &Border{}
			if h, err := pdf.GetNumber(r, border[0]); err == nil {
				b.HCornerRadius = float64(h)
			}
			if v, err := pdf.GetNumber(r, border[1]); err == nil {
				b.VCornerRadius = float64(v)
			}
			if w, err := pdf.GetNumber(r, border[2]); err == nil {
				b.Width = float64(w)
			}
			if len(border) > 3 {
				if dashArray, err := pdf.GetArray(r, border[3]); err == nil {
					dashes := make([]float64, len(dashArray))
					for i, dash := range dashArray {
						if num, err := pdf.GetNumber(r, dash); err == nil {
							dashes[i] = float64(num)
						}
					}
					b.DashArray = dashes
				}
			}
			link.BS = b
		}
	}

	return link, nil
}

func (l *Link) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	dict, err := l.AsDict(rm)
	if err != nil {
		return nil, pdf.Unused{}, err
	}

	if l.SingleUse {
		return dict, pdf.Unused{}, nil
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	return ref, pdf.Unused{}, err
}

func (l *Link) AsDict(rm *pdf.ResourceManager) (pdf.Dict, error) {
	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Link"),
	}

	// Add common annotation fields
	if err := l.Common.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add link-specific fields
	if l.A != 0 {
		if err := pdf.CheckVersion(rm.Out, "link annotation A entry", pdf.V1_1); err != nil {
			return nil, err
		}
		dict["A"] = l.A
	} else if l.Dest != nil {
		dict["Dest"] = l.Dest
	}

	if l.H != "" {
		if err := pdf.CheckVersion(rm.Out, "link annotation H entry", pdf.V1_2); err != nil {
			return nil, err
		}
		dict["H"] = l.H
	}

	if l.PA != 0 {
		if err := pdf.CheckVersion(rm.Out, "link annotation PA entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["PA"] = l.PA
	}

	if l.QuadPoints != nil {
		if err := pdf.CheckVersion(rm.Out, "link annotation QuadPoints entry", pdf.V1_6); err != nil {
			return nil, err
		}
		quadArray := make(pdf.Array, len(l.QuadPoints))
		for i, v := range l.QuadPoints {
			quadArray[i] = pdf.Number(v)
		}
		dict["QuadPoints"] = quadArray
	}

	if l.BS != nil && !l.BS.isDefault() {
		if err := pdf.CheckVersion(rm.Out, "link annotation BS entry", pdf.V1_6); err != nil {
			return nil, err
		}
		borderArray := pdf.Array{
			pdf.Number(l.BS.HCornerRadius),
			pdf.Number(l.BS.VCornerRadius),
			pdf.Number(l.BS.Width),
		}
		if l.BS.DashArray != nil {
			if err := pdf.CheckVersion(rm.Out, "annotation Border dash array", pdf.V1_1); err != nil {
				return nil, err
			}
			dashArray := make(pdf.Array, len(l.BS.DashArray))
			for i, v := range l.BS.DashArray {
				dashArray[i] = pdf.Number(v)
			}
			borderArray = append(borderArray, dashArray)
		}
		dict["BS"] = borderArray
	}

	return dict, nil
}
