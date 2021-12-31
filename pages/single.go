// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pages

import (
	"seehuhn.de/go/pdf"
)

// SinglePage sets up w to be a simple single-page document.
// The returned page object can be used to draw the page contents.
//
// The page object is closed automatically when w.Close() is called.
// This function calls w.SetCatalog(), so SetCatalog must not be
// called manually when SinglePage is used.
func SinglePage(w *pdf.Writer, attr *Attributes) (*Page, error) {
	tree := NewPageTree(w, nil)
	contentRef, mediaBox, err := tree.addPageInternal(attr)
	if err != nil {
		return nil, err
	}

	pages, err := tree.Finish()
	if err != nil {
		return nil, err
	}

	page, err := tree.newPage(contentRef, mediaBox)
	if err != nil {
		return nil, err
	}

	w.SetCatalog(&pdf.Catalog{
		Pages: pages,
	})
	w.OnClose(func(_ *pdf.Writer) error { return page.Close() })

	return page, nil
}
