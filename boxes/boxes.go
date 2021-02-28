// seehuhn.de/go/pdf - support for reading and writing PDF files
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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/fonts"
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

type text struct {
	font     pdf.Name
	fontSize float64
	layout   *fonts.Layout
}

func (obj *text) Extent() *stuffExtent {
	return &stuffExtent{
		Width:  obj.layout.Width,
		Height: obj.layout.Height,
		Depth:  obj.layout.Depth,
	}
}

func (obj *text) Draw(page *pages.Page, xPos, yPos float64) {
	if len(obj.layout.Fragments) == 0 {
		return
	}

	// TODO(voss): use "Tj" if len(Fragments)==1

	fmt.Fprintln(page, "q")
	fmt.Fprintln(page, "BT")
	obj.font.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", obj.fontSize)
	fmt.Fprintf(page, "%f %f Td\n", xPos, yPos)
	fmt.Fprint(page, "[")
	for i, frag := range obj.layout.Fragments {
		if i > 0 {
			kern := obj.layout.Kerns[i-1]
			iKern := int64(kern)
			if float64(iKern) == kern {
				fmt.Fprintf(page, " %d ", iKern)
			} else {
				fmt.Fprintf(page, " %f ", kern)
			}
		}
		pdf.String(frag).PDF(page)
	}
	fmt.Fprint(page, "] TJ\n")
	fmt.Fprintln(page, "ET")
	fmt.Fprintln(page, "Q")
}
