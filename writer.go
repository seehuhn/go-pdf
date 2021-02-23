// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// Writer represents a PDF file open for writing.
type Writer struct {
	ver      Version
	id       [][]byte
	w        *posWriter
	xref     map[int]*xRefEntry
	nextRef  int
	inStream bool
	catalog  *Reference
	info     *Reference
}

// WriterOptions allows to influence the way a PDF file is generated.
type WriterOptions struct {
	Version Version
	ID      [][]byte

	UserPassword   string
	OwnerPassword  string
	UserPermission Perm
}

var defaultOptions = &WriterOptions{
	Version: V1_7,
}

// NewWriter prepares a PDF file for writing.
func NewWriter(w io.Writer, opt *WriterOptions) (*Writer, error) {
	if opt == nil {
		opt = defaultOptions
	} else {
		if opt.Version == 0 {
			opt.Version = defaultOptions.Version
		}
	}

	pdf := &Writer{
		ver: opt.Version,

		w:       &posWriter{w: w},
		nextRef: 1,
		xref:    make(map[int]*xRefEntry),
	}
	pdf.xref[0] = &xRefEntry{
		Pos:        -1,
		Generation: 65535,
	}
	if opt.ID != nil {
		switch len(opt.ID) {
		case 0:
			id := make([]byte, 16)
			_, err := io.ReadFull(rand.Reader, id)
			if err != nil {
				return nil, err
			}
			pdf.id = [][]byte{id, id}
		case 1, 2:
			for i := 0; i < 2; i++ {
				id := make([]byte, 16)
				if i < len(opt.ID) && opt.ID[i] != nil {
					id = append(id[:0], opt.ID[i]...) // copy the value
					if len(id) != 16 {
						return nil, errors.New("wrong File Identifier length")
					}
				} else {
					_, err := io.ReadFull(rand.Reader, id)
					if err != nil {
						return nil, err
					}
				}
				pdf.id = append(pdf.id, id)
			}
		default:
			return nil, errors.New("more than 2 File Identifiers given")
		}
	}

	if opt.UserPassword != "" || opt.OwnerPassword != "" {
		if err := pdf.checkVersion("encryption", V1_1); err != nil {
			return nil, err
		}
		if pdf.id == nil {
			id := make([]byte, 16)
			_, err := io.ReadFull(rand.Reader, id)
			if err != nil {
				return nil, err
			}
			pdf.id = [][]byte{id, id}
		}
		var cf *cryptFilter
		var V int
		if pdf.ver >= V1_6 {
			cf = &cryptFilter{
				Cipher: cipherAES,
				Length: 128,
			}
			V = 4
		} else if pdf.ver >= V1_4 {
			cf = &cryptFilter{
				Cipher: cipherRC4,
				Length: 128,
			}
			V = 2
		} else {
			cf = &cryptFilter{
				Cipher: cipherRC4,
				Length: 40,
			}
			V = 1
		}
		sec := createStdSecHandler(pdf.id[0], opt.UserPassword,
			opt.OwnerPassword, opt.UserPermission, V)
		pdf.w.enc = &encryptInfo{
			sec:  sec,
			stmF: cf,
			strF: cf,
			eff:  cf,
		}
	}

	_, err := fmt.Fprintf(pdf.w, "%%PDF-1.%d\n%%\x80\x80\x80\x80\n",
		opt.Version-V1_0)
	if err != nil {
		return nil, err
	}

	return pdf, nil
}

// Create creates the named PDF file and opens it for output.  If a previous
// file with the same name exists, it is overwritten.  After writing is
// complete, Close() must be called to write the trailer and to close the
// underlying file.
func Create(name string) (*Writer, error) {
	fd, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	return NewWriter(fd, nil)
}

