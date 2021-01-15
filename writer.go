package pdf

import (
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
// io.Writer.  If the underlying io.Writer as a Close() method, this writer is
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

// WriteIndirect writes an object to the PDF file, as an indirect object.  The
// returned reference can be used to refer to this object from other parts of
// the file.
func (pdf *Writer) WriteIndirect(obj Object, ref *Reference) (*Reference, error) {
	pos := pdf.w.pos

	if ref == nil {
		ref = pdf.Alloc()
	} else {
		_, seen := pdf.xref[ref.Number]
		if seen {
			return nil, errors.New("object already written")
		}
	}

	if obj == nil {
		// missing objects are treated as null
		pos = -1
	} else {
		_, err := fmt.Fprintf(pdf.w, "%d %d obj\n", ref.Number, ref.Generation)
		if err != nil {
			return nil, err
		}
		err = obj.PDF(pdf.w)
		if err != nil {
			return nil, err
		}
		_, err = pdf.w.Write([]byte("\nendobj\n"))
		if err != nil {
			return nil, err
		}
	}

	pdf.xref[ref.Number] = &xRefEntry{Pos: pos, Generation: ref.Generation}

	return ref, nil
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

type posWriter struct {
	w   io.Writer
	pos int64
}

func (w *posWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.pos += int64(n)
	return n, err
}
