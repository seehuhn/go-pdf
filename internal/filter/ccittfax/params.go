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

package ccittfax

//go:generate go run ./generate/

// Params holds the parameters that control CCITT Fax encoding and decoding behavior.
type Params struct {
	// Columns specifies image width in pixels.
	// The default value of 0 is interpreted as 1728 pixels.
	Columns int

	// K determines the algorithm variant:
	//   K < 0: Group 4 (pure 2D encoding)
	//   K = 0: Group 3 one-dimensional
	//   K > 0: Group 3 two-dimensional (K-1 lines use 2D, then 1 line uses 1D)
	K int

	// MaxRows specifies the maximum number of rows to encode/decode
	// (0 = use all rows).
	MaxRows int

	// EndOfLine indicates whether EOL codes are present in the stream
	EndOfLine bool

	// EncodedByteAlign indicates whether each scan line is padded to byte boundary
	EncodedByteAlign bool

	// BlackIs1 controls the interpretation of bit values.
	// If this is true, bit values are flipped before encoding and after decoding.
	BlackIs1 bool

	// IgnoreEndOfBlock indicates whether to ignore EOFB/RTC termination patterns
	// false: respect end-of-block patterns (PDF default)
	// true:  ignore termination patterns, decode entire stream
	IgnoreEndOfBlock bool

	// DamagedRowsBeforeError is the number of damaged rows of data that shall
	// be tolerated before an error occurs.
	DamagedRowsBeforeError int
}