// Close closes the Writer, flushing any unwritten data to the underlying
// io.Writer.  If the underlying io.Writer has a Close() method, this writer is
// also closed.
func (pdf *Writer) Close() error {
	if pdf.catalog == nil {
		return errors.New("missing /Catalog")
	}

	xRefDict := Dict{
		"Root": pdf.catalog,
		"Size": Integer(pdf.nextRef),
	}
	if pdf.info != nil {
		xRefDict["Info"] = pdf.info
	}
	if len(pdf.id) == 2 {
		xRefDict["ID"] = Array{String(pdf.id[0]), String(pdf.id[1])}
	}
	if pdf.w.enc != nil {
		xRefDict["Encrypt"] = pdf.w.enc.ToDict()
	}

	// don't encrypt the encryption dictionary and the xref dict
	pdf.w.enc = nil

	xRefPos := pdf.w.pos
	var err error
	if pdf.ver < V1_5 {
		err = pdf.writeXRefTable(xRefDict)
	} else {
		err = pdf.writeXRefStream(xRefDict)
	}
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(pdf.w, "startxref\n%d\n%%%%EOF\n", xRefPos)
	if err != nil {
		return err
	}

	closer, ok := pdf.w.w.(io.Closer)
	if ok {
		return closer.Close()
	}

	// Since we couldn't close the writer, make sure we don't accidentally
	// write beyond the end of file.
	pdf.w = nil

	return nil
}

// SetCatalog sets the Document Catalog for the file.  This must be called
// exactly once before the file is closed.  The argument `cat` can either be a
// Dict (which is then written to the file), or a *Reference pointing to a
// Dict.  No changes can be made to the catalog after SetCatalog has been
// called.
//
// The Document Catalog is documented in section 7.7.2 of PDF 32000-1:2008.
func (pdf *Writer) SetCatalog(cat Object) error {
	if pdf.catalog != nil {
		return errors.New("cannot set /Catalog twice")
	}
	switch x := cat.(type) {
	case *Reference:
		pdf.catalog = x
	case Dict:
		ref, err := pdf.Write(x, nil)
		if err != nil {
			return err
		}
		pdf.catalog = ref
	default:
		return errors.New("/Catalog must be Dict or *Reference")
	}
	return nil
}

// SetInfo sets the Document Information Dictionary for the file.  This can be
// called at most once before the file is closed.  The argument `info` can
// either be a Dict (which is then written to the file), or a *Reference
// pointing to a Dict.  No changes can be made to the /Info dictionary after
// SetInfo has been called.
//
// The Document Information Dictionary is documented in section
// 14.3.3 of PDF 32000-1:2008.
func (pdf *Writer) SetInfo(info Object) error {
	if pdf.info != nil {
		return errors.New("cannot set /Info twice")
	}
	switch x := info.(type) {
	case *Reference:
		pdf.info = x
	case Dict:
		ref, err := pdf.Write(x, nil)
		if err != nil {
			return err
		}
		pdf.info = ref
	default:
		return errors.New("/Info must be Dict or *Reference")
	}
	return nil
}

// Alloc allocates an object number for an indirect object.
func (pdf *Writer) Alloc() *Reference {
	res := &Reference{
		Number:     pdf.nextRef,
		Generation: 0,
	}
	pdf.nextRef++
	return res
}

// Write writes an object to the PDF file, as an indirect object.  The
// returned reference can be used to refer to this object from other parts of
// the file.
func (pdf *Writer) Write(obj Object, ref *Reference) (*Reference, error) {
	if pdf.inStream {
		return nil, errors.New("Write() while stream is open")
	}

	if ref == nil {
		ref = pdf.Alloc()
	} else {
		_, seen := pdf.xref[ref.Number]
		if seen {
			return nil, errors.New("object already written")
		}
	}
	pdf.w.ref = ref

	pos := pdf.w.pos

	_, err := fmt.Fprintf(pdf.w, "%d %d obj\n", ref.Number, ref.Generation)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		_, err = fmt.Fprint(pdf.w, "null")
	} else {
		err = obj.PDF(pdf.w)
	}
	if err != nil {
		return nil, err
	}
	_, err = pdf.w.Write([]byte("\nendobj\n"))
	if err != nil {
		return nil, err
	}

	pdf.xref[ref.Number] = &xRefEntry{Pos: pos, Generation: ref.Generation}

	return ref, nil
}

