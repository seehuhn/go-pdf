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

package predict_test

import (
	"bytes"
	"io"
	"testing"

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/internal/filter/predict"
)

// FuzzReader exercises the predictor row decoder with arbitrary
// (params × input) combinations.  Reading from the resulting io.Reader
// may return data or an error, but it must not panic.
func FuzzReader(f *testing.F) {
	// representative seeds spanning the valid predictor / BPC matrix
	f.Add(byte(2), byte(16), byte(1), uint16(2), []byte{0x00, 0x00, 0x00})
	f.Add(byte(2), byte(8), byte(3), uint16(2), []byte{1, 2, 3, 4, 5})
	f.Add(byte(2), byte(1), byte(1), uint16(8), []byte{0xff, 0xaa})
	f.Add(byte(10), byte(8), byte(3), uint16(2), []byte{0, 1, 2, 3, 4, 5, 6})
	f.Add(byte(12), byte(8), byte(1), uint16(4), []byte{2, 10, 20, 30, 40})
	f.Add(byte(14), byte(16), byte(4), uint16(3), []byte{})

	predictors := []int{1, 2, 10, 11, 12, 13, 14, 15}
	bpcs := []int{1, 2, 4, 8, 16}

	f.Fuzz(func(t *testing.T, predictorByte, bpcByte, colors byte, columns uint16, data []byte) {
		p := &predict.Params{
			Predictor:        predictors[int(predictorByte)%len(predictors)],
			BitsPerComponent: bpcs[int(bpcByte)%len(bpcs)],
			Colors:           int(colors)%256 + 1,
			Columns:          int(columns)%1024 + 1,
		}
		if err := p.Validate(); err != nil {
			return
		}

		r, err := predict.NewReader(io.NopCloser(bytes.NewReader(data)), p, membudget.New(1<<30))
		if err != nil {
			return
		}
		// drain — must not panic
		_, _ = io.ReadAll(r)
		_ = r.Close()
	})
}
