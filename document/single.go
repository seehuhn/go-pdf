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
	"io"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pagetree"
)

// TODO(voss): can we merge SinglePage and Page?
type SinglePage struct {
	*graphics.Page
	PageDict pdf.Dict
	Out      *pdf.Writer

	base      io.Writer
	closeBase bool
	pages     *pagetree.Writer
}

func CreateSinglePage(fileName string, width, height float64, opt *pdf.WriterOptions) (*SinglePage, error) {
	fd, err := os.Create(fileName)
	if err != nil {
		return nil, err
	}
	doc, err := WriteSinglePage(fd, width, height, opt)
	if err != nil {
		fd.Close()
		return nil, err
	}
	doc.closeBase = true
	return doc, nil
}

func WriteSinglePage(w io.Writer, width, height float64, opt *pdf.WriterOptions) (*SinglePage, error) {
	out, err := pdf.NewWriter(w, opt)
	if err != nil {
		return nil, err
	}

	tree := pagetree.NewWriter(out, &pagetree.InheritableAttributes{
		MediaBox: &pdf.Rectangle{
			URx: width,
			URy: height,
		},
	})

	page := graphics.NewPage(&bytes.Buffer{})

	return &SinglePage{
		Page: page,
		PageDict: pdf.Dict{
			"Type": pdf.Name("Page"),
		},

		base:  w,
		Out:   out,
		pages: tree,
	}, nil
}

func (doc *SinglePage) Close() error {
	if doc.Page.Err != nil {
		return doc.Page.Err
	}

	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if doc.Out.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}
	contentRef := doc.Out.Alloc()
	stream, err := doc.Out.OpenStream(contentRef, nil, compress)
	if err != nil {
		return err
	}
	_, err = io.Copy(stream, doc.Page.Content.(*bytes.Buffer))
	if err != nil {
		return err
	}
	err = stream.Close()
	if err != nil {
		return err
	}
	doc.PageDict["Contents"] = contentRef

	if doc.Page.Resources != nil {
		doc.PageDict["Resources"] = pdf.AsDict(doc.Page.Resources)
	}
	err = doc.pages.AppendPage(doc.PageDict)
	if err != nil {
		return err
	}

	ref, err := doc.pages.Close()
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
