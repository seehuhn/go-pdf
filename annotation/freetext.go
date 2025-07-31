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
	"fmt"

	"seehuhn.de/go/pdf"
)

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

	// DefaultStyle (optional) is a default style string.
	//
	// This corresponds to the /DS entry in the PDF annotation dictionary.
	DefaultStyle string

	// Padding (optional) describes the numerical differences between
	// the Rect entry and an inner rectangle where the text should be displayed.
	//
	// Slice of four numbers: [left, top, right, bottom]
	//
	// This corresponds to the /RD entry in the PDF annotation dictionary.
	Padding []float64

	// Align specifies the text alignment used for the annotation's text.
	// The zero value if [FreeTextAlignLeft].
	// The other allowed values are [FreeTextAlignCenter] and
	// [FreeTextAlignRight].
	//
	// This corresponds to the /Q entry in the PDF annotation dictionary.
	Align FreeTextAlign

	// CalloutLine (used only if Markup.Intent is [FreeTextIntentCallout]; PDF 1.6)
	// specifies a callout line attached to the free text annotation.
	// Four numbers [x1 y1 x2 y2] represent start and end coordinates.
	// Six numbers [x1 y1 x2 y2 x3 y3] represent start, knee, and end coordinates.
	//
	// This corresponds to the /CL entry in the PDF annotation dictionary.
	CalloutLine []float64

	// LineEndingStyle (optional; meaningful only if CalloutLine is present)
	// specifies the line ending style for the callout line endpoint (x1, y1).
	//
	// When writing annotations an empty string may be used as a shorthand
	// for [LineEndingStyleNone]
	//
	// This corresponds to the /LE entry in the PDF annotation dictionary.
	LineEndingStyle LineEndingStyle

	// BorderEffect (optional) is a border effect dictionary used in
	// conjunction with the border style dictionary specified by BorderStyle.
	BorderEffect *BorderEffect

	// BorderStyle (optional) is a border style dictionary specifying the line
	// width and dash pattern for drawing the annotation's border.
	//
	// If the BorderStyle field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle
}

var _ Annotation = (*FreeText)(nil)

// AnnotationType returns "FreeText".
// This implements the [Annotation] interface.
func (f *FreeText) AnnotationType() pdf.Name {
	return "FreeText"
}

func extractFreeText(r pdf.Getter, dict pdf.Dict) (*FreeText, error) {
	f := &FreeText{}

	if err := decodeCommon(r, &f.Common, dict); err != nil {
		return nil, err
	}
	if err := decodeMarkup(r, dict, &f.Markup); err != nil {
		return nil, err
	}

	if da, err := pdf.Optional(pdf.GetTextString(r, dict["DA"])); err != nil {
		return nil, err
	} else {
		f.DefaultAppearance = string(da)
	}

	if q, err := pdf.Optional(pdf.GetInteger(r, dict["Q"])); err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		f.Align = FreeTextAlign(q)
	}

	if ds, err := pdf.Optional(pdf.GetTextString(r, dict["DS"])); err != nil {
		return nil, err
	} else {
		f.DefaultStyle = string(ds)
	}

	if cl, err := pdf.Optional(pdf.GetArray(r, dict["CL"])); err != nil {
		return nil, err
	} else if f.Intent == FreeTextIntentCallout && (len(cl) == 4 || len(cl) == 6) {
		coords := make([]float64, len(cl))
		for i, coord := range cl {
			if num, err := pdf.GetNumber(r, coord); err == nil {
				coords[i] = float64(num)
			}
		}
		f.CalloutLine = coords
	}

	if be, err := pdf.Optional(ExtractBorderEffect(r, dict["BE"])); err != nil {
		return nil, err
	} else {
		f.BorderEffect = be
	}

	if rd, err := pdf.Optional(pdf.GetArray(r, dict["RD"])); err != nil {
		return nil, err
	} else if len(rd) == 4 {
		a := make([]float64, 4)
		for i, diff := range rd {
			num, _ := pdf.GetNumber(r, diff)
			a[i] = max(float64(num), 0)
		}
		f.Padding = a
	}

	if bs, err := pdf.Optional(ExtractBorderStyle(r, dict["BS"])); err != nil {
		return nil, err
	} else {
		f.BorderStyle = bs
	}

	if f.Intent == FreeTextIntentCallout {
		if le, err := pdf.Optional(pdf.GetName(r, dict["LE"])); err != nil {
			return nil, err
		} else if le != "" {
			f.LineEndingStyle = LineEndingStyle(le)
		} else {
			f.LineEndingStyle = LineEndingStyleNone
		}
	}

	return f, nil
}

