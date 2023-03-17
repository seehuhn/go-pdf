package simple

import (
	"io"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

type Document struct {
	*graphics.Page
	PageDict pdf.Dict
	Out      *pdf.Writer

	base      io.Writer
	closeBase bool
	pages     *pages.Tree
}

func CreateSinglePage(name string, width, height float64) (*Document, error) {
	fd, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	doc, err := WriteSinglePage(fd, width, height)
	if err != nil {
		fd.Close()
		return nil, err
	}
	doc.closeBase = true
	return doc, nil
}

func WriteSinglePage(w io.Writer, width, height float64) (*Document, error) {
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

	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if out.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}
	stream, contentRef, err := out.OpenStream(nil, nil, compress)
	if err != nil {
		return nil, err
	}
	page := graphics.NewPage(stream)

	return &Document{
		Page: page,
		PageDict: pdf.Dict{
			"Type":     pdf.Name("Page"),
			"Contents": contentRef,
		},

		base:  w,
		Out:   out,
		pages: tree,
	}, nil
}

func (doc *Document) Close() error {
	if doc.Page.Err != nil {
		return doc.Page.Err
	}
	err := doc.Page.Content.(io.Closer).Close()
	if err != nil {
		return err
	}

	if doc.Page.Resources != nil {
		doc.PageDict["Resources"] = pdf.AsDict(doc.Page.Resources)
	}
	_, err = doc.pages.AppendPage(doc.PageDict, nil)
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
