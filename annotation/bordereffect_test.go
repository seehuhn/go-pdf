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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestBorderEffectRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		effect *BorderEffect
	}{
		{
			name: "solid",
			effect: &BorderEffect{
				Style:     "S",
				Intensity: 0,
			},
		},
		{
			name: "cloudy",
			effect: &BorderEffect{
				Style: "C",
			},
		},
		{
			name: "cloudy1",
			effect: &BorderEffect{
				Style:     "C",
				Intensity: 1,
			},
		},
		{
			name: "cloudy2",
			effect: &BorderEffect{
				Style:     "C",
				Intensity: 2,
			},
		},
		{
			name: "singleuse",
			effect: &BorderEffect{
				Style:     "C",
				Intensity: 2,
				SingleUse: true,
			},
		},
		{
			name:   "empty",
			effect: &BorderEffect{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
			rm := pdf.NewResourceManager(buf)

			// embed the border effect
			embedded, err := rm.Embed(tt.effect)
			if err != nil {
				t.Fatal(err)
			}

			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			// extract it back
			x := pdf.NewExtractor(buf)
			extracted, err := ExtractBorderEffect(x, embedded)
			if err != nil {
				t.Fatal(err)
			}

			expected := *tt.effect
			if expected.Style == "" {
				// empty style gets normalized to "S" during extraction
				expected.Style = "S"
			}

			if diff := cmp.Diff(expected, *extracted); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}
