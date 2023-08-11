// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package cmap

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRoundTrip(t *testing.T) {
	for _, name := range AllPredefined {
		t.Run(name, func(t *testing.T) {
			r, err := OpenPredefined(name)
			if err != nil {
				t.Fatal(err)
			}

			info, err := Read(r, nil)
			if err != nil {
				t.Fatal(err)
			}

			buf := &bytes.Buffer{}
			err = info.Write(buf)
			if err != nil {
				t.Fatal(err)
			}

			info2, err := Read(buf, nil)
			if err != nil {
				t.Fatal(err)
			}

			if d := cmp.Diff(info, info2); d != "" {
				t.Error(d)
			}
		})
	}
}

func FuzzCMap(f *testing.F) {
	for _, name := range AllPredefined {
		raw, err := predefined.Open("predefined/" + name + ".gz")
		if err != nil {
			f.Fatal(err)
		}
		r, err := gzip.NewReader(raw)
		if err != nil {
			f.Fatal(err)
		}
		body, err := io.ReadAll(r)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(body))
		err = r.Close()
		if err != nil {
			f.Fatal(err)
		}
		err = raw.Close()
		if err != nil {
			f.Fatal(err)
		}
	}
	f.Fuzz(func(t *testing.T, a string) {
		info, err := Read(bytes.NewReader([]byte(a)), nil)
		if err != nil {
			return
		}

		buf := &bytes.Buffer{}
		err = info.Write(buf)
		if err != nil {
			t.Fatal(err)
		}

		info2, err := Read(buf, nil)
		if err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(info, info2); d != "" {
			t.Error(d)
		}
	})
}
