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

	"seehuhn.de/go/geom/vec"

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

	// Align specifies the text alignment used for the annotation's text.
	// The zero value if [TextAlignLeft].
	// The other allowed values are [TextAlignCenter] and [TextAlignRight].
	//
	// This corresponds to the /Q entry in the PDF annotation dictionary.
	Align TextAlign

	// Margin (optional) describes the numerical differences between the
	// Common.Rect entry and an inner rectangle where the text should be
	// displayed.  The border, if any, applies to the inner rectangle. If
	// Markup.Intent is [FreeTextIntentCallout], the callout line is usually
	// located in the margin between the inner rectangle and Common.Rect.
	//
	// Slice of four numbers: [left, bottom, right, top]
	//
	// TODO(voss): review this once
	// https://github.com/pdf-association/pdf-issues/issues/592 is resolved.
	//
	// This corresponds to the /RD entry in the PDF annotation dictionary.
	Margin []float64

	// BorderStyle (optional) is a border style dictionary specifying the line
	// width and dash pattern for drawing the annotation's border.
	//
	// If the BorderStyle field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// BorderEffect (optional) is a border effect dictionary used in
	// conjunction with the border style dictionary specified by BorderStyle.
	//
	// This corresponds to the /BE entry in the PDF annotation dictionary.
	BorderEffect *BorderEffect

	// CalloutLine (used only if Markup.Intent is [FreeTextIntentCallout]; PDF 1.6)
	// specifies a callout line attached to the free text annotation.
	// Must contain either 2 points (start and end) or 3 points (start, knee, and end).
	//
	// This corresponds to the /CL entry in the PDF annotation dictionary.
	CalloutLine []vec.Vec2

	// LineEndingStyle (optional; meaningful only if CalloutLine is present)
	// specifies the line ending style for the callout line endpoint (x1, y1).
	//
	// When writing annotations an empty name may be used as a shorthand for
	// [LineEndingStyleNone].
	//
	// This corresponds to the /LE entry in the PDF annotation dictionary.
	LineEndingStyle LineEndingStyle
}

var _ Annotation = (*FreeText)(nil)

// AnnotationType returns "FreeText".
// This implements the [Annotation] interface.
func (f *FreeText) AnnotationType() pdf.Name {
	return "FreeText"
}

func decodeFreeText(x *pdf.Extractor, dict pdf.Dict) (*FreeText, error) {
	f := &FreeText{}

	if err := decodeCommon(x, &f.Common, dict); err != nil {
		return nil, err
	}
	if err := decodeMarkup(x, dict, &f.Markup); err != nil {
		return nil, err
	}

	if da, err := pdf.Optional(pdf.GetTextString(x.R, dict["DA"])); err != nil {
		return nil, err
	} else {
		f.DefaultAppearance = string(da)
	}

	if q, err := pdf.Optional(x.GetInteger(dict["Q"])); err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		f.Align = TextAlign(q)
	}

	if ds, err := pdf.Optional(pdf.GetTextString(x.R, dict["DS"])); err != nil {
		return nil, err
	} else {
		f.DefaultStyle = string(ds)
	}

	if cl, err := pdf.Optional(x.GetArray(dict["CL"])); err != nil {
		return nil, err
	} else if f.Intent == FreeTextIntentCallout && (len(cl) == 4 || len(cl) == 6) {
		points := make([]vec.Vec2, len(cl)/2)
		for i := 0; i < len(points); i++ {
			if px, err := x.GetNumber(cl[i*2]); err == nil {
				if py, err := x.GetNumber(cl[i*2+1]); err == nil {
					points[i] = vec.Vec2{X: px, Y: py}
				}
			}
		}
		f.CalloutLine = points
	}

	if be, err := pdf.Optional(ExtractBorderEffect(x.R, dict["BE"])); err != nil {
		return nil, err
	} else {
		f.BorderEffect = be
	}

	if rd, err := pdf.Optional(x.GetArray(dict["RD"])); err != nil {
		return nil, err
	} else if len(rd) == 4 {
		a := make([]float64, 4)
		for i, diff := range rd {
			num, _ := x.GetNumber(diff)
			a[i] = max(num, 0)
		}
		f.Margin = a
	}

	if bs, err := pdf.ExtractorGetOptional(x, dict["BS"], ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		f.BorderStyle = bs
	}

	if f.Intent == FreeTextIntentCallout {
		if le, err := pdf.Optional(x.GetName(dict["LE"])); err != nil {
			return nil, err
		} else if le != "" {
			f.LineEndingStyle = LineEndingStyle(le)
		} else {
			f.LineEndingStyle = LineEndingStyleNone
		}
	}

	return f, nil
}

func (f *FreeText) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
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

	if f.Align != TextAlignLeft {
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
		if len(f.CalloutLine) != 2 && len(f.CalloutLine) != 3 {
			return nil, errors.New("invalid CalloutLine length: must contain 2 or 3 points")
		}
		clArray := make(pdf.Array, len(f.CalloutLine)*2)
		for i, point := range f.CalloutLine {
			clArray[i*2] = pdf.Number(pdf.Round(point.X, 2))
			clArray[i*2+1] = pdf.Number(pdf.Round(point.Y, 2))
		}
		dict["CL"] = clArray
	}

	if f.BorderEffect != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation BE entry", pdf.V1_6); err != nil {
			return nil, err
		}
		be, err := rm.Embed(f.BorderEffect)
		if err != nil {
			return nil, err
		}
		dict["BE"] = be
	}

	if f.Margin != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation RD entry", pdf.V1_6); err != nil {
			return nil, err
		}
		if len(f.Margin) != 4 {
			return nil, errors.New("invalid length for RD array")
		}
		rd := make(pdf.Array, len(f.Margin))
		for i, xi := range f.Margin {
			if xi < 0 {
				return nil, fmt.Errorf("invalid entry %f in RD array", xi)
			}
			rd[i] = pdf.Number(pdf.Round(xi, 4))
		}

		if f.Margin[0]+f.Margin[2] >= f.Rect.Dx() {
			return nil, errors.New("left and right margins exceed rectangle width")
		}
		if f.Margin[1]+f.Margin[3] >= f.Rect.Dy() {
			return nil, errors.New("top and bottom margins exceed rectangle height")
		}
		dict["RD"] = rd
	}

	if f.BorderStyle != nil {
		if err := pdf.CheckVersion(rm.Out, "free text annotation BS entry", pdf.V1_3); err != nil {
			return nil, err
		}
		bs, err := rm.Embed(f.BorderStyle)
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

func (f *FreeText) BorderWidth() float64 {
	if f.BorderStyle != nil {
		return f.BorderStyle.Width
	}
	if f.Common.Border != nil {
		return f.Common.Border.Width
	}
	return 0 // Go default = no border
}

// TextAlign represents the text justification options for free text
// annotations.
type TextAlign int

const (
	TextAlignLeft   TextAlign = 0
	TextAlignCenter TextAlign = 1
	TextAlignRight  TextAlign = 2
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
