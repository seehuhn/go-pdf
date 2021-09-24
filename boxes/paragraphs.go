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

package boxes

var parEndFill = Glue(0, 1, 2, 0, 0)

// Flow breaks the content into lines of the given width.
func Flow(contents []Box, width float64) []Box {
	contents = append(contents, parEndFill)

	cost := func(k, l int) int {
		ll := tryLength(contents[k:l], width)
	}
	_ = cost
	panic("not implemented")
}

type lineFillInfo struct {
	length  float64
	stretch float64
}

func tryLength(contents []Box, width float64) lineFillInfo {
	contentsTotal := 0.0
	for _, child := range contents {
		ext := child.Extent()
		contentsTotal += ext.Width
	}
	stretchTotal := 0.0
	if contentsTotal < width-1e-3 {
		level := -1
		var ii []int
		for i, child := range contents {
			stretch, ok := child.(stretcher)
			if !ok {
				continue
			}
			info := stretch.Stretch()

			if info.Level > level {
				level = info.Level
				ii = nil
				stretchTotal = 0
			}
			ii = append(ii, i)
			stretchTotal += info.Val
		}

		if stretchTotal > 0 {
			q := (width - contentsTotal) / stretchTotal
			if level == 0 && q > 1 {
				q = 1
			}
			stretchTotal *= q
		}
	} else if contentsTotal > width+1e-3 {
		level := -1
		var ii []int
		shrinkTotal := 0.0
		for i, child := range contents {
			shrink, ok := child.(shrinker)
			if !ok {
				continue
			}
			info := shrink.Shrink()

			if info.Level > level {
				level = info.Level
				ii = nil
				shrinkTotal = 0
			}
			ii = append(ii, i)
			shrinkTotal += info.Val
		}

		if shrinkTotal > 0 {
			q := (contentsTotal - width) / shrinkTotal
			if level == 0 && q > 1 {
				q = 1
			}
			stretchTotal = -shrinkTotal * q
		}
	}

	return lineFillInfo{
		length:  contentsTotal + stretchTotal,
		stretch: stretchTotal,
	}
}
