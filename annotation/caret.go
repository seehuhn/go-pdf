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

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.11

// Caret represents a caret annotation that is a visual symbol indicating
// the presence of text edits. When opened, it displays a pop-up window
// containing the text of an associated note.
//
//   - The caret is drawn as a filled shape within the annotation Rect,
//     inset by the Margin (RD) if specified.
//   - The caret color is specified by the Common.Color field.
//     If this is nil, no visible caret is drawn.
//   - When Symbol is set to "P", a pilcrow (¶) is drawn below the caret
//     and the Rect and Margin are expanded to accommodate it.
type Caret struct {
	Common
	Markup

	// Margin (optional) describes the numerical differences between the Rect
	// entry of the annotation and the actual boundaries of the underlying
	// caret. This can occur when a paragraph symbol specified by Sy is
	// displayed along with the caret.
	//
	// Slice of four numbers: [left, bottom, right, top]
	//
	// TODO(voss): review this once
	// https://github.com/pdf-association/pdf-issues/issues/592 is resolved.
	//
	// This corresponds to the /RD entry in the PDF annotation dictionary.
	Margin []float64

	// Symbol (optional) specifies a symbol that is associated with the caret:
	//  - "P": A new paragraph symbol (¶) is associated with the caret
	//  - "None": No symbol is associated with the caret
	//
	// On write, an empty string is treated as "None".
	//
	// This corresponds to the /SY entry in the PDF annotation dictionary.
	Symbol pdf.Name
}

var _ Annotation = (*Caret)(nil)

// AnnotationType returns "Caret".
// This implements the [Annotation] interface.
func (c *Caret) AnnotationType() pdf.Name {
	return "Caret"
}

func decodeCaret(x *pdf.Extractor, dict pdf.Dict) (*Caret, error) {
	caret := &Caret{}

	// Extract common annotation fields
	if err := decodeCommon(x, &caret.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, dict, &caret.Markup); err != nil {
		return nil, err
	}

	// Extract caret-specific fields
	// RD (optional)
	if rd, err := pdf.GetFloatArray(x.R, dict["RD"]); err == nil && len(rd) == 4 {
		caret.Margin = rd
	}

	// Sy (optional)
	if sy, err := x.GetName(dict["Sy"]); err == nil && sy != "" {
		caret.Symbol = sy
	}
	if caret.Symbol == "" {
		caret.Symbol = "None"
	}

	return caret, nil
}

func (c *Caret) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "caret annotation", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Caret"),
	}

	// Add common annotation fields
	if err := c.Common.fillDict(rm, dict, isMarkup(c), false); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := c.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add caret-specific fields
	// RD (optional)
	if len(c.Margin) == 4 {
		rdArray := make(pdf.Array, 4)
		for i, diff := range c.Margin {
			rdArray[i] = pdf.Number(diff)
		}
		dict["RD"] = rdArray
	}

	// Sy (optional) - only write if not the default value "None"
	if c.Symbol != "" && c.Symbol != "None" {
		dict["Sy"] = c.Symbol
	}

	return dict, nil
}
