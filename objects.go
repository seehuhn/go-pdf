// seehuhn.de/go/pdf - support for reading and writing PDF files
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

// Object represents an object in a PDF file.  There are nine native types of
// PDF objects, which implement this interface: Array, Bool, Dict, Integer,
// Name, Real, Reference, Stream, and String.
type Object interface {
	// PDF writes the PDF file representation of the object to w.
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
	if !strings.Contains(s, ".") {
		s = s + "."
	}
	_, err := w.Write([]byte(s))
	return err
}

// String represents a raw string in a PDF file.  The character set encoding,
// if any, is determined by the context.
type String []byte

// PDF implements the Object interface.
func (x String) PDF(w io.Writer) error {
	l := []byte(x)

	if wenc, ok := w.(*posWriter); ok && wenc.enc != nil {
		enc, err := wenc.enc.EncryptBytes(wenc.ref, l)
		if err != nil {
			return err
		}
		l = enc
	}

	level := 0
	for _, c := range l {
		if c == '(' {
			level++
		} else if c == ')' {
			level--
			if level < 0 {
				break
			}
		}
	}
	balanced := level == 0

	var funny []int
	for i, c := range l {
		if c == '\r' || c == '\n' || c == '\t' {
			continue
		}
		if c < 32 || c >= 127 || c == '\\' ||
			!balanced && (c == '(' || c == ')') {
			funny = append(funny, i)
		}
	}
	n := len(l)

	buf := &bytes.Buffer{}
	if 3*len(funny) <= n {
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

// AsTextString interprets x as a PDF "text string" and returns
// the corresponding utf-8 encoded string.
func (x String) AsTextString() string {
	if isUTF16(string(x)) {
		return utf16Decode(x[2:])
	}
	return pdfDocDecode(x)
}

// TextString creates a String object using the "text string" encoding,
// i.e. using either UTF-16BE encoding (with a BOM) or PdfDocEncoding.
func TextString(s string) String {
	rr := []rune(s)
	buf := make([]byte, len(rr))
	for i, r := range rr {
		c, ok := pdfDocEncode(r)
		if ok {
			buf[i] = c
		} else {
			goto useUTF
		}
	}
	return String(buf)

useUTF:
	enc := utf16.Encode(rr)
	buf = make([]byte, 2*len(enc)+2)
	buf[0] = 0xFE
	buf[1] = 0xFF
	for i, c := range enc {
		buf[2*i+2] = byte(c >> 8)
		buf[2*i+3] = byte(c)
	}
	return String(buf)
}

// AsDate converts a PDF date string to a time.Time object.
// If the string does not have the correct format, an error is returned.
func (x String) AsDate() (time.Time, error) {
	s := x.AsTextString()
	if s == "D:" {
		return time.Time{}, nil
	}
	s = strings.ReplaceAll(s, "'", "")

	formats := []string{
		"D:20060102150405-0700",
		"D:20060102150405-07",
		"D:20060102150405Z0000",
		"D:20060102150405Z00",
		"D:20060102150405Z",
		"D:20060102150405",
		"D:200601021504",
		"D:2006010215",
		"D:20060102",
		"D:200601",
		"D:2006",
		time.ANSIC,
	}
	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, errNoDate
}

// Date creates a PDF String object encoding the given date and time.
func Date(t time.Time) String {
	s := t.Format("D:20060102150405-0700")
	k := len(s) - 2
	s = s[:k] + "'" + s[k:]
	return String(s)
}

// Name represents a name in a PDF file.
type Name string

// PDF implements the Object interface.
func (x Name) PDF(w io.Writer) error {
	l := []byte(x)

	var funny []int
	for i, c := range l {
		if isSpace[c] || isDelimiter[c] || c < 0x21 || c > 0x7e || c == '#' {
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

func toName(obj Object) (Name, error) {
	name, ok := obj.(Name)
	if !ok {
		return "", fmt.Errorf("wrong type, expected Name but got %T", obj)
	}
	return name, nil
}

// Array represent an array of objects in a PDF file.
type Array []Object

func (x Array) String() string {
	res := []string{}
	res = append(res, "Array")
	res = append(res, strconv.FormatInt(int64(len(x)), 10)+" elements")
	return "<" + strings.Join(res, ", ") + ">"
}

// PDF implements the Object interface.
func (x Array) PDF(w io.Writer) error {
	_, err := w.Write([]byte("["))
	if err != nil {
		return err
	}
	for i, val := range x {
		if i > 0 {
			_, err := w.Write([]byte(" "))
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

func (x Dict) String() string {
	res := []string{}
	tp, ok := x["Type"].(Name)
	if ok {
		res = append(res, string(tp)+" Dict")
	} else {
		res = append(res, "Dict")
	}
	res = append(res, strconv.FormatInt(int64(len(x)), 10)+" entries")
	return "<" + strings.Join(res, ", ") + ">"
}

// PDF implements the Object interface.
func (x Dict) PDF(w io.Writer) error {
	if x == nil {
		_, err := w.Write([]byte("null"))
		return err
	}

	_, err := w.Write([]byte("<<"))
	if err != nil {
		return err
	}

	var keys []Name
	for key := range x {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool {
		return keys[i] < keys[j]
	})

	for _, name := range keys {
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

func toDict(obj Object) (Dict, error) {
	if obj == nil {
		return nil, nil
	}
	dict, ok := obj.(Dict)
	if !ok {
		return nil, fmt.Errorf("wrong type, expected Dict but got %T", obj)
	}
	return dict, nil
}

// Stream represent a stream object in a PDF file.
type Stream struct {
	Dict
	R io.Reader

	isEncrypted bool
}

func (x *Stream) String() string {
	res := []string{}
	tp, ok := x.Dict["Type"].(Name)
	if ok {
		res = append(res, string(tp)+" Stream")
	} else {
		res = append(res, "Stream")
	}
	length, ok := x.Dict["Length"].(Integer)
	if ok {
		res = append(res, strconv.FormatInt(int64(length), 10)+" bytes")
	}
	switch filter := x.Dict["Filter"].(type) {
	case Name:
		res = append(res, string(filter))
	case Array:
		for _, f := range filter {
			if name, ok := f.(Name); ok {
				res = append(res, string(name))
			}
		}
	}
	return "<" + strings.Join(res, ", ") + ">"
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

	if wenc, ok := w.(*posWriter); ok && wenc.enc != nil {
		enc, err := wenc.enc.cryptFilter(wenc.ref, withoutClose{w})
		if err != nil {
			return err
		}
		w = enc
	}
	_, err = io.Copy(w, x.R)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("\nendstream"))
	return err
}

// Filters extracts the information contained in the /Filter and /DecodeParms
// entries of the stream dictionary.
func (x *Stream) Filters(resolve func(Object) (Object, error)) ([]*FilterInfo, error) {
	if resolve == nil {
		resolve = func(obj Object) (Object, error) {
			return obj, nil
		}
	}
	parms, err := resolve(x.Dict["DecodeParms"])
	if err != nil {
		return nil, err
	}
	var filters []*FilterInfo
	filter, err := resolve(x.Dict["Filter"])
	if err != nil {
		return nil, err
	}
	switch f := filter.(type) {
	case nil:
		// pass
	case Array:
		pa, _ := parms.(Array)
		for i, fi := range f {
			fi, err := resolve(fi)
			if err != nil {
				return nil, err
			}
			name, err := toName(fi)
			if err != nil {
				return nil, err
			}
			var pDict Dict
			if len(pa) > i {
				pai, err := resolve(pa[i])
				if err != nil {
					return nil, err
				}
				x, err := toDict(pai)
				if err != nil {
					return nil, err
				}
				pDict = x
			}
			filter := &FilterInfo{
				Name:  name,
				Parms: pDict,
			}
			filters = append(filters, filter)
		}
	case Name:
		pDict, err := toDict(parms)
		if err != nil {
			return nil, err
		}
		filters = append(filters, &FilterInfo{
			Name:  f,
			Parms: pDict,
		})
	default:
		return nil, errors.New("invalid /Filter field")
	}
	return filters, nil
}

// Decode returns a reader for the decoded stream data.
//
// TODO(voss): allow to decode only the first few filters?
func (x *Stream) Decode(resolve func(Object) (Object, error)) (io.Reader, error) {
	filters, err := x.Filters(resolve)
	if err != nil {
		return nil, err
	}
	r := x.R
	for _, fi := range filters {
		filter, err := fi.getFilter()
		if err != nil {
			return nil, err
		}
		r, err = filter.Decode(r)
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Reference represents a reference to an indirect object in a PDF file.
type Reference struct {
	Number     int
	Generation uint16
}

func (x *Reference) String() string {
	res := []string{
		"obj_",
		strconv.FormatInt(int64(x.Number), 10),
	}
	if x.Generation > 0 {
		res = append(res, "@", strconv.FormatUint(uint64(x.Generation), 10))
	}
	return strings.Join(res, "")
}

// PDF implements the Object interface.
func (x *Reference) PDF(w io.Writer) error {
	var err error
	if x == nil {
		_, err = fmt.Fprint(w, "null")
	} else {
		_, err = fmt.Fprintf(w, "%d %d R", x.Number, x.Generation)
	}
	return err
}
