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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pagetree"
)

func CreateSinglePage(fileName string, pageSize *pdf.Rectangle, opt *pdf.WriterOptions) (*Page, error) {
	out, err := pdf.Create(fileName, opt)
	if err != nil {
		return nil, err
	}
	return singlePage(out, pageSize)
}

func WriteSinglePage(w io.Writer, pageSize *pdf.Rectangle, opt *pdf.WriterOptions) (*Page, error) {
	out, err := pdf.NewWriter(w, opt)
	if err != nil {
		return nil, err
	}
	return singlePage(out, pageSize)
}

func singlePage(w *pdf.Writer, pageSize *pdf.Rectangle) (*Page, error) {
	tree := pagetree.NewWriter(w, &pagetree.InheritableAttributes{
		MediaBox: pageSize,
	})

	page := graphics.NewPage(&bytes.Buffer{})

	p := &Page{
		Page:     page,
		PageDict: pdf.Dict{"Type": pdf.Name("Page")},
		Out:      w,
		tree:     tree,
		closeFn:  closePage,
	}
	return p, nil
}

func closePage(p *Page) error {
	ref, err := p.tree.Close()
	if err != nil {
		return err
	}
	p.Out.GetMeta().Catalog.Pages = ref

	return p.Out.Close()
}
