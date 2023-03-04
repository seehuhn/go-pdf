package pages

import (
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// Page is a PDF page.
type Page struct {
	*graphics.Page
	w          *pdf.Writer
	contentRef *pdf.Reference

	tree *Tree
}

// AppendPage creates a new page and appends it to a page tree.
func AppendPage(tree *Tree) (*Page, error) {
	p, err := NewPage(tree.Out)
	if err != nil {
		return nil, err
	}

	p.tree = tree

	return p, nil
}

// NewPage creates a new page without appending it to the page tree.
// Once the page is finished, the page dictionary returned by the [Close]
// method can be used to add the page to the page tree.
func NewPage(w *pdf.Writer) (*Page, error) {
	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if w.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}

	stream, contentRef, err := w.OpenStream(nil, nil, compress)
	if err != nil {
		return nil, err
	}

	return &Page{
		Page:       graphics.NewPage(stream),
		w:          w,
		contentRef: contentRef,
	}, nil
}

// Close must be called after drawing the page is complete.
// Any error that occurred during drawing is returned here.
// If the page was created with AppendPage, the returned page dictionary
// has already been added to the page tree and must not be modified.
func (p *Page) Close() (pdf.Dict, error) {
	if p.Err != nil {
		return nil, p.Err
	}

	err := p.Content.(io.WriteCloser).Close()
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Type":     pdf.Name("Page"),
		"Contents": p.contentRef,
	}
	if p.Resources != nil {
		dict["Resources"] = pdf.AsDict(p.Resources)
	}

	if p.tree != nil {
		_, err = p.tree.AppendPage(dict)
		if err != nil {
			return nil, err
		}
	}

	return dict, nil
}
