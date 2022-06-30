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
	"errors"

	"seehuhn.de/go/pdf"
)

const pageTreeWidth = 12

// PageTree represents a PDF page tree.
type PageTree struct {
	w      *pdf.Writer
	attr   *DefaultAttributes
	ranges []*PageRange
}

// NewPageTree allocates a new PageTree object.
// Use .AddPage() to add pages to the tree.
func NewPageTree(w *pdf.Writer, defaults *DefaultAttributes) *PageTree {
	tree := &PageTree{
		w:    w,
		attr: defaults,
	}
	w.OnClose(tree.finish)
	return tree
}

func (tree *PageTree) finish(w *pdf.Writer) error {
	root := &PageRange{
		tree: tree,
		attr: tree.attr,
	}
	for _, r := range tree.ranges {
		node, ref, err := r.finish()
		if err != nil {
			return err
		}
		if node != nil {
			root.dicts = append(root.dicts, node)
			root.refs = append(root.refs, ref)
			root.total += r.total
		}
	}
	if len(root.dicts) == 0 {
		return errors.New("no pages added")
	}

	dict, ref, err := root.finish()
	if err != nil {
		return err
	}
	mergeDefaults(dict, tree.attr)

	rootRef := ref
	// if the tree consists of a single leaf, insert an internal node
	if dict["Type"] == pdf.Name("Page") {
		parent := pdf.Dict{
			"Type":  pdf.Name("Pages"),
			"Kids":  pdf.Array{ref},
			"Count": pdf.Integer(1),
		}
		rootRef, err = w.Write(parent, nil)
		if err != nil {
			return err
		}
		dict["Parent"] = rootRef
	}

	_, err = w.Write(dict, ref)
	if err != nil {
		return err
	}

	w.Catalog.Pages = rootRef

	return nil
}

// NewPage adds a new page to the page tree and returns an object which
// can be used to write the content stream for the page.  The new page
// is appended at the end of the file.
func (tree *PageTree) NewPage(attr *Attributes) (*Page, error) {
	pp := tree.defaultPageRange()
	return pp.NewPage(attr)
}

func (tree *PageTree) defaultPageRange() *PageRange {
	if len(tree.ranges) == 0 {
		return tree.NewPageRange(tree.attr)
	}
	return tree.ranges[len(tree.ranges)-1]
}

// NewPageRange creates a new page range, which can be used to later add
// pages inside the PDF file.
func (tree *PageTree) NewPageRange(attr *DefaultAttributes) *PageRange {
	r := &PageRange{
		tree: tree,
		attr: attr,
	}
	tree.ranges = append(tree.ranges, r)
	return r
}

// A PageRange represents a consecutive range of pages in a PDF file.
type PageRange struct {
	tree   *PageTree
	attr   *DefaultAttributes
	total  int
	inPage bool

	// dicts contains /Page or /Pages objects which are complete
	// except for the /Parent field.
	// New pages are appended at the end.
	// Refs contains references to the /Page or /Pages objects.
	dicts []pdf.Dict
	refs  []*pdf.Reference

	// TODO(voss): add page labels here, see section 12.4.2
}

// Append adds a new page to the page range.
// This function is for special cases, where the caller constructs the page
// dictionary manually.
// Most callers should use .AddPage() instead.
func (pp *PageRange) Append(page pdf.Dict) error {
	if page["Type"] != pdf.Name("Page") {
		return errors.New("page must be of type /Page")
	}

	pp.dicts = append(pp.dicts, page)
	pp.refs = append(pp.refs, pp.tree.w.Alloc())
	pp.total++

	total := pp.total
	for total%pageTreeWidth == 0 {
		total /= pageTreeWidth

		k := len(pp.dicts) - pageTreeWidth
		err := pp.merge(k)
		if err != nil {
			return err
		}
	}
	return nil
}

func (pp *PageRange) finish() (pdf.Dict, *pdf.Reference, error) {
	if pp.inPage {
		return nil, nil, errors.New("page not closed")
	}

	if len(pp.dicts) == 0 {
		return nil, nil, nil
	}

	for len(pp.dicts) > 1 {
		k := len(pp.dicts) - pageTreeWidth
		if k < 0 {
			k = 0
		}
		err := pp.merge(k)
		if err != nil {
			return nil, nil, err
		}
	}

	res := pp.dicts[0]
	mergeDefaults(res, pp.attr)
	return res, pp.refs[0], nil
}

// Merge writes the child nodes k, k+1, ... to the PDF file, replacing them
// with a \Pages node.  Nodes 0, 1, ..., k-1 are kept unchanged.
// On return, the list of children has k+1 elements.
func (pp *PageRange) merge(k int) error {
	kids := pp.dicts[k:]
	kidsRefs := pp.refs[k:]

	parentRef := pp.tree.w.Alloc()
	objs := make([]pdf.Object, len(kids))
	var kidsTotal pdf.Integer
	for i, d := range kids {
		d["Parent"] = parentRef
		objs[i] = d

		var count pdf.Integer = 1
		if d["Type"] == pdf.Name("Pages") {
			count = d["Count"].(pdf.Integer)
		}
		kidsTotal += count
	}
	_, err := pp.tree.w.WriteCompressed(kidsRefs, objs...)
	if err != nil {
		return err
	}

	kidsArray := make(pdf.Array, len(kids))
	for i, ref := range kidsRefs {
		kidsArray[i] = ref
	}
	parent := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Kids":  kidsArray,
		"Count": kidsTotal,
	}
	pp.dicts = append(pp.dicts[:k], parent)
	pp.refs = append(pp.refs[:k], parentRef)
	return nil
}

func mergeDefaults(node pdf.Dict, attr *DefaultAttributes) {
	if attr == nil {
		return
	}
	if _, present := node["Resources"]; !present && attr.Resources != nil {
		node["Resources"] = pdf.AsDict(attr.Resources)
	}
	if _, present := node["MediaBox"]; !present && attr.MediaBox != nil {
		node["MediaBox"] = attr.MediaBox
	}
	if _, present := node["CropBox"]; !present && attr.CropBox != nil {
		node["CropBox"] = attr.CropBox
	}
	if _, present := node["Rotate"]; !present && attr.Rotate != 0 {
		node["Rotate"] = pdf.Integer(attr.Rotate)
	}
}
