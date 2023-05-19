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

	autoclose map[Reference]Resource
}

type allocatedObject struct {
	ref Reference
	obj Object
}

type Resource interface {
	// Write writes the resource to the PDF file.  No changes can be
	// made to the resource after it has been written.
	Close() error

	Reference() Reference
}

// Create creates a PDF file with the given name and opens it for output. If a
// file with the same name already exists, it will be overwritten.
//
// After writing the content to the file, [Writer.Close] must be called to
// write the PDF trailer and close the underlying file.
func Create(name string, opt *WriterOptions) (*Writer, error) {
	fd, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	pdf, err := NewWriter(fd, opt)
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
func NewWriter(w io.Writer, opt *WriterOptions) (*Writer, error) {
	if opt == nil {
		opt = defaultWriterOptions
	}

	version := opt.Version
	if version == 0 {
		version = defaultWriterOptions.Version
	}
	versionString, err := version.ToString() // check for valid version
	if err != nil {
		return nil, err
	}

	useEncryption := opt.UserPassword != "" || opt.OwnerPassword != ""
	if useEncryption && version == V1_0 {
		return nil, &VersionError{Operation: "PDF encryption", Earliest: V1_1}
	}
	needID := opt.ID != nil || useEncryption || version >= V2_0
	if needID && version == V1_0 {
		return nil, &VersionError{Operation: "PDF file identifiers", Earliest: V1_1}
	}

	trailer := Dict{}

	var ID [][]byte
	if needID {
		for _, id := range opt.ID {
			if len(id) < 16 {
				return nil, errInvalidID
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
		if version >= V2_0 {
			cf = &cryptFilter{
				Cipher: cipherAES,
				Length: 256,
			}
			V = 5
		} else if version >= V1_6 {
			cf = &cryptFilter{
				Cipher: cipherAES,
				Length: 128,
			}
			V = 4
		} else if version >= V1_4 {
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

		encryptDict, err := enc.AsDict(version)
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

	pdf := &Writer{
		meta: MetaInfo{
			Version: version,
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

		autoclose: make(map[Reference]Resource),
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
	if pdf.inStream {
		return errors.New("Close() while stream is open")
	}

	var rr []Resource
	for _, r := range pdf.autoclose {
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

	trailer := pdf.meta.Trailer

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
	}

	// don't encrypt the encryption dictionary and the xref dict
	pdf.w.enc = nil

	// write the cross reference table and trailer
	xRefPos := pdf.w.pos
	trailer["Size"] = Integer(pdf.nextRef)
	if pdf.meta.Version < V1_5 {
		err = pdf.writeXRefTable(trailer)
	} else {
		err = pdf.writeXRefStream(trailer)
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

	// Make sure we don't accidentally write beyond the end of file.
	pdf.w = nil

	return nil
}

func (pdf *Writer) AutoClose(res Resource) {
	ref := res.Reference()
	pdf.autoclose[ref] = res
}

func (pdf *Writer) GetMeta() *MetaInfo {
	return &pdf.meta
}

// Alloc allocates an object number for an indirect object.
func (pdf *Writer) Alloc() Reference {
	res := NewReference(pdf.nextRef, 0)
	pdf.nextRef++
	return res
}

// Put writes an indirect object to the PDF file, using the given reference.
func (pdf *Writer) Put(ref Reference, obj Object) error {
	if pdf.inStream {
		pdf.afterStream = append(pdf.afterStream, allocatedObject{ref, obj})
		return nil
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

	if pdf.meta.Version < V1_5 {
		// Object streams are only availble in PDF version 1.5 and higher.
		for i, obj := range objects {
			err := pdf.Put(refs[i], obj)
			if err != nil {
				return fmt.Errorf("Writer.WriteCompressed (V<1.5): %w", err)
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

func writeObject(w io.Writer, obj Object) error {
	if obj == nil {
		_, err := w.Write([]byte("null"))
		return err
	}
	return obj.PDF(w)
}
