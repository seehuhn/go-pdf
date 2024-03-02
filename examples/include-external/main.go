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

package main

import (
	"bytes"
	"io"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/matrix"
	"seehuhn.de/go/pdf/pagetree"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	paper := document.A4
	page, err := document.CreateSinglePage("test.pdf", paper, nil)
	if err != nil {
		return err
	}

	B, err := type1.TimesBold.Embed(page.Out, nil)
	if err != nil {
		return err
	}
	F, err := type1.TimesRoman.Embed(page.Out, nil)
	if err != nil {
		return err
	}

	figure, bbox, err := LoadFigure("fig.pdf", page.Out)
	if err != nil {
		return err
	}

	width := bbox.Dx()
	height := bbox.Dy()
	base := paper.URy - 72 - height
	left := paper.LLx + 0.5*(paper.Dx()-width)
	page.PushGraphicsState()
	page.Transform(matrix.Translate(left-bbox.LLx, base-bbox.LLy))
	page.DrawXObject(figure)
	page.PopGraphicsState()

	base -= 12
	page.TextStart()
	page.TextFirstLine(72, base)
	page.TextSetFont(B, 10)
	page.TextShow("Figure 1.  ")
	page.TextSetFont(F, 10)
	page.TextShow("A grid of pair scatter plots for R's built-in iris dataset.  The plot illustrates the joint distribution")
	page.TextSecondLine(0, -12)
	page.TextShow("of sepal length, sepal width, petal length, and petal width for three species of iris.")
	page.TextEnd()

	err = page.Close()
	if err != nil {
		return err
	}
	return nil
}

func LoadFigure(fname string, w pdf.Putter) (*graphics.XObject, *pdf.Rectangle, error) {
	r, err := pdf.Open(fname, nil)
	if err != nil {
		return nil, nil, err
	}
	dict, err := pagetree.GetPage(r, 0)
	if err != nil {
		return nil, nil, err
	}

	cropBox := dict["CropBox"]
	if cropBox == nil {
		cropBox = dict["MediaBox"]
	}
	bbox, err := pdf.GetRectangle(r, cropBox)
	if err != nil {
		return nil, nil, err
	}

	copier := NewCopier(w, r)

	origResources, err := pdf.GetDict(r, dict["Resources"])
	if err != nil {
		return nil, nil, err
	}
	resourceObj, err := copier.Copy(origResources)
	resourceDict := resourceObj.(pdf.Dict)
	resources := &pdf.Resources{}
	err = pdf.DecodeDict(nil, resources, resourceDict)
	if err != nil {
		return nil, nil, err
	}

	body := &bytes.Buffer{}
	contents, err := pdf.Resolve(r, dict["Contents"])
	switch x := contents.(type) {
	case *pdf.Stream:
		stm, err := pdf.DecodeStream(r, x, 0)
		if err != nil {
			return nil, nil, err
		}
		_, err = io.Copy(body, stm)
		if err != nil {
			return nil, nil, err
		}
	case pdf.Array:
		for _, ref := range x {
			obj, err := pdf.GetStream(r, ref)
			if err != nil {
				return nil, nil, err
			}
			stm, err := pdf.DecodeStream(r, obj, 0)
			if err != nil {
				return nil, nil, err
			}
			_, err = io.Copy(body, stm)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	err = r.Close()
	if err != nil {
		return nil, nil, err
	}

	figure := &form.FormProperties{
		BBox: bbox,
	}

	obj, err := form.Raw(w, figure, body.Bytes(), resources)
	if err != nil {
		return nil, nil, err
	}
	return obj, bbox, nil
}

type copier struct {
	trans map[pdf.Reference]pdf.Reference
	w     pdf.Putter
	r     pdf.Getter
}

func NewCopier(w pdf.Putter, r pdf.Getter) *copier {
	res := &copier{
		trans: make(map[pdf.Reference]pdf.Reference),
		w:     w,
		r:     r,
	}
	return res
}

func (w *copier) Copy(obj pdf.Object) (pdf.Object, error) {
	switch x := obj.(type) {
	case pdf.Dict:
		res := pdf.Dict{}
		for key, val := range x {
			repl, err := w.Copy(val)
			if err != nil {
				return nil, err
			}
			res[key] = repl
		}
		return res, nil
	case pdf.Array:
		var res pdf.Array
		for _, val := range x {
			repl, err := w.Copy(val)
			if err != nil {
				return nil, err
			}
			res = append(res, repl)
		}
		return res, nil
	case *pdf.Stream:
		res := &pdf.Stream{
			Dict: make(pdf.Dict),
			R:    x.R,
		}
		for key, val := range x.Dict {
			repl, err := w.Copy(val)
			if err != nil {
				return nil, err
			}
			res.Dict[key] = repl
		}
		return res, nil
	case pdf.Reference:
		other, ok := w.trans[x]
		if ok {
			return other, nil
		}
		other = w.w.Alloc()
		w.trans[x] = other

		val, err := pdf.Resolve(w.r, x)
		if err != nil {
			return nil, err
		}
		trans, err := w.Copy(val)
		if err != nil {
			return nil, err
		}
		err = w.w.Put(other, trans)
		if err != nil {
			return nil, err
		}
		return other, nil
	}
	return obj, nil
}