// ObjectStream writes objects in a compressed object stream.
func (pdf *Writer) ObjectStream(refs []*Reference, objects ...Object) ([]*Reference, error) {
	if pdf.inStream {
		return nil, errors.New("ObjectStream() while stream is open")
	}
	if err := pdf.checkVersion("using object streams", V1_5); err != nil {
		return nil, err
	}

	sRef := pdf.Alloc()
	if refs == nil {
		refs = make([]*Reference, len(objects))
	} else if len(refs) != len(objects) {
		return nil, errors.New("lengths of ref and objects differ")
	}
	for i, ref := range refs {
		if _, isStream := objects[i].(*Stream); isStream {
			return nil, errors.New("cannot store streams in object streams")
		} else if _, isRef := objects[i].(*Reference); isRef {
			return nil, errors.New("cannot store references in object streams")
		}

		if ref == nil {
			refs[i] = pdf.Alloc()
		} else if ref.Generation > 0 {
			return nil, errors.New("cannot store generation >0 in stream")
		} else {
			_, seen := pdf.xref[ref.Number]
			if seen {
				return nil, errors.New("object already written")
			}
		}
	}

	// get the offsets
	N := len(objects)
	head := &bytes.Buffer{}
	body := &bytes.Buffer{}
	for i := 0; i < N; i++ {
		ref := refs[i]
		idx := strconv.Itoa(ref.Number) + " " + strconv.Itoa(body.Len()) + "\n"
		_, err := head.WriteString(idx)
		if err != nil {
			return nil, err
		}

		pdf.xref[ref.Number] = &xRefEntry{InStream: sRef, Pos: int64(i)}

		if i < N-1 {
			// No need to buffer the last object, we will stream is separately
			// at the end.
			err = objects[i].PDF(body)
			if err != nil {
				return nil, err
			}
			err = body.WriteByte('\n')
			if err != nil {
				return nil, err
			}
		}
	}

	dict := Dict{
		"Type":  Name("ObjStm"),
		"N":     Integer(N),
		"First": Integer(head.Len()),
	}
	opt := &StreamOptions{
		Filters: []*FilterInfo{
			{Name: "FlateDecode"},
		},
	}
	w, _, err := pdf.OpenStream(dict, sRef, opt)
	if err != nil {
		return nil, err
	}

	_, err = w.Write(head.Bytes())
	if err != nil {
		return nil, err
	}

	_, err = w.Write(body.Bytes())
	if err != nil {
		return nil, err
	}

	// write the last object separately
	err = objects[N-1].PDF(w)
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return refs, nil
}

// OpenStream adds a PDF Stream to the file and returns an io.Writer which can
// be used to add the stream's data.  No other objects can be added to the file
// until the stream is closed.
func (pdf *Writer) OpenStream(dict Dict, ref *Reference, opt *StreamOptions) (io.WriteCloser, *Reference, error) {
	if pdf.inStream {
		return nil, nil, errors.New("OpenStream() while stream is open")
	}

	if ref == nil {
		ref = pdf.Alloc()
	} else {
		_, seen := pdf.xref[ref.Number]
		if seen {
			return nil, nil, errors.New(ref.String() + " already written")
		}
	}
	pdf.xref[ref.Number] = &xRefEntry{Pos: pdf.w.pos, Generation: ref.Generation}

	pdf.w.ref = ref

	// Copy dict and dict["Filter"] as well as dict["DecodeParms"], so that
	// we can register the new filters without changing the caller's dict.
	d2 := make(Dict)
	for key, val := range dict {
		if key == "Filter" || key == "DecodeParms" {
			if a, ok := val.(Array); ok {
				if len(a) == 0 {
					continue
				}
				val = append(Array{}, a...)
			}
		}
		d2[key] = val
	}
	length := &placeholder{
		size:  12,
		alloc: pdf.Alloc,
		store: pdf.Write,
	}
	d2["Length"] = length

	var w io.WriteCloser = &streamWriter{
		parent: pdf,
		dict:   d2,
		ref:    ref,
		length: length,
	}
	if pdf.w.enc != nil {
		enc, err := pdf.w.enc.cryptFilter(ref, w)
		if err != nil {
			return nil, nil, err
		}
		w = enc
	}
	if opt != nil {
		for _, fi := range opt.Filters {
			filter, err := fi.getFilter()
			if err != nil {
				return nil, nil, err
			}
			w, err = filter.Encode(w)
			if err != nil {
				return nil, nil, err
			}

			switch x := d2["Filter"].(type) {
			case nil:
				d2["Filter"] = fi.Name
				if len(fi.Parms) > 0 {
					d2["DecodeParms"] = fi.Parms
				}
			case Name:
				d2["Filter"] = Array{x, fi.Name}
				if d2["DecodeParms"] != nil || len(fi.Parms) > 0 {
					d2["DecodeParms"] = Array{d2["DecodeParms"], fi.Parms}
				}
			case Array:
				d2["Filter"] = append(x, fi.Name)
				b, ok := d2["DecodeParms"].(Array)
				if d2["DecodeParms"] != nil && !ok {
					return nil, nil, errors.New("wrong type for /DecodeParms")
				}
				if len(b) > 0 || len(fi.Parms) > 0 {
					for len(b) < len(x) {
						b = append(b, nil)
					}
					d2["DecodeParms"] = append(b, fi.Parms)
				}
			}
		}
	}
	pdf.inStream = true
	return w, ref, nil
}

