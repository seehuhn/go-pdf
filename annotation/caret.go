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

// Caret represents a caret annotation that is a visual symbol indicating
// the presence of text edits.
type Caret struct {
	Common
	Markup

	// RD (optional; PDF 1.5) describes the numerical differences between
	// the Rect entry of the annotation and the actual boundaries of the
	// underlying caret. This can occur when a paragraph symbol specified
	// by Sy is displayed along with the caret. The four numbers correspond
	// to the differences in default user space between the left, top, right,
	// and bottom coordinates of Rect and those of the caret, respectively.
	RD []float64

	// Sy (optional) specifies a symbol that is associated with the caret:
	// "P" - A new paragraph symbol (¶) is associated with the caret
	// "None" - No symbol is associated with the caret
	// Default value: "None"
	Sy pdf.Name
}

var _ Annotation = (*Caret)(nil)

// AnnotationType returns "Caret".
// This implements the [Annotation] interface.
func (c *Caret) AnnotationType() pdf.Name {
	return "Caret"
}

func extractCaret(r pdf.Getter, dict pdf.Dict) (*Caret, error) {
	caret := &Caret{}

	// Extract common annotation fields
	if err := decodeCommon(r, &caret.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(r, dict, &caret.Markup); err != nil {
		return nil, err
	}

	// Extract caret-specific fields
	// RD (optional)
	if rd, err := pdf.GetFloatArray(r, dict["RD"]); err == nil && len(rd) == 4 {
		caret.RD = rd
	}

	// Sy (optional)
	if sy, err := pdf.GetName(r, dict["Sy"]); err == nil && sy != "" {
		caret.Sy = sy
	}

	return caret, nil
}

func (c *Caret) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "caret annotation", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Caret"),
	}

	// Add common annotation fields
	if err := c.Common.fillDict(rm, dict, isMarkup(c)); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := c.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add caret-specific fields
	// RD (optional)
	if len(c.RD) == 4 {
		rdArray := make(pdf.Array, 4)
		for i, diff := range c.RD {
			rdArray[i] = pdf.Number(diff)
		}
		dict["RD"] = rdArray
	}

	// Sy (optional) - only write if not the default value "None"
	if c.Sy != "" && c.Sy != "None" {
		dict["Sy"] = c.Sy
	}

	return dict, nil
}
