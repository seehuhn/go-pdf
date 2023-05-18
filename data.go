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
	meta MetaInfo

	Objects map[Reference]Object

	autoclose map[Reference]Resource
}

// Read reads a complete PDF document into memory.
func Read(r io.ReadSeeker, opt *ReaderOptions) (*Data, error) {
	pdf, err := NewReader(r, opt)
	if err != nil {
		return nil, err
	}

	res := &Data{
		meta:    pdf.meta,
		Objects: map[Reference]Object{},

		autoclose: map[Reference]Resource{},
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

		obj, err := pdf.GetObject(ref)
		if err != nil {
			return nil, err
		}
		if _, isDict := obj.(Dict); isDict {
			if pdf.meta.Trailer["Root"] == ref || pdf.meta.Trailer["Info"] == ref {
				continue
			}
		}
		if s, isStream := obj.(*Stream); isStream {
			data, err := io.ReadAll(s.R)
			if err != nil {
				return nil, err
			}
			s.Dict["Length"] = Integer(len(data))
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
		Version: d.meta.Version,
		ID:      d.meta.ID,
	}
	pdf, err := NewWriter(w, opt)
	if err != nil {
		return err
	}
	meta := pdf.GetMeta()
	meta.Catalog = d.meta.Catalog
	meta.Info = d.meta.Info

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

func (d *Data) Close() error {
	var rr []Resource
	for _, r := range d.autoclose {
		rr = append(rr, r)
	}
	sort.Slice(rr, func(i, j int) bool {
		ri := rr[i].Reference()
		rj := rr[j].Reference()
		if ri.Generation() != rj.Generation() {
			return ri.Generation() < rj.Generation()
		}
		return ri.Number() < rj.Number()
	})
	for _, r := range rr {
		err := r.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Data) GetMeta() *MetaInfo {
	return &d.meta
}

// Alloc allocates an object number for an indirect object.
func (d *Data) Alloc() Reference {
	number := uint32(len(d.Objects) + 1)
	for {
		ref := NewReference(number, 0)
		if _, ok := d.Objects[ref]; !ok {
			return ref
		}
		number++
	}
}

func (d *Data) GetObject(ref Reference) (Object, error) {
	return d.Objects[ref], nil
}

func (d *Data) Put(ref Reference, obj Object) error {
	if obj == nil {
		delete(d.Objects, ref)
	} else {
		d.Objects[ref] = obj
	}
	return nil
}

func (d *Data) OpenStream(ref Reference, dict Dict, filters ...Filter) (io.WriteCloser, error) {
	// Copy dict, dict["Filter"], and dict["DecodeParms"], so that we don't
	// change the caller's dict.
	streamDict := maps.Clone(dict)
	if filter, ok := streamDict["Filter"].(Array); ok {
		streamDict["Filter"] = append(Array{}, filter...)
	}
	if decodeParms, ok := streamDict["DecodeParms"].(Array); ok {
		streamDict["DecodeParms"] = append(Array{}, decodeParms...)
	}

	s := &Stream{
		Dict: streamDict,
	}
	d.Objects[ref] = s

	var w io.WriteCloser = &dataStreamWriter{s: s}
	var err error
	for _, filter := range filters {
		w, err = filter.Encode(d.meta.Version, w)
		if err != nil {
			return nil, err
		}

		name, parms, err := filter.Info(d.meta.Version)
		if err != nil {
			return nil, err
		}
		appendFilter(streamDict, name, parms)
	}
	return w, err
}

type dataStreamWriter struct {
	bytes.Buffer
	s *Stream
}

func (w *dataStreamWriter) Close() error {
	w.s.R = bytes.NewReader(w.Bytes())
	w.s.Dict["Length"] = Integer(w.Len())
	return nil
}

func (d *Data) WriteCompressed(refs []Reference, objects ...Object) error {
	err := checkCompressed(refs, objects)
	if err != nil {
		return err
	}

	// TODO(voss): implement this
	for i, obj := range objects {
		d.Put(refs[i], obj)
	}
	return nil
}

func (d *Data) AutoClose(res Resource) {
	ref := res.Reference()
	d.autoclose[ref] = res
}
