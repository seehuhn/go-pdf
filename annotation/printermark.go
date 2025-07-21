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

// PrinterMark represents a printer's mark annotation that contains a graphic
// symbol, such as a registration target, color bar, or cut mark, that may be
// added to a page to assist production personnel in identifying components
// of a multiple-plate job and maintaining consistent output during production.
//
// The visual presentation is defined by a form XObject specified as an
// appearance stream in the AP entry. The Print and ReadOnly flags in the F
// entry is set and all others clear.
type PrinterMark struct {
	Common

	// MN (optional) is an arbitrary name identifying the type of printer's mark,
	// such as "ColorBar" or "RegistrationTarget".
	MN pdf.Name
}

var _ pdf.Annotation = (*PrinterMark)(nil)

// AnnotationType returns "PrinterMark".
// This implements the [pdf.Annotation] interface.
func (p *PrinterMark) AnnotationType() pdf.Name {
	return "PrinterMark"
}

func extractPrinterMark(r pdf.Getter, dict pdf.Dict, singleUse bool) (*PrinterMark, error) {
	printerMark := &PrinterMark{}

	// Extract common annotation fields
	if err := extractCommon(r, &printerMark.Common, dict, singleUse); err != nil {
		return nil, err
	}

	// Extract printer's mark-specific fields
	// MN (optional)
	if mn, err := pdf.GetName(r, dict["MN"]); err == nil && mn != "" {
		printerMark.MN = mn
	}

	return printerMark, nil
}

func (p *PrinterMark) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	dict, err := p.AsDict(rm)
	if err != nil {
		return nil, zero, err
	}

	if p.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	return ref, zero, err
}

func (p *PrinterMark) AsDict(rm *pdf.ResourceManager) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "printer's mark annotation", pdf.V1_4); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("PrinterMark"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add printer's mark-specific fields
	// MN (optional)
	if p.MN != "" {
		dict["MN"] = p.MN
	}

	return dict, nil
}
