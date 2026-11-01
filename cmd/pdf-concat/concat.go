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
	"fmt"
	"os"
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

	// merged interactive form
	forms               []*formInput             // the inputs' forms, in order
	usedNames           map[string]bool          // root field partial names already taken
	dr                  map[pdf.Name]*drCategory // union of the inputs' /DR, by resource category
	formNeedAppearances bool                     // OR of the inputs' NeedAppearances
	formSigFlags        pdf.Integer              // OR of the inputs' SigFlags
}

// Child represents a child document in the concatenated file.
type Child struct {
	Title     pdf.TextString
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
		Producer:     pdf.Text("seehuhn.de/go/pdf/cmd/pdf-concat"),
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
	outlineRef, err := c.rm.Store(outlineTree)
	if err != nil {
		return err
	}
	c.rm.Out.GetMeta().Catalog.Outlines = outlineRef

	// merged interactive form
	acro, err := c.writeForm()
	if err != nil {
		return err
	}
	if acro != nil {
		acroRef := c.w.Alloc()
		if err := c.w.Put(acroRef, acro); err != nil {
			return err
		}
		meta.Catalog.AcroForm = acroRef
	}

	err = c.rm.Close()
	if err != nil {
		return err
	}

	return c.w.Close()
}

// Append appends a PDF file to the output.
func (c *Concat) Append(fname string) error {
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()
	fi, err := fd.Stat()
	if err != nil {
		return err
	}
	r, err := pdf.NewReader(fd, fi.Size(), nil)
	if err != nil {
		return pdf.Wrap(err, fname)
	}
	defer r.Close()

	// /DA strings are rewritten as they are copied, once prepareForm has
	// found the renamed resources
	g := newDARenamer(r, c)
	copy := pdf.NewCopier(c.w, g)

	// Set up the form's root fields before copying pages, so that widgets copied
	// as part of a page have their /Parent redirected to the renamed field.
	prepared, form, err := c.prepareForm(r, g, copy, fingerprintBudget*fi.Size())
	if err != nil {
		return err
	}

	meta := r.GetMeta()
	outlineTree, err := pdf.NewCursor(r).DecodeOptional(meta.Catalog.Outlines, outline.Decode)
	if err != nil {
		return err
	}

	var title pdf.TextString
	if meta.Info != nil && !meta.Info.Title.IsZero() {
		title = meta.Info.Title
	} else {
		title = pdf.Text(fname)
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

		// the page dictionary is read without g, so a direct annotation
		// in its /Annots array is patched here
		dict, _, err := g.patchDict(dict)
		if err != nil {
			copyError = err
			break
		}
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

	// copy the (renamed) root field dictionaries now that their widgets,
	// copied with the pages, exist and reference the redirected field objects
	if form != nil {
		if err := c.finishForm(g, copy, prepared, form); err != nil {
			return err
		}
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

// uniqueName reserves a partial field name for a merged root field, returning ""
// if the original name is free (keep it) or a fresh, non-colliding name to use
// instead. Anonymous fields (no name) never collide.
func (c *Concat) uniqueName(name string) string {
	if name == "" {
		return ""
	}
	if !c.usedNames[name] {
		c.usedNames[name] = true
		return ""
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s_%d", name, i)
		if !c.usedNames[cand] {
			c.usedNames[cand] = true
			return cand
		}
	}
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

		// copy destination with page reference translation
		if child.Destination != nil {
			entry.Destination, err = copyDestination(cp, c.rm, child.Destination)
			if err != nil {
				return nil, err
			}
		}

		// copy action with page reference translation
		if child.Action != nil {
			entry.Action, err = copyAction(cp, c.rm, child.Action)
			if err != nil {
				return nil, err
			}
		}

		out[i] = entry
	}
	return out, nil
}

// copyDestination copies a destination, translating page references.
func copyDestination(cp *pdf.Copier, rm *pdf.ResourceManager,
	dest destination.Destination) (destination.Destination, error) {
	if _, ok := dest.(*destination.Named); ok {
		return dest, nil
	}
	encoded, err := dest.Encode(rm)
	if err != nil {
		return nil, err
	}
	copied, err := cp.Copy(encoded)
	if err != nil {
		return nil, err
	}
	return pdf.NewCursor(rm.Out).Decode(copied, destination.Decode)
}

// copyAction copies an action, translating page references.
func copyAction(cp *pdf.Copier, rm *pdf.ResourceManager,
	act pdf.Action) (pdf.Action, error) {
	encoded, err := act.Encode(rm)
	if err != nil {
		return nil, err
	}
	copied, err := cp.Copy(encoded)
	if err != nil {
		return nil, err
	}
	return pdf.NewCursor(rm.Out).Decode(copied, action.Decode)
}
