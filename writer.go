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
	"sort"
	"strconv"

	"golang.org/x/exp/maps"
)

// Writer represents a PDF file open for writing.
// Use the functions [Create] or [NewWriter] to create a new Writer.
type Writer struct {
	// Version is the PDF version used in this file.  This field is
	// read-only.  Use the opt argument of NewWriter to set the PDF version for
	// a new file.
	Version Version

	// The Document Catalog is documented in section 7.7.2 of PDF 32000-1:2008.
	Catalog *Catalog

	Tagged bool

	info *Info

	w               *posWriter
	origW           io.WriteCloser
	closeDownstream bool

	id      [][]byte
	xref    map[uint32]*xRefEntry
	nextRef uint32

	inStream bool
	// TODO(voss): change afterStream into a list of (Reference, Object)
	// pairs to be written as soon as possible.  Maybe change the Put() method
	// to append to afterStream if inStream is true?
	afterStream []func(*Writer) error

	Resources map[Reference]Resource
}

// WriterOptions allows to influence the way a PDF file is generated.
type WriterOptions struct {
	Version Version
	ID      [][]byte

	UserPassword    string
	OwnerPassword   string
	UserPermissions Perm
}

var defaultWriterOptions = &WriterOptions{
	Version: V1_7,
}

type Resource interface {
	// Write writes the resource to the PDF file.  No changes can be
	// made to the resource after it has been written.
	Close() error

	Reference() Reference
}

// Create creates the named PDF file and opens it for output.  If a previous
// file with the same name exists, it is overwritten.  After writing is
// complete, [Writer.Close] must be called to write the trailer and to close the
// underlying file.
func Create(name string, opt *WriterOptions) (*Writer, error) {
	fd, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	pdf, err := NewWriter(fd, opt)
	if err != nil {
		return nil, err
	}
	pdf.closeDownstream = true
	return pdf, nil
}

