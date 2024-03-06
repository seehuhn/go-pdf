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

func CreateSinglePage(fileName string, pageSize *pdf.Rectangle, v pdf.Version, opt *pdf.WriterOptions) (*Page, error) {
	out, err := pdf.Create(fileName, v, opt)
	if err != nil {
		return nil, err
	}
	return singlePage(out, pageSize)
}

func WriteSinglePage(w io.Writer, pageSize *pdf.Rectangle, v pdf.Version, opt *pdf.WriterOptions) (*Page, error) {
	out, err := pdf.NewWriter(w, v, opt)
	if err != nil {
		return nil, err
	}
	return singlePage(out, pageSize)
}

func singlePage(w pdf.Putter, pageSize *pdf.Rectangle) (*Page, error) {
	tree := pagetree.NewWriter(w)

	page := graphics.NewWriter(&bytes.Buffer{}, pdf.GetVersion(w))

	pageDict := pdf.Dict{
		"Type": pdf.Name("Page"),
	}
	if pageSize != nil {
		pageDict["MediaBox"] = pageSize
	}

	p := &Page{
		Writer:   page,
		PageDict: pageDict,
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
