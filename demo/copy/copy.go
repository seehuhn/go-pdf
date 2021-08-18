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

package main

import (
	"log"
	"os"

	"seehuhn.de/go/pdf"
)

type walker struct {
	trans map[pdf.Reference]*pdf.Reference
	r     *pdf.Reader
	w     *pdf.Writer
}

func (w *walker) Transfer(obj pdf.Object) (pdf.Object, error) {
	switch x := obj.(type) {
	case pdf.Dict:
		res := pdf.Dict{}
		for key, val := range x {
			repl, err := w.Transfer(val)
			if err != nil {
				return nil, err
			}
			res[key] = repl
		}
		return res, nil
	case pdf.Array:
		var res pdf.Array
		for _, val := range x {
			repl, err := w.Transfer(val)
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
			repl, err := w.Transfer(val)
			if err != nil {
				return nil, err
			}
			res.Dict[key] = repl
		}
		return res, nil
	case *pdf.Reference:
		other, ok := w.trans[*x]
		if ok {
			return other, nil
		}
		other = w.w.Alloc()
		w.trans[*x] = other

		val, err := w.r.Resolve(x)
		if err != nil {
			return nil, err
		}
		trans, err := w.Transfer(val)
		if err != nil {
			return nil, err
		}
		_, err = w.w.Write(trans, other)
		if err != nil {
			return nil, err
		}
		return other, nil
	}
	return obj, nil
}

func main() {
	r, err := pdf.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	out, err := os.Create("out.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	w, err := pdf.NewWriter(out, &pdf.WriterOptions{
		Version: r.Version,
	})
	if err != nil {
		log.Fatal(err)
	}

	catalog, err := r.GetCatalog()
	if err != nil {
		log.Fatal(err)
	}

	trans := &walker{
		trans: map[pdf.Reference]*pdf.Reference{},
		r:     r,
		w:     w,
	}
	catDict := pdf.Struct(catalog)
	newCatDict := pdf.Dict{}
	for key, val := range catDict {
		obj, err := trans.Transfer(val)
		if err != nil {
			log.Fatal(err)
		}
		newCatDict[key] = obj
	}
	newCatDict.AsStruct(catalog, r.Resolve)

	info, err := r.GetInfo()
	if err != nil {
		log.Fatal(err)
	}
	trans.w.SetInfo(info)

	trans.w.SetCatalog(catalog)

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}
