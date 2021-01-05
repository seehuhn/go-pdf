package pdf

import (
	"errors"
	"fmt"
	"io"
)

// Writer represents a PDF file open for writing.
type Writer struct {
	w       *posWriter
	ver     Version
	nextRef int
	xref    map[int]*xRefEntry
}

// NewWriter prepares a PDF file for writing.
func NewWriter(w io.Writer, ver Version) (*Writer, error) {
	pdf := &Writer{
		w:       &posWriter{w: w},
		ver:     ver,
		nextRef: 1,
		xref:    make(map[int]*xRefEntry),
	}
	pdf.xref[0] = &xRefEntry{
		Pos:        -1,
		Generation: 65535,
	}

	err := ver.PDF(pdf.w)
	if err != nil {
		return nil, err
	}

	return pdf, nil
}

// Close closes the Writer, flushing any unwritten data to the underlying
// io.Writer, but does not close the underlying io.Writer.
func (pdf *Writer) Close(catalog *Reference, info *Reference) error {
	if catalog == nil {
		return errors.New("missing /Catalog")
	}

	xrefDict := Dict{
		"Type": Name("XRef"), // only needed for new-style xref
		"Size": Integer(pdf.nextRef),
		"Root": catalog, // required, indirect (page 43)
		// "ID" - optional (required for Encrypted), PDF1.1 (page 43)
	}
	if info != nil {
		xrefDict["Info"] = info
	}

	xrefPos := pdf.w.pos
	_, err := fmt.Fprintf(pdf.w, "xref\n0 %d\n", pdf.nextRef)
	if err != nil {
		return err
	}
	for i := 0; i < pdf.nextRef; i++ {
		entry := pdf.xref[i]
		if entry != nil && entry.InStream != nil {
			panic("object streams not supported") // TODO(voss)
		}
		if entry != nil && entry.Pos >= 0 {
			_, err = fmt.Fprintf(pdf.w, "%010d %05d n\r\n",
				entry.Pos, entry.Generation)
		} else {
			// free object
			_, err = pdf.w.Write([]byte("0000000000 65535 f\r\n"))
		}
		if err != nil {
			return err
		}
	}

	_, err = pdf.w.Write([]byte("trailer\n"))
	if err != nil {
		return err
	}
	err = xrefDict.PDF(pdf.w)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(pdf.w, "\nstartxref\n%d\n%%%%EOF\n", xrefPos)
	if err != nil {
		return err
	}

	pdf.w = nil
	return nil
}

// WriteIndirect writes an object to the PDF file, as an indirect object.  The
// returned reference must be used to refer to this object from other parts of
// the file.
func (pdf *Writer) WriteIndirect(obj Object, ref *Reference) (*Reference, error) {
	pos := pdf.w.pos

	ind, ok := obj.(*Indirect)
	if ok {
		if ref != nil && *ref != ind.Reference {
			panic("inconsistent references")
		}
		if ind.Number >= pdf.nextRef {
			pdf.nextRef = ind.Number + 1
		}
		ref = &ind.Reference
	} else {
		ind = &Indirect{Obj: obj}
	}

	if ref == nil {
		ref = pdf.Alloc()
	}
	ind.Reference = *ref

	err := ind.PDF(pdf.w)
	if err != nil {
		return nil, err
	}

	pdf.xref[ind.Reference.Number] = &xRefEntry{Pos: pos, Generation: ind.Reference.Generation}

	return &ind.Reference, nil
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
