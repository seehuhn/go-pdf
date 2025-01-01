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
	"slices"
	"strconv"

	"golang.org/x/exp/maps"
)

// WriterOptions allows to influence the way a PDF file is generated.
type WriterOptions struct {
	ID [][]byte

	UserPassword    string
	OwnerPassword   string
	UserPermissions Perm

	// If this flag is true, the writer tries to generate a PDF file which is
	// more human-readable, at the expense of increased file size.
	HumanReadable bool
}

// Writer represents a PDF file open for writing.
// Use [Create] or [NewWriter] to create a new Writer.
type Writer struct {
	meta MetaInfo

	w          *posWriter
	origW      io.Writer
	closeOrigW bool

	xref    map[uint32]*xRefEntry
	nextRef uint32

	inStream    bool
	afterStream []allocatedObject

	outputOptions OutputOptions
}

// TODO(voss): is this more generally useful?
type allocatedObject struct {
	ref Reference
	obj Object
}

// Create creates a PDF file with the given name and opens it for output. If a
// file with the same name already exists, it will be overwritten.
//
// After writing the content to the file, [Writer.Close] must be called to
// write the PDF trailer and close the underlying file.
func Create(name string, v Version, opt *WriterOptions) (*Writer, error) {
	fd, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	pdf, err := NewWriter(fd, v, opt)
	if err != nil {
		return nil, err
	}
	pdf.closeOrigW = true
	return pdf, nil
}

