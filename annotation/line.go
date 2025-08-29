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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/measure"
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.7

// Line represents a line annotation that displays a single straight line on
// the page.
//
// The line style and dash pattern is determined as follows:
//   - If the BorderStyle field is set, this is used.
//   - Otherwise, if the Common.Border field is set, this is used.
//   - Otherwise, a solid line with a width of 1 point is used.
type Line struct {
	Common
	Markup

	// Coords is an array of four numbers [x1 y1 x2 y2] specifying the
	// starting and ending coordinates of the line in default user space.
	//
	// This corresponds to the /L entry in the PDF annotation dictionary.
	Coords [4]float64

	// BorderStyle (optional) is a border style dictionary specifying the width
	// and dash pattern that is used in drawing the line.
	//
	// If the BorderStyle field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// LineEndingStyle (PDF 1.4) is an array of two names specifying the line
	// ending styles for the start and end points respectively.
	//
	// When writing annotations empty names may be used as a shorthand for
	// [LineEndingStyleNone].
	//
	// This corresponds to the /LE entry in the PDF annotation dictionary.
	LineEndingStyle [2]LineEndingStyle

	// FillColor (optional; PDF 1.4) is the color used to fill the
	// annotation's line endings, if applicable.
	//
	// Only certain color types are allowed:
	//  - colors in the [color.DeviceGray] color space
	//  - colors in the [color.DeviceRGB] color space
	//  - colors in the [color.DeviceCMYK] color space
	//  - the [Transparent] color
	//
	// This corresponds to the /IC entry in the PDF annotation dictionary.
	FillColor color.Color

	// LL (Leader Line Length) shifts the line perpendicular to its direction
	// and connects the original endpoints to the shifted line with
	// perpendicular segments.
	//
	//  - Positive values: shift the line to the left (when viewed from start to end)
	//  - Negative values: shift the line to the right
	//
	// This creates a bracket-like shape: two perpendicular segments connecting
	// the original endpoints to a parallel shifted line. Commonly used for
	// dimension lines in technical drawings.
	LL float64

	// LLE (Leader Line Extensions) extends the perpendicular connecting
	// segments beyond the shifted line (when LL is non-zero). This makes the
	// "bracket arms" longer, creating more prominent dimension-line tick
	// marks.
	LLE float64

	// LLO (Leader Line Offset) creates empty space at the original endpoints
	// before the perpendicular connecting segments begin (when LL is
	// non-zero). This adds a gap between what you're measuring and the
	// dimension line.
	LLO float64

	// Caption indicates whether the text specified by Common.Contents or
	// Markup.RC entries are replicated as part of the line's appearance.
	//
	// This corresponds to the /Cap entry in the PDF annotation dictionary.
	Caption bool

	// CaptionAbove specifies whether the caption appears above the line. When
	// true, the caption is written parallel the line instead of embedded into
	// it.
	//
	// This corresponds to the /CP entry in the PDF annotation dictionary.
	CaptionAbove bool

	// CaptionOffset is an array of two numbers that specify the offset of the
	// caption text from its normal position: [hoffset, voffset].
	//
	// This corresponds to the /CO entry in the PDF annotation dictionary.
	CaptionOffset []float64

	// Measure (optional) is a measure dictionary that specifies the scale and
	// units that apply to the line annotation.
	Measure measure.Measure
}

var _ Annotation = (*Line)(nil)

// AnnotationType returns "Line".
// This implements the [Annotation] interface.
func (l *Line) AnnotationType() pdf.Name {
	return "Line"
}