// StreamOptions describes how Writer.OpenStream() processes the stream
// data while writing.
type StreamOptions struct {
	Filters []*FilterInfo
}

type streamWriter struct {
	parent   *Writer
	dict     Dict
	ref      *Reference
	started  bool
	startPos int64
	length   *placeholder
	buf      []byte
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if !w.started {
		if len(w.buf)+len(p) < 1024 {
			w.buf = append(w.buf, p...)
			return len(p), nil
		}

		err := w.flush()
		if err != nil {
			return 0, err
		}
	}

	return w.parent.w.Write(p)
}

func (w *streamWriter) flush() error {
	_, err := fmt.Fprintf(w.parent.w, "%d %d obj\n",
		w.ref.Number, w.ref.Generation)
	if err != nil {
		return err
	}
	err = w.dict.PDF(w.parent.w)
	if err != nil {
		return err
	}
	_, err = w.parent.w.Write([]byte("\nstream\n"))
	if err != nil {
		return err
	}
	w.startPos = w.parent.w.pos
	_, err = w.parent.w.Write(w.buf)
	if err != nil {
		return err
	}
	w.buf = nil
	w.started = true
	return nil
}

func (w *streamWriter) Close() error {
	var endPos int64
	if !w.started {
		err := w.length.Set(Integer(len(w.buf)))
		if err != nil {
			return err
		}
		err = w.flush()
		if err != nil {
			return err
		}
		endPos = -1
	} else {
		endPos = w.parent.w.pos
	}

	_, err := w.Write([]byte("\nendstream\nendobj\n"))
	if err != nil {
		return err
	}

	w.parent.inStream = false

	if endPos >= 0 {
		err = w.length.Set(Integer(endPos - w.startPos))
		if err != nil {
			return err
		}
	}

	return nil
}

type posWriter struct {
	w   io.Writer
	pos int64

	ref *Reference
	enc *encryptInfo
}

func (w *posWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.pos += int64(n)
	return n, err
}

type placeholder struct {
	value string
	size  int

	alloc func() *Reference
	store func(Object, *Reference) (*Reference, error)
	ref   *Reference

	fill io.WriteSeeker
	pos  []int64
}

func (x *placeholder) PDF(w io.Writer) error {
	if x.value != "" {
		_, err := w.Write([]byte(x.value))
		return err
	}

	u := w
	if uu, ok := w.(*posWriter); ok {
		u = uu.w
	}
	fill, ok := u.(io.WriteSeeker)
	if ok {
		// We can seek back: write a placeholder for now and fill in the actual
		// value later.
		pos, err := fill.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}

		x.fill = fill
		x.pos = append(x.pos, pos)

		buf := bytes.Repeat([]byte{' '}, x.size)
		_, err = w.Write(buf)
		return err
	}

	// We need to use an indirect reference.
	if x.alloc == nil {
		return errors.New("cannot seek to fill in placeholder")
	}
	x.ref = x.alloc()
	buf := &bytes.Buffer{}
	err := x.ref.PDF(buf)
	if err != nil {
		return err
	}
	x.value = buf.String()
	_, err = w.Write([]byte(x.value))
	return err
}

func (x *placeholder) Set(val Object) error {
	if x.ref != nil {
		ref, err := x.store(val, x.ref)
		x.ref = ref
		return err
	}

	buf := &bytes.Buffer{}
	err := val.PDF(buf)
	if err != nil {
		return err
	}
	if buf.Len() > x.size {
		return errors.New("too long replacement text")
	}
	x.value = buf.String()

	if len(x.pos) == 0 {
		return nil
	}

	currentPos, err := x.fill.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	for _, pos := range x.pos {
		_, err = x.fill.Seek(pos, io.SeekStart)
		if err != nil {
			return err
		}
		_, err = x.fill.Write([]byte(x.value))
		if err != nil {
			return err
		}
	}

	_, err = x.fill.Seek(currentPos, io.SeekStart)
	return err
}

func (pdf *Writer) checkVersion(operation string, minVersion Version) error {
	if pdf.ver >= minVersion {
		return nil
	}
	return &VersionError{
		Earliest:  minVersion,
		Operation: operation,
	}
}
