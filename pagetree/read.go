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

import "seehuhn.de/go/pdf"

func FindPages(r pdf.Getter) ([]pdf.Reference, error) {
	meta := r.GetMeta()
	catalog := meta.Catalog
	if catalog.Pages == 0 {
		return nil, errInvalidPageTree
	}

	var res []pdf.Reference
	todo := []pdf.Reference{catalog.Pages}
	seen := map[pdf.Reference]bool{
		catalog.Pages: true,
	}
	for len(todo) > 0 {
		k := len(todo) - 1
		ref := todo[k]
		todo = todo[:k]

		node, err := pdf.GetDict(r, ref)
		if err != nil {
			return nil, err
		}
		tp, err := pdf.GetName(r, node["Type"])
		if err != nil {
			return nil, err
		}
		switch tp {
		case "Page":
			res = append(res, ref)
		case "Pages":
			kids, err := pdf.GetArray(r, node["Kids"])
			if err != nil {
				return nil, err
			}
			for i := len(kids) - 1; i >= 0; i-- {
				kid := kids[i]
				if kidRef, ok := kid.(pdf.Reference); ok && !seen[kidRef] {
					todo = append(todo, kidRef)
					seen[kidRef] = true
				} else {
					res = append(res, 0)
				}
			}
		}
	}

	return res, nil
}
