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

		val, err := w.r.Get(x)
		if err != nil {
			return nil, err
		}
		trans, err := w.Transfer(val)
		if err != nil {
			return nil, err
		}
		_, err = w.w.WriteIndirect(trans, other)
		if err != nil {
			return nil, err
		}
		return other, nil
	}
	return obj, nil
}

func main() {
	fname := os.Args[1]
	in, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		log.Fatal(err)
	}
	r, err := pdf.NewReader(in, fi.Size(), nil)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("out.pdf")
	if err != nil {
		log.Fatal(err)
	}
	w, err := pdf.NewWriter(out, r.HeaderVersion)

	trans := &walker{
		trans: map[pdf.Reference]*pdf.Reference{},
		r:     r,
		w:     w,
	}
	obj, err := trans.Transfer(r.Trailer)
	if err != nil {
		log.Fatal(err)
	}

	trailer := obj.(pdf.Dict)
	err = w.Close(trailer["Root"].(*pdf.Reference), trailer["Info"].(*pdf.Reference))
	if err != nil {
		log.Fatal(err)
	}
}
