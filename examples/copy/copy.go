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

package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"

	"seehuhn.de/go/pdf"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

type walker struct {
	trans map[pdf.Reference]pdf.Reference
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
		trans, err := w.Transfer(val)
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

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	r, err := pdf.Open(flag.Arg(0), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	out, err := os.Create("out.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	w, err := pdf.NewWriter(out, r.GetMeta().Version, nil)
	if err != nil {
		log.Fatal(err)
	}

	catalog := r.GetMeta().Catalog

	trans := &walker{
		trans: map[pdf.Reference]pdf.Reference{},
		r:     r,
		w:     w,
	}
	catDict := pdf.AsDict(catalog)
	newCatDict := pdf.Dict{}
	for key, val := range catDict {
		obj, err := trans.Transfer(val)
		if err != nil {
			log.Fatal(err)
		}
		newCatDict[key] = obj
	}
	err = pdf.DecodeDict(r, catalog, newCatDict)
	if err != nil {
		log.Fatal(err)
	}

	trans.w.GetMeta().Info = r.GetMeta().Info
	trans.w.GetMeta().Catalog = catalog

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}
