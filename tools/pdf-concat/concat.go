// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package main

import (
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/outline"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/pdfcopy"
)

// Concat represents a PDF file made by concatenating other PDF files.
type Concat struct {
	v pdf.Version
	w *pdf.Writer

	pages    *pagetree.Writer
	numPages int

	children []*Child
}

// Child represents a child document in the concatenated file.
type Child struct {
	Title     string
	FirstPage pdf.Reference
	Outline   []*outline.Tree
}

// NewConcat creates a new Concat object.
func NewConcat(out string, v pdf.Version) (*Concat, error) {
	w, err := pdf.Create(out, v, nil)
	if err != nil {
		return nil, err
	}

	pages := pagetree.NewWriter(w)

	c := &Concat{
		v:     v,
		w:     w,
		pages: pages,
	}

	return c, nil
}

// Close closes the output file.
func (c *Concat) Close() error {
	pagesRef, err := c.pages.Close()
	if err != nil {
		return err
	}

	meta := c.w.GetMeta()
	now := time.Now()
	meta.Info = &pdf.Info{
		Producer:     "seehuhn.de/go/pdf/examples/concat",
		CreationDate: pdf.Date(now),
		ModDate:      pdf.Date(now),
	}
	meta.Catalog.Pages = pagesRef

	outline := &outline.Tree{}
	for _, child := range c.children {
		entry := outline.AddChild(child.Title)
		entry.Action = pdf.Dict{
			"S": pdf.Name("GoTo"),
			"D": pdf.Array{child.FirstPage, pdf.Name("Fit")},
		}
		entry.Children = child.Outline
	}
	err = outline.Write(c.w)
	if err != nil {
		return err
	}

	return c.w.Close()
}

// Append appends a PDF file to the output.
func (c *Concat) Append(fname string) error {
	r, err := pdf.Open(fname, nil)
	if err != nil {
		return err
	}
	defer r.Close()

	copy := pdfcopy.NewCopier(c.w, r)

	meta := r.GetMeta()
	outlineTree, _ := outline.Read(r)

	var title string
	if meta.Info != nil && meta.Info.Title != "" {
		title = string(meta.Info.Title)
	} else if outlineTree != nil && outlineTree.Title != "" {
		title = outlineTree.Title
	} else {
		title = fname
	}

	child := &Child{
		Title: title,
	}

	var copyError error
	for oldRef, dict := range pagetree.NewIterator(r).All() {
		newRef := c.w.Alloc()

		// Since we rebuild the page tree, we can't use `copy` to copy the page
		// dictionary.  Instead, we manually install a redirect from the newly
		// constructed page dict to the old one.
		copy.Redirect(oldRef, newRef)

		newDict, err := copy.CopyDict(dict)
		if err != nil {
			copyError = err
			break
		}
		err = c.pages.AppendPageRef(newRef, newDict)
		if err != nil {
			copyError = err
			break
		}

		if child.FirstPage == 0 {
			child.FirstPage = newRef
		}

		c.numPages++
	}
	if copyError != nil {
		return copyError
	}

	if outlineTree != nil {
		outline, err := c.CopyOutline(copy, outlineTree.Children)
		if err != nil {
			return err
		}
		child.Outline = outline
	}

	c.children = append(c.children, child)

	return nil
}

// CopyOutline copies an outline tree from the source file to the target file.
func (c *Concat) CopyOutline(copy *pdfcopy.Copier, in []*outline.Tree) ([]*outline.Tree, error) {
	out := make([]*outline.Tree, len(in))
	for i, child := range in {
		cc, err := c.CopyOutline(copy, child.Children)
		if err != nil {
			return nil, err
		}

		action, err := copy.CopyDict(child.Action)
		if err != nil {
			return nil, err
		}

		out[i] = &outline.Tree{
			Title:    child.Title,
			Children: cc,
			Open:     child.Open,
			Action:   action,
		}
	}
	return out, nil
}
