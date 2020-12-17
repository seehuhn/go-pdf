package pdflib

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// PDFObject represents an object in a PDF file.
type PDFObject interface{}

// PDFBool represents a boolean value in a PDF file.
type PDFBool bool

// PDFInt represents an integer constant in a PDF file.
type PDFInt int64

// PDFReal represents an real number in a PDF file.
type PDFReal float64

// PDFString represents a string constant in a PDF file.
type PDFString string

func (s PDFString) String() string {
	l := []byte(s)

	var funny []int
	for i, c := range l {
		if c == '\r' || c == '\n' {
			continue
		}
		if c < 32 || c >= 80 || c == '(' || c == ')' || c == '\\' {
			funny = append(funny, i)
		}
	}
	n := len(s)

	buf := &strings.Builder{}
	if n+2*len(funny) < 2*n {
		buf.WriteString("(")
		pos := 0
		for _, i := range funny {
			if pos < i {
				buf.Write(l[pos:i])
			}
			c := l[i]
			switch c {
			case '\t':
				buf.WriteString(`\t`)
			case '\b':
				buf.WriteString(`\b`)
			case '\f':
				buf.WriteString(`\f`)
			case '(':
				buf.WriteString(`\(`)
			case ')':
				buf.WriteString(`\)`)
			case '\\':
				buf.WriteString(`\\`)
			default:
				fmt.Fprintf(buf, `\%03o`, c)
			}
			pos = i + 1
		}
		if pos < n {
			buf.Write(l[pos:n])
		}
		buf.WriteString(")")
	} else {
		fmt.Fprintf(buf, "<%02x>", l)
	}

	return buf.String()
}

// PDFName represents a name in a PDF file.
type PDFName string

// PDFArray represent an array in a PDF file.
type PDFArray []PDFObject

// PDFDict represent a Dictionary object in a PDF file.
type PDFDict struct {
	Data map[PDFName]PDFObject
	Ref  *PDFReference
}

func (d *PDFDict) String() string {
	var keys []string
	for key := range d.Data {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)

	buf := &strings.Builder{}
	buf.WriteString("<<")
	for _, key := range keys {
		buf.WriteString("\n")
		buf.WriteString(key)
		buf.WriteString(" ")
		fmt.Fprint(buf, d.Data[PDFName(key)])
	}
	buf.WriteString("\n>>")
	return buf.String()
}

// PDFStream represent a stream object in a PDF file.
type PDFStream struct {
	PDFDict
	R io.Reader
}

// PDFReference represents an indirect object in a PDF file.
type PDFReference struct {
	no, gen int64
}
