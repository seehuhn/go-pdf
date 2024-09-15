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
	"errors"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pagetree"
)

// Page represents a page in a PDF document.
// The contents of the page can be drawn using the [graphics.Writer] methods.
type Page struct {
	// Writer is used to draw the contents of the page.
	*graphics.Writer

	// PageDict is the PDF dictionary for this page.
	// This can be modified by the user.  The values at the time
	// when the page is closed will be written to the PDF file.
	//
	// See section 7.7.3.3. of PDF 32000-1:2008 for a list of
	// possible entries in this dictionary:
	// https://opensource.adobe.com/dc-acrobat-sdk-docs/pdfstandards/PDF32000_2008.pdf#page=85
	PageDict pdf.Dict

	// Out is the PDF file which contains this page.
	// This can be used to embed fonts, images, etc.
	Out *pdf.Writer

	// Ref, if non-nil, is the pdf reference for this page.
	// This can be set by the user, to use a specific reference.
	// If Ref is nil when the page is closed, a new reference will
	// be allocated.
	Ref pdf.Reference

	tree    *pagetree.Writer
	closeFn func(p *Page) error
}

func (p *Page) SetPageSize(paper *pdf.Rectangle) {
	p.PageDict["MediaBox"] = paper
}

func (p *Page) GetPageSize() *pdf.Rectangle {
	paper, _ := p.PageDict["MediaBox"].(*pdf.Rectangle)
	return paper
}

// Close writes the page to the PDF file.
// The page contents can no longer be modified after this call.
func (p *Page) Close() error {
	if p.Writer.Err != nil {
		return p.Writer.Err
	}
	if p.PageDict["MediaBox"] == nil || p.PageDict["MediaBox"] == (*pdf.Rectangle)(nil) {
		return errors.New("page size not set")
	}

	var filters []pdf.Filter
	opt := p.Out.GetOptions()
	if !opt.HasAny(pdf.OptPretty) {
		filters = append(filters, pdf.FilterCompress{})
	}

	contentRef := p.Out.Alloc()
	stream, err := p.Out.OpenStream(contentRef, nil, filters...)
	if err != nil {
		return err
	}
	_, err = io.Copy(stream, p.Writer.Content.(*bytes.Buffer))
	if err != nil {
		return err
	}
	err = stream.Close()
	if err != nil {
		return err
	}
	p.PageDict["Contents"] = contentRef
	p.PageDict["Resources"] = pdf.AsDict(p.Writer.Resources)

	ref := p.Ref
	if ref == 0 {
		ref = p.Out.Alloc()
	}
	err = p.tree.AppendPageRef(ref, p.PageDict)
	if err != nil {
		return err
	}

	err = p.closeFn(p)
	if err != nil {
		return err
	}

	// Disable the page, since it cannot be modified anymore.
	p.Writer.Content = nil
	p.Writer = nil
	return nil
}
