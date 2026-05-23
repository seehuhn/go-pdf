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
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

// Page represents a page in a PDF document.
// The contents of the page can be drawn using the [builder.Builder] methods.
type Page struct {
	// Builder is used to draw the contents of the page.
	*builder.Builder

	// RM is the resource manager for embedding resources.
	RM *pdf.ResourceManager

	// Page is the typed page object.
	// This can be modified by the user. The values at the time
	// when the page is closed will be written to the PDF file.
	Page *page.Page

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

	// appended holds extra content-stream segments to be emitted after
	// the builder's output when the page is closed.  Populated by
	// [Page.AppendContent]; each entry becomes a separate stream object
	// in the page's /Contents array.
	appended []page.Segment
}

func (p *Page) SetPageSize(paper *pdf.Rectangle) {
	p.Page.MediaBox = paper
}

func (p *Page) GetPageSize() *pdf.Rectangle {
	return p.Page.MediaBox
}

// AppendContent queues s to be emitted as an additional content-stream
// segment after the builder's output when this page is closed.  The same
// [page.Segment] value can be appended to many pages to share one PDF
// stream object across them (pointer-identity deduplication); this is
// useful for headers, footers, watermarks, and similar shared boilerplate.
//
// Each call to AppendContent adds one entry; segments appear in the order
// they were appended.
func (p *Page) AppendContent(s page.Segment) {
	p.appended = append(p.appended, s)
}

// Close writes the page to the PDF file.
// The page contents can no longer be modified after this call.
func (p *Page) Close() error {
	if p.Builder == nil {
		return errors.New("page already closed")
	}
	if p.Builder.Err != nil {
		return p.Builder.Err
	}
	if p.Page.MediaBox == nil {
		return errors.New("page size not set")
	}

	// Compose the page's content: builder output first, then any segments
	// queued via AppendContent.  The builder's operator slice is wrapped
	// in a [*content.Operators] so the segment also acts as a [pdf.Embedder]
	// (for pointer-identity deduplication when shared across pages).
	contents := make([]page.Segment, 0, 1+len(p.appended))
	contents = append(contents, &content.Operators{Ops: p.Builder.Stream})
	contents = append(contents, p.appended...)
	p.Page.Contents = contents

	ref := p.Ref
	if ref == 0 {
		ref = p.Out.Alloc()
	}
	err := p.tree.AppendPageRef(ref, p.Page)
	if err != nil {
		return err
	}

	err = p.closeFn(p)
	if err != nil {
		return err
	}

	// Disable the builder, but keep p.Page accessible for inspection.
	p.Builder = nil
	return nil
}
