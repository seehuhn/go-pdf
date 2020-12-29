package pdflib

import (
	"fmt"
	"io"
)

// Writer represents a PDF file open for writing.
type Writer struct {
	w       *posWriter
	ver     PDFVersion
	nextRef int
	xref    map[int]*xRefEntry

	info    Dict
	catalog Dict
	pages   *Indirect
}

// NewWriter prepares a PDF file for writing.
func NewWriter(w io.Writer, ver PDFVersion) (*Writer, error) {
	pdf := &Writer{
		w:       &posWriter{w: w},
		ver:     ver,
		nextRef: 1,
		xref:    make(map[int]*xRefEntry),

		catalog: make(Dict),
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

// Close closes the Writer, flushing any unwritten data to the underlying
// io.Writer, but does not close the underlying io.Writer.
func (pdf *Writer) Close() error {
	pages, err := pdf.WriteObject(pdf.pages)
	if err != nil {
		return err
	}

	// page 73
	pdf.catalog["Type"] = Name("Catalog")
	pdf.catalog["Pages"] = pages
	root, err := pdf.WriteObject(pdf.catalog)
	if err != nil {
		return err
	}

	xrefDict := Dict{
		"Type": Name("XRef"),
		"Size": Integer(pdf.nextRef),
		"Root": root, // required, indirect (page 43)
		// "ID" - optional (required for Encrypted), PDF1.1 (page 43)
	}
	if pdf.info != nil {
		info, err := pdf.WriteObject(pdf.info)
		if err != nil {
			return err
		}
		xrefDict["Info"] = info // optional, indirect
	}

	xrefPos := pdf.w.pos
	_, err = fmt.Fprintf(pdf.w, "xref\n0 %d\n", pdf.nextRef)
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

// WriteObject writes an object to the PDF file.  The returned reference
// must be used to refer to this object from other parts of the file.
func (pdf *Writer) WriteObject(obj Object) (*Reference, error) {
	pos := pdf.w.pos

	ind, ok := obj.(*Indirect)
	if !ok {
		ind = &Indirect{
			Reference: Reference{
				Number:     pdf.nextRef,
				Generation: 0,
			},
			Obj: obj,
		}
		pdf.nextRef++
	}

	err := ind.PDF(pdf.w)
	if err != nil {
		return nil, err
	}

	pdf.xref[ind.Reference.Number] = &xRefEntry{Pos: pos, Generation: 0}

	return &ind.Reference, nil
}

// ReserveNumber allocates an object number for an indirect object.
func (pdf *Writer) ReserveNumber(obj Object) (*Indirect, *Reference) {
	res := &Indirect{
		Reference: Reference{
			Number:     pdf.nextRef,
			Generation: 0,
		},
		Obj: obj,
	}
	pdf.nextRef++

	return res, &res.Reference
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
