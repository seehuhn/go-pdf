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
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
)

// pathNode describes a node on a path from the root of a page tree to a page.
// This only includes page tree nodes, but not the final page node.
type pathNode struct {
	// attr contains the inheritable attributes of the page tree node,
	// with all the parents' attributes merged in.
	attr *InheritableAttributes

	// kids is the decoded /Kids entry of the page tree node.
	kids []pdf.Reference

	// pos is the current position within the kids slice.
	pos int

	// count is the number of page (leaf) nodes below this node.
	count int

	start int
}

type Reader struct {
	r      pdf.Getter
	branch []*pathNode
	pageNo int
}

func NewReader(r pdf.Getter) (*Reader, error) {
	catalog := r.GetMeta().Catalog

	res := &Reader{
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
	attr := &InheritableAttributes{Rotate: pdf.Rotate0}
	node, err := res.decodePageTreeDict(dict, attr)
	if err != nil {
		return nil, err
	}
	if len(node.kids) == 0 {
		return nil, fmt.Errorf("page tree without pages")
	}
	res.branch = append(res.branch, node)

	res.pageNo = -1

	return res, nil
}

func (r *Reader) GetPage(pageNo int) (pdf.Dict, *InheritableAttributes, error) {
	start := r.pageNo // TODO(voss): initialization!!!
	count := 1
	for {
		k := len(r.branch) - 1
		node := r.branch[k]
		if pageNo >= node.start && pageNo < node.start+node.count {
			break
		}

		if k == 0 {
			return nil, nil, fmt.Errorf("page %d not found", pageNo)
		}

		start = node.start
		count = node.count
		r.branch = r.branch[:k]
	}

	var aPage, bIdx, bPage int
	node := r.branch[len(r.branch)-1]
	if pageNo < start {
		node.pos = 0
		aPage = node.start
		bIdx = node.pos
		bPage = start
	} else {
		if pageNo < start+count {
			panic("not reached") // TODO(voss): remove
		}
		node.pos = node.pos + 1
		aPage = start + count
		bIdx = len(node.kids)
		bPage = node.start + node.count
	}

	// iterate over children until we find the right one
	for node.pos < bIdx && aPage <= pageNo && pageNo < bPage {
		// read the next child
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

			if pageNo < aPage+child.count {
				child.start = aPage
				r.branch = append(r.branch, child)

				node = child
				bIdx = len(child.kids)
				bPage = child.start + child.count
			} else {
				node.pos++
				aPage += child.count
			}
		case pdf.Name("Page"):
			attr, err := r.decodeAttrs(dict, node.attr)
			if err != nil {
				return nil, nil, err
			}
			if pageNo == aPage {
				r.pageNo = pageNo
				return dict, attr, nil
			}
			node.pos++
			aPage++
		default:
			return nil, nil, fmt.Errorf("unexpected page type %v", dict["Type"])
		}
	}

	return nil, nil, errors.New("page tree corrupted")
}

func (r *Reader) decodeAttrs(dict pdf.Dict, parentAttr *InheritableAttributes) (*InheritableAttributes, error) {
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
	rotate, err := pdf.DecodeRotation(rotateInt)
	if err != nil {
		return nil, err
	}
	if rotate != pdf.RotateInherit {
		attr.Rotate = rotate
	}
	return attr, nil
}

func (r *Reader) decodePageTreeDict(dict pdf.Dict, parentAttr *InheritableAttributes) (*pathNode, error) {
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

	// read the /Count entry
	count, err := pdf.GetInt(r.r, dict["Count"])
	if err != nil {
		return nil, err
	} else if count < 0 || count > math.MaxInt32 {
		return nil, fmt.Errorf("invalid page count %d", count)
	}
	node.count = int(count)

	return node, nil
}
