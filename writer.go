package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

// Writer represents a PDF file open for writing.
type Writer struct {
	PDFVersion Version

	w       *posWriter
	xref    map[int]*xRefEntry
	ver     Version
	nextRef int

	inStream bool
}

// NewWriter prepares a PDF file for writing.
func NewWriter(w io.Writer, ver Version) (*Writer, error) {
	pdf := &Writer{
		PDFVersion: ver,

		w:       &posWriter{w: w},
		ver:     ver,
		nextRef: 1,
		xref:    make(map[int]*xRefEntry),
	}
	pdf.xref[0] = &xRefEntry{
		Pos:        -1,
		Generation: 65535,
	}

	_, err := fmt.Fprintf(pdf.w, "%%PDF-1.%d\n%%\x80\x80\x80\x80\n", ver)
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
	return NewWriter(fd, V1_7)
}

// Close closes the Writer, flushing any unwritten data to the underlying
// io.Writer.  If the underlying io.Writer has a Close() method, this writer is
// also closed.
func (pdf *Writer) Close(catalog *Reference, info *Reference) error {
	if catalog == nil {
		return errors.New("missing /Catalog")
	}

	xRefDict := Dict{
		"Size": Integer(pdf.nextRef),
		"Root": catalog, // required, indirect (page 43)
		// "Encrypt" - optional, PDF1.1 (page 43)
		// "ID" - optional (required for Encrypted), PDF1.1 (page 43)
	}
	if info != nil {
		xRefDict["Info"] = info
	}

	xRefPos := pdf.w.pos
	var err error
	if pdf.PDFVersion < V1_5 {
		err = pdf.writeXRefTable(xRefDict)
	} else {
		err = pdf.writeXRefStream(xRefDict)
	}
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(pdf.w, "\nstartxref\n%d\n%%%%EOF\n", xRefPos)
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
		return nil, errors.New("Write() called while stream is open")
	}

	pos := pdf.w.pos

	if ref == nil {
		ref = pdf.Alloc()
	} else {
		_, seen := pdf.xref[ref.Number]
		if seen {
			return nil, errors.New("object already written")
		}
	}

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

// StreamOptions describes how Writer.OpenStream() processes the stream
// data while writing.
type StreamOptions struct {
	Filters []*FilterInfo
}

// OpenStream adds a PDF Stream to the file and returns an io.Writer which can
// be used to add the stream's data.  No other objects can be added to the file
// until the stream is closed.
func (pdf *Writer) OpenStream(dict Dict, ref *Reference, opt *StreamOptions) (io.WriteCloser, *Reference, error) {
	if ref == nil {
		ref = pdf.Alloc()
	} else {
		_, seen := pdf.xref[ref.Number]
		if seen {
			return nil, nil, errors.New(ref.String() + " already written")
		}
	}
	pdf.xref[ref.Number] = &xRefEntry{Pos: pdf.w.pos, Generation: ref.Generation}

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