func (f *FreeText) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "free text annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	switch f.Markup.Intent {
	case "":
		// intent is optional
	case FreeTextIntentPlain, FreeTextIntentCallout, FreeTextIntentTypeWriter:
		// valid intent values
	default:
		return nil, fmt.Errorf("invalid Intent %q for free text annotation", f.Intent)
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("FreeText"),
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Annot")
	}

	if err := f.Common.fillDict(rm, dict, isMarkup(f)); err != nil {
		return nil, err
	}
	if err := f.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add free text-specific fields
	dict["DA"] = pdf.TextString(f.DefaultAppearance)

	if f.Align != FreeTextAlignLeft {
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

	if f.CalloutLine != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation CL entry", pdf.V1_6); err != nil {
			return nil, err
		}
		if f.Intent != FreeTextIntentCallout {
			return nil, errors.New("CL entry present but Intent is not FreeTextIntentCallout")
		}
		if len(f.CalloutLine) != 4 && len(f.CalloutLine) != 6 {
			return nil, errors.New("invalid length for CL array")
		}
		clArray := make(pdf.Array, len(f.CalloutLine))
		for i, coord := range f.CalloutLine {
			clArray[i] = pdf.Number(coord)
		}
		dict["CL"] = clArray
	}

	if f.BorderEffect != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation BE entry", pdf.V1_6); err != nil {
			return nil, err
		}
		be, _, err := f.BorderEffect.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["BE"] = be
	}

	if f.Padding != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation RD entry", pdf.V1_6); err != nil {
			return nil, err
		}
		if len(f.Padding) != 4 {
			return nil, errors.New("invalid length for RD array")
		}
		rd := make(pdf.Array, len(f.Padding))
		for i, xi := range f.Padding {
			if xi < 0 {
				return nil, fmt.Errorf("invalid entry %f in RD array", xi)
			}
			rd[i] = pdf.Number(xi)
		}
		dict["RD"] = rd
	}

	if f.BorderStyle != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation BS entry", pdf.V1_3); err != nil {
			return nil, err
		}
		bs, _, err := f.BorderStyle.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	if f.CalloutLine != nil && f.LineEndingStyle != "" && f.LineEndingStyle != LineEndingStyleNone {
		if err := pdf.CheckVersion(rm.Out, "free text annotation LE entry", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["LE"] = pdf.Name(f.LineEndingStyle)
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

// These constants represent the allowed values for the Markup.Intent
// field in free text annotations.
const (
	// FreeTextIntentPlain creates a standard text box annotation with visible
	// borders and background. Use this for general text comments and notes.
	FreeTextIntentPlain pdf.Name = "FreeText"

	// FreeTextIntentCallout creates a callout annotation that points to a
	// specific area on the page using an arrow or line.  Use this to annotate
	// or reference particular elements in the document.
	FreeTextIntentCallout pdf.Name = "FreeTextCallout"

	// FreeTextIntentTypeWriter creates a typewriter-style annotation with
	// transparent background and opaque text. This is similar to how one might
	// fill in a paper form using a typewriter.
	FreeTextIntentTypeWriter pdf.Name = "FreeTextTypeWriter"
)
