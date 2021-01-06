package pdf

import (
	"io"
)

type pdfError struct {
	err error
}

func (err *pdfError) PDF(w io.Writer) error {
	panic("do not use")
}

type pdfTrailer struct {
	info    Object
	catalog Object
}

func (trailer *pdfTrailer) PDF(w io.Writer) error {
	panic("do not use")
}

// NewSequentialReader reads all objects in a PDF file sequentially and
// returns these objects via a channel.
//
// TODO(voss): remove this from the API
// func NewSequentialReader(r io.Reader) <-chan Object {
// 	getInt := func(obj Object) (Integer, error) {
// 		if x, ok := obj.(Integer); ok {
// 			return x, nil
// 		}
// 		panic("Rumpelstilzchen")
// 	}

// 	c := make(chan Object, 1)
// 	s := newScanner(r, getInt)
// 	go func(c chan<- Object, s *scanner) {
// 		defer close(c)

// 		var Info, Catalog Object
// 		checkTrailer := func(dict Dict) {
// 			if val, ok := dict["Root"]; ok {
// 				Catalog = val
// 			}
// 			if val, ok := dict["Info"]; ok {
// 				Info = val
// 			}
// 		}

// 		ver, err := s.readHeaderVersion()
// 		if err != nil {
// 			c <- &pdfError{err}
// 			return
// 		}
// 		c <- ver
// 		for {
// 			for {
// 				err = s.SkipWhiteSpace()
// 				if err != nil {
// 					c <- &pdfError{err}
// 					return
// 				}
// 				obj, err := s.ReadIndirectObject()
// 				if err != nil {
// 					break
// 				}

// 				if s, ok := obj.Obj.(*Stream); ok && s.Dict["Type"] == Name("ObjStm") {
// 					panic("not implemented")
// 				}

// 				if s, ok := obj.Obj.(*Stream); ok && s.Dict["Type"] == Name("XRef") {
// 					checkTrailer(s.Dict)
// 				}

// 				c <- obj
// 			}

// 			buf, err := s.Peek(9)
// 			if bytes.HasPrefix(buf, []byte("startxref")) {
// 				s.SkipString("startxref")
// 				err = s.SkipWhiteSpace()
// 				if err != nil {
// 					c <- &pdfError{err}
// 					return
// 				}
// 				_, err = s.ReadInteger()
// 				if err != nil {
// 					c <- &pdfError{err}
// 					return
// 				}
// 			} else if bytes.HasPrefix(buf, []byte("xref")) {
// 				s.SkipAfter("trailer")
// 				err = s.SkipWhiteSpace()
// 				if err != nil {
// 					c <- &pdfError{err}
// 					return
// 				}
// 				dict, err := s.ReadDict()
// 				if err != nil {
// 					c <- &pdfError{err}
// 					return
// 				}
// 				checkTrailer(dict)
// 			} else {
// 				if err != nil {
// 					fmt.Println("error:", err)
// 				} else if len(buf) > 0 {
// 					fmt.Println("next:", string(buf))
// 				} else {
// 					fmt.Println("done")
// 				}
// 				break
// 			}
// 		}
// 		c <- &pdfTrailer{
// 			info:    Info,
// 			catalog: Catalog,
// 		}
// 	}(c, s)

// 	return c
// }
