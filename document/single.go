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
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

// CreateSinglePage creates a new PDF document consisting of a single page.
// The document is written to the file with the given name.
func CreateSinglePage(fileName string, pageSize *pdf.Rectangle, v pdf.Version, opt *pdf.WriterOptions) (*Page, error) {
	out, err := pdf.Create(fileName, v, opt)
	if err != nil {
		return nil, err
	}
	return singlePage(out, pageSize)
}

// WriteSinglePage creates a new PDF document consisting of a single page.
// The document is written to the given writer.
func WriteSinglePage(w io.Writer, pageSize *pdf.Rectangle, v pdf.Version, opt *pdf.WriterOptions) (*Page, error) {
	out, err := pdf.NewWriter(w, v, opt)
	if err != nil {
		return nil, err
	}
	return singlePage(out, pageSize)
}

func singlePage(w *pdf.Writer, pageSize *pdf.Rectangle) (*Page, error) {
	rm := pdf.NewResourceManager(w)
	tree := pagetree.NewWriter(w, rm)

	// Create shared resources between page and builder
	res := &content.Resources{}

	b := builder.New(content.Page, res)
	pg := &page.Page{
		MediaBox:  pageSize,
		Resources: res,
	}

	p := &Page{
		Builder: b,
		RM:      rm,
		Page:    pg,
		Out:     w,
		tree:    tree,
		closeFn: closePage,
	}
	return p, nil
}

func closePage(p *Page) error {
	ref, err := p.tree.Close()
	if err != nil {
		return err
	}
	p.Out.GetMeta().Catalog.Pages = ref

	err = p.RM.Close()
	if err != nil {
		return err
	}

	return p.Out.Close()
}
