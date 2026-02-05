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

package annotation

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var borderEffectTestCases = []struct {
	name string
	data *BorderEffect
}{
	{
		name: "solid",
		data: &BorderEffect{
			Style: "S",
		},
	},
	{
		name: "cloudy",
		data: &BorderEffect{
			Style: "C",
		},
	},
	{
		name: "cloudyIntensity1",
		data: &BorderEffect{
			Style:     "C",
			Intensity: 1,
		},
	},
	{
		name: "cloudyIntensity2",
		data: &BorderEffect{
			Style:     "C",
			Intensity: 2,
		},
	},
	{
		name: "singleUse",
		data: &BorderEffect{
			Style:     "C",
			Intensity: 2,
			SingleUse: true,
		},
	},
}

func borderEffectRoundTrip(t *testing.T, version pdf.Version, data *BorderEffect) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	embedded, err := rm.Embed(data)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := pdf.ExtractorGet(x, embedded, ExtractBorderEffect)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(data, decoded); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestBorderEffectRoundTrip(t *testing.T) {
	versions := []pdf.Version{pdf.V1_7, pdf.V2_0}
	for _, v := range versions {
		for _, tc := range borderEffectTestCases {
			t.Run(tc.name+"-"+v.String(), func(t *testing.T) {
				borderEffectRoundTrip(t, v, tc.data)
			})
		}
	}
}

func FuzzBorderEffectRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	versions := []pdf.Version{pdf.V1_7, pdf.V2_0}
	for _, v := range versions {
		for _, tc := range borderEffectTestCases {
			w, buf := memfile.NewPDFWriter(v, opt)

			err := memfile.AddBlankPage(w)
			if err != nil {
				continue
			}

			rm := pdf.NewResourceManager(w)

			embedded, err := rm.Embed(tc.data)
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
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		data, err := pdf.ExtractorGet(x, obj, ExtractBorderEffect)
		if err != nil {
			t.Skip("malformed object")
		}

		borderEffectRoundTrip(t, pdf.GetVersion(r), data)
	})
}
