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

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.6

// FreeText represents a free text annotation that displays text directly on the page.
type FreeText struct {
	Common
	Markup

	// DefaultAppearance is the default appearance string that is used in
	// formatting the text.
	//
	// This corresponds to the /DA entry in the PDF annotation dictionary.
	DefaultAppearance string

	// Align specifies the text alignment used for the annotation's text.
	// The zero value if [FreeTextAlignLeft].
	// The other allowed values are [FreeTextAlignCenter] and
	// [FreeTextAlignRight].
	//
	// This corresponds to the /Q entry in the PDF annotation dictionary.
	Align FreeTextAlign

	// DefaultStyle (optional) is a default style string.
	//
	// This corresponds to the /DS entry in the PDF annotation dictionary.
	DefaultStyle string

	// CL (optional; meaningful only if IT is FreeTextCallout; PDF 1.6)
	// specifies a callout line attached to the free text annotation.
	// Four numbers [x1 y1 x2 y2] represent start and end coordinates.
	// Six numbers [x1 y1 x2 y2 x3 y3] represent start, knee, and end coordinates.
	CL []float64

	// BE (optional; PDF 1.6) is a border effect dictionary used in
	// conjunction with the border style dictionary specified by BS.
	BE *BorderEffect

	// RD (optional; PDF 1.6) describes the numerical differences between
	// the Rect entry and an inner rectangle where the text should be displayed.
	// Array of four numbers: [left, top, right, bottom] differences.
	RD []float64

	// BS (optional; PDF 1.3) is a border style dictionary specifying
	// the line width and dash pattern for drawing the annotation's border.
	BS *BorderStyle

	// LE (optional; meaningful only if CL is present; PDF 1.6) specifies
	// the line ending style for the callout line endpoint (x1, y1).
	LE pdf.Name
}

var _ Annotation = (*FreeText)(nil)

// AnnotationType returns "FreeText".
// This implements the [Annotation] interface.
func (f *FreeText) AnnotationType() pdf.Name {
	return "FreeText"
}

func extractFreeText(r pdf.Getter, dict pdf.Dict) (*FreeText, error) {
	freeText := &FreeText{}

	// Extract common annotation fields
	if err := decodeCommon(r, &freeText.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(r, dict, &freeText.Markup); err != nil {
		return nil, err
	}

	// Extract free text-specific fields
	// DA is required for free text annotations
	if dict["DA"] == nil {
		return nil, pdf.Error("missing required DA field in free text annotation")
	}
	da, err := pdf.GetTextString(r, dict["DA"])
	if err != nil {
		return nil, err
	}
	freeText.DefaultAppearance = string(da)

	q, err := pdf.Optional(pdf.GetInteger(r, dict["Q"]))
	if err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		freeText.Align = FreeTextAlign(q)
	}

	if dict["DS"] != nil {
		if ds, err := pdf.Optional(pdf.GetTextString(r, dict["DS"])); err != nil {
			return nil, err
		} else {
			freeText.DefaultStyle = string(ds)
		}
	}

	if cl, err := pdf.Optional(pdf.GetArray(r, dict["CL"])); err != nil {
		return nil, err
	} else if cl != nil {
		// CL must have exactly 4 or 6 numbers
		if len(cl) != 4 && len(cl) != 6 {
			// Ignore invalid CL arrays per permissive reading guidelines
		} else {
			coords := make([]float64, len(cl))
			valid := true
			for i, coord := range cl {
				if num, err := pdf.GetNumber(r, coord); err == nil {
					coords[i] = float64(num)
				} else {
					valid = false
					break
				}
			}
			if valid {
				freeText.CL = coords
			}
		}
	}

	if beObj := dict["BE"]; beObj != nil {
		if be, err := ExtractBorderEffect(r, beObj); err == nil {
			freeText.BE = be
		}
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

	if bsObj := dict["BS"]; bsObj != nil {
		if bs, err := ExtractBorderStyle(r, bsObj); err == nil {
			freeText.BS = bs
		}
	}

	if le, err := pdf.Optional(pdf.GetName(r, dict["LE"])); err != nil {
		return nil, err
	} else if le != "" {
		freeText.LE = le
	}

	return freeText, nil
}

func (f *FreeText) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
	dict := pdf.Dict{
		"Subtype": pdf.Name("FreeText"),
	}

	// Add Type field if required by options
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Annot")
	}

	// Add common annotation fields
	if err := f.Common.fillDict(rm, dict, isMarkup(f)); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := f.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Validate Intent field for free text annotations
	if f.Markup.Intent != "" {
		switch f.Markup.Intent {
		case "FreeText", "FreeTextCallout", "FreeTextTypeWriter":
			// Valid intent values
		default:
			return nil, pdf.Error("invalid Intent value for free text annotation: " + string(f.Markup.Intent))
		}
	}

	// Add free text-specific fields
	if f.DefaultAppearance != "" {
		dict["DA"] = pdf.TextString(f.DefaultAppearance)
	}

	if f.Align != 0 {
		if err := pdf.CheckVersion(rm.Out, "free text annotation Q entry", pdf.V1_4); err != nil {
			return nil, err
		}
		dict["Q"] = pdf.Integer(f.Align)
	}

	if f.DefaultStyle != "" {
		if err := pdf.CheckVersion(rm.Out, "free text annotation DS entry", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["DS"] = pdf.TextString(f.DefaultStyle)
	}

	if f.CL != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation CL entry", pdf.V1_6); err != nil {
			return nil, err
		}
		// CL must have exactly 4 or 6 numbers
		if len(f.CL) != 4 && len(f.CL) != 6 {
			return nil, pdf.Error("CL array must have exactly 4 or 6 numbers")
		}
		clArray := make(pdf.Array, len(f.CL))
		for i, coord := range f.CL {
			clArray[i] = pdf.Number(coord)
		}
		dict["CL"] = clArray
	}

	if f.BE != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation BE entry", pdf.V1_6); err != nil {
			return nil, err
		}
		beObj, _, err := f.BE.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["BE"] = beObj
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

	if f.BS != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation BS entry", pdf.V1_3); err != nil {
			return nil, err
		}
		bsObj, _, err := f.BS.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bsObj
	}

	if f.LE != "" {
		if err := pdf.CheckVersion(rm.Out, "free text annotation LE entry", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["LE"] = f.LE
	}

	return dict, nil
}

// FreeTextAlign represents the text justification options for free text
// annotations.
type FreeTextAlign int

const (
	FreeTextAlignLeft   FreeTextAlign = 0
	FreeTextAlignCenter FreeTextAlign = 1
	FreeTextAlignRight  FreeTextAlign = 2
)
