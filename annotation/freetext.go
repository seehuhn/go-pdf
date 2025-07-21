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

// FreeText represents a free text annotation that displays text directly on the page.
type FreeText struct {
	Common
	Markup

	// DA (required) is the default appearance string that is used in
	// formatting the text.
	DA string

	// Q (optional; PDF 1.4) is a code specifying the form of quadding
	// (justification) used in displaying the annotation's text:
	// 0 = Left-justified, 1 = Centred, 2 = Right-justified
	// Default value: 0 (left-justified).
	Q pdf.Integer

	// DS (optional; PDF 1.5) is a default style string.
	DS string

	// CL (optional; meaningful only if IT is FreeTextCallout; PDF 1.6)
	// specifies a callout line attached to the free text annotation.
	// Four numbers [x1 y1 x2 y2] represent start and end coordinates.
	// Six numbers [x1 y1 x2 y2 x3 y3] represent start, knee, and end coordinates.
	CL []float64

	// BE (optional; PDF 1.6) is a border effect dictionary used in
	// conjunction with the border style dictionary specified by BS.
	BE pdf.Reference

	// RD (optional; PDF 1.6) describes the numerical differences between
	// the Rect entry and an inner rectangle where the text should be displayed.
	// Array of four numbers: [left, top, right, bottom] differences.
	RD []float64

	// BS (optional; PDF 1.6) is a border style dictionary specifying
	// the line width and dash pattern for drawing the annotation's border.
	BS pdf.Reference

	// LE (optional; meaningful only if CL is present; PDF 1.6) specifies
	// the line ending style for the callout line endpoint (x1, y1).
	LE pdf.Name
}

var _ pdf.Annotation = (*FreeText)(nil)

// AnnotationType returns "FreeText".
// This implements the [pdf.Annotation] interface.
func (f *FreeText) AnnotationType() pdf.Name {
	return "FreeText"
}

func extractFreeText(r pdf.Getter, dict pdf.Dict, singleUse bool) (*FreeText, error) {
	freeText := &FreeText{}

	// Extract common annotation fields
	if err := extractCommon(r, &freeText.Common, dict, singleUse); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &freeText.Markup); err != nil {
		return nil, err
	}

	// Extract free text-specific fields
	if da, err := pdf.GetTextString(r, dict["DA"]); err == nil {
		freeText.DA = string(da)
	}

	if q, err := pdf.GetInteger(r, dict["Q"]); err == nil {
		freeText.Q = q
	}

	if ds, err := pdf.GetTextString(r, dict["DS"]); err == nil {
		freeText.DS = string(ds)
	}

	if cl, err := pdf.GetArray(r, dict["CL"]); err == nil && len(cl) > 0 {
		coords := make([]float64, len(cl))
		for i, coord := range cl {
			if num, err := pdf.GetNumber(r, coord); err == nil {
				coords[i] = float64(num)
			}
		}
		freeText.CL = coords
	}

	if be, ok := dict["BE"].(pdf.Reference); ok {
		freeText.BE = be
	}

	if rd, err := pdf.GetArray(r, dict["RD"]); err == nil && len(rd) == 4 {
		diffs := make([]float64, 4)
		for i, diff := range rd {
			if num, err := pdf.GetNumber(r, diff); err == nil {
				diffs[i] = float64(num)
			}
		}
		freeText.RD = diffs
	}

	if bs, ok := dict["BS"].(pdf.Reference); ok {
		freeText.BS = bs
	}

	if le, err := pdf.GetName(r, dict["LE"]); err == nil {
		freeText.LE = le
	}

	return freeText, nil
}

func (f *FreeText) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	dict, err := f.AsDict(rm)
	if err != nil {
		return nil, zero, err
	}

	if f.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	return ref, zero, err
}

func (f *FreeText) AsDict(rm *pdf.ResourceManager) (pdf.Dict, error) {

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("FreeText"),
	}

	// Add common annotation fields
	if err := f.Common.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := f.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add free text-specific fields
	if f.DA != "" {
		dict["DA"] = pdf.TextString(f.DA)
	}

	if f.Q != 0 {
		if err := pdf.CheckVersion(rm.Out, "free text annotation Q entry", pdf.V1_4); err != nil {
			return nil, err
		}
		dict["Q"] = f.Q
	}

	if f.DS != "" {
		if err := pdf.CheckVersion(rm.Out, "free text annotation DS entry", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["DS"] = pdf.TextString(f.DS)
	}

	if f.CL != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation CL entry", pdf.V1_6); err != nil {
			return nil, err
		}
		clArray := make(pdf.Array, len(f.CL))
		for i, coord := range f.CL {
			clArray[i] = pdf.Number(coord)
		}
		dict["CL"] = clArray
	}

	if f.BE != 0 {
		if err := pdf.CheckVersion(rm.Out, "free text annotation BE entry", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["BE"] = f.BE
	}

	if f.RD != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation RD entry", pdf.V1_6); err != nil {
			return nil, err
		}
		rdArray := make(pdf.Array, len(f.RD))
		for i, diff := range f.RD {
			rdArray[i] = pdf.Number(diff)
		}
		dict["RD"] = rdArray
	}

	if f.BS != 0 {
		if err := pdf.CheckVersion(rm.Out, "free text annotation BS entry", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["BS"] = f.BS
	}

	if f.LE != "" {
		if err := pdf.CheckVersion(rm.Out, "free text annotation LE entry", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["LE"] = f.LE
	}

	return dict, nil
}
