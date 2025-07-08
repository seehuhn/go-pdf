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

// Line represents a line annotation that displays a single straight line on the page.
type Line struct {
	Common
	Markup

	// L (required) is an array of four numbers [x1 y1 x2 y2] specifying the
	// starting and ending coordinates of the line in default user space.
	L []float64

	// BS (optional) is a border style dictionary specifying the width and
	// dash pattern that is used in drawing the line.
	BS pdf.Reference

	// LE (optional; PDF 1.4) is an array of two names specifying the line
	// ending styles for the start and end points respectively.
	// Default value: [/None /None].
	LE []pdf.Name

	// IC (optional; PDF 1.4) is an array of numbers in the range 0.0 to 1.0
	// specifying the interior colour used to fill the annotation's line endings.
	IC []float64

	// LL (required if LLE is present; PDF 1.6) is the length of leader lines
	// that extend from each endpoint perpendicular to the line itself.
	// Positive values appear clockwise, negative values counterclockwise.
	// Default value: 0 (no leader lines).
	LL float64

	// LLE (optional; PDF 1.6) is the length of leader line extensions that
	// extend from the line proper 180 degrees from the leader lines.
	// Default value: 0 (no leader line extensions).
	LLE float64

	// Cap (optional; PDF 1.6) indicates whether the text specified by Contents
	// or RC entries are replicated as a caption in the line's appearance.
	// Default value: false.
	Cap bool

	// LLO (optional; PDF 1.7) is the length of the leader line offset,
	// which is the amount of empty space between the endpoints of the
	// annotation and the beginning of the leader lines.
	LLO float64

	// CP (optional; meaningful only if Cap is true; PDF 1.7) describes the
	// annotation's caption positioning. Valid values are "Inline" (centered
	// inside the line) and "Top" (on top of the line).
	// Default value: "Inline".
	CP pdf.Name

	// Measure (optional; PDF 1.7) is a measure dictionary that specifies
	// the scale and units that apply to the line annotation.
	Measure pdf.Reference

	// CO (optional; meaningful only if Cap is true; PDF 1.7) is an array of
	// two numbers that specify the offset of the caption text from its normal
	// position. [horizontal_offset, vertical_offset]
	// Default value: [0, 0] (no offset).
	CO []float64
}

var _ pdf.Annotation = (*Line)(nil)

// AnnotationType returns "Line".
// This implements the [pdf.Annotation] interface.
func (l *Line) AnnotationType() pdf.Name {
	return "Line"
}

func extractLine(r pdf.Getter, dict pdf.Dict) (*Line, error) {
	line := &Line{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &line.Common); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &line.Markup); err != nil {
		return nil, err
	}

	// Extract line-specific fields
	// L (required)
	if l, err := pdf.GetArray(r, dict["L"]); err == nil && len(l) == 4 {
		coords := make([]float64, 4)
		for i, coord := range l {
			if num, err := pdf.GetNumber(r, coord); err == nil {
				coords[i] = float64(num)
			}
		}
		line.L = coords
	}

	// BS (optional)
	if bs, ok := dict["BS"].(pdf.Reference); ok {
		line.BS = bs
	}

	// LE (optional)
	if le, err := pdf.GetArray(r, dict["LE"]); err == nil && len(le) > 0 {
		endings := make([]pdf.Name, len(le))
		for i, ending := range le {
			if name, err := pdf.GetName(r, ending); err == nil {
				endings[i] = name
			}
		}
		line.LE = endings
	}

	// IC (optional)
	if ic, err := pdf.GetArray(r, dict["IC"]); err == nil && len(ic) > 0 {
		colors := make([]float64, len(ic))
		for i, color := range ic {
			if num, err := pdf.GetNumber(r, color); err == nil {
				colors[i] = float64(num)
			}
		}
		line.IC = colors
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
		line.Cap = bool(cap)
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
	if measure, ok := dict["Measure"].(pdf.Reference); ok {
		line.Measure = measure
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

func (l *Line) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Line"),
	}

	// Add common annotation fields
	if err := l.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add markup annotation fields
	if err := l.Markup.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add line-specific fields
	// L (required)
	if len(l.L) == 4 {
		lArray := make(pdf.Array, 4)
		for i, coord := range l.L {
			lArray[i] = pdf.Number(coord)
		}
		dict["L"] = lArray
	}

	// BS (optional)
	if l.BS != 0 {
		dict["BS"] = l.BS
	}

	// LE (optional)
	if l.LE != nil {
		if err := pdf.CheckVersion(rm.Out, "line annotation LE entry", pdf.V1_4); err != nil {
			return nil, zero, err
		}
		leArray := make(pdf.Array, len(l.LE))
		for i, ending := range l.LE {
			leArray[i] = ending
		}
		dict["LE"] = leArray
	}

	// IC (optional)
	if l.IC != nil {
		if err := pdf.CheckVersion(rm.Out, "line annotation IC entry", pdf.V1_4); err != nil {
			return nil, zero, err
		}
		icArray := make(pdf.Array, len(l.IC))
		for i, color := range l.IC {
			icArray[i] = pdf.Number(color)
		}
		dict["IC"] = icArray
	}

	// LL (optional)
	if l.LL != 0 {
		if err := pdf.CheckVersion(rm.Out, "line annotation LL entry", pdf.V1_6); err != nil {
			return nil, zero, err
		}
		dict["LL"] = pdf.Number(l.LL)
	}

	// LLE (optional)
	if l.LLE != 0 {
		if err := pdf.CheckVersion(rm.Out, "line annotation LLE entry", pdf.V1_6); err != nil {
			return nil, zero, err
		}
		dict["LLE"] = pdf.Number(l.LLE)
	}

	// Cap (optional)
	if l.Cap {
		if err := pdf.CheckVersion(rm.Out, "line annotation Cap entry", pdf.V1_6); err != nil {
			return nil, zero, err
		}
		dict["Cap"] = pdf.Boolean(l.Cap)
	}

	// LLO (optional)
	if l.LLO != 0 {
		if err := pdf.CheckVersion(rm.Out, "line annotation LLO entry", pdf.V1_7); err != nil {
			return nil, zero, err
		}
		dict["LLO"] = pdf.Number(l.LLO)
	}

	// CP (optional)
	if l.CP != "" {
		if err := pdf.CheckVersion(rm.Out, "line annotation CP entry", pdf.V1_7); err != nil {
			return nil, zero, err
		}
		dict["CP"] = l.CP
	}

	// Measure (optional)
	if l.Measure != 0 {
		if err := pdf.CheckVersion(rm.Out, "line annotation Measure entry", pdf.V1_7); err != nil {
			return nil, zero, err
		}
		dict["Measure"] = l.Measure
	}

	// CO (optional)
	if len(l.CO) == 2 {
		if err := pdf.CheckVersion(rm.Out, "line annotation CO entry", pdf.V1_7); err != nil {
			return nil, zero, err
		}
		coArray := make(pdf.Array, 2)
		for i, offset := range l.CO {
			coArray[i] = pdf.Number(offset)
		}
		dict["CO"] = coArray
	}

	return dict, zero, nil
}
