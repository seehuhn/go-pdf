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

package pagetree

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
)

type Reader struct {
	r    *pdf.Reader
	root pdf.Dict
}

func NewReader(r *pdf.Reader) (*Reader, error) {
	root, err := pdf.GetDict(r, r.Catalog.Pages)
	if err != nil {
		return nil, err
	}

	res := &Reader{
		r:    r,
		root: root,
	}
	return res, nil
}

func (r *Reader) NumPages() (pdf.Integer, error) {
	return pdf.GetInt(r.r, r.root["Count"])
}

func (r *Reader) Get(pageNo pdf.Integer) (pdf.Dict, error) {
	var pos pdf.Integer
	dict := r.root
treeLoop:
	for dict["Type"] != pdf.Name("Page") {
		children, err := pdf.GetArray(r.r, dict["Kids"])
		if err != nil {
			return nil, err
		}
		for _, kid := range children {
			childDict, err := pdf.GetDict(r.r, kid)
			if err != nil {
				return nil, err
			}
			var count pdf.Integer
			switch childDict["Type"] {
			case pdf.Name("Pages"):
				count, err = pdf.GetInt(r.r, childDict["Count"])
				if err != nil {
					return nil, err
				}
			case pdf.Name("Page"):
				count = 1
			default:
				return nil, fmt.Errorf("unexpected page type %v", childDict["Type"])
			}

			if pageNo < pos+count {
				dict = childDict
				continue treeLoop
			}
			pos += count
		}
		return nil, fmt.Errorf("page %d not found", pageNo)
	}
	if pageNo != pos {
		return nil, fmt.Errorf("page %d not found", pageNo)
	}
	return dict, nil
}

// pathNode describes a node on a path from the root of a page tree to a page.
// This only includes page tree nodes, but not the final page node.
type pathNode struct {
	// attr contains the inheritable attributes of the page tree node,
	// with all the parents' attributes merged in.
	attr *InheritableAttributes

	// kids is the decoded /Kids entry of the page tree node.
	kids []pdf.Reference

	// pos is the position of the current page within the kids slice.
	pos int

	// count is the number of page (leaf) nodes below this node.
	count pdf.Integer
}

type IterableReader struct {
	r      pdf.Getter
	branch []*pathNode
}

func NewIterableReader(r pdf.Getter) (*IterableReader, error) {
	catalog := r.GetCatalog()

	res := &IterableReader{
		r: r,
	}

	ref := catalog.Pages
	dict, err := pdf.GetDict(r, ref)
	if err != nil {
		return nil, err
	}
	if dict["Type"] != pdf.Name("Pages") {
		return nil, fmt.Errorf("unexpected page type %v", dict["Type"])
	}
	attr := &InheritableAttributes{Rotate: Rotate0}
	node, err := res.decodePageTreeDict(dict, attr)
	if err != nil {
		return nil, err
	}
	res.branch = append(res.branch, node)

	return res, nil
}

func (r *IterableReader) decodeAttrs(dict pdf.Dict, parentAttr *InheritableAttributes) (*InheritableAttributes, error) {
	attr := &InheritableAttributes{}
	*attr = *parentAttr // fill the defaults from the parent

	resourcesDict, err := pdf.GetDict(r.r, dict["Resources"])
	if err != nil {
		return nil, err
	}
	if resourcesDict != nil {
		resources := &pdf.Resources{}
		err = pdf.DecodeDict(r.r, resources, resourcesDict)
		if err != nil {
			return nil, err
		}
		attr.Resources = resources
	}
	mediaBox, err := pdf.GetRectangle(r.r, dict["MediaBox"])
	if err != nil {
		return nil, err
	}
	if mediaBox != nil {
		attr.MediaBox = mediaBox
	}
	cropBox, err := pdf.GetRectangle(r.r, dict["CropBox"])
	if err != nil {
		return nil, err
	}
	if cropBox != nil {
		attr.CropBox = cropBox
	}
	rotateInt, err := pdf.GetInt(r.r, dict["Rotate"])
	if err != nil {
		return nil, err
	}
	rotate, err := DecodeRotation(rotateInt)
	if err != nil {
		return nil, err
	}
	if rotate != RotateInherit {
		attr.Rotate = rotate
	}
	return attr, nil
}

func (r *IterableReader) decodePageTreeDict(dict pdf.Dict, parentAttr *InheritableAttributes) (*pathNode, error) {
	node := &pathNode{}

	// Read the inheritable page attributes.
	attr, err := r.decodeAttrs(dict, parentAttr)
	if err != nil {
		return nil, err
	}
	node.attr = attr

	// read the /Kids entry
	kids, err := pdf.GetArray(r.r, dict["Kids"])
	if err != nil {
		return nil, err
	}
	node.kids = make([]pdf.Reference, 0, len(kids))
	for i, kid := range kids {
		kRef, ok := kid.(pdf.Reference)
		if !ok {
			return nil, fmt.Errorf("kid %d is not a reference", i)
		}
		node.kids = append(node.kids, kRef)
	}
	node.pos = -1

	// read the /Count entry
	node.count, err = pdf.GetInt(r.r, dict["Count"])
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (r *IterableReader) NextPage() (pdf.Dict, *InheritableAttributes, error) {
	for {
		// discard all nodes which are not needed any more
		for {
			k := len(r.branch) - 1
			node := r.branch[k]
			node.pos++
			if node.pos >= len(r.branch) {
				if k == 0 {
					return nil, nil, io.EOF
				}
				r.branch = r.branch[:k]
			} else {
				break
			}
		}

		// read the next child
		node := r.branch[len(r.branch)-1]
		ref := node.kids[node.pos]
		dict, err := pdf.GetDict(r.r, ref)
		if err != nil {
			return nil, nil, err
		}
		switch dict["Type"] {
		case pdf.Name("Pages"):
			child, err := r.decodePageTreeDict(dict, node.attr)
			if err != nil {
				return nil, nil, err
			}
			r.branch = append(r.branch, child)
		case pdf.Name("Page"):
			attr, err := r.decodeAttrs(dict, node.attr)
			if err != nil {
				return nil, nil, err
			}
			return dict, attr, nil
		}
	}
}

func (r *IterableReader) GetPage(pageNo int) (pdf.Dict, *InheritableAttributes, error) {
	panic("not implemented")
}
