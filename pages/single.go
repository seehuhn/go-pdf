package pages

import "seehuhn.de/go/pdf"

func SinglePage(w *pdf.Writer, attr *Attributes) (*Page, error) {
	tree := NewPageTree(w, nil)
	contentRef, mediaBox, err := tree.addPageInternal(attr)
	if err != nil {
		return nil, err
	}

	pages, err := tree.Flush()
	if err != nil {
		return nil, err
	}

	w.SetCatalog(pdf.Struct(&pdf.Catalog{
		Pages: pages,
	}))

	return tree.newPage(contentRef, mediaBox)
}
