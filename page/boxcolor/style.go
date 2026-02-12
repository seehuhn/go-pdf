// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package boxcolor

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 14.11.2

// Style specifies the visual characteristics for displaying a page boundary
// guideline.
type Style struct {
	// Color specifies the line color to use.
	//
	// This corresponds to the /C entry in the PDF box style dictionary.
	Color color.DeviceRGB

	// LineWidth specifies the line width to use, in default user space units.
	//
	// On write, the value 0 can be used as a shorthand for 1.
	//
	// This corresponds to the /W entry in the PDF box style dictionary.
	LineWidth float64

	// Style specifies the line style to use.
	//
	// On write, an empty value can be used as a shorthand for [StyleSolid].
	//
	// This corresponds to the /S entry in the PDF box style dictionary.
	Style LineStyle

	// DashPattern specifies the dash pattern for dashed lines,
	// in default user space units.
	// Must be nil for solid line styles.
	//
	// On write, a nil value is equivalent to []float64{3}.
	//
	// This corresponds to the /D entry in the PDF box style dictionary.
	DashPattern []float64

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractStyle extracts a box style dictionary from a PDF object.
func ExtractStyle(x *pdf.Extractor, obj pdf.Object) (*Style, error) {
	singleUse := !x.IsIndirect

	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	style := &Style{}
	style.SingleUse = singleUse

	// color (clamp to valid range [0.0, 1.0])
	if cArray, err := pdf.Optional(x.GetArray(dict["C"])); err != nil {
		return nil, err
	} else if len(cArray) >= 3 {
		r, _ := x.GetNumber(cArray[0])
		g, _ := x.GetNumber(cArray[1])
		b, _ := x.GetNumber(cArray[2])
		style.Color = color.DeviceRGB{
			clamp(float64(r), 0, 1),
			clamp(float64(g), 0, 1),
			clamp(float64(b), 0, 1),
		}
	}

	// line width
	if w, err := pdf.Optional(x.GetNumber(dict["W"])); err != nil {
		return nil, err
	} else if w != 0 {
		style.LineWidth = float64(w)
	} else {
		style.LineWidth = 1 // PDF default
	}

	// line style
	if s, err := pdf.Optional(x.GetName(dict["S"])); err != nil {
		return nil, err
	} else if s != "" {
		style.Style = LineStyle(s)
	} else {
		style.Style = StyleSolid // PDF default
	}

	// dash pattern (only for non-solid styles)
	if dArray, err := pdf.Optional(x.GetArray(dict["D"])); err != nil {
		return nil, err
	} else if style.Style != StyleSolid {
		if len(dArray) > 0 {
			style.DashPattern = make([]float64, len(dArray))
			for i, v := range dArray {
				n, _ := x.GetNumber(v)
				style.DashPattern[i] = max(0, float64(n))
			}
		} else {
			style.DashPattern = []float64{3}
		}
	}

	return style, nil
}

// Embed converts the box style dictionary to a PDF dictionary.
func (s *Style) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "box style dictionary", pdf.V1_4); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}

	// color (must be in range [0.0, 1.0])
	for i, c := range s.Color {
		if c < 0 || c > 1 {
			return nil, fmt.Errorf("color component %d out of range [0, 1]: %g", i, c)
		}
	}
	if s.Color != (color.DeviceRGB{0, 0, 0}) {
		dict["C"] = pdf.Array{
			pdf.Number(s.Color[0]),
			pdf.Number(s.Color[1]),
			pdf.Number(s.Color[2]),
		}
	}

	// line width
	if s.LineWidth < 0 {
		return nil, fmt.Errorf("line width must be non-negative: %g", s.LineWidth)
	}
	if s.LineWidth != 0 && s.LineWidth != 1 {
		dict["W"] = pdf.Number(s.LineWidth)
	}

	// line style
	if s.Style != "" && s.Style != StyleSolid {
		dict["S"] = pdf.Name(s.Style)
	}

	// dash pattern (must be nil for solid styles)
	if (s.Style == "" || s.Style == StyleSolid) && s.DashPattern != nil {
		return nil, fmt.Errorf("dash pattern must be nil for solid line style")
	}
	dashPattern := s.DashPattern
	for i, v := range dashPattern {
		if v < 0 {
			return nil, fmt.Errorf("dash pattern element %d must be non-negative: %g", i, v)
		}
	}
	if len(dashPattern) > 0 && (len(dashPattern) != 1 || dashPattern[0] != defaultDashPattern[0]) {
		dashArray := make(pdf.Array, len(dashPattern))
		for i, v := range dashPattern {
			dashArray[i] = pdf.Number(v)
		}
		dict["D"] = dashArray
	}

	if s.SingleUse {
		return dict, nil
	}

	ref := e.Alloc()
	err := e.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// defaultDashPattern is the PDF default dash pattern for box style guidelines.
var defaultDashPattern = []float64{3}

type LineStyle pdf.Name

// LineStyle values for box color styles.
// These correspond to the styles defined in PDF specification.
const (
	// StyleSolid represents a solid line style.
	StyleSolid LineStyle = "S"

	// StyleDashed represents a dashed line style.
	StyleDashed LineStyle = "D"
)

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
