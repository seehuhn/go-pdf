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
	*pagetree.Writer
	Out *pdf.Writer

	numOpen   int
	base      io.Writer
	closeBase bool
}

func CreateMultiPage(fileName string, width, height float64) (*MultiPage, error) {
	fd, err := os.Create(fileName)
	if err != nil {
		return nil, err
	}
	doc, err := WriteMultiPage(fd, width, height)
	if err != nil {
		fd.Close()
		return nil, err
	}
	doc.closeBase = true
	return doc, nil
}

func WriteMultiPage(w io.Writer, width, height float64) (*MultiPage, error) {
	out, err := pdf.NewWriter(w, nil)
	if err != nil {
		return nil, err
	}

	tree := pagetree.NewWriter(out, &pagetree.InheritableAttributes{
		MediaBox: &pdf.Rectangle{
			URx: width,
			URy: height,
		},
	})

	return &MultiPage{
		base:   w,
		Out:    out,
		Writer: tree,
	}, nil
}

func (doc *MultiPage) Close() error {
	if doc.numOpen != 0 {
		return fmt.Errorf("%d pages still open", doc.numOpen)
	}

	ref, err := doc.Writer.Close()
	if err != nil {
		return err
	}
	doc.Out.Catalog.Pages = ref

	err = doc.Out.Close()
	if err != nil {
		return err
	}
	if doc.closeBase {
		err = doc.base.(io.Closer).Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (doc *MultiPage) AddPage() *Page {
	doc.numOpen++

	page := graphics.NewPage(&bytes.Buffer{})
	return &Page{
		Page: page,
		PageDict: pdf.Dict{
			"Type": pdf.Name("Page"),
		},
		doc: doc,
	}
}

type Page struct {
	*graphics.Page
	PageDict pdf.Dict

	doc *MultiPage
}

func (p *Page) Close() error {
	if p.Page.Err != nil {
		return p.Page.Err
	}

	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if p.doc.Out.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}
	contentRef := p.doc.Out.Alloc()
	stream, err := p.doc.Out.OpenStream(contentRef, nil, compress)
	if err != nil {
		return err
	}
	_, err = io.Copy(stream, p.Page.Content.(*bytes.Buffer))
	if err != nil {
		return err
	}
	err = stream.Close()
	if err != nil {
		return err
	}
	p.PageDict["Contents"] = contentRef

	if p.Page.Resources != nil {
		p.PageDict["Resources"] = pdf.AsDict(p.Page.Resources)
	}

	// Disable the page, since it has been written out and cannot be modified
	// anymore.
	p.Page.Content = nil
	p.Page = nil
	p.doc.numOpen--

	err = p.doc.Writer.AppendPage(p.PageDict)
	if err != nil {
		return err
	}

	return nil
}
