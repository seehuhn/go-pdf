package numtree

import (
	"io"

	"seehuhn.de/go/pdf"
)

type NumberTree interface {
	Get(key pdf.Integer) (pdf.Object, error)
	First() (pdf.Integer, error)
	Next(after pdf.Integer) (pdf.Integer, error)
}

func Read(r pdf.Reader, root pdf.Object) (NumberTree, error) {
	panic("not implemented")
}

func Write(w *pdf.Writer, tree NumberTree) (*pdf.Reference, error) {
	sw := NewSequentialWriter(w)
	pos, err := tree.First()
	if err != nil {
		return nil, err
	}
	for {
		val, err := tree.Get(pos)
		if err != nil {
			return nil, err
		}
		err = sw.Append(pos, val)
		if err != nil {
			return nil, err
		}
		pos, err = tree.Next(pos)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	err = sw.Close()
	if err != nil {
		return nil, err
	}
	return sw.Reference(), nil
}

type SequentialWriter struct {
	w   *pdf.Writer
	ref *pdf.Reference
}

func NewSequentialWriter(w *pdf.Writer) *SequentialWriter {
	res := &SequentialWriter{w: w}
	_ = res
	panic("not implemented")
}

func (sw *SequentialWriter) Append(key pdf.Integer, val pdf.Object) error {
	panic("not implemented")
}

func (sw *SequentialWriter) Close() error {
	panic("not implemented")
}

func (sw *SequentialWriter) Reference() *pdf.Reference {
	return sw.ref
}
