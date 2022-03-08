// seehuhn.de/go/pdf - a library for reading and writing PDF files
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
	"bufio"
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// Writer represents a PDF file open for writing.
// Use the functions Create() or NewWriter() to create a new Writer.
type Writer struct {
	// Version is the PDF version used in this file.  This field is
	// read-only.  Use the opt argument of NewWriter to set the PDF version for
	// a new file.
	Version Version

	// The Document Catalog is documented in section 7.7.2 of PDF 32000-1:2008.
	Catalog *Catalog

	w               *posWriter
	origW           io.WriteCloser
	closeDownstream bool

	id       [][]byte
	xref     map[int]*xRefEntry
	nextRef  int
	inStream bool
	info     *Info

	onClose []func(*Writer) error
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

// Create creates the named PDF file and opens it for output.  If a previous
// file with the same name exists, it is overwritten.  After writing is
// complete, Close() must be called to write the trailer and to close the
// underlying file.
//
// If non-default settings are required, NewWriter() can be used to set
// options.
func Create(name string) (*Writer, error) {
	fd, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	pdf, err := NewWriter(fd, nil)
	if err != nil {
		return nil, err
	}
	pdf.closeDownstream = true
	return pdf, nil
}

// NewWriter prepares a PDF file for writing.
//
// The .Close() method must be called after the file contents have been
// written.  It is the callers responsibility, to close the writer w after
// the pdf.Writer has been closed.
func NewWriter(w io.Writer, opt *WriterOptions) (*Writer, error) {
	if opt == nil {
		opt = defaultOptions
	}

	version := opt.Version
	if version == 0 {
		version = defaultOptions.Version
	}
	versionString, err := version.ToString()
	if err != nil {
		return nil, err
	}

	var origW io.WriteCloser
	if wc, ok := w.(io.WriteCloser); ok {
		origW = wc
	}

	ww, ok := w.(writeFlusher)
	if !ok {
		ww = bufio.NewWriter(w)
	}

	pdf := &Writer{
		Version: version,
		Catalog: &Catalog{},

		w:     &posWriter{w: ww},
		origW: origW,

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
		if err := pdf.CheckVersion("encryption", V1_1); err != nil {
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
		if pdf.Version >= V1_6 {
			cf = &cryptFilter{
				Cipher: cipherAES,
				Length: 128,
			}
			V = 4
		} else if pdf.Version >= V1_4 {
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

	_, err = fmt.Fprintf(pdf.w, "%%PDF-%s\n%%\x80\x80\x80\x80\n", versionString)
	if err != nil {
		return nil, err
	}

	return pdf, nil
}

// Close closes the Writer, flushing any unwritten data to the underlying
// io.Writer.
func (pdf *Writer) Close() error {
	for i := len(pdf.onClose) - 1; i >= 0; i-- {
		err := pdf.onClose[i](pdf)
		if err != nil {
			return err
		}
	}

	if pdf.Catalog.Pages == nil {
		return errors.New("no pages in PDF")
	}
	catRef, err := pdf.Write(AsDict(pdf.Catalog), nil)
	if err != nil {
		return err
	}

	xRefDict := Dict{
		"Root": catRef,
		"Size": Integer(pdf.nextRef),
	}
	if pdf.info != nil {
		infoRef, err := pdf.Write(AsDict(pdf.info), nil)
		if err != nil {
			return err
		}
		xRefDict["Info"] = infoRef
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
	if pdf.Version < V1_5 {
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

	err = pdf.w.w.Flush()
	if err != nil {
		return err
	}

	if pdf.closeDownstream && pdf.origW != nil {
		return pdf.origW.Close()
	}

	// Make sure we don't accidentally write beyond the end of file.
	pdf.w = nil

	return nil
}

// OnClose registers a callback function which is called before the writer is
// closed.  Callbacks are executed in the reverse order, i.e. the last callback
// registered is the first one to run.
func (pdf *Writer) OnClose(callback func(*Writer) error) {
	pdf.onClose = append(pdf.onClose, callback)
}

// SetInfo sets the Document Information Dictionary for the file.
// The Document Information Dictionary is documented in section 14.3.3 of PDF
// 32000-1:2008.
func (pdf *Writer) SetInfo(info *Info) {
	info2 := *info
	pdf.info = &info2
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
			return nil, errDuplicateRef
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

// WriteCompressed writes a number of objects to the file as a compressed
// object stream.  Object streams are only available for PDF version 1.5 and
// newer; in case the file version is too low, the objects are written directly
// into the PDF file, without compression.
func (pdf *Writer) WriteCompressed(refs []*Reference, objects ...Object) ([]*Reference, error) {
	if pdf.inStream {
		return nil, errors.New("WriteCompressed() while stream is open")
	}

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
				return nil, errDuplicateRef
			}
		}
	}

	if pdf.Version < V1_5 {
		// Object streams are only availble in PDF version 1.5 and higher.
		for i, obj := range objects {
			_, err := pdf.Write(obj, refs[i])
			if err != nil {
				return nil, err
			}
		}
		return refs, nil
	}

	// get the offsets
	N := len(objects)
	head := &bytes.Buffer{}
	body := &bytes.Buffer{}
	sRef := pdf.Alloc()
	for i := 0; i < N; i++ {
		ref := refs[i]
		idx := strconv.Itoa(ref.Number) + " " + strconv.Itoa(body.Len()) + "\n"
		_, err := head.WriteString(idx)
		if err != nil {
			return nil, err
		}

		pdf.xref[ref.Number] = &xRefEntry{InStream: sRef, Pos: int64(i)}

		if i < N-1 {
			// No need to buffer the last object, since we can stream it
			// separately at the end.
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
	w, _, err := pdf.OpenStream(dict, sRef, &FilterInfo{Name: "FlateDecode"})
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
func (pdf *Writer) OpenStream(dict Dict, ref *Reference, filters ...*FilterInfo) (io.WriteCloser, *Reference, error) {
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
	length := pdf.NewPlaceholder(12)
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
	for _, fi := range filters {
		if fi == nil {
			continue
		}

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
	pdf.inStream = true
	return w, ref, nil
}

type streamWriter struct {
	parent   *Writer
	dict     Dict
	ref      *Reference
	started  bool // TODO(voss): can this be replaced by `buf == nil`?
	startPos int64
	length   *Placeholder
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

// A Placeholder can be used to reserve space in a PDF file where some value
// can be filled in later.  This is for example used to store the content
// length of a compressed stream in the stream dictionary. Placeholer objects
// are created using Writer.NewPlaceholder(). As soon as the value is known, it
// can be filled in using the Set() method.
type Placeholder struct {
	value string
	size  int

	alloc func() *Reference
	write func(Object, *Reference) (*Reference, error)
	ref   *Reference

	fill io.WriteSeeker
	pos  []int64
}

// NewPlaceholder creates a new placeholder for a value which is not yet known.
// The argument size must be an upper bound to the length of the replacement
// text.  Once the value becomes known, it can be filled in using the Set()
// method.
func (pdf *Writer) NewPlaceholder(size int) *Placeholder {
	return &Placeholder{
		size:  size,
		alloc: pdf.Alloc,
		write: pdf.Write,
	}
}

// PDF implements the Object interface.
func (x *Placeholder) PDF(w io.Writer) error {
	// method 1: If the value is already known, we can just write it to the
	// file.
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
		// method 2: If we can seek back, write whitespace for now and fill in
		// the actual value later.
		// TODO(voss): verify that this is triggered when embedding fonts
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

	// method 3: If all else fails, use an indirect reference.
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

// Set fills in the value of the placeholder object.  This should be called
// as soon as possible after the value becomes known.
func (x *Placeholder) Set(val Object) error {
	if x.ref != nil {
		// TODO(voss): what happens if we are in a stream?
		ref, err := x.write(val, x.ref)
		x.ref = ref
		return err
	}

	buf := &bytes.Buffer{}
	err := val.PDF(buf)
	if err != nil {
		return err
	}
	if buf.Len() > x.size {
		return errors.New("Placeholder: replacement text too long")
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

// CheckVersion checks whether the PDF file being written has version
// minVersion or later.  If the version is new enough, nil is returned.
// Otherwise a VersionError for the given operation is returned.
func (pdf *Writer) CheckVersion(operation string, minVersion Version) error {
	if pdf.Version >= minVersion {
		return nil
	}
	return &VersionError{
		Earliest:  minVersion,
		Operation: operation,
	}
}

type writeFlusher interface {
	io.Writer
	Flush() error
}

type posWriter struct {
	w   writeFlusher
	pos int64

	ref *Reference
	enc *encryptInfo
}

func (w *posWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.pos += int64(n)
	return n, err
}
