// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pdf

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestFilterChaining(t *testing.T) {
	F1 := &FilterASCII85{}
	F2 := &FilterASCIIHex{}
	F3 := &FilterLZW{"Predictor": Integer(10)}
	F4 := &FilterCompress{}

	testData := "Hello, World!\n"

	testCases := [][]Filter{
		{F1, F2, F3},
		{F3, F2, F1},
		{F1, F3, F2},

		{F1, F2, F4},
		{F4, F2, F1},
		{F1, F4, F2},
	}
	for i, filters := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, V2_0, nil)
			if err != nil {
				t.Fatal(err)
			}

			ref := w.Alloc()

			out, err := w.OpenStream(ref, nil, filters...)
			if err != nil {
				t.Fatal(err)
			}
			_, err = io.WriteString(out, testData)
			if err != nil {
				t.Fatal(err)
			}
			err = out.Close()
			if err != nil {
				t.Fatal(err)
			}

			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}

			opt := &ReaderOptions{
				ErrorHandling: ErrorHandlingReport,
			}
			r, err := NewReader(bytes.NewReader(buf.Bytes()), opt)
			if err != nil {
				t.Fatal(err)
			}
			stmObj, err := GetStream(r, ref)
			if err != nil {
				t.Fatal(err)
			}
			in, err := DecodeStream(r, stmObj, 0)
			if err != nil {
				t.Fatal(err)
			}

			res, err := io.ReadAll(in)
			if err != nil {
				t.Fatal(err)
			}
			if string(res) != testData {
				t.Errorf("wrong result: %q vs %q", res, testData)
			}
		})
	}
}

// TODO(voss): remove
func newFlateFilter(parms Dict, isLZW bool) *flateFilter {
	res := &flateFilter{ // set defaults
		Predictor:        1,
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          1,
		EarlyChange:      true,
		IsLZW:            isLZW,
	}
	if parms == nil {
		return res
	}

	if val, ok := parms["Predictor"].(Integer); ok {
		res.Predictor = int(val)
	}
	if val, ok := parms["Colors"].(Integer); ok {
		res.Colors = int(val)
	}
	if val, ok := parms["BitsPerComponent"].(Integer); ok {
		res.BitsPerComponent = int(val)
	}
	if val, ok := parms["Columns"].(Integer); ok {
		res.Columns = int(val)
	}
	if val, ok := parms["EarlyChange"].(Integer); ok {
		res.EarlyChange = (val != 0)
	}
	return res
}

func TestFlate(t *testing.T) {
	parmsss := []Dict{
		nil,
		{},
		{"Predictor": Integer(1)},
		{"Predictor": Integer(12), "Columns": Integer(5)},
	}
	for _, isLZW := range []bool{false, true} {
		for _, parms := range parmsss {
			if isLZW {
				if parms == nil {
					parms = Dict{}
				}
				parms["EarlyChange"] = Integer(0)
			}
			ff := newFlateFilter(parms, isLZW)
			for _, in := range []string{"", "12345", "1234567890"} {
				buf := &bytes.Buffer{}
				w, err := ff.Encode(withDummyClose{buf})
				if err != nil {
					t.Error(in, err)
					continue
				}
				_, err = w.Write([]byte(in))
				if err != nil {
					t.Error(in, err)
					continue
				}
				err = w.Close()
				if err != nil {
					t.Error(in, err)
					continue
				}

				r, err := ff.Decode(buf)
				if err != nil {
					t.Error(in, err)
					continue
				}
				out, err := io.ReadAll(r)
				if err != nil {
					t.Error(in, err)
					continue
				}

				if in != string(out) {
					t.Errorf("wrong results: %q vs %q", in, string(out))
				}
			}
		}
	}
}

func TestPngUp(t *testing.T) {
	columns := 2

	ff := &flateFilter{
		Predictor:        12,
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          columns,
	}
	for _, in := range []string{"", "11121314151617", "123456"} {
		buf := &bytes.Buffer{}
		w := ff.newPngWriter(buf, nil)
		n, err := w.Write([]byte(in))
		if err != nil {
			t.Error("unexpected error:", err)
			continue
		}
		if n != len(in) {
			t.Errorf("wrong n: %d vs %d", n, len(in))
		}

		r := ff.newPngReader(buf)
		res, err := io.ReadAll(r)
		if err != nil {
			t.Error("unexpected error:", err)
			continue
		}

		if string(res) != in {
			t.Errorf("wrong result: %q vs %q", res, in)
		}
	}
}

func TestPngFilters(t *testing.T) {
	cDat := []byte("sonderbar und anderswohl\000")
	pDat := []byte("\377merkwuerdig und kiloklar")
	out := make([]byte, len(cDat))
	for ft := 0; ft < nFilter; ft++ {
		for bpp := 1; bpp <= 4; bpp++ {
			pngEnc[ft](out, cDat, pDat, bpp)
			pngDec[ft](out, pDat, bpp)
			if !bytes.Equal(out, cDat) {
				t.Errorf("%q != %q for ft=%d, bpp=%d",
					string(out), string(cDat), ft, bpp)
			}
		}
	}
}
