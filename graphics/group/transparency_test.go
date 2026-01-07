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

package group

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var testCases = []struct {
	name  string
	attrs *TransparencyAttributes
}{
	{
		name:  "empty",
		attrs: &TransparencyAttributes{},
	},
	{
		name: "isolated",
		attrs: &TransparencyAttributes{
			Isolated: true,
		},
	},
	{
		name: "knockout",
		attrs: &TransparencyAttributes{
			Knockout: true,
		},
	},
	{
		name: "isolated and knockout",
		attrs: &TransparencyAttributes{
			Isolated: true,
			Knockout: true,
		},
	},
	{
		name: "DeviceRGB color space",
		attrs: &TransparencyAttributes{
			CS: color.SpaceDeviceRGB,
		},
	},
	{
		name: "DeviceGray color space isolated",
		attrs: &TransparencyAttributes{
			CS:       color.SpaceDeviceGray,
			Isolated: true,
		},
	},
	{
		name: "DeviceCMYK all flags",
		attrs: &TransparencyAttributes{
			CS:       color.SpaceDeviceCMYK,
			Isolated: true,
			Knockout: true,
		},
	},
	{
		name: "single use",
		attrs: &TransparencyAttributes{
			Isolated:  true,
			Knockout:  true,
			SingleUse: true,
		},
	},
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc.attrs)
		})
	}
}

func roundTripTest(t *testing.T, original *TransparencyAttributes) {
	t.Helper()

	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	embedded, err := rm.Embed(original)
	if err != nil {
		t.Fatal(err)
	}

	err = buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(buf)
	readAttrs, err := ExtractTransparencyAttributes(x, embedded)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(original, readAttrs); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)

		embedded, err := rm.Embed(tc.attrs)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = embedded

		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing object")
		}

		x := pdf.NewExtractor(r)
		attrs, err := ExtractTransparencyAttributes(x, obj)
		if err != nil {
			t.Skip("malformed group attributes")
		}

		roundTripTest(t, attrs)
	})
}
