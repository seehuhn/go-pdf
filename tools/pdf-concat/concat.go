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
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/outline"
	"seehuhn.de/go/pdf/pagetree"
)

// Concat represents a PDF file made by concatenating other PDF files.
type Concat struct {
	v  pdf.Version
	w  *pdf.Writer
	rm *pdf.ResourceManager

	pages    *pagetree.Writer
	numPages int

	children []*Child
}

// Child represents a child document in the concatenated file.
type Child struct {
	Title     string
	FirstPage pdf.Reference
	Outline   []*outline.Item
}

// NewConcat creates a new Concat object.
func NewConcat(out string, v pdf.Version) (*Concat, error) {
	w, err := pdf.Create(out, v, nil)
	if err != nil {
		return nil, err
	}

	rm := pdf.NewResourceManager(w)
	pages := pagetree.NewWriter(w, rm)

	c := &Concat{
		v:     v,
		w:     w,
		rm:    rm,
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

	outlineTree := &outline.Outline{}
	for _, child := range c.children {
		entry := outlineTree.AddItem(child.Title)
		entry.Destination = &destination.Fit{Page: child.FirstPage}
		entry.Children = child.Outline
	}
	err = outlineTree.Write(c.rm)
	if err != nil {
		return err
	}

	err = c.rm.Close()
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

	copy := pdf.NewCopier(c.w, r)

	meta := r.GetMeta()
	outlineTree, _ := outline.Read(r)

	var title string
	if meta.Info != nil && meta.Info.Title != "" {
		title = string(meta.Info.Title)
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
		err = c.pages.AppendPageDict(newRef, newDict)
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
		items, err := c.CopyOutlineItems(copy, outlineTree.Items)
		if err != nil {
			return err
		}
		child.Outline = items
	}

	c.children = append(c.children, child)

	return nil
}

// CopyOutlineItems copies outline items from the source file to the target file.
func (c *Concat) CopyOutlineItems(cp *pdf.Copier, in []*outline.Item) ([]*outline.Item, error) {
	out := make([]*outline.Item, len(in))
	for i, child := range in {
		cc, err := c.CopyOutlineItems(cp, child.Children)
		if err != nil {
			return nil, err
		}

		entry := &outline.Item{
			Title:    child.Title,
			Children: cc,
			Open:     child.Open,
			Color:    child.Color,
			Bold:     child.Bold,
			Italic:   child.Italic,
		}

		// Copy destination with page reference translation
		if child.Destination != nil {
			entry.Destination, err = copyDestination(cp, child.Destination)
			if err != nil {
				return nil, err
			}
		}

		// Copy action with page reference translation (for GoTo actions)
		if child.Action != nil {
			entry.Action, err = copyAction(cp, child.Action)
			if err != nil {
				return nil, err
			}
		}

		out[i] = entry
	}
	return out, nil
}

// copyDestination copies a destination, translating page references.
func copyDestination(cp *pdf.Copier, dest destination.Destination) (destination.Destination, error) {
	switch d := dest.(type) {
	case *destination.XYZ:
		newPage, err := copyPageTarget(cp, d.Page)
		if err != nil {
			return nil, err
		}
		return &destination.XYZ{Page: newPage, Left: d.Left, Top: d.Top, Zoom: d.Zoom}, nil
	case *destination.Fit:
		newPage, err := copyPageTarget(cp, d.Page)
		if err != nil {
			return nil, err
		}
		return &destination.Fit{Page: newPage}, nil
	case *destination.FitH:
		newPage, err := copyPageTarget(cp, d.Page)
		if err != nil {
			return nil, err
		}
		return &destination.FitH{Page: newPage, Top: d.Top}, nil
	case *destination.FitV:
		newPage, err := copyPageTarget(cp, d.Page)
		if err != nil {
			return nil, err
		}
		return &destination.FitV{Page: newPage, Left: d.Left}, nil
	case *destination.FitR:
		newPage, err := copyPageTarget(cp, d.Page)
		if err != nil {
			return nil, err
		}
		return &destination.FitR{Page: newPage, Left: d.Left, Bottom: d.Bottom, Right: d.Right, Top: d.Top}, nil
	case *destination.FitB:
		newPage, err := copyPageTarget(cp, d.Page)
		if err != nil {
			return nil, err
		}
		return &destination.FitB{Page: newPage}, nil
	case *destination.FitBH:
		newPage, err := copyPageTarget(cp, d.Page)
		if err != nil {
			return nil, err
		}
		return &destination.FitBH{Page: newPage, Top: d.Top}, nil
	case *destination.FitBV:
		newPage, err := copyPageTarget(cp, d.Page)
		if err != nil {
			return nil, err
		}
		return &destination.FitBV{Page: newPage, Left: d.Left}, nil
	case *destination.Named:
		// Named destinations don't need translation
		return &destination.Named{Name: d.Name}, nil
	default:
		return nil, nil
	}
}

// copyPageTarget copies a page target (reference or integer).
func copyPageTarget(cp *pdf.Copier, page destination.Target) (destination.Target, error) {
	if ref, ok := page.(pdf.Reference); ok {
		newRef, err := cp.CopyReference(ref)
		if err != nil {
			return nil, err
		}
		return newRef, nil
	}
	// Page numbers don't need translation
	return page, nil
}

// copyAction copies an action, translating page references in GoTo actions.
func copyAction(cp *pdf.Copier, act pdf.Action) (pdf.Action, error) {
	switch a := act.(type) {
	case *action.GoTo:
		newDest, err := copyDestination(cp, a.Dest)
		if err != nil {
			return nil, err
		}
		return &action.GoTo{Dest: newDest}, nil
	default:
		// For other action types, just return the same action
		// (they may contain references that need copying, but that's complex)
		return act, nil
	}
}
