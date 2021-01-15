package fuzzing

import (
	"bytes"
	"io"
	"io/ioutil"

	"seehuhn.de/go/pdf"
)

// Walk performs a depth-first walk through the object graph rooted at obj.
func Walk(r *pdf.Reader, obj pdf.Object, seen map[pdf.Reference]pdf.Object,
	leaf func(pdf.Object) (pdf.Object, error)) (pdf.Object, error) {
	switch x := obj.(type) {
	case pdf.Dict:
		res := pdf.Dict{}
		for _, key := range x.SortedKeys() {
			repl, err := Walk(r, x[key], seen, leaf)
			if err != nil {
				return nil, err
			}
			res[key] = repl
		}
		return res, nil
	case pdf.Array:
		var res pdf.Array
		for _, val := range x {
			repl, err := Walk(r, val, seen, leaf)
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
		for _, key := range x.SortedKeys() {
			repl, err := Walk(r, x.Dict[key], seen, leaf)
			if err != nil {
				return nil, err
			}
			res.Dict[key] = repl
		}
		return res, nil
	case *pdf.Reference:
		if other, ok := seen[*x]; ok {
			return other, nil
		}
		res, err := leaf(x)
		if err != nil {
			return nil, err
		}
		seen[*x] = res

		ind, err := r.Get(x)
		if err != nil {
			return nil, err
		}
		_, err = Walk(r, ind, seen, leaf)
		if err != nil {
			return nil, err
		}

		return res, nil
	}
	return leaf(obj)
}

// Fuzz is the entrance point for github.com/dvyukov/go-fuzz
func Fuzz(data []byte) int {
	buf := bytes.NewReader(data)
	r, err := pdf.NewReader(buf, buf.Size(), nil)
	if err != nil {
		return 0
	}

	seen := make(map[pdf.Reference]pdf.Object)
	_, err = Walk(r, pdf.Array{pdf.Struct(r.Catalog), pdf.Struct(r.Info)},
		seen, func(o pdf.Object) (pdf.Object, error) {
			if stream, ok := o.(*pdf.Stream); ok {
				_, err := io.Copy(ioutil.Discard, stream.R)
				if err != nil {
					return nil, err
				}
			}
			return nil, nil
		})

	if err != nil {
		return 0
	}
	return 1
}