// NewWriter prepares a PDF file for writing, using the provided io.Writer.
//
// After writing the content to the file, [Writer.Close] must be called to
// write the PDF trailer.
func NewWriter(w io.Writer, v Version, opt *WriterOptions) (*Writer, error) {
	if opt == nil {
		opt = &WriterOptions{}
	}

	versionString, err := v.ToString() // check for valid version
	if err != nil {
		return nil, err
	}

	useEncryption := opt.UserPassword != "" || opt.OwnerPassword != ""
	if useEncryption && v == V1_0 {
		return nil, &VersionError{Operation: "PDF encryption", Earliest: V1_1}
	}
	needID := opt.ID != nil || useEncryption || v >= V2_0
	if needID && v == V1_0 {
		return nil, &VersionError{Operation: "PDF file identifiers", Earliest: V1_1}
	}

	trailer := Dict{}

	var ID [][]byte
	if needID {
		if v >= V2_0 {
			for _, id := range opt.ID {
				if len(id) < 16 {
					return nil, errInvalidID
				}
			}
		}
		switch len(opt.ID) {
		case 0:
			id := make([]byte, 16)
			_, err := io.ReadFull(rand.Reader, id)
			if err != nil {
				return nil, err
			}
			ID = [][]byte{id, id}
		case 1:
			id := make([]byte, 16)
			_, err := io.ReadFull(rand.Reader, id)
			if err != nil {
				return nil, err
			}
			ID = [][]byte{opt.ID[0], id}
		default:
			ID = opt.ID[:2]
		}
		trailer["ID"] = Array{String(ID[0]), String(ID[1])}
	}

	var enc *encryptInfo
	if useEncryption {
		var cf *cryptFilter
		var V int
		if v >= V2_0 {
			cf = &cryptFilter{
				Cipher: cipherAES,
				Length: 256,
			}
			V = 5
		} else if v >= V1_6 {
			cf = &cryptFilter{
				Cipher: cipherAES,
				Length: 128,
			}
			V = 4
		} else if v >= V1_4 {
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
		sec, err := createStdSecHandler(ID[0], opt.UserPassword,
			opt.OwnerPassword, opt.UserPermissions, cf.Length, V)
		if err != nil {
			return nil, err
		}
		enc = &encryptInfo{
			sec:  sec,
			stmF: cf,
			strF: cf,
			efF:  cf,
		}

		encryptDict, err := enc.AsDict(v)
		if err != nil {
			return nil, err
		}
		trailer["Encrypt"] = encryptDict
	}

	bufferedW, ok := w.(writeFlusher)
	if !ok {
		bufferedW = bufio.NewWriter(w)
	}

	xref := make(map[uint32]*xRefEntry)
	xref[0] = &xRefEntry{
		Pos:        -1,
		Generation: 65535,
	}

	outOpt := defaultOutputOptions(v)
	if opt.HumanReadable {
		outOpt &= ^(optObjStm | optXRefStream)
		outOpt |= OptPretty
	}

	pdf := &Writer{
		meta: MetaInfo{
			Version: v,
			Catalog: &Catalog{},
			Info:    &Info{},
			ID:      ID,
			Trailer: trailer,
		},

		w: &posWriter{
			w:   bufferedW,
			enc: enc,
		},
		origW: w,

		nextRef: 1,
		xref:    xref,

		outputOptions: outOpt,
	}

	_, err = fmt.Fprintf(pdf.w, "%%PDF-%s\n%%\x80\x80\x80\x80\n", versionString)
	if err != nil {
		return nil, err
	}

	if outOpt.HasAny(OptPretty) {
		_, err := pdf.w.Write([]byte("\n"))
		if err != nil {
			return nil, err
		}
	}

	return pdf, nil
}

// Close closes the Writer, flushing any unwritten data to the underlying
// io.Writer.
func (pdf *Writer) Close() error {
	if pdf.inStream {
		return errors.New("Close() while stream is open")
	}

	trailer := pdf.meta.Trailer.Clone()

	catRef := pdf.Alloc()
	err := pdf.Put(catRef, AsDict(pdf.meta.Catalog))
	if err != nil {
		return fmt.Errorf("failed to write document catalog: %w", err)
	}
	trailer["Root"] = catRef

	infoDict := AsDict(pdf.meta.Info)
	if len(infoDict) > 0 {
		infoRef := pdf.Alloc()
		err := pdf.Put(infoRef, infoDict)
		if err != nil {
			return err
		}
		trailer["Info"] = infoRef
	} else {
		delete(trailer, "Info")
	}

	if pdf.meta.ID != nil {
		trailer["ID"] = Array{String(pdf.meta.ID[0]), String(pdf.meta.ID[1])}
	} else {
		delete(trailer, "ID")
	}

	// don't encrypt the encryption dictionary and the xref dict
	pdf.w.enc = nil

	// write the cross reference table and trailer
	xRefPos := pdf.w.pos
	trailer["Size"] = Integer(pdf.nextRef)
	if pdf.outputOptions.HasAny(optXRefStream) {
		err = pdf.writeXRefStream(trailer)
	} else {
		err = pdf.writeXRefTable(trailer)
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
	if pdf.closeOrigW {
		err = pdf.origW.(io.Closer).Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// GetMeta returns the MetaInfo for the PDF file.
func (pdf *Writer) GetMeta() *MetaInfo {
	return &pdf.meta
}

func (pdf *Writer) GetOptions() OutputOptions {
	return pdf.outputOptions
}

// Alloc allocates an object number for a new indirect object.
func (pdf *Writer) Alloc() Reference {
	res := NewReference(pdf.nextRef, 0)
	pdf.nextRef++
	return res
}

// Get returns the object with the given reference from the PDF file.
//
// If the underlying io.Writer does not support seeking, Get will return an
// error.
func (pdf *Writer) Get(ref Reference, canObjStm bool) (obj Native, err error) {
	r, ok := pdf.origW.(io.ReadSeeker)
	if !ok {
		return nil, errors.New("Get() not supported by the underlying io.Writer")
	}

	entry := pdf.xref[ref.Number()]
	if entry.IsFree() || entry.Generation != ref.Generation() {
		return nil, nil
	}

	if entry.InStream != 0 {
		if !canObjStm {
			return nil, &MalformedFileError{
				Err: errors.New("object in object stream"),
				Loc: []string{"object " + ref.String()},
			}
		}
		getInt := safeGetInteger(pdf, r, true)
		return getFromObjStm(pdf, ref.Number(), entry.InStream, getInt, pdf.w.enc)
	}

	err = pdf.w.w.Flush()
	if err != nil {
		return nil, err
	}

	savedPos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, e2 := r.Seek(savedPos, io.SeekStart)
		if err == nil {
			err = e2
		}
	}()

	s, err := pdf.scannerFrom(entry.Pos, canObjStm)
	if err != nil {
		return nil, err
	}

	obj, fileRef, err := s.ReadIndirectObject()
	if err != nil {
		return nil, err
	}
	if ref != fileRef {
		return nil, &MalformedFileError{
			Err: errors.New("xref corrupted"),
			Loc: []string{"object " + ref.String() + "*"},
		}
	}
	return obj, nil
}

func (w *Writer) scannerFrom(pos int64, canObjStm bool) (*scanner, error) {
	r := w.origW.(io.ReadSeeker)
	getInt := safeGetInteger(w, r, canObjStm)
	s := newScanner(r, getInt, w.w.enc)

	_, err := r.Seek(pos, io.SeekStart)
	if err != nil {
		return nil, err
	}
	s.filePos = pos

	return s, nil
}

// Put writes an indirect object to the PDF file, using the given reference.
func (pdf *Writer) Put(ref Reference, obj Object) error {
	if pdf.inStream {
		pdf.afterStream = append(pdf.afterStream, allocatedObject{ref, obj})
		return nil
	}

	if stm, isStream := obj.(*Stream); isStream {
		w, err := pdf.OpenStream(ref, stm.Dict)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, stm.R)
		if err != nil {
			return err
		}
		err = w.Close()
		if err != nil {
			return err
		}
	} else {
		err := pdf.setXRef(ref, &xRefEntry{Pos: pdf.w.pos, Generation: ref.Generation()})
		if err != nil {
			return fmt.Errorf("Writer.Put: %w", err)
		}
		pdf.w.ref = ref

		_, err = fmt.Fprintf(pdf.w, "%d %d obj\n", ref.Number(), ref.Generation())
		if err != nil {
			return err
		}
		err = Format(pdf.w, pdf.outputOptions, obj)
		if err != nil {
			return err
		}
		_, err = pdf.w.Write([]byte("\nendobj\n"))
		if err != nil {
			return err
		}

		if pdf.outputOptions.HasAny(OptPretty) {
			_, err := pdf.w.Write([]byte("\n"))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// WriteCompressed writes a number of objects to the file as a compressed
// object stream.
//
// Object streams are only available for PDF version 1.5 and newer; in case
// object streams are not available, the objects are written directly into the
// PDF file, without compression.
func (pdf *Writer) WriteCompressed(refs []Reference, objects ...Object) error {
	if pdf.inStream {
		return errors.New("WriteCompressed() while stream is open")
	}
	err := checkCompressed(refs, objects)
	if err != nil {
		return err
	}

	if !pdf.outputOptions.HasAny(optObjStm) {
		// If object streams are disabled, write the objects directly.
		for i, obj := range objects {
			err := pdf.Put(refs[i], obj)
			if err != nil {
				return fmt.Errorf("Writer.WriteCompressed (no object streams): %w", err)
			}
		}
		return nil
	}

	sRef := pdf.Alloc()
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
			// We buffer the first N-1 object to determine the starting offsets
			// within the stream.  To reduce memory use, the last object is
			// written separately at the end without buffering.
			err = Format(body, pdf.outputOptions, objects[i])
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
	err = Format(w, pdf.outputOptions, objects[N-1])
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
// be used to add the stream's data.  No other objects can be written to the file
// until the stream is closed.
//
// Filters are specified in order from outermost to innermost.
// When reading, filters are applied in the given order.
// When writing, filters are applied in reverse order.
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
	// modify the caller's dict.
	streamDict := maps.Clone(dict)
	if streamDict == nil {
		streamDict = Dict{}
	}
	if filter, _ := streamDict["Filter"].(Array); len(filter) > 0 {
		streamDict["Filter"] = slices.Clone(filter)
	}
	if decodeParms, _ := streamDict["DecodeParms"].(Array); len(decodeParms) > 0 {
		streamDict["DecodeParms"] = slices.Clone(decodeParms)
	}

	var length *Placeholder
	if _, exists := streamDict["Length"]; !exists {
		length = NewPlaceholder(pdf, 12)
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
		w, err = filter.Encode(pdf.meta.Version, w)
		if err != nil {
			return nil, err
		}

		name, parms, err := filter.Info(pdf.meta.Version)
		if err != nil {
			return nil, err
		}
		appendFilter(streamDict, name, parms)
	}

	pdf.inStream = true
	return w, nil
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
	err = Format(w.parent.w, w.parent.outputOptions, w.streamDict)
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

	if w.parent.outputOptions.HasAny(OptPretty) {
		_, err := w.Write([]byte("\n"))
		if err != nil {
			return err
		}
	}

	w.parent.inStream = false
	for _, pair := range w.parent.afterStream {
		err = w.parent.Put(pair.ref, pair.obj)
		if err != nil {
			return err
		}
	}
	w.parent.afterStream = w.parent.afterStream[:0]

	return nil
}

type writeFlusher interface {
	io.Writer
	Flush() error
}

type posWriter struct {
	w   writeFlusher
	pos int64

	// ref is the reference of the top-level object currently being written.
	// This is needed to derive the key when strings or streams are encrypted.
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
