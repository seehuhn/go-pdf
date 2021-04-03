package boxes

import (
	"fmt"

	"seehuhn.de/go/pdf/pages"
)

// VBox represents a Box which contains a column of sub-objects.
type VBox struct {
	BoxExtent

	Contents []Box
}

// NewVBox creates a new VBox
func (p *Parameters) NewVBox(children ...Box) *VBox {
	vbox := &VBox{}
	totalHeight := 0.0
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
	vbox.Height = totalHeight - lastDepth
	vbox.Depth = lastDepth
	return vbox
}

// NewVBoxTo creates a new VBox with a given total height
func (p *Parameters) NewVBoxTo(total float64, children ...Box) *VBox {
	vbox := &VBox{}
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
func (obj *VBox) Draw(page *pages.Page, xPos, yPos float64) {
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
