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
	"fmt"
	"io"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

type MultiPage struct {
	Out  *pdf.Writer
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
	tree := pagetree.NewWriter(out, rm)

	return &MultiPage{
		Out:      out,
		RM:       rm,
		Tree:     tree,
		mediaBox: pageSize,
	}, nil
}

func AddMultiPage(out *pdf.Writer, pageSize *pdf.Rectangle) (*MultiPage, error) {
	rm := pdf.NewResourceManager(out)
	tree := pagetree.NewWriter(out, rm)

	return &MultiPage{
		Out:      out,
		RM:       rm,
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

	// Create shared resources between page and builder
	res := &content.Resources{}

	b := builder.New(content.Page, res)
	p := &page.Page{
		MediaBox:  doc.mediaBox,
		Resources: res,
	}
	return &Page{
		Builder: b,
		RM:      doc.RM,
		Page:    p,
		Out:     doc.Out,
		tree:    doc.Tree,
		closeFn: func(pg *Page) error {
			doc.numOpen--
			return nil
		},
	}
}
