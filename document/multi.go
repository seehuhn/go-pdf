package document

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

type MultiPage struct {
	*pages.Tree
	Out *pdf.Writer

	numOpen   int
	base      io.Writer
	closeBase bool
}

func CreateMultiPage(name string, width, height float64) (*MultiPage, error) {
	fd, err := os.Create(name)
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

	tree := pages.NewTree(out, &pages.InheritableAttributes{
		MediaBox: &pdf.Rectangle{
			URx: width,
			URy: height,
		},
	})

	return &MultiPage{
		base: w,
		Out:  out,
		Tree: tree,
	}, nil
}

func (doc *MultiPage) Close() error {
	if doc.numOpen != 0 {
		return fmt.Errorf("%d pages still open", doc.numOpen)
	}

	ref, err := doc.Tree.Close()
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
	stream, contentRef, err := p.doc.Out.OpenStream(nil, nil, compress)
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

	_, err = p.doc.Tree.AppendPage(p.PageDict, nil)
	if err != nil {
		return err
	}

	return nil
}
