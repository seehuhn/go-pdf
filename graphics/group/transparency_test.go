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
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
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

	buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
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
	readAttrs, err := pdf.CursorAt(x, nil).Decode(embedded, ExtractTransparencyAttributes)
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
		w, buf := memfile.NewPDFWriter(f, pdf.V2_0, opt)

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
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing object")
		}

		x := pdf.NewExtractor(r)
		attrs, err := pdf.CursorAt(x, nil).Decode(obj, ExtractTransparencyAttributes)
		if err != nil {
			t.Skip("malformed group attributes")
		}

		roundTripTest(t, attrs)
	})
}

// TestExtractDanglingReference checks that a reference to a free object
// reads the same as an absent entry.
func TestExtractDanglingReference(t *testing.T) {
	bases := map[string]struct {
		dict pdf.Dict
		keys []pdf.Name
	}{
		"plain":    {pdf.Dict{"S": pdf.Name("Transparency")}, []pdf.Name{"CS", "I", "K"}},
		"isolated": {pdf.Dict{"S": pdf.Name("Transparency"), "I": pdf.Boolean(true)}, []pdf.Name{"CS", "K"}},
	}
	dangling := pdf.NewReference(99, 0)
	x := pdf.NewExtractor(mock.Getter)
	for baseName, base := range bases {
		for _, key := range base.keys {
			t.Run(baseName+"/"+string(key), func(t *testing.T) {
				dict := maps.Clone(base.dict)
				dict[key] = dangling

				want, err := ExtractTransparencyAttributes(pdf.CursorAt(x, nil), base.dict, true)
				if err != nil {
					t.Fatal(err)
				}
				got, err := ExtractTransparencyAttributes(pdf.CursorAt(x, nil), dict, true)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("dangling reference differs from absent entry (-want +got):\n%s", diff)
				}
			})
		}
	}
}

// TestExtractMalformedFlag checks that a flag of the wrong type reads the
// same as an absent entry.
func TestExtractMalformedFlag(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)
	base := pdf.Dict{"S": pdf.Name("Transparency")}
	want, err := ExtractTransparencyAttributes(pdf.CursorAt(x, nil), base, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []pdf.Name{"I", "K"} {
		dict := maps.Clone(base)
		dict[key] = pdf.String("bad")
		got, err := ExtractTransparencyAttributes(pdf.CursorAt(x, nil), dict, true)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s: malformed value differs from absent entry (-want +got):\n%s", key, diff)
		}
	}
}

// TestExtractMalformedCS checks that a CS entry of the wrong type reads the
// same as an absent entry.
func TestExtractMalformedCS(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)
	base := pdf.Dict{"S": pdf.Name("Transparency")}
	want, err := ExtractTransparencyAttributes(pdf.CursorAt(x, nil), base, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []pdf.Object{pdf.Integer(1), pdf.String("bad"), pdf.Dict{}, pdf.Name("NoSuchSpace")} {
		dict := maps.Clone(base)
		dict["CS"] = bad
		got, err := ExtractTransparencyAttributes(pdf.CursorAt(x, nil), dict, true)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("malformed CS %v differs from absent entry (-want +got):\n%s", bad, diff)
		}
	}
}
