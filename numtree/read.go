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

package numtree

import (
	"io"
	"sort"

	"seehuhn.de/go/pdf"
)

type numTree struct {
	Data []numTreeNode
}

type numTreeNode struct {
	Key   pdf.Integer
	Value pdf.Object
}

func Read(r pdf.Getter, root pdf.Object) (Tree, error) {
	if root == nil {
		return nil, nil
	}

	res := &numTree{}

	todo := []pdf.Object{root}
	for len(todo) > 0 {
		node := todo[len(todo)-1]
		todo = todo[:len(todo)-1]

		dict, _ := pdf.GetDict(r, node)

		nums, _ := pdf.GetArray(r, dict["Nums"])
		for i := 0; i+1 < len(nums); i += 2 {
			key, err := pdf.GetInteger(r, nums[i])
			if err != nil {
				return nil, err
			}
			value := nums[i+1]
			if len(res.Data) == 0 || key > res.Data[len(res.Data)-1].Key {
				res.Data = append(res.Data, numTreeNode{Key: key, Value: value})
			}
		}

		kids, _ := pdf.GetArray(r, dict["Kids"])
		for i := len(kids) - 1; i >= 0; i-- {
			todo = append(todo, kids[i])
		}
	}

	return res, nil
}

func (t *numTree) Get(key pdf.Integer) (pdf.Object, error) {
	idx := sort.Search(len(t.Data), func(i int) bool {
		return t.Data[i].Key >= key
	})
	if idx == len(t.Data) || t.Data[idx].Key != key {
		return nil, ErrKeyNotFound
	}
	return t.Data[idx].Value, nil
}

func (t *numTree) First() (pdf.Integer, error) {
	if len(t.Data) == 0 {
		return 0, ErrKeyNotFound
	}
	return t.Data[0].Key, nil
}

func (t *numTree) Next(after pdf.Integer) (pdf.Integer, error) {
	idx := sort.Search(len(t.Data), func(i int) bool {
		return t.Data[i].Key > after
	})
	if idx == len(t.Data) {
		return 0, ErrKeyNotFound
	}
	return t.Data[idx].Key, nil
}

func (t *numTree) Prev(before pdf.Integer) (pdf.Integer, error) {
	idx := sort.Search(len(t.Data), func(i int) bool {
		return t.Data[i].Key > before
	})
	if idx == 0 {
		return 0, io.EOF
	}
	return t.Data[idx-1].Key, nil
}
