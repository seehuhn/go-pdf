// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package color

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
)

// color.Space implements pdf.Embedder
var (
	_ pdf.Embedder[pdf.Unused] = Space(nil)
)

// The following types implement the ColorSpace interface:
var (
	_ Space = SpaceDeviceGray{}
	_ Space = SpaceDeviceRGB{}
	_ Space = SpaceDeviceCMYK{}
	_ Space = (*SpaceCalGray)(nil)
	_ Space = (*SpaceCalRGB)(nil)
	_ Space = (*SpaceLab)(nil)
	_ Space = (*SpaceICCBased)(nil)
	_ Space = spacePatternColored{}
	_ Space = spacePatternUncolored{}
	_ Space = (*SpaceIndexed)(nil)
	// TODO(voss): Separation colour spaces
	// TODO(voss): DeviceN colour spaces
)

func TestSpaceRoundTrip(t *testing.T) {
	testCases := []Space{
		SpaceDeviceGray{},
		SpaceDeviceRGB{},
		spacePatternColored{},
	}
	s, err := CalGray(WhitePointD65, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	testCases = append(testCases, s)
	s, err = CalGray(WhitePointD65, []float64{0.1, 0.2, 0.05}, 1.2)
	if err != nil {
		t.Fatal(err)
	}
	testCases = append(testCases, s)

	for i, space := range testCases {
		t.Run(fmt.Sprintf("%02d-%s", i+1, space.ColorSpaceFamily()), func(t *testing.T) {
			data := pdf.NewData(pdf.V2_0)
			rm := pdf.NewResourceManager(data)
			obj, _, err := space.Embed(rm)
			if err != nil {
				t.Fatal(err)
			}

			space2, err := ReadSpace(data, obj)
			if err != nil {
				t.Fatal(err)
			}

			if space.ColorSpaceFamily() != space2.ColorSpaceFamily() {
				t.Errorf("expected %s, got %s", space.ColorSpaceFamily(), space2.ColorSpaceFamily())
			}

			opts := []cmp.Option{
				cmp.AllowUnexported(SpaceCalGray{}),
			}
			if d := cmp.Diff(space, space2, opts...); d != "" {
				t.Errorf("unexpected diff: %s", d)
			}
		})
	}
}
