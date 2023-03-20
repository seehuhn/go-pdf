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

	"seehuhn.de/go/pdf"
)

type Reader struct {
	r    *pdf.Reader
	root pdf.Dict
}

func NewReader(r *pdf.Reader) (*Reader, error) {
	root, err := r.GetDict(r.Catalog.Pages)
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
	return r.r.GetInt(r.root["Count"])
}

func (r *Reader) Get(pageNo pdf.Integer) (pdf.Dict, error) {
	var pos pdf.Integer
	dict := r.root
treeLoop:
	for dict["Type"] != pdf.Name("Page") {
		children, err := r.r.GetArray(dict["Kids"])
		if err != nil {
			return nil, err
		}
		for _, kid := range children {
			childDict, err := r.r.GetDict(kid)
			if err != nil {
				return nil, err
			}
			var count pdf.Integer
			switch childDict["Type"] {
			case pdf.Name("Pages"):
				count, err = r.r.GetInt(childDict["Count"])
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
