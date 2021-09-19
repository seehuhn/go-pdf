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

func TestFlate(t *testing.T) {
	parmsss := []Dict{
		nil,
		{"Predictor": Integer(1)},
		{"Predictor": Integer(12), "Columns": Integer(5)},
	}
	for _, parms := range parmsss {
		ff := ffFromDict(parms)
		for _, in := range []string{"", "12345", "1234567890"} {
			buf := &bytes.Buffer{}
			w, err := ff.Encode(withoutClose{buf})
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

			fmt.Printf("%d %q\n", buf.Len(), buf.String())

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
