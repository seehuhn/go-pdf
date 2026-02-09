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

package image

import "seehuhn.de/go/pdf/graphics/color"

// DefaultDecode returns the default Decode array for an image
// with the given color space and bits per component.
// The returned array has 2*cs.Channels() entries, with pairs
// [Dmin, Dmax] for each channel.
func DefaultDecode(cs color.Space, bpc int) []float64 {
	n := cs.Channels()
	d := make([]float64, 2*n)
	switch cs := cs.(type) {
	case *color.SpaceLab:
		d[0], d[1] = 0, 100
		d[2], d[3] = cs.Ranges[0], cs.Ranges[1]
		d[4], d[5] = cs.Ranges[2], cs.Ranges[3]
	case *color.SpaceICCBased:
		copy(d, cs.Ranges[:2*n])
	case *color.SpaceIndexed:
		d[0], d[1] = 0, float64(int(1)<<bpc-1)
	default:
		for i := range n {
			d[2*i+1] = 1
		}
	}
	return d
}
