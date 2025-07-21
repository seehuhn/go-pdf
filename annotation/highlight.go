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

// Highlight represents a highlight annotation that appears as highlighted text.
// When opened, it displays a popup window containing the text of the associated note.
type Highlight struct {
	Common
	Markup

	// QuadPoints (required) is an array of 8×n numbers specifying the coordinates
	// of n quadrilaterals in default user space. Each quadrilateral encompasses
	// a word or group of contiguous words in the text underlying the annotation.
	// The coordinates for each quadrilateral are given in the order:
	// x1 y1 x2 y2 x3 y3 x4 y4
	// specifying the quadrilateral's four vertices in counterclockwise order.
	QuadPoints []float64
}

var _ pdf.Annotation = (*Highlight)(nil)

// AnnotationType returns "Highlight".
// This implements the [pdf.Annotation] interface.
func (h *Highlight) AnnotationType() pdf.Name {
	return "Highlight"
}

func extractHighlight(r pdf.Getter, dict pdf.Dict, singleUse bool) (*Highlight, error) {
	highlight := &Highlight{}

	// Extract common annotation fields
	if err := extractCommon(r, &highlight.Common, dict, singleUse); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &highlight.Markup); err != nil {
		return nil, err
	}

	// Extract highlight-specific fields
	// QuadPoints (required)
	if quadPoints, err := pdf.GetArray(r, dict["QuadPoints"]); err == nil && len(quadPoints) > 0 {
		coords := make([]float64, len(quadPoints))
		for i, point := range quadPoints {
			if num, err := pdf.GetNumber(r, point); err == nil {
				coords[i] = float64(num)
			}
		}
		highlight.QuadPoints = coords
	}

	return highlight, nil
}

func (h *Highlight) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	dict, err := h.AsDict(rm)
	if err != nil {
		return nil, zero, err
	}

	if h.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	return ref, zero, err
}

func (h *Highlight) AsDict(rm *pdf.ResourceManager) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "highlight annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Highlight"),
	}

	// Add common annotation fields
	if err := h.Common.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := h.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add highlight-specific fields
	// QuadPoints (required)
	if len(h.QuadPoints) > 0 {
		quadArray := make(pdf.Array, len(h.QuadPoints))
		for i, point := range h.QuadPoints {
			quadArray[i] = pdf.Number(point)
		}
		dict["QuadPoints"] = quadArray
	}

	return dict, nil
}
