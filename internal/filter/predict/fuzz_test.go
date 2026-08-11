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

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/internal/filter/predict"
)

// nopCloser turns an [io.Writer] into an [io.WriteCloser].
type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

// decode undoes the prediction filter.
func decode(p *predict.Params, encoded []byte) ([]byte, error) {
	r, err := predict.NewReader(io.NopCloser(bytes.NewReader(encoded)), p, membudget.New(1<<30))
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(r)
	if closeErr := r.Close(); err == nil {
		err = closeErr
	}
	return data, err
}

// encode applies the prediction filter.
func encode(p *predict.Params, data []byte) ([]byte, error) {
	buf := &bytes.Buffer{}
	w, err := predict.NewWriter(nopCloser{buf}, p)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// roundTripTest performs the read-write-read cycle: the sample data recovered
// from encoded is written out again and read back, and the two reads must
// agree.  Reading is permissive and may repair a malformed stream, but
// whatever it hands back has to survive being written and read once more.
func roundTripTest(t *testing.T, p *predict.Params, encoded []byte) {
	t.Helper()

	first, err := decode(p, encoded)
	if err != nil {
		// a stream which cannot be read has nothing to round-trip
		return
	}

	reencoded, err := encode(p, first)
	if err != nil {
		t.Fatalf("re-encode failed: %v", err)
	}

	second, err := decode(p, reencoded)
	if err != nil {
		t.Fatalf("second read failed: %v", err)
	}

	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// FuzzRoundTrip exercises the predictor with arbitrary (params × input)
// combinations.  Decoding must not panic, and anything it does decode must
// survive a write-read cycle unchanged.
//
// The rows of a stream do not always end on a byte boundary, and the spare
// bits which follow the last component of a row are carried through both
// directions untouched.  Sweeping the geometry is what checks that: the
// seeds below place those bits differently in each case.
func FuzzRoundTrip(f *testing.F) {
	// representative seeds spanning the valid predictor / BPC matrix
	f.Add(byte(2), byte(16), byte(1), uint16(2), []byte{0x00, 0x00, 0x00})
	f.Add(byte(2), byte(8), byte(3), uint16(2), []byte{1, 2, 3, 4, 5})
	f.Add(byte(2), byte(1), byte(1), uint16(8), []byte{0xff, 0xaa})
	f.Add(byte(10), byte(8), byte(3), uint16(2), []byte{0, 1, 2, 3, 4, 5, 6})
	f.Add(byte(12), byte(8), byte(1), uint16(4), []byte{2, 10, 20, 30, 40})
	f.Add(byte(14), byte(16), byte(4), uint16(3), []byte{})

	// geometries whose rows leave spare bits in the last byte
	f.Add(byte(2), byte(2), byte(3), uint16(5), []byte{0x1b, 0xe4, 0x7f, 0x00})
	f.Add(byte(15), byte(4), byte(1), uint16(5), []byte{0, 0x12, 0x34, 0x5f})
	f.Add(byte(11), byte(1), byte(5), uint16(7), []byte{1, 0x5a, 0xc3, 0xff, 0x81})

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

		roundTripTest(t, p, data)
	})
}
