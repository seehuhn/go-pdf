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

// Line represents a line annotation that displays a single straight line on the page.
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

	// FillColor (optional; PDF 1.4) is the interior colour used to fill the
	// annotation's line endings.
	//
	// Only certain color types are allowed:
	//  - colors in the [color.DeviceGray] color space
	//  - colors in the [color.DeviceRGB] color space
	//  - colors in the [color.DeviceCMYK] color space
	//  - the [Transparent] color
	//
	// This corresponds to the /IC entry in the PDF annotation dictionary.
	FillColor color.Color

	// LL (required if LLE is present; PDF 1.6) is the length of leader lines
	// that extend from each endpoint perpendicular to the line itself.
	// Positive values appear clockwise, negative values counterclockwise.
	// Default value: 0 (no leader lines).
	LL float64

	// LLE (optional; PDF 1.6) is the length of leader line extensions that
	// extend from the line proper 180 degrees from the leader lines.
	// Default value: 0 (no leader line extensions).
	LLE float64

	// LLO (optional; PDF 1.7) is the length of the leader line offset,
	// which is the amount of empty space between the endpoints of the
	// annotation and the beginning of the leader lines.
	LLO float64

	// Caption (PDF 1.6) indicates whether the text specified by
	// Contents or RC entries are replicated as a caption in the line's
	// appearance.
	Caption bool

	// CP (optional; meaningful only if Cap is true; PDF 1.7) describes the
	// annotation's caption positioning. Valid values are "Inline" (centered
	// inside the line) and "Top" (on top of the line).
	// Default value: "Inline".
	CP pdf.Name

	// CO (optional; meaningful only if Cap is true; PDF 1.7) is an array of
	// two numbers that specify the offset of the caption text from its normal
	// position. [horizontal_offset, vertical_offset]
	// Default value: [0, 0] (no offset).
	CO []float64

	// Measure (optional; PDF 1.7) is a measure dictionary that specifies
	// the scale and units that apply to the line annotation.
	Measure measure.Measure
}

var _ Annotation = (*Line)(nil)

// AnnotationType returns "Line".
// This implements the [Annotation] interface.
func (l *Line) AnnotationType() pdf.Name {
	return "Line"
}

func extractLine(r pdf.Getter, dict pdf.Dict) (*Line, error) {
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

	// LL (optional)
	if ll, err := pdf.GetNumber(r, dict["LL"]); err == nil {
		line.LL = float64(ll)
	}

	// LLE (optional)
	if lle, err := pdf.GetNumber(r, dict["LLE"]); err == nil {
		line.LLE = float64(lle)
	}

	// Cap (optional)
	if cap, err := pdf.GetBoolean(r, dict["Cap"]); err == nil {
		line.Caption = bool(cap)
	}

	// LLO (optional)
	if llo, err := pdf.GetNumber(r, dict["LLO"]); err == nil {
		line.LLO = float64(llo)
	}

	// CP (optional)
	if cp, err := pdf.GetName(r, dict["CP"]); err == nil {
		line.CP = cp
	}

	// Measure (optional)
	if dict["Measure"] != nil {
		if m, err := pdf.Optional(measure.Extract(r, dict["Measure"])); err != nil {
			return nil, err
		} else {
			line.Measure = m
		}
	}

	// CO (optional)
	if co, err := pdf.GetArray(r, dict["CO"]); err == nil && len(co) == 2 {
		offsets := make([]float64, 2)
		for i, offset := range co {
			if num, err := pdf.GetNumber(r, offset); err == nil {
				offsets[i] = float64(num)
			}
		}
		line.CO = offsets
	}

	return line, nil
}

func (l *Line) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
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
		bs, _, err := l.BorderStyle.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	// LE (optional; PDF 1.4) - only write if not default [None, None]
	if l.LineEndingStyle != [2]LineEndingStyle{LineEndingStyleNone, LineEndingStyleNone} {
		if err := pdf.CheckVersion(rm.Out, "line annotation LE entry", pdf.V1_4); err != nil {
			return nil, err
		}
		leArray := make(pdf.Array, 2)
		leArray[0] = pdf.Name(l.LineEndingStyle[0])
		leArray[1] = pdf.Name(l.LineEndingStyle[1])
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

	// LL (optional)
	if l.LL != 0 {
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
		dict["LLE"] = pdf.Number(l.LLE)
	}

	// Cap (optional)
	if l.Caption {
		if err := pdf.CheckVersion(rm.Out, "line annotation Cap entry", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["Cap"] = pdf.Boolean(l.Caption)
	}

	// LLO (optional)
	if l.LLO != 0 {
		if err := pdf.CheckVersion(rm.Out, "line annotation LLO entry", pdf.V1_7); err != nil {
			return nil, err
		}
		dict["LLO"] = pdf.Number(l.LLO)
	}

	// CP (optional)
	if l.CP != "" {
		if err := pdf.CheckVersion(rm.Out, "line annotation CP entry", pdf.V1_7); err != nil {
			return nil, err
		}
		dict["CP"] = l.CP
	}

	// Measure (optional)
	if l.Measure != nil {
		if err := pdf.CheckVersion(rm.Out, "line annotation Measure entry", pdf.V1_7); err != nil {
			return nil, err
		}
		embedded, _, err := l.Measure.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["Measure"] = embedded
	}

	// CO (optional)
	if len(l.CO) == 2 {
		if err := pdf.CheckVersion(rm.Out, "line annotation CO entry", pdf.V1_7); err != nil {
			return nil, err
		}
		coArray := make(pdf.Array, 2)
		for i, offset := range l.CO {
			coArray[i] = pdf.Number(offset)
		}
		dict["CO"] = coArray
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
