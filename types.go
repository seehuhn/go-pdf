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
	PDF(w io.Writer) error
}

// Bool represents a boolean value in a PDF file.
type Bool bool

// PDF implements the Object interface.
func (x Bool) PDF(w io.Writer) error {
	var s string
	if x {
		s = "true"
	} else {
		s = "false"
	}
	_, err := w.Write([]byte(s))
	return err
}

// Integer represents an integer constant in a PDF file.
// TODO(voss): change this to `int`?
type Integer int64

// PDF implements the Object interface.
func (x Integer) PDF(w io.Writer) error {
	s := strconv.FormatInt(int64(x), 10)
	_, err := w.Write([]byte(s))
	return err
}

// Real represents an real number in a PDF file.
type Real float64

// PDF implements the Object interface.
func (x Real) PDF(w io.Writer) error {
	s := strconv.FormatFloat(float64(x), 'f', -1, 64)
	_, err := w.Write([]byte(s))
	return err
}

// String represents a string constant in a PDF file.
type String string

// PDF implements the Object interface.
func (x String) PDF(w io.Writer) error {
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
		fmt.Fprintf(buf, "<%x>", l)
	}

	_, err := w.Write(buf.Bytes())
	return err
}

// Name represents a name in a PDF file.
type Name string

// PDF implements the Object interface.
func (x Name) PDF(w io.Writer) error {
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

	_, err := w.Write(buf.Bytes())
	return err
}

// Array represent an array in a PDF file.
type Array []Object

// PDF implements the Object interface.
func (x Array) PDF(w io.Writer) error {
	_, err := w.Write([]byte("["))
	if err != nil {
		return err
	}
	for i, val := range x {
		if i > 0 {
			_, err := w.Write([]byte(" ")) // TODO(voss): use '\n' here?
			if err != nil {
				return err
			}
		}
		if val == nil {
			_, err = w.Write([]byte("null"))
		} else {
			err = val.PDF(w)
		}
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte("]"))
	return err
}

// Dict represent a Dictionary object in a PDF file.
type Dict map[Name]Object

// PDF implements the Object interface.
func (x Dict) PDF(w io.Writer) error {
	if x == nil {
		_, err := w.Write([]byte("null"))
		return err
	}

	var keys []string
	for key := range x {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)

	_, err := w.Write([]byte("<<"))
	if err != nil {
		return err
	}

	for _, key := range keys {
		name := Name(key)
		val := x[name]
		if val == nil {
			continue
		}

		_, err = w.Write([]byte("\n"))
		if err != nil {
			return err
		}
		err = name.PDF(w)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(" "))
		if err != nil {
			return err
		}
		err = val.PDF(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte("\n>>"))
	return err
}

// Stream represent a stream object in a PDF file.
type Stream struct {
	Dict
	R io.Reader
}

// PDF implements the Object interface.
func (x *Stream) PDF(w io.Writer) error {
	err := x.Dict.PDF(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("\nstream\n"))
	if err != nil {
		return err
	}
	_, err = io.Copy(w, x.R)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("\nendstream"))
	return err
}

// Decode returns a reader for the decoded stream data.
func (x *Stream) Decode() io.Reader {
	r := x.R
	filter := x.Dict["Filter"]
	param := x.Dict["DecodeParms"]
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

// Reference represents a reference to an indirect object in a PDF file.
type Reference struct {
	Number     int
	Generation uint16
}

// PDF implements the Object interface.
func (x *Reference) PDF(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%d %d R", x.Number, x.Generation)
	return err
}

// Indirect represents an indirect object in a PDF file.
type Indirect struct {
	Reference
	Obj Object
}

// PDF implements the Object interface.
func (x *Indirect) PDF(w io.Writer) error {
	if x.Obj == nil {
		// missing objects are treated as null
		return nil
	}
	_, err := fmt.Fprintf(w, "%d %d obj\n", x.Number, x.Generation)
	if err != nil {
		return err
	}
	err = x.Obj.PDF(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("\nendobj\n"))
	return err
}

func format(x Object) string {
	buf := &bytes.Buffer{}
	if x == nil {
		buf.WriteString("null")
	} else {
		_ = x.PDF(buf)
	}
	return buf.String()
}
