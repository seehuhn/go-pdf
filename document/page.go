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

type Page struct {
	*graphics.Page
	PageDict pdf.Dict
	Out      pdf.Putter

	tree    *pagetree.Writer
	closeFn func(p *Page) error
}

func (p *Page) Close() error {
	if p.Page.Err != nil {
		return p.Page.Err
	}

	contentRef := p.Out.Alloc()
	stream, err := p.Out.OpenStream(contentRef, nil, pdf.FilterCompress{})
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

	err = p.tree.AppendPage(p.PageDict)
	if err != nil {
		return err
	}

	return p.closeFn(p)
}
