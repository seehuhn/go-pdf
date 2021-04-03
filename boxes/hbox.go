package boxes

import (
	"math"

	"seehuhn.de/go/pdf/pages"
)

// hBox represents a Box which contains a row of sub-objects.
type hBox struct {
	BoxExtent

	Contents []Box
}

// NewHBox creates a new HBox
func NewHBox(children ...Box) Box {
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

// NewHBoxTo creates a new HBox with the given width
func NewHBoxTo(total float64, boxes ...Box) Box {
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
func (obj *hBox) Draw(page *pages.Page, xPos, yPos float64) {
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
		// TODO(voss)
	}

	x := xPos
	for _, child := range obj.Contents {
		ext := child.Extent()
		child.Draw(page, x, yPos)
		x += ext.Width
	}
}
