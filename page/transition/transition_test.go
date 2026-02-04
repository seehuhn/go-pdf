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

package transition

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var testCases = []struct {
	name    string
	version pdf.Version
	trans   *Transition
}{
	{
		name:    "empty",
		version: pdf.V1_4,
		trans:   &Transition{},
	},
	{
		name:    "split_vertical_outward",
		version: pdf.V1_4,
		trans: &Transition{
			Style:     StyleSplit,
			Duration:  2.5,
			Dimension: DimensionVertical,
			Motion:    MotionOutward,
		},
	},
	{
		name:    "blinds_vertical",
		version: pdf.V1_4,
		trans: &Transition{
			Style:     StyleBlinds,
			Dimension: DimensionVertical,
		},
	},
	{
		name:    "box_outward",
		version: pdf.V1_4,
		trans: &Transition{
			Style:    StyleBox,
			Duration: 1.5,
			Motion:   MotionOutward,
		},
	},
	{
		name:    "wipe_bottom_to_top",
		version: pdf.V1_4,
		trans: &Transition{
			Style:     StyleWipe,
			Direction: 90,
		},
	},
	{
		name:    "wipe_right_to_left",
		version: pdf.V1_4,
		trans: &Transition{
			Style:     StyleWipe,
			Direction: 180,
		},
	},
	{
		name:    "dissolve",
		version: pdf.V1_4,
		trans: &Transition{
			Style:    StyleDissolve,
			Duration: 3.0,
		},
	},
	{
		name:    "glitter_diagonal",
		version: pdf.V1_4,
		trans: &Transition{
			Style:     StyleGlitter,
			Direction: 315,
		},
	},
	{
		name:    "fly_with_scale",
		version: pdf.V1_5,
		trans: &Transition{
			Style:     StyleFly,
			Motion:    MotionOutward,
			Direction: DirNone,
			Scale:     0.5,
		},
	},
	{
		name:    "fly_opaque",
		version: pdf.V1_5,
		trans: &Transition{
			Style:  StyleFly,
			Opaque: true,
		},
	},
	{
		name:    "push",
		version: pdf.V1_5,
		trans: &Transition{
			Style:     StylePush,
			Direction: 270,
		},
	},
	{
		name:    "cover",
		version: pdf.V1_5,
		trans: &Transition{
			Style: StyleCover,
		},
	},
	{
		name:    "uncover",
		version: pdf.V1_5,
		trans: &Transition{
			Style: StyleUncover,
		},
	},
	{
		name:    "fade",
		version: pdf.V1_5,
		trans: &Transition{
			Style:    StyleFade,
			Duration: 2.0,
		},
	},
	{
		name:    "single_use",
		version: pdf.V1_4,
		trans: &Transition{
			Style:     StyleWipe,
			SingleUse: true,
		},
	},
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc.version, tc.trans)
		})
	}
}

func roundTripTest(t *testing.T, version pdf.Version, original *Transition) {
	t.Helper()

	buf, _ := memfile.NewPDFWriter(version, nil)
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
	extracted, err := pdf.ExtractorGet(x, embedded, Extract)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(original, extracted); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)

		embedded, err := rm.Embed(tc.trans)
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
		trans, err := pdf.ExtractorGet(x, obj, Extract)
		if err != nil {
			t.Skip("malformed transition")
		}

		roundTripTest(t, pdf.GetVersion(r), trans)
	})
}
