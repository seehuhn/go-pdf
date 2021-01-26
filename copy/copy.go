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
	w, err := pdf.NewWriter(out, &pdf.WriterOptions{
		Version: r.Version,
	})
	if err != nil {
		log.Fatal(err)
	}

	trans := &walker{
		trans: map[pdf.Reference]*pdf.Reference{},
		r:     r,
		w:     w,
	}

	root, err := r.Catalog()
	if err != nil {
		log.Fatal(err)
	}
	catalog, err := trans.Transfer(root)
	if err != nil {
		log.Fatal(err)
	}
	catRef, err := trans.w.Write(catalog, nil)
	if err != nil {
		log.Fatal(err)
	}

	infoDict, err := r.Info()
	if err != nil {
		log.Fatal(err)
	}
	info, err := trans.Transfer(infoDict)
	if err != nil {
		log.Fatal(err)
	}
	infoRef, err := trans.w.Write(info, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = w.Close(catRef, infoRef)
	if err != nil {
		log.Fatal(err)
	}
}
