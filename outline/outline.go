// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package outline

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 12.3.3

// Outline represents the root of a document outline.
// Use [Read] to read an outline from a PDF file, or create a new outline
// and populate it using [Outline.AddItem].
type Outline struct {
	// Items contains the top-level outline items.
	Items []*Item
}

// Item represents an outline item, with a title and a destination or action.
// This is used both for leaves and for internal nodes in the outline tree (apart from the root).
// Items form a tree structure via the Children field.
type Item struct {
	// Title is the text displayed for this outline item.
	Title string

	// Color (PDF 1.4) specifies the color for the outline entry's text.
	// Components must be in the range 0.0 to 1.0.
	Color color.DeviceRGB

	// Bold (PDF 1.4) displays the item in bold.
	Bold bool

	// Italic (PDF 1.4) displays the item in italic.
	Italic bool

	// Destination (optional) specifies the view to show when the outline item is activated.
	// Use for navigation within the same document.
	// Destination and Action are mutually exclusive.
	Destination destination.Destination

	// Action (optional; PDF 1.1) is performed when the outline item is activated.
	// Use for external links, other PDF files, or special actions.
	// Destination and Action are mutually exclusive.
	Action pdf.Action

	// Children contains the child outline items (e.g. subsections of a section).
	Children []*Item

	// Open indicates whether the item is initially expanded (children visible)
	// or collapsed when the document is opened.
	Open bool

	// StructEntry (optional, PDF 1.3) is an indirect reference to the structure
	// element associated with this item.
	StructEntry pdf.Reference
}

// AddItem appends a new top-level item with the given title and returns it.
func (o *Outline) AddItem(title string) *Item {
	item := &Item{
		Title: title,
	}
	o.Items = append(o.Items, item)
	return item
}

// AddChild appends a new child item with the given title and returns it.
func (item *Item) AddChild(title string) *Item {
	child := &Item{
		Title: title,
	}
	item.Children = append(item.Children, child)
	return child
}

// Read reads the document outline from a PDF file.
// Returns nil if the document has no outline.
func Read(r pdf.Getter) (*Outline, error) {
	rootRef := r.GetMeta().Catalog.Outlines
	if rootRef == 0 {
		return nil, nil
	}

	seen := map[pdf.Reference]bool{}
	seen[rootRef] = true

	rootDict, err := pdf.GetDictTyped(r, rootRef, "Outlines")
	if err != nil {
		return nil, err
	}

	firstRef, _ := rootDict["First"].(pdf.Reference)
	items, err := readChildren(r, seen, firstRef)
	if err != nil {
		return nil, err
	}

	return &Outline{Items: items}, nil
}

func readItem(r pdf.Getter, seen map[pdf.Reference]bool, ref pdf.Reference) (*Item, pdf.Dict, error) {
	if seen[ref] {
		return nil, nil, pdf.Errorf("outline tree contains a loop")
	}
	seen[ref] = true
	if len(seen) > 65536 {
		return nil, nil, errors.New("outline too large")
	}

	dict, err := pdf.GetDict(r, ref)
	if err != nil {
		return nil, nil, err
	}

	item := &Item{}

	title, err := pdf.GetTextString(r, dict["Title"])
	if err != nil {
		return nil, nil, pdf.Wrap(err, "/Title in outline")
	}
	item.Title = string(title)

	count, _ := pdf.GetInteger(r, dict["Count"])
	item.Open = count > 0

	v := pdf.GetVersion(r)

	x := pdf.NewExtractor(r)
	if dict["Dest"] != nil {
		dest, err := destination.Decode(x, dict["Dest"])
		if err != nil {
			return nil, nil, pdf.Wrap(err, "/Dest in outline")
		}
		item.Destination = dest
	} else if dict["A"] != nil && v >= pdf.V1_1 {
		a, err := pdf.Optional(action.Decode(x, dict["A"]))
		if err != nil {
			return nil, nil, pdf.Wrap(err, "/A in outline")
		}
		item.Action = a
	}

	if v >= pdf.V1_4 {
		if cArr, _ := pdf.GetArray(r, dict["C"]); len(cArr) == 3 {
			cr, _ := pdf.GetNumber(r, cArr[0])
			cg, _ := pdf.GetNumber(r, cArr[1])
			cb, _ := pdf.GetNumber(r, cArr[2])
			item.Color = color.DeviceRGB{float64(cr), float64(cg), float64(cb)}
		}

		if f, _ := pdf.GetInteger(r, dict["F"]); f != 0 {
			item.Italic = f&1 != 0
			item.Bold = f&2 != 0
		}
	}

	if se, ok := dict["SE"].(pdf.Reference); ok && v >= pdf.V1_3 {
		item.StructEntry = se
	}

	firstRef, _ := dict["First"].(pdf.Reference)
	children, err := readChildren(r, seen, firstRef)
	if err != nil {
		return nil, nil, err
	}
	item.Children = children

	return item, dict, nil
}

func readChildren(r pdf.Getter, seen map[pdf.Reference]bool, ref pdf.Reference) ([]*Item, error) {
	var res []*Item
	for ref != 0 {
		item, dict, err := readItem(r, seen, ref)
		if err != nil {
			return nil, err
		}

		res = append(res, item)

		ref, _ = dict["Next"].(pdf.Reference)
	}
	return res, nil
}