func decodeLine(r pdf.Getter, dict pdf.Dict) (*Line, error) {
	line := &Line{}

	// Extract common annotation fields
	if err := decodeCommon(r, &line.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(r, dict, &line.Markup); err != nil {
		return nil, err
	}

	// Extract line-specific fields
	// L (required)
	if l, err := pdf.GetArray(r, dict["L"]); err == nil && len(l) == 4 {
		for i, coord := range l {
			if num, err := pdf.GetNumber(r, coord); err == nil {
				line.Coords[i] = float64(num)
			}
		}
	}

	// BS (optional)
	if bs, err := pdf.Optional(ExtractBorderStyle(r, dict["BS"])); err != nil {
		return nil, err
	} else {
		line.BorderStyle = bs
	}

	// LE (optional; PDF 1.4) - default is [None, None]
	line.LineEndingStyle = [2]LineEndingStyle{LineEndingStyleNone, LineEndingStyleNone}
	if le, err := pdf.Optional(pdf.GetArray(r, dict["LE"])); err != nil {
		return nil, err
	} else if len(le) >= 1 {
		if name, err := pdf.GetName(r, le[0]); err == nil {
			line.LineEndingStyle[0] = LineEndingStyle(name)
		}
		if len(le) >= 2 {
			if name, err := pdf.GetName(r, le[1]); err == nil {
				line.LineEndingStyle[1] = LineEndingStyle(name)
			}
		} else {
			// if only one element, copy first element to second
			line.LineEndingStyle[1] = line.LineEndingStyle[0]
		}
	}

	// IC (optional; PDF 1.4)
	if ic, err := pdf.Optional(extractColor(r, dict["IC"])); err != nil {
		return nil, err
	} else {
		line.FillColor = ic
	}

	// Cap (optional)
	if cap, err := pdf.GetBoolean(r, dict["Cap"]); err == nil {
		line.Caption = bool(cap)
	}

	if line.Caption {
		// CP (optional)
		if cp, err := pdf.GetName(r, dict["CP"]); err == nil && cp == "Top" {
			line.CaptionAbove = true
		}

		// CO (optional)
		if co, err := pdf.Optional(pdf.GetFloatArray(r, dict["CO"])); err != nil {
			return nil, err
		} else if len(co) == 2 {
			line.CaptionOffset = co
		}
	}

	// LL (optional)
	if ll, err := pdf.Optional(pdf.GetNumber(r, dict["LL"])); err != nil {
		return nil, err
	} else {
		line.LL = float64(ll)
	}

	// LLE (optional)
	if lle, err := pdf.Optional(pdf.GetNumber(r, dict["LLE"])); err != nil {
		return nil, err
	} else {
		line.LLE = max(float64(lle), 0)
	}

	// LLO (optional)
	if llo, err := pdf.Optional(pdf.GetNumber(r, dict["LLO"])); err != nil {
		return nil, err
	} else {
		line.LLO = max(float64(llo), 0)
	}

	// Measure (optional)
	if m, err := pdf.Optional(measure.Extract(r, dict["Measure"])); err != nil {
		return nil, err
	} else {
		line.Measure = m
	}

	return line, nil
}

func (l *Line) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	dict := pdf.Dict{
		"Subtype": pdf.Name("Line"),
	}

	// Add common annotation fields
	if err := l.Common.fillDict(rm, dict, isMarkup(l)); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := l.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add line-specific fields
	// L (required)
	lArray := make(pdf.Array, 4)
	for i, coord := range l.Coords {
		lArray[i] = pdf.Number(coord)
	}
	dict["L"] = lArray

	// BS (optional)
	if l.BorderStyle != nil {
		bs, _, err := pdf.ResourceManagerEmbed(rm, l.BorderStyle)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	// LE (optional; PDF 1.4) - only write if not default [None, None]
	// normalize empty strings to None as documented
	normalized := l.LineEndingStyle
	if normalized[0] == "" {
		normalized[0] = LineEndingStyleNone
	}
	if normalized[1] == "" {
		normalized[1] = LineEndingStyleNone
	}

	if normalized != [2]LineEndingStyle{LineEndingStyleNone, LineEndingStyleNone} {
		if err := pdf.CheckVersion(rm.Out, "line annotation LE entry", pdf.V1_4); err != nil {
			return nil, err
		}
		leArray := make(pdf.Array, 2)
		leArray[0] = pdf.Name(normalized[0])
		leArray[1] = pdf.Name(normalized[1])
		dict["LE"] = leArray
	}

	// IC (optional; PDF 1.4)
	if l.FillColor != nil {
		if err := pdf.CheckVersion(rm.Out, "line annotation IC entry", pdf.V1_4); err != nil {
			return nil, err
		}
		if icArray, err := encodeColor(l.FillColor); err != nil {
			return nil, err
		} else if icArray != nil {
			dict["IC"] = icArray
		}
	}

	// Cap (optional)
	if l.Caption {
		if err := pdf.CheckVersion(rm.Out, "line annotation Cap entry", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["Cap"] = pdf.Boolean(l.Caption)

		if l.CaptionAbove {
			if err := pdf.CheckVersion(rm.Out, "line annotation CP entry", pdf.V1_7); err != nil {
				return nil, err
			}
			dict["CP"] = pdf.Name("Top")
		}

		if l.CaptionOffset != nil {
			if err := pdf.CheckVersion(rm.Out, "line annotation CO entry", pdf.V1_7); err != nil {
				return nil, err
			}
			if len(l.CaptionOffset) != 2 {
				return nil, pdf.Error("invalid CaptionOffset")
			}
			coArray := make(pdf.Array, 2)
			for i, offset := range l.CaptionOffset {
				coArray[i] = pdf.Number(offset)
			}
			dict["CO"] = coArray
		}
	} else {
		if l.CaptionAbove {
			return nil, pdf.Error("unexpected CaptionAbove")
		}
		if l.CaptionOffset != nil {
			return nil, pdf.Error("unexpected CaptionOffset")
		}
	}

	// LL (optional)
	if l.LL != 0 || l.LLE != 0 {
		if err := pdf.CheckVersion(rm.Out, "line annotation LL entry", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["LL"] = pdf.Number(l.LL)
	}

	// LLE (optional)
	if l.LLE != 0 {
		if err := pdf.CheckVersion(rm.Out, "line annotation LLE entry", pdf.V1_6); err != nil {
			return nil, err
		}
		if l.LLE < 0 {
			return nil, pdf.Error("negative LLE value")
		}
		dict["LLE"] = pdf.Number(l.LLE)
	}

	// LLO (optional)
	if l.LLO != 0 {
		if err := pdf.CheckVersion(rm.Out, "line annotation LLO entry", pdf.V1_7); err != nil {
			return nil, err
		}
		if l.LLO < 0 {
			return nil, pdf.Error("negative LLO value")
		}
		dict["LLO"] = pdf.Number(l.LLO)
	}

	// Measure (optional)
	if l.Measure != nil {
		if err := pdf.CheckVersion(rm.Out, "line annotation Measure entry", pdf.V1_7); err != nil {
			return nil, err
		}
		embedded, _, err := pdf.ResourceManagerEmbed(rm, l.Measure)
		if err != nil {
			return nil, err
		}
		dict["Measure"] = embedded
	}

	return dict, nil
}

type LineEndingStyle pdf.Name

const (
	LineEndingStyleSquare       LineEndingStyle = "Square"
	LineEndingStyleCircle       LineEndingStyle = "Circle"
	LineEndingStyleDiamond      LineEndingStyle = "Diamond"
	LineEndingStyleOpenArrow    LineEndingStyle = "OpenArrow"
	LineEndingStyleClosedArrow  LineEndingStyle = "ClosedArrow"
	LineEndingStyleNone         LineEndingStyle = "None"
	LineEndingStyleButt         LineEndingStyle = "Butt"
	LineEndingStyleROpenArrow   LineEndingStyle = "ROpenArrow"
	LineEndingStyleRClosedArrow LineEndingStyle = "RClosedArrow"
	LineEndingStyleSlash        LineEndingStyle = "Slash"
)
