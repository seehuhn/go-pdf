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
	Out      *pdf.Writer

	tree    *pagetree.Writer
	closeFn func(p *Page) error
}

func (p *Page) Close() error {
	if p.Page.Err != nil {
		return p.Page.Err
	}

	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if p.Out.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}
	contentRef := p.Out.Alloc()
	stream, err := p.Out.OpenStream(contentRef, nil, compress)
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
