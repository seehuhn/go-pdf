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
	"slices"
	"sort"

	"golang.org/x/exp/maps"
)

// Data is an in-memory representation of a PDF document.
type Data struct {
	meta      MetaInfo
	objects   map[Reference]Object
	lastRef   uint32
	autoclose []io.Closer
}

func NewData(v Version) *Data {
	res := &Data{
		meta: MetaInfo{
			Version: v,
			Catalog: &Catalog{},
		},
		objects: map[Reference]Object{},
		lastRef: 0,
	}
	return res
}

// Read reads a complete PDF document into memory.
func Read(r io.ReadSeeker, opt *ReaderOptions) (*Data, error) {
	pdf, err := NewReader(r, opt)
	if err != nil {
		return nil, err
	}

	res := &Data{
		meta:    pdf.meta,
		objects: map[Reference]Object{},
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

		obj, err := pdf.Get(ref, true)
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
			res.objects[ref] = obj
		}
	}

	return res, nil
}

// Write writes the PDF document to w.
// TODO(voss): take a *WriterOptions argument?
func (d *Data) Write(w io.Writer) error {
	opt := &WriterOptions{
		ID: d.meta.ID,
	}
	pdf, err := NewWriter(w, d.meta.Version, opt)
	if err != nil {
		return err
	}
	meta := pdf.GetMeta()
	meta.Catalog = d.meta.Catalog
	meta.Info = d.meta.Info

	refs := maps.Keys(d.objects)
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Number() < refs[j].Number()
	})

	for _, ref := range refs {
		err := pdf.Put(ref, d.objects[ref])
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
	for _, obj := range d.autoclose {
		err := obj.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Data) GetMeta() *MetaInfo {
	return &d.meta
}

// Alloc allocates a new object number for an indirect object.
func (d *Data) Alloc() Reference {
	for {
		d.lastRef++
		ref := NewReference(d.lastRef, 0)
		if _, isUsed := d.objects[ref]; !isUsed {
			return ref
		}
	}
}

// Get implements the [Getter] interface.
func (d *Data) Get(ref Reference, _ bool) (Object, error) {
	if ref.IsInternal() {
		panic("internal reference") // TODO(voss): return an error instead?
	}
	obj := d.objects[ref]
	switch x := obj.(type) {
	case *Stream:
		if ss, ok := x.R.(io.Seeker); ok {
			_, err := ss.Seek(0, io.SeekStart)
			if err != nil {
				return nil, err
			}
		}
	case Dict:
		obj = maps.Clone(x)
	case Array:
		obj = slices.Clone(x)
	}
	return obj, nil
}

func (d *Data) Put(ref Reference, obj Object) error {
	if obj == nil {
		delete(d.objects, ref)
	} else if _, exists := d.objects[ref]; exists {
		return errDuplicateRef
	} else {
		d.objects[ref] = obj
	}
	return nil
}

func (d *Data) OpenStream(ref Reference, dict Dict, filters ...Filter) (io.WriteCloser, error) {
	// Copy dict, dict["Filter"], and dict["DecodeParms"], so that we don't
	// change the caller's dict.
	streamDict := maps.Clone(dict)
	if streamDict == nil {
		streamDict = Dict{}
	}
	if filter, ok := streamDict["Filter"].(Array); ok {
		streamDict["Filter"] = append(Array{}, filter...)
	}
	if decodeParms, ok := streamDict["DecodeParms"].(Array); ok {
		streamDict["DecodeParms"] = append(Array{}, decodeParms...)
	}

	s := &Stream{
		Dict: streamDict,
	}
	d.objects[ref] = s

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
		err = d.Put(refs[i], obj)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Data) AutoClose(obj io.Closer) {
	d.autoclose = append(d.autoclose, obj)
}
