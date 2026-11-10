// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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

	b := builder.New(content.Page, res, pdf.GetVersion(w))
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
