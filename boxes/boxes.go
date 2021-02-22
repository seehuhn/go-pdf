package boxes

import (
	"fmt"

	"seehuhn.de/go/pdf/pages"
)

type stuff interface {
	Extent() *stuffExtent
	Draw(page *pages.Page, xPos, yPos float64)
}

type stretcher interface {
	Stretch() *stretchAmount
}

type stretchAmount struct {
	Val   float64
	Level int
}

type stuffExtent struct {
	Width, Height, Depth float64
}

func (obj stuffExtent) Extent() *stuffExtent {
	return &obj
}

type rule struct {
	stuffExtent
}

func (obj *rule) Draw(page *pages.Page, xPos, yPos float64) {
	fmt.Fprintf(page, "%f %f %f %f re f\n",
		xPos, yPos-obj.Depth, obj.Width, obj.Depth+obj.Height)
}

type vBox struct {
	stuffExtent

	Contents []stuff
}

func (obj *vBox) Draw(page *pages.Page, xPos, yPos float64) {
	fmt.Fprintln(page, "q")
	fmt.Fprintln(page, "0 .8 0 RG")
	fmt.Fprintf(page, "%f %f %f %f re s\n",
		xPos, yPos-obj.Depth, obj.Width, obj.Depth+obj.Height)
	fmt.Fprintln(page, "Q")

	boxTotal := obj.Depth + obj.Height
	contentsTotal := 0.0
	for _, child := range obj.Contents {
		ext := child.Extent()
		contentsTotal += ext.Depth + ext.Height
	}
	if contentsTotal < boxTotal-1e-3 {
		fmt.Println("underfull")
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
				obj.Contents[i] = kern(amount)
			}
		}
	} else if contentsTotal > boxTotal+1e-3 {
		fmt.Println("overfull")
	}

	y := yPos + obj.Height
	for _, child := range obj.Contents {
		ext := child.Extent()
		y -= ext.Height
		child.Draw(page, xPos, y)
		y -= ext.Depth
	}
}

type hBox struct {
	stuffExtent

	Contents []stuff
}

func (obj *hBox) Draw(page *pages.Page, xPos, yPos float64) {
	fmt.Fprintln(page, "q")
	fmt.Fprintln(page, ".7 .7 1 RG")
	fmt.Fprintf(page, "%f %f %f %f re s\n",
		xPos, yPos-obj.Depth, obj.Width, obj.Depth+obj.Height)
	fmt.Fprintln(page, "Q")

	boxTotal := obj.Width
	contentsTotal := 0.0
	for _, child := range obj.Contents {
		ext := child.Extent()
		contentsTotal += ext.Width
	}
	if contentsTotal < boxTotal-1e-3 {
		fmt.Println("underfull")
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
				obj.Contents[i] = kern(amount)
			}
		}
	} else if contentsTotal > boxTotal+1e-3 {
		fmt.Println("overfull")
	}

	x := xPos
	for _, child := range obj.Contents {
		ext := child.Extent()
		child.Draw(page, x, yPos)
		x += ext.Width
	}
}

type kern float64

func (obj kern) Extent() *stuffExtent {
	return &stuffExtent{
		Width:  float64(obj),
		Height: float64(obj),
	}
}

func (obj kern) Draw(page *pages.Page, xPos, yPos float64) {}

type glue struct {
	Length float64
	Plus   stretchAmount
	Minus  stretchAmount
}

func (obj *glue) Extent() *stuffExtent {
	return &stuffExtent{
		Width:  obj.Length,
		Height: obj.Length,
	}
}

func (obj *glue) Draw(page *pages.Page, xPos, yPos float64) {}

func (obj *glue) Stretch() *stretchAmount {
	return &obj.Plus
}
