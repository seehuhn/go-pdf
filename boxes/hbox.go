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

import (
	"math"

	"seehuhn.de/go/pdf/graphics"
)

// hBox represents a Box which contains a row of sub-objects.
type hBox struct {
	BoxExtent

	Contents []Box
}

// HBox creates a new HBox
func HBox(children ...Box) Box {
	hbox := &hBox{
		BoxExtent: BoxExtent{
			Height: math.Inf(-1),
			Depth:  math.Inf(-1),
		},
		Contents: children,
	}
	for _, box := range children {
		ext := box.Extent()
		hbox.Width += ext.Width
		if ext.Height > hbox.Height && !ext.WhiteSpaceOnly {
			hbox.Height = ext.Height
		}
		if ext.Depth > hbox.Depth && !ext.WhiteSpaceOnly {
			hbox.Depth = ext.Depth
		}
	}
	return hbox
}

// HBoxTo creates a new HBox with the given width
func HBoxTo(total float64, boxes ...Box) Box {
	hbox := &hBox{
		BoxExtent: BoxExtent{
			Width:  total,
			Height: math.Inf(-1),
			Depth:  math.Inf(-1),
		},
		Contents: boxes,
	}
	for _, box := range boxes {
		ext := box.Extent()
		if ext.Height > hbox.Height && !ext.WhiteSpaceOnly {
			hbox.Height = ext.Height
		}
		if ext.Depth > hbox.Depth && !ext.WhiteSpaceOnly {
			hbox.Depth = ext.Depth
		}
	}
	return hbox
}

// Draw implements the Box interface.
func (obj *hBox) Draw(page *graphics.Page, xPos, yPos float64) {
	boxTotal := obj.Width
	contentsTotal := 0.0
	for _, child := range obj.Contents {
		ext := child.Extent()
		contentsTotal += ext.Width
	}
	if contentsTotal < boxTotal-1e-3 {
		level := -1
		var ii []int
		stretchTotal := 0.0
		for i, child := range obj.Contents {
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
			q := (boxTotal - contentsTotal) / stretchTotal
			if level == 0 && q > 1 {
				q = 1
			}
			for _, i := range ii {
				child := obj.Contents[i]
				ext := child.Extent()
				amount := ext.Width + child.(stretcher).Stretch().Val*q
				obj.Contents[i] = Kern(amount)
			}
		}
	} else if contentsTotal > boxTotal+1e-3 {
		level := -1
		var ii []int
		shrinkTotal := 0.0
		for i, child := range obj.Contents {
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
			q := (contentsTotal - boxTotal) / shrinkTotal
			if level == 0 && q > 1 {
				q = 1
			}
			for _, i := range ii {
				child := obj.Contents[i]
				ext := child.Extent()
				amount := ext.Width - child.(shrinker).Shrink().Val*q
				obj.Contents[i] = Kern(amount)
			}
		}
	}

	x := xPos
	for _, child := range obj.Contents {
		ext := child.Extent()
		child.Draw(page, x, yPos)
		x += ext.Width
	}
}
