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

package pdf

import (
	"bytes"
	"io"
	"sort"

	"golang.org/x/exp/maps"
)

// Data is an in-memory representation of a PDF document.
type Data struct {
	Version Version
	Catalog *Catalog
	Info    *Info
	ID      [][]byte
	Objects map[Reference]Object
}

// Read reads a complete PDF document into memory.
func Read(r io.ReadSeeker, opt *ReaderOptions) (*Data, error) {
	pdf, err := NewReader(r, opt)
	if err != nil {
		return nil, err
	}

	res := &Data{
		Version: pdf.Version,
		Catalog: pdf.Catalog,
		Info:    pdf.Info,
		ID:      pdf.ID,
		Objects: map[Reference]Object{},
	}

	isObjectStream := make(map[Reference]bool)
	for _, entry := range pdf.xref {
		if entry.InStream != 0 {
			isObjectStream[entry.InStream] = true
		}
	}

	for number, entry := range pdf.xref {
		if entry.IsFree() {
			continue
		}
		ref := NewReference(number, entry.Generation)
		if isObjectStream[ref] {
			continue
		}

		obj, err := pdf.Get(ref)
		if err != nil {
			return nil, err
		}
		if d, isDict := obj.(Dict); isDict {
			if d["Type"] == Name("Catalog") {
				// TODO(voss): find a better way to find the Document Catalog
				continue
			}
		}
		if s, isStream := obj.(*Stream); isStream {
			data, err := io.ReadAll(s.R)
			if err != nil {
				return nil, err
			}
			obj = &Stream{
				Dict: s.Dict,
				R:    bytes.NewReader(data),
			}
		}
		if obj != nil {
			res.Objects[ref] = obj
		}
	}

	return res, nil
}

// Write writes the PDF document to w.
func (d *Data) Write(w io.Writer) error {
	opt := &WriterOptions{
		Version: d.Version,
		ID:      d.ID,
	}
	pdf, err := NewWriter(w, opt)
	if err != nil {
		return err
	}
	pdf.Catalog = d.Catalog
	pdf.SetInfo(d.Info)

	refs := maps.Keys(d.Objects)
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Number() < refs[j].Number()
	})

	for _, ref := range refs {
		err := pdf.Put(ref, d.Objects[ref])
		if err != nil {
			return err
		}
	}

	err = pdf.Close()
	if err != nil {
		return err
	}

	return nil
}

func (d *Data) Get(ref Reference) (Object, error) {
	return d.Objects[ref], nil
}

func (d *Data) Put(ref Reference, obj Object) error {
	if obj == nil {
		delete(d.Objects, ref)
		return nil
	}
	d.Objects[ref] = obj
	return nil
}
