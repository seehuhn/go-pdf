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
	"errors"
	"io"
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf"
)

type Tree interface {
	Get(key pdf.Integer) (pdf.Object, error)
	First() (pdf.Integer, error)
	Next(after pdf.Integer) (pdf.Integer, error)
}

func Read(r pdf.Reader, root pdf.Object) (Tree, error) {
	panic("not implemented")
}

func Write(w pdf.Putter, tree Tree) (pdf.Reference, error) {
	sw := NewSequentialWriter(w)
	pos, err := tree.First()
	if err != nil {
		return 0, err
	}
	for {
		val, err := tree.Get(pos)
		if err != nil {
			return 0, err
		}
		err = sw.Append(pos, val)
		if err != nil {
			return 0, err
		}
		pos, err = tree.Next(pos)
		if err == io.EOF {
			break
		} else if err != nil {
			return 0, err
		}
	}
	err = sw.Close()
	if err != nil {
		return 0, err
	}
	return sw.Reference(), nil
}

type SequentialWriter struct {
	w    pdf.Putter
	ref  pdf.Reference
	data map[pdf.Integer]pdf.Object
}

func NewSequentialWriter(w pdf.Putter) *SequentialWriter {
	sw := &SequentialWriter{
		w:    w,
		ref:  w.Alloc(),
		data: make(map[pdf.Integer]pdf.Object),
	}
	return sw
}

func (sw *SequentialWriter) Append(key pdf.Integer, val pdf.Object) error {
	sw.data[key] = val
	return nil
}

func (sw *SequentialWriter) Close() error {
	if len(sw.data) == 0 {
		return errors.New("numtree: empty tree")
	}

	keys := maps.Keys(sw.data)
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	min := keys[0]
	max := keys[len(keys)-1]

	var Nums pdf.Array
	for _, key := range keys {
		Nums = append(Nums, key, sw.data[key])
	}

	dict := pdf.Dict{
		"Nums":   Nums,
		"Limits": pdf.Array{min, max},
	}
	err := sw.w.Put(sw.ref, dict)
	return err
}

func (sw *SequentialWriter) Reference() pdf.Reference {
	return sw.ref
}
