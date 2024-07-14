// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package document

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pagetree"
)

type MultiPage struct {
	Out  pdf.Putter
	RM   *pdf.ResourceManager
	Tree *pagetree.Writer

	mediaBox *pdf.Rectangle

	numOpen int
	base    io.Closer
}

func CreateMultiPage(fileName string, pageSize *pdf.Rectangle, v pdf.Version, opt *pdf.WriterOptions) (*MultiPage, error) {
	fd, err := os.Create(fileName)
	if err != nil {
		return nil, err
	}
	doc, err := WriteMultiPage(fd, pageSize, v, opt)
	if err != nil {
		fd.Close()
		return nil, err
	}
	doc.base = fd
	return doc, nil
}

func WriteMultiPage(w io.Writer, pageSize *pdf.Rectangle, v pdf.Version, opt *pdf.WriterOptions) (*MultiPage, error) {
	out, err := pdf.NewWriter(w, v, opt)
	if err != nil {
		return nil, err
	}

	rm := pdf.NewResourceManager(out)

	tree := pagetree.NewWriter(out)

	return &MultiPage{
		Out:      out,
		RM:       rm,
		Tree:     tree,
		mediaBox: pageSize,
	}, nil
}

func AddMultiPage(out pdf.Putter, pageSize *pdf.Rectangle) (*MultiPage, error) {
	tree := pagetree.NewWriter(out)

	return &MultiPage{
		Out:      out,
		Tree:     tree,
		mediaBox: pageSize,
	}, nil
}

func (doc *MultiPage) Close() error {
	if doc.numOpen != 0 {
		return fmt.Errorf("%d pages still open", doc.numOpen)
	}

	ref, err := doc.Tree.Close()
	if err != nil {
		return err
	}
	doc.Out.GetMeta().Catalog.Pages = ref

	err = doc.RM.Close()
	if err != nil {
		return err
	}

	err = doc.Out.Close()
	if err != nil {
		return err
	}
	if doc.base != nil {
		err = doc.base.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (doc *MultiPage) AddPage() *Page {
	doc.numOpen++

	page := graphics.NewWriter(&bytes.Buffer{}, doc.RM)
	return &Page{
		Writer: page,
		PageDict: pdf.Dict{
			"Type":     pdf.Name("Page"),
			"MediaBox": doc.mediaBox,
		},
		Out:  doc.Out,
		tree: doc.Tree,
		closeFn: func(p *Page) error {
			doc.numOpen--
			return nil
		},
	}
}