// Write writes the outline to the PDF file and installs it in the catalog.
func (o *Outline) Write(rm *pdf.ResourceManager) error {
	if o == nil || len(o.Items) == 0 {
		return nil
	}

	ww := &writer{
		rm:    rm,
		count: map[*Item]int{},
	}

	// compute counts for all items
	var rootCount int
	for _, item := range o.Items {
		rootCount += ww.getCount(item)
	}

	rootRef := rm.Out.Alloc()
	rootDict := pdf.Dict{}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		rootDict["Type"] = pdf.Name("Outlines")
	}

	first := rm.Out.Alloc()
	var last pdf.Reference
	if len(o.Items) > 1 {
		last = rm.Out.Alloc()
	} else {
		last = first
	}
	rootDict["First"] = first
	rootDict["Last"] = last
	if ww.hasOpen {
		rootDict["Count"] = pdf.Integer(rootCount)
	}

	ww.objs = append(ww.objs, rootDict)
	ww.refs = append(ww.refs, rootRef)

	err := ww.writeChildren(rootRef, first, last, o.Items)
	if err != nil {
		return err
	}

	err = ww.flush()
	if err != nil {
		return err
	}

	rm.Out.GetMeta().Catalog.Outlines = rootRef
	return nil
}

type writer struct {
	rm      *pdf.ResourceManager
	refs    []pdf.Reference
	objs    []pdf.Object
	count   map[*Item]int
	hasOpen bool
}

// getCount computes the Count value for an item.
// Returns positive count if item is open, negative if closed.
func (ww *writer) getCount(item *Item) int {
	if item == nil || len(item.Children) == 0 {
		return 1
	}

	// count this item plus all descendants
	total := 1
	for _, child := range item.Children {
		childCount := ww.getCount(child)
		if childCount > 0 {
			total += childCount
		} else {
			total++ // closed child counts as 1
		}
	}

	// store count for this item (number of descendants)
	descendantCount := total - 1
	if item.Open {
		ww.hasOpen = true
		ww.count[item] = descendantCount
	} else if descendantCount > 0 {
		ww.count[item] = -descendantCount
	}

	// return value for parent's calculation
	if item.Open {
		return total
	}
	return 1
}

func (ww *writer) writeItem(ref pdf.Reference, dict pdf.Dict, item *Item) error {
	if item.Destination != nil && item.Action != nil {
		return errors.New("outline item has both Dest and Action")
	}

	dict["Title"] = pdf.TextString(item.Title)

	if item.Color != (color.DeviceRGB{}) {
		if err := pdf.CheckVersion(ww.rm.Out, "outline item color", pdf.V1_4); err != nil {
			return err
		}
		for i, c := range item.Color {
			if c < 0 || c > 1 {
				return fmt.Errorf("outline item color component %d out of range: %g", i, c)
			}
		}
		dict["C"] = pdf.Array{
			pdf.Number(item.Color[0]),
			pdf.Number(item.Color[1]),
			pdf.Number(item.Color[2]),
		}
	}

	var flags int
	if item.Italic {
		flags |= 1
	}
	if item.Bold {
		flags |= 2
	}
	if flags != 0 {
		if err := pdf.CheckVersion(ww.rm.Out, "outline item flags", pdf.V1_4); err != nil {
			return err
		}
		dict["F"] = pdf.Integer(flags)
	}

	if item.Destination != nil {
		dest, err := item.Destination.Encode(ww.rm)
		if err != nil {
			return err
		}
		dict["Dest"] = dest
	} else if item.Action != nil {
		if err := pdf.CheckVersion(ww.rm.Out, "outline item action", pdf.V1_1); err != nil {
			return err
		}
		a, err := item.Action.Encode(ww.rm)
		if err != nil {
			return err
		}
		dict["A"] = a
	}

	if item.StructEntry != 0 {
		if err := pdf.CheckVersion(ww.rm.Out, "outline item SE", pdf.V1_3); err != nil {
			return err
		}
		dict["SE"] = item.StructEntry
	}

	if len(item.Children) > 0 {
		first := ww.rm.Out.Alloc()
		var last pdf.Reference
		if len(item.Children) > 1 {
			last = ww.rm.Out.Alloc()
		} else {
			last = first
		}
		dict["First"] = first
		dict["Last"] = last
		if count, ok := ww.count[item]; ok {
			dict["Count"] = pdf.Integer(count)
		}

		ww.objs = append(ww.objs, dict)
		ww.refs = append(ww.refs, ref)
		if len(ww.objs) >= chunkSize {
			err := ww.flush()
			if err != nil {
				return err
			}
		}

		err := ww.writeChildren(ref, first, last, item.Children)
		if err != nil {
			return err
		}
	} else {
		ww.objs = append(ww.objs, dict)
		ww.refs = append(ww.refs, ref)
		if len(ww.objs) >= chunkSize {
			err := ww.flush()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ww *writer) writeChildren(parent, first, last pdf.Reference, items []*Item) error {
	refs := make([]pdf.Reference, len(items))
	for i := range items {
		if i == 0 {
			refs[i] = first
		} else if i == len(items)-1 {
			refs[i] = last
		} else {
			refs[i] = ww.rm.Out.Alloc()
		}
	}

	for i, item := range items {
		dict := pdf.Dict{
			"Parent": parent,
		}
		if i > 0 {
			dict["Prev"] = refs[i-1]
		}
		if i < len(items)-1 {
			dict["Next"] = refs[i+1]
		}
		err := ww.writeItem(refs[i], dict, item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ww *writer) flush() error {
	if len(ww.objs) == 0 {
		return nil
	}
	err := ww.rm.Out.WriteCompressed(ww.refs, ww.objs...)
	if err != nil {
		return err
	}
	ww.refs = ww.refs[:0]
	ww.objs = ww.objs[:0]
	return nil
}

const chunkSize = 32
