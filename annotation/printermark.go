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
)

// PDF 2.0 sections: 12.5.2 12.5.6.20 14.11.3

// PrinterMark represents a printer's mark annotation that contains a graphic
// symbol, such as a registration target, color bar, or cut mark, that may be
// added to a page to assist production personnel in identifying components of
// a multiple-plate job and maintaining consistent output during production.
//
// The visual presentation is defined by Common.Appearance, which is required
// for this annotation type.
//
// Common.Flags must be set to FlagPrint|FlagReadOnly .
type PrinterMark struct {
	Common

	// MarkName (optional) is an arbitrary name identifying the type of
	// printer's mark, such as "ColorBar" or "RegistrationTarget".
	//
	// This corresponds to the /MN entry in the PDF annotation dictionary.
	MarkName pdf.Name
}

var _ Annotation = (*PrinterMark)(nil)

// AnnotationType returns "PrinterMark".
// This implements the [Annotation] interface.
func (p *PrinterMark) AnnotationType() pdf.Name {
	return "PrinterMark"
}

func (p *PrinterMark) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "printer's mark annotation", pdf.V1_4); err != nil {
		return nil, err
	}

	if p.Flags != FlagPrint|FlagReadOnly {
		return nil, errors.New("printer's mark needs Print and ReadOnly flags only")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("PrinterMark"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict, isMarkup(p), false); err != nil {
		return nil, err
	}

	// Add printer's mark-specific fields

	// MN (optional)
	if p.MarkName != "" {
		dict["MN"] = p.MarkName
	}

	return dict, nil
}
