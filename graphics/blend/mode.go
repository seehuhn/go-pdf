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

package blend

import (
	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 8.4.5, 11.3.5, 11.6.3

// Mode represents a PDF blend mode.
// Internally stored as a slice to handle the deprecated array form.
// When writing: len==1 emits name, len>1 emits array.
// When reading: name becomes len==1 slice, array becomes full slice.
type Mode []pdf.Name

// All 16 standard blend mode names (section 11.3.5)
const (
	ModeNormal     pdf.Name = "Normal"
	ModeCompatible pdf.Name = "Compatible" // deprecated in PDF 2.0
	ModeMultiply   pdf.Name = "Multiply"
	ModeScreen     pdf.Name = "Screen"
	ModeOverlay    pdf.Name = "Overlay"
	ModeDarken     pdf.Name = "Darken"
	ModeLighten    pdf.Name = "Lighten"
	ModeColorDodge pdf.Name = "ColorDodge"
	ModeColorBurn  pdf.Name = "ColorBurn"
	ModeHardLight  pdf.Name = "HardLight"
	ModeSoftLight  pdf.Name = "SoftLight"
	ModeDifference pdf.Name = "Difference"
	ModeExclusion  pdf.Name = "Exclusion"
	ModeHue        pdf.Name = "Hue"
	ModeSaturation pdf.Name = "Saturation"
	ModeColor      pdf.Name = "Color"
	ModeLuminosity pdf.Name = "Luminosity"
)

// AsPDF returns the PDF representation: name for single mode, array for multiple.
func (m Mode) AsPDF() pdf.Object {
	switch len(m) {
	case 0:
		return nil
	case 1:
		return m[0]
	default:
		arr := make(pdf.Array, len(m))
		for i, n := range m {
			arr[i] = n
		}
		return arr
	}
}

// IsZero returns true if the Mode is empty (unset).
func (m Mode) IsZero() bool {
	return len(m) == 0
}

// Equal reports whether two Modes are equal.
func (m Mode) Equal(other Mode) bool {
	if len(m) != len(other) {
		return false
	}
	for i, n := range m {
		if n != other[i] {
			return false
		}
	}
	return true
}

// Extract extracts a blend mode from a PDF object.
// Handles both name and array forms (array deprecated in PDF 2.0).
func Extract(r pdf.Getter, obj pdf.Object) (Mode, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}

	switch v := obj.(type) {
	case pdf.Name:
		return Mode{v}, nil
	case pdf.Array:
		result := make(Mode, 0, len(v))
		for _, elem := range v {
			name, err := pdf.GetName(r, elem)
			if err != nil {
				continue // skip malformed entries
			}
			result = append(result, name)
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil
	default:
		return nil, pdf.Errorf("invalid blend mode type: %T", obj)
	}
}
