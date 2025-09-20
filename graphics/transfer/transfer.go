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

package transfer

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
)

// Functions holds the transfer functions for the color components.  Each
// function must have one input and one output.
//
// The special value nil for a component represents the device-specific default
// transfer function.
type Functions struct {
	Red   pdf.Function
	Green pdf.Function
	Blue  pdf.Function
	Gray  pdf.Function
}

var (
	// Identity is the identity transfer function. This specific function
	// object corresponds to the name /Default in PDF graphics state parameter
	// dictionaries.
	Identity = &function.Type2{
		XMin: 0,
		XMax: 1,
		C0:   []float64{0},
		C1:   []float64{1},
		N:    1,
	}
)
