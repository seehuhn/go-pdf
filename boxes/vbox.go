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
	"fmt"

	"seehuhn.de/go/pdf/graphics"
)

// vBox represents a Box which contains a column of sub-objects.
type vBox struct {
	BoxExtent

	Contents []Box
}

// VBox creates a new VBox, where the baseline coincides with the baseline of
// the last child.
func (p *Parameters) VBox(children ...Box) Box {
	return p.vBoxInternal(false, children...)
}

// VTop creates a new VBox, where the baseline coincides with the baseline of
// the first child.
func (p *Parameters) VTop(children ...Box) Box {
	return p.vBoxInternal(true, children...)
}

func (p *Parameters) vBoxInternal(top bool, children ...Box) *vBox {
	vbox := &vBox{}
	totalHeight := 0.0
	firstHeight := 0.0
	lastDepth := 0.0
	first := true
	for len(children) > 0 {
		child := children[0]
		children = children[1:]
		ext := child.Extent()

		if first {
			firstHeight = ext.Height
		}

		if first || ext.WhiteSpaceOnly {
			first = ext.WhiteSpaceOnly
		} else {
			gap := lastDepth + ext.Height
			if gap < p.BaseLineSkip {
				extra := p.BaseLineSkip - gap
				vbox.Contents = append(vbox.Contents, Kern(extra))
				totalHeight += extra
			}
		}
		vbox.Contents = append(vbox.Contents, child)
		totalHeight += ext.Depth + ext.Height

		if ext.Width > vbox.Width && !ext.WhiteSpaceOnly {
			vbox.Width = ext.Width
		}

		lastDepth = ext.Depth
	}
	if top {
		vbox.Height = firstHeight
		vbox.Depth = totalHeight - firstHeight
	} else {
		vbox.Height = totalHeight - lastDepth
		vbox.Depth = lastDepth
	}
	return vbox
}

// VBoxTo creates a new VBox with a given total height
func (p *Parameters) VBoxTo(total float64, children ...Box) Box {
	vbox := &vBox{}
	lastDepth := 0.0
	first := true
	for len(children) > 0 {
		child := children[0]
		children = children[1:]
		ext := child.Extent()

		if first || ext.WhiteSpaceOnly {
			first = ext.WhiteSpaceOnly
		} else {
			gap := lastDepth + ext.Height
			if gap < p.BaseLineSkip {
				extra := p.BaseLineSkip - gap
				vbox.Contents = append(vbox.Contents, Kern(extra))
			}
		}
		vbox.Contents = append(vbox.Contents, child)

		if ext.Width > vbox.Width && !ext.WhiteSpaceOnly {
			vbox.Width = ext.Width
		}

		lastDepth = ext.Depth
	}
	vbox.Height = total - lastDepth
	vbox.Depth = lastDepth
	return vbox
}

// Draw implements the Box interface.
func (obj *vBox) Draw(page *graphics.Page, xPos, yPos float64) {
	boxTotal := obj.Depth + obj.Height
	contentsTotal := 0.0
	for _, child := range obj.Contents {
		ext := child.Extent()
		contentsTotal += ext.Depth + ext.Height
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
				amount := ext.Depth + ext.Height + child.(stretcher).Stretch().Val*q
				obj.Contents[i] = Kern(amount)
			}
		}
	} else if contentsTotal > boxTotal+1e-3 {
		fmt.Println("overful vbox")
		// TODO(voss)
	}

	y := yPos + obj.Height
	for _, child := range obj.Contents {
		ext := child.Extent()
		y -= ext.Height
		child.Draw(page, xPos, y)
		y -= ext.Depth
	}
}
