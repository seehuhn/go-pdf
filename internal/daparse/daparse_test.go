// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package daparse

import (
	"testing"

	"seehuhn.de/go/pdf/graphics/color"
)

// TestParse checks that a default-appearance string is tokenised as a
// content-stream fragment: delimiters end tokens, NUL is white space,
// comments are ignored, and only a numeric token gives an operand.
func TestParse(t *testing.T) {
	cases := []struct {
		name string
		da   string
		size float64
		col  color.Color
	}{
		{"plain", "/Helv 12 Tf 0 0 1 rg", 12, color.DeviceRGB{0, 0, 1}},
		{"name after operator", "0 0 1 rg/Helv 12 Tf", 12, color.DeviceRGB{0, 0, 1}},
		{"NUL separator", "/Helv 12 Tf\x000 0 1 rg", 12, color.DeviceRGB{0, 0, 1}},
		{"comment", "/Helv 12 Tf 0 g %1 0 0 rg\n", 12, color.DeviceGray(0)},
		{"comment between operands", "/Helv 12 Tf 0 0%x\n1 rg", 12, color.DeviceRGB{0, 0, 1}},
		{"vertical tab is regular", "/Helv 12 Tf 0\v1 g", 12, def},
		{"exponent format", "/Helv 1e1 Tf 1e0 g", 10, color.DeviceGray(1)},
		{"NaN is no number", "/Helv 12 Tf NaN g", 12, def},
		{"gray", "/Helv 9.5 Tf 0.5 g", 9.5, color.DeviceGray(0.5)},
		{"cmyk", "/Helv 12 Tf 0 1 1 0 k", 12, color.DeviceCMYK{0, 1, 1, 0}},
		{"last colour wins", "1 g /Helv 12 Tf 0 0 1 rg", 12, color.DeviceRGB{0, 0, 1}},
		{"out of range clipped", "/Helv 12 Tf 2 -1 0.5 rg", 12, color.DeviceRGB{1, 0, 0.5}},
		{"too few operands", "/Helv 12 Tf 0 1 rg", 12, def},
		{"font operand not a name", "(Helv) 12 Tf", 0, def},
		{"negative size", "/Helv -12 Tf", 0, def},
		{"empty", "", 0, def},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			size, col := Parse(c.da, def)
			if size != c.size {
				t.Errorf("Parse(%q) size = %v, want %v", c.da, size, c.size)
			}
			if col != c.col {
				t.Errorf("Parse(%q) colour = %v, want %v", c.da, col, c.col)
			}
		})
	}
}

var def = color.DeviceRGB{0.102, 0.094, 0.078}
