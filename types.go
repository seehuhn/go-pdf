package pdflib

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// Object represents an object in a PDF file.
type Object interface {
	PDF() []byte
}

// Bool represents a boolean value in a PDF file.
type Bool bool

// PDF implements the Object interface
func (x Bool) PDF() []byte {
	if x {
		return []byte("true")
	}
	return []byte("false")
}

// Integer represents an integer constant in a PDF file.
type Integer int64

// PDF implements the Object interface
func (x Integer) PDF() []byte {
	return []byte(strconv.FormatInt(int64(x), 10))
}

// Real represents an real number in a PDF file.
type Real float64

// PDF implements the Object interface
func (x Real) PDF() []byte {
	return []byte(strconv.FormatFloat(float64(x), 'f', -1, 64))
}

// String represents a string constant in a PDF file.
type String string

// PDF implements the Object interface
func (x String) PDF() []byte {
	l := []byte(x)

	var funny []int
	for i, c := range l {
		if c == '\r' || c == '\n' || c == '\t' {
			continue
		}
		if c < 32 || c >= 127 || c == '(' || c == ')' || c == '\\' {
			funny = append(funny, i)
		}
	}
	n := len(l)

	// TODO(voss): don't escape brackets if they are balanced

	buf := &bytes.Buffer{}
	if n+2*len(funny) <= 2*n {
		buf.WriteString("(")
		pos := 0
		for _, i := range funny {
			if pos < i {
				buf.Write(l[pos:i])
			}
			c := l[i]
			switch c {
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

	return buf.Bytes()
}

// Name represents a name in a PDF file.
type Name string

// PDF implements the Object interface
func (x Name) PDF() []byte {
	l := []byte(x)

	var funny []int
	for i, c := range l {
		if isSpace[c] || isDelimiter[c] || c < 0x21 || c > 0x7e {
			funny = append(funny, i)
		}
	}
	n := len(l)

	buf := &bytes.Buffer{}
	buf.WriteString("/")
	pos := 0
	for _, i := range funny {
		if pos < i {
			buf.Write(l[pos:i])
		}
		c := l[i]
		fmt.Fprintf(buf, "#%02x", c)
		pos = i + 1
	}
	if pos < n {
		buf.Write(l[pos:n])
	}

	return buf.Bytes()
}

// Array represent an array in a PDF file.
type Array []Object

// PDF implements the Object interface
func (x Array) PDF() []byte {
	buf := &bytes.Buffer{}
	buf.WriteByte('[')
	for i, val := range x {
		if i > 0 {
			buf.WriteByte(' ') // TODO(voss): use '\n' here?
		}
		buf.Write(val.PDF())
	}
	buf.WriteByte(']')
	return buf.Bytes()
}

// Dict represent a Dictionary object in a PDF file.
// TODO(voss): any chance we can remove the struct?
type Dict struct {
	Data map[Name]Object
	Ref  *Reference
}

// PDF implements the Object interface
func (x *Dict) PDF() []byte {
	var keys []string
	for key := range x.Data {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)

	buf := &bytes.Buffer{}
	buf.WriteString("<<")
	for _, key := range keys {
		name := Name(key)
		buf.WriteString("\n")
		buf.Write(name.PDF())
		buf.WriteString(" ")
		buf.Write(x.Data[name].PDF())
	}
	buf.WriteString("\n>>")
	return buf.Bytes()
}

// Stream represent a stream object in a PDF file.
type Stream struct {
	Dict
	R io.Reader
}

// PDF implements the Object interface
func (x *Stream) PDF() []byte {
	buf := &bytes.Buffer{}
	buf.Write(x.Dict.PDF())
	buf.WriteString("\nstream\n")
	io.Copy(buf, x.R)
	buf.WriteString("\nendstream")
	return buf.Bytes()
}

// Decode returns a reader for the decoded stream data.
func (x *Stream) Decode() io.Reader {
	r := x.R
	filter := x.Dict.Data["Filter"]
	param := x.Dict.Data["DecodeParms"]
	switch f := filter.(type) {
	case nil:
		// pass
	case Array:
		pa, ok := param.(Array)
		if len(pa) != len(f) {
			ok = false
		}
		for i, name := range f {
			var pi Object
			if ok {
				pi = pa[i]
			}
			r = applyFilter(r, name, pi)
		}
	default:
		r = applyFilter(r, f, param)
	}
	return r
}

// Reference represents an indirect object in a PDF file.
type Reference struct {
	Index      int64 // TODO(voss): use int and uint16
	Generation uint16
}

// PDF implements the Object interface
func (x *Reference) PDF() []byte {
	return []byte(fmt.Sprintf("%d %d R", x.Index, x.Generation))
}

func format(x Object) string {
	if x == nil {
		return "null"
	}
	return string(x.PDF())
}
