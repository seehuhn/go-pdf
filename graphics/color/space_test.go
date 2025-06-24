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
	"reflect"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// color.Space implements pdf.Embedder
var (
	_ pdf.Embedder[pdf.Unused] = Space(nil)
)

// The following types implement the ColorSpace interface:
var (
	_ Space = spaceDeviceGray{}
	_ Space = spaceDeviceRGB{}
	_ Space = spaceDeviceCMYK{}
	_ Space = (*SpaceCalGray)(nil)
	_ Space = (*SpaceCalRGB)(nil)
	_ Space = (*SpaceLab)(nil)
	_ Space = (*SpaceICCBased)(nil)
	_ Space = spaceSRGB{} // a special case of ICCBased (built-in profiles)
	_ Space = spacePatternColored{}
	_ Space = spacePatternUncolored{}
	_ Space = (*SpaceIndexed)(nil)
	_ Space = (*SpaceSeparation)(nil)
	_ Space = (*SpaceDeviceN)(nil)
)

var testColorSpaces = []Space{
	spaceDeviceGray{},

	spaceDeviceRGB{},

	spaceDeviceCMYK{},

	must(CalGray(WhitePointD65, nil, 1)),
	must(CalGray(WhitePointD65, []float64{0.1, 0.1, 0.1}, 1.2)),

	must(CalRGB(WhitePointD50, nil, nil, nil)),
	must(CalRGB(WhitePointD50, []float64{0.1, 0.1, 0.1}, []float64{1.2, 1.1, 1.0},
		[]float64{0.9, 0.1, 0, 0, 1, 0, 0, 0, 1})),

	must(Lab(WhitePointD65, nil, nil)),
	must(Lab(WhitePointD65, []float64{0.1, 0, 0}, []float64{-90, 90, -110, 110})),

	must(ICCBased(sRGBv2, nil)),
	must(ICCBased(sRGBv4, nil)),

	spacePatternColored{},

	spacePatternUncolored{base: spaceDeviceGray{}},
	spacePatternUncolored{base: must(CalGray(WhitePointD65, nil, 1.2))},

	must(Indexed([]Color{DeviceRGB(0, 0, 0), DeviceRGB(1, 1, 1)})),

	must(Separation("foo", DeviceRGBSpace, &function.Type2{
		XMin: 0,
		XMax: 1,
		C0:   []float64{1, 0, 0},
		C1:   []float64{0, 1, 0},
		N:    1,
	})),

	must(DeviceN([]pdf.Name{"bar"}, DeviceRGBSpace, &function.Type2{
		XMin: 0,
		XMax: 1,
		C0:   []float64{1, 0, 0},
		C1:   []float64{0, 1, 0},
		N:    1,
	}, nil)),
	// TODO(voss): DeviceN colour spaces
}

func TestDecodeSpace(t *testing.T) {
	for i, space := range testColorSpaces {
		t.Run(fmt.Sprintf("%02d-%s", i, space.Family()), func(t *testing.T) {
			r, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(r)

			obj, _, err := pdf.ResourceManagerEmbed(rm, space)
			if err != nil {
				t.Fatal(err)
			}

			space2, err := ExtractSpace(r, obj)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(space, space2) {
				t.Errorf("got %#v, want %#v", space2, space)
			}
		})
	}
}

func must(space Space, err error) Space {
	if err != nil {
		panic(err)
	}
	return space
}