// NewWriter prepares a PDF file for writing.
//
// The [Writer.Close] method must be called after the file contents have been
// written, to add the trailer and the cross reference table to the PDF file.
// It is the callers responsibility, to close the writer w after
// the pdf.Writer has been closed.
func NewWriter(w io.Writer, opt *WriterOptions) (*Writer, error) {
	if opt == nil {
		opt = defaultWriterOptions
	}

	version := opt.Version
	if version == 0 {
		version = defaultWriterOptions.Version
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
		xref:    make(map[uint32]*xRefEntry),

		Resources: make(map[Reference]Resource),
	}
	pdf.xref[0] = &xRefEntry{
		Pos:        -1,
		Generation: 65535,
	}

	needID := opt.ID != nil ||
		version >= V2_0 ||
		opt.UserPassword != "" ||
		opt.OwnerPassword != ""
	if needID {
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
					id = append(id[:0], opt.ID[i]...)
					if len(id) < 16 {
						return nil, errShortID
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
		var cf *cryptFilter
		var V int
		if pdf.Version >= V2_0 {
			cf = &cryptFilter{
				Cipher: cipherAES,
				Length: 256,
			}
			V = 5
		} else if pdf.Version >= V1_6 {
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
		sec, err := createStdSecHandler(pdf.id[0], opt.UserPassword,
			opt.OwnerPassword, opt.UserPermissions, cf.Length, V)
		if err != nil {
			return nil, err
		}
		pdf.w.enc = &encryptInfo{
			sec:  sec,
			stmF: cf,
			strF: cf,
			efF:  cf,
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
	var rr []Resource
	for _, r := range pdf.Resources {
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

	if pdf.Tagged {
		MarkInfo, _ := pdf.Catalog.MarkInfo.(Dict)
		if MarkInfo == nil {
			MarkInfo = Dict{}
		}
		MarkInfo["Marked"] = Bool(true)
		pdf.Catalog.MarkInfo = MarkInfo
	}
	catRef := pdf.Alloc()
	err := pdf.Put(catRef, AsDict(pdf.Catalog))
	if err != nil {
		return fmt.Errorf("failed to write document catalog: %w", err)
	}

	xRefDict := Dict{
		"Root": catRef,
		"Size": Integer(pdf.nextRef),
	}
	if pdf.info != nil {
		infoRef := pdf.Alloc()
		err := pdf.Put(infoRef, AsDict(pdf.info))
		if err != nil {
			return err
		}
		xRefDict["Info"] = infoRef
	}
	if len(pdf.id) == 2 {
		xRefDict["ID"] = Array{String(pdf.id[0]), String(pdf.id[1])}
	}
	if pdf.w.enc != nil {
		encryptDict, err := pdf.w.enc.AsDict(pdf.Version)
		if err != nil {
			return err
		}
		xRefDict["Encrypt"] = encryptDict
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

func (pdf *Writer) AutoClose(res Resource) {
	ref := res.Reference()
	pdf.Resources[ref] = res
}

// SetInfo sets the Document Information Dictionary for the file.
func (pdf *Writer) SetInfo(info *Info) {
	if info == nil {
		pdf.info = nil
		return
	}
	infoCopy := *info
	pdf.info = &infoCopy
}

// Alloc allocates an object number for an indirect object.
func (pdf *Writer) Alloc() Reference {
	res := NewReference(pdf.nextRef, 0)
	pdf.nextRef++
	return res
}

// Put writes an object to the PDF file, as an indirect object using the
// given reference.
func (pdf *Writer) Put(ref Reference, obj Object) error {
	if pdf.inStream {
		return errors.New("Put() while stream is open")
	}

	err := pdf.setXRef(ref, &xRefEntry{Pos: pdf.w.pos, Generation: ref.Generation()})
	if err != nil {
		return fmt.Errorf("Writer.Put: %w", err)
	}
	pdf.w.ref = ref

	_, err = fmt.Fprintf(pdf.w, "%d %d obj\n", ref.Number(), ref.Generation())
	if err != nil {
		return err
	}
	err = writeObject(pdf.w, obj)
	if err != nil {
		return err
	}
	_, err = pdf.w.Write([]byte("\nendobj\n"))
	if err != nil {
		return err
	}

	return nil
}

// WriteCompressed writes a number of objects to the file as a compressed
// object stream.
//
// Object streams are only available for PDF version 1.5 and newer; in case the
// file version is too low, the objects are written directly into the PDF file,
// without compression.
func (pdf *Writer) WriteCompressed(refs []Reference, objects ...Object) error {
	if pdf.inStream {
		return errors.New("WriteCompressed() while stream is open")
	}
	err := checkCompressed(refs, objects)
	if err != nil {
		return err
	}

	if pdf.Version < V1_5 {
		// Object streams are only availble in PDF version 1.5 and higher.
		for i, obj := range objects {
			err := pdf.Put(refs[i], obj)
			if err != nil {
				return fmt.Errorf("Writer.WriteCompressed (V<1.5): %w", err)
			}
		}
		return nil
	}

	sRef := pdf.Alloc() // TODO(voss): pass this in as an argument?
	for i, ref := range refs {
		err := pdf.setXRef(ref, &xRefEntry{InStream: sRef, Pos: int64(i)})
		if err != nil {
			return fmt.Errorf("Writer.WriteCompressed: %w", err)
		}
	}

	// get the offsets
	N := len(objects)
	head := &bytes.Buffer{}
	body := &bytes.Buffer{}
	for i := 0; i < N; i++ {
		ref := refs[i]
		idx := strconv.Itoa(int(ref.Number())) + " " + strconv.Itoa(body.Len()) + "\n"
		_, err := head.WriteString(idx)
		if err != nil {
			return err
		}

		if i < N-1 {
			// No need to buffer the last object, since we can stream it
			// separately at the end.
			err = objects[i].PDF(body)
			if err != nil {
				return err
			}
			err = body.WriteByte('\n')
			if err != nil {
				return err
			}
		}
	}

	dict := Dict{
		"Type":  Name("ObjStm"),
		"N":     Integer(N),
		"First": Integer(head.Len()),
	}
	w, err := pdf.OpenStream(sRef, dict, FilterFlate{})
	if err != nil {
		return err
	}

	_, err = w.Write(head.Bytes())
	if err != nil {
		return err
	}

	_, err = w.Write(body.Bytes())
	if err != nil {
		return err
	}

	// write the last object separately
	err = objects[N-1].PDF(w)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return nil
}

func checkCompressed(refs []Reference, objects []Object) error {
	if len(refs) != len(objects) {
		return errors.New("lengths of refs and objects differ")
	}
	for i, ref := range refs {
		if _, isStream := objects[i].(*Stream); isStream {
			return errors.New("cannot store streams in object streams")
		} else if _, isRef := objects[i].(Reference); isRef {
			return errors.New("cannot store references in object streams")
		} else if ref.Generation() > 0 {
			return errors.New("cannot use non-zero generation inside object stream")
		}
	}
	return nil
}

// OpenStream adds a PDF Stream to the file and returns an io.Writer which can
// be used to add the stream's data.  No other objects can be added to the file
// until the stream is closed.
func (pdf *Writer) OpenStream(ref Reference, dict Dict, filters ...Filter) (io.WriteCloser, error) {
	if pdf.inStream {
		return nil, errors.New("OpenStream() while stream is open")
	}

	err := pdf.setXRef(ref, &xRefEntry{Pos: pdf.w.pos, Generation: ref.Generation()})
	if err != nil {
		return nil, fmt.Errorf("Writer.OpenStream: %w", err)
	}
	pdf.w.ref = ref

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

	length := NewPlaceholder(pdf, 12)
	if _, exists := streamDict["Length"]; !exists {
		streamDict["Length"] = length
	}

	var w io.WriteCloser = &streamWriter{
		parent:     pdf,
		streamDict: streamDict,
		ref:        ref,
		length:     length,
	}
	if pdf.w.enc != nil {
		enc, err := pdf.w.enc.EncryptStream(ref, w)
		if err != nil {
			return nil, err
		}
		w = enc
	}
	for _, filter := range filters {
		w, err = filter.Encode(pdf.Version, w)
		if err != nil {
			return nil, err
		}

		name, parms, err := filter.Info(pdf.Version)
		if err != nil {
			return nil, err
		}
		appendFilter(streamDict, name, parms)
	}

	pdf.inStream = true
	return w, nil
}

func appendFilter(dict Dict, name Name, parms Dict) {
	switch filter := dict["Filter"].(type) {
	case Name:
		dict["Filter"] = Array{filter, name}
		p0, _ := dict["DecodeParms"].(Dict)
		if len(p0)+len(parms) > 0 {
			dict["DecodeParms"] = Array{p0, parms}
		}

	case Array:
		dict["Filter"] = append(filter, name)
		pp, _ := dict["DecodeParms"].(Array)
		needsParms := len(parms) > 0
		for i := 0; i < len(pp) && !needsParms; i++ {
			pi, _ := pp[i].(Dict)
			needsParms = len(pi) > 0
		}
		if needsParms {
			for len(pp) < len(filter) {
				pp = append(pp, nil)
			}
			pp := pp[:len(filter)]
			dict["DecodeParms"] = append(pp, parms)
		}

	default:
		dict["Filter"] = name
		if len(parms) > 0 {
			dict["DecodeParms"] = parms
		}
	}
}

// TODO(voss): find a better name for this
type Putter interface {
	Alloc() Reference
	Put(ref Reference, obj Object) error
	WriteCompressed(refs []Reference, objects ...Object) error
	OpenStream(ref Reference, dict Dict, filters ...Filter) (io.WriteCloser, error)
}

type streamWriter struct {
	parent     *Writer
	streamDict Dict
	ref        Reference
	started    bool
	startPos   int64
	length     *Placeholder
	buf        []byte
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if !w.started {
		if len(w.buf)+len(p) < 1024 {
			w.buf = append(w.buf, p...)
			return len(p), nil
		}

		err := w.startWriting()
		if err != nil {
			return 0, err
		}
	}

	return w.parent.w.Write(p)
}

func (w *streamWriter) startWriting() error {
	_, err := fmt.Fprintf(w.parent.w, "%d %d obj\n",
		w.ref.Number(), w.ref.Generation())
	if err != nil {
		return err
	}
	err = w.streamDict.PDF(w.parent.w)
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
	var length Integer
	if w.started {
		length = Integer(w.parent.w.pos - w.startPos)
		err := w.length.Set(length)
		if err != nil {
			return err
		}
	} else {
		length = Integer(len(w.buf))
		err := w.length.Set(length)
		if err != nil {
			return err
		}
		err = w.startWriting()
		if err != nil {
			return err
		}
	}

	if l, isInteger := w.streamDict["Length"].(Integer); isInteger && l != length {
		return fmt.Errorf("stream length mismatch: %d (specified) != %d (actual)",
			l, length)
	}

	_, err := w.Write([]byte("\nendstream\nendobj\n"))
	if err != nil {
		return err
	}

	w.parent.inStream = false
	for _, fn := range w.parent.afterStream {
		err = fn(w.parent)
		if err != nil {
			return err
		}
	}
	w.parent.afterStream = nil

	return nil
}

// A Placeholder is a space reserved in a PDF file that can later be filled
// with a value.  One common use case is to store the length of compressed
// content in a PDF stream dictionary.  To create Placeholder objects,
// use the [Writer.NewPlaceholder] method.
type Placeholder struct {
	value []byte
	size  int

	pdf *Writer
	pos []int64
	ref Reference
}

// NewPlaceholder creates a new placeholder for a value which is not yet known.
// The argument size must be an upper bound to the length of the replacement
// text.  Once the value becomes known, it can be filled in using the
// [Placeholder.Set] method.
func NewPlaceholder(pdf *Writer, size int) *Placeholder {
	return &Placeholder{
		size: size,
		pdf:  pdf,
	}
}

// PDF implements the [Object] interface.
func (x *Placeholder) PDF(w io.Writer) error {
	// method 1: If the value is already known, we can just write it to the
	// file.
	if x.value != nil {
		_, err := w.Write(x.value)
		return err
	}

	// method 2: If we can seek, write whitespace for now and fill in
	// the actual value later.
	if _, ok := x.pdf.origW.(io.WriteSeeker); ok {
		x.pos = append(x.pos, x.pdf.w.pos)

		buf := bytes.Repeat([]byte{' '}, x.size)
		_, err := w.Write(buf)
		return err
	}

	// method 3: If all else fails, use an indirect reference.
	x.ref = x.pdf.Alloc()
	buf := &bytes.Buffer{}
	err := x.ref.PDF(buf)
	if err != nil {
		return err
	}
	x.value = buf.Bytes()
	_, err = w.Write(x.value)
	return err
}

// Set fills in the value of the placeholder object.  This should be called
// as soon as possible after the value becomes known.
func (x *Placeholder) Set(val Object) error {
	if x.ref != 0 {
		pdf := x.pdf
		if pdf.inStream {
			pdf.afterStream = append(pdf.afterStream, func(w *Writer) error {
				err := w.Put(x.ref, val)
				if err != nil {
					return fmt.Errorf("Placeholder.Set (afterstream): %w", err)
				}
				return nil
			})
		} else {
			err := pdf.Put(x.ref, val)
			if err != nil {
				return fmt.Errorf("Placeholder.Set: %w", err)
			}
			return nil
		}
	}

	buf := bytes.NewBuffer(make([]byte, 0, x.size))
	err := val.PDF(buf)
	if err != nil {
		return err
	}
	if buf.Len() > x.size {
		return errors.New("Placeholder: replacement text too long")
	}
	x.value = buf.Bytes()

	if len(x.pos) == 0 {
		return nil
	}

	x.pdf.w.Flush()

	fill := x.pdf.origW.(io.WriteSeeker)
	currentPos, err := fill.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	for _, pos := range x.pos {
		_, err = fill.Seek(pos, io.SeekStart)
		if err != nil {
			return err
		}
		_, err = fill.Write(x.value)
		if err != nil {
			return err
		}
	}

	_, err = fill.Seek(currentPos, io.SeekStart)
	return err
}

// CheckVersion checks whether the PDF file being written has version
// minVersion or later.  If the version is new enough, nil is returned.
// Otherwise a [VersionError] for the given operation is returned.
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

	ref Reference
	enc *encryptInfo
}

func (w *posWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.pos += int64(n)
	return n, err
}

func (w *posWriter) Flush() error {
	return w.w.Flush()
}
