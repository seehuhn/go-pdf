package pdflib

import (
	"fmt"
	"io"
)

// Write serialises a PDF tree and writes it to w.
func Write(w io.Writer, catalog, info Object, ver PDFVersion) error {
	pdf := &writer{
		w:       w, // TODO(voss): use a wrapper which updates pos
		nextRef: 1,
		xref:    make(map[int64]int64),
	}

	l := fmt.Sprintf("%%PDF-1.%d\n%%\x80\x80\x80\x80\n", ver)
	err := pdf.WriteString(l)
	if err != nil {
		return err
	}

	err = pdf.WriteIndirect(info)
	if err != nil {
		return err
	}

	err = pdf.WriteIndirect(catalog)
	if err != nil {
		return err
	}

	xrefPos := pdf.pos
	pdf.Writef("xref\n0 %d\n0000000000 65535 f\r\n", pdf.nextRef)
	for i := int64(1); i < pdf.nextRef; i++ {
		pdf.Writef("%010d 00000 n\r\n", pdf.xref[i])
	}

	pdf.WriteString("trailer\n")
	pdf.WriteDirect(&Dict{
		Data: map[Name]Object{
			"Type": Name("XRef"),
			"Size": Integer(pdf.nextRef),
			"Info": info,
			"Root": catalog,
		},
	}, true)
	pdf.Writef("\nstartxref\n%d\n%%%%EOF\n", xrefPos)

	return nil
}

type writer struct {
	w       io.Writer
	pos     int64
	nextRef int64
	xref    map[int64]int64
}

func (pdf *writer) WriteString(s string) error {
	n, err := pdf.w.Write([]byte(s))
	if err != nil {
		return err
	}
	pdf.pos += int64(n)
	return nil
}

func (pdf *writer) Writef(format string, args ...interface{}) error {
	s := fmt.Sprintf(format, args...)
	return pdf.WriteString(s)
}

func (pdf *writer) WriteIndirect(obj Object) error {
	switch obj := obj.(type) {
	case *Dict:
		if obj.Ref != nil {
			return nil
		}
		ref := pdf.nextRef
		pdf.nextRef++
		obj.Ref = &Reference{ref, 0}

		for _, val := range obj.Data {
			err := pdf.WriteIndirect(val)
			if err != nil {
				return err
			}
		}

		pdf.xref[ref] = pdf.pos
		pdf.Writef("%d 0 obj\n", ref)
		pdf.WriteDirect(obj, true)
		pdf.WriteString("\nendobj\n")
	case *Stream: // TODO(voss): any way to avoid duplications with PDFDict?
		if obj.Ref != nil {
			return nil
		}
		ref := pdf.nextRef
		pdf.nextRef++
		obj.Ref = &Reference{ref, 0}

		for _, val := range obj.Data {
			err := pdf.WriteIndirect(val)
			if err != nil {
				return err
			}
		}

		pdf.xref[ref] = pdf.pos
		pdf.Writef("%d 0 obj\n", ref)
		pdf.WriteDirect(obj, true)
		pdf.WriteString("\nendobj\n")
	case Array:
		for _, val := range obj {
			err := pdf.WriteIndirect(val)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (pdf *writer) WriteDirect(obj Object, forceInline bool) (Object, error) {
	// TODO(voss): use the .PDF() methods
	switch obj := obj.(type) {
	case nil:
		pdf.WriteString("null")
	case Bool:
		if obj {
			pdf.WriteString("true")
		} else {
			pdf.WriteString("false")
		}
	case Integer:
		pdf.Writef("%d", obj)
	case Real:
		pdf.Writef("%f", obj) // TODO(voss): follow the spec here
	case String:
		pdf.Writef("(%s)", obj) // TODO(voss): implement this
	case Name:
		pdf.Writef("/%s", obj) // TODO(voss): implement this
	case Array:
		pdf.WriteString("[")
		for i, val := range obj {
			if i > 0 {
				pdf.WriteString("\n")
			}
			pdf.WriteDirect(val, false)
		}
		pdf.WriteString("]")
	case *Dict:
		if ref := obj.Ref; ref != nil && !forceInline {
			pdf.Writef("%d %d R", ref.Index, ref.Generation)
		} else {
			pdf.WriteString("<<")
			for key, val := range obj.Data {
				pdf.WriteString("\n")
				pdf.WriteDirect(key, false)
				pdf.WriteString(" ")
				pdf.WriteDirect(val, false)
			}
			pdf.WriteString("\n>>")
		}
	case *Stream:
		if ref := obj.Ref; ref != nil && !forceInline {
			pdf.Writef("%d %d R", ref.Index, ref.Generation)
		} else {
			pdf.WriteString("<<")
			for key, val := range obj.Data {
				pdf.WriteString("\n")
				pdf.WriteDirect(key, false)
				pdf.WriteString(" ")
				pdf.WriteDirect(val, false)
			}
			pdf.WriteString("\n>>\nstream\n")
			// TODO(voss): check that the length is correct?
			n, err := io.Copy(pdf.w, obj.R)
			if err != nil {
				return nil, err
			}
			pdf.pos += int64(n)
			pdf.WriteString("\nendstream")
		}
	default:
		panic("not implemented")
	}
	return nil, nil
}
