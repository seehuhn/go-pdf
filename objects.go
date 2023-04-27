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
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Object represents an object in a PDF file.  There are nine native types of
// PDF objects, which implement this interface: [Array], [Bool], [Dict],
// [Integer], [Name], [Real], [Reference], [*Stream], and [String].
// Custom types can be constructed out of these basic types, by implementing
// the Object interface.
type Object interface {
	// PDF writes the PDF file representation of the object to w.
	PDF(w io.Writer) error
}

// Bool represents a boolean value in a PDF file.
type Bool bool

// PDF implements the [Object] interface.
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

// PDF implements the [Object] interface.
func (x Integer) PDF(w io.Writer) error {
	s := strconv.FormatInt(int64(x), 10)
	_, err := w.Write([]byte(s))
	return err
}

// Real represents an real number in a PDF file.
type Real float64

// PDF implements the [Object] interface.
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

// ParseString parses a string from the given buffer.  The buffer must include
// the surrounding parentheses or angle brackets.
func ParseString(buf []byte) (String, error) {
	scanner := newScanner(bytes.NewReader(buf), nil, nil)
	b, _ := scanner.Peek(1)
	if len(b) < 1 {
		return nil, errInvalidString
	}
	var s String
	var err error
	if b[0] == '(' {
		scanner.bufPos += 1
		s, err = scanner.ReadQuotedString()
	} else if b[0] == '<' {
		scanner.bufPos += 1
		s, err = scanner.ReadHexString()
	} else {
		err = errInvalidString
	}
	if err != nil {
		return nil, err
	}
	if scanner.currentPos() != int64(len(buf)) {
		return nil, errInvalidString
	}
	return s, nil
}

var errInvalidString = errors.New("malformed PDF string")

// PDF implements the [Object] interface.
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
			case '\r':
				buf.WriteString(`\r`)
			case '\n':
				buf.WriteString(`\n`)
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
	buf, ok := pdfDocEncode(s)
	if ok {
		return buf
	}
	return utf16Encode(s)
}

// AsDate converts a PDF date string to a time.Time object.
// If the string does not have the correct format, an error is returned.
func (x String) AsDate() (time.Time, error) {
	s := x.AsTextString()
	if s == "D:" || s == "" {
		return time.Time{}, nil
	}
	s = strings.ReplaceAll(s, "'", "")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "19") || strings.HasPrefix(s, "20") {
		s = "D:" + s
	}

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

// Name represents a name object in a PDF file.
type Name string

// ParseName parses a PDF name from the given buffer.  The buffer must include
// the leading slash.
func ParseName(buf []byte) (Name, error) {
	scanner := newScanner(bytes.NewReader(buf), nil, nil)
	b, _ := scanner.Peek(1)
	if len(b) < 1 || b[0] != '/' {
		return "", errInvalidName
	}
	n, err := scanner.ReadName()
	if err != nil {
		return "", err
	}
	if scanner.currentPos() != int64(len(buf)) {
		return "", errInvalidString
	}
	return n, nil
}

var errInvalidName = errors.New("malformed PDF name")

// PDF implements the [Object] interface.
func (x Name) PDF(w io.Writer) error {
	l := []byte(x)

	var funny []int
	for i, c := range l {
		if isSpace[c] || isDelimiter[c] || c < 0x21 || c > 0x7e || c == '#' {
			funny = append(funny, i)
		}
	}
	n := len(l)

	_, err := w.Write([]byte{'/'})
	if err != nil {
		return err
	}
	pos := 0
	for _, i := range funny {
		if pos < i {
			_, err = w.Write(l[pos:i])
			if err != nil {
				return err
			}
		}
		c := l[i]
		_, err = fmt.Fprintf(w, "#%02x", c)
		if err != nil {
			return err
		}
		pos = i + 1
	}
	if pos < n {
		_, err = w.Write(l[pos:n])
		if err != nil {
			return err
		}
	}

	return nil
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

// PDF implements the [Object] interface.
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
		err = writeObject(w, val)
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
	if len(x) != 1 {
		res = append(res, strconv.FormatInt(int64(len(x)), 10)+" entries")
	} else {
		res = append(res, "1 entry")
	}
	return "<" + strings.Join(res, ", ") + ">"
}

// PDF implements the [Object] interface.
func (x Dict) PDF(w io.Writer) error {
	if x == nil {
		_, err := w.Write([]byte("null"))
		return err
	}

	keys := make([]Name, 0, len(x))
	for key, val := range x {
		if val == nil {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool {
		return keys[i] < keys[j]
	})

	_, err := w.Write([]byte("<<"))
	if err != nil {
		return err
	}

	for _, name := range keys {
		val := x[name]

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

// PDF implements the [Object] interface.
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
		enc, err := wenc.enc.EncryptStream(wenc.ref, withDummyClose{w})
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

// Reference represents a reference to an indirect object in a PDF file.
// The lower 32 bits represent the object number, the next 16 bits the
// generation number.
type Reference uint64

func NewReference(number uint32, generation uint16) Reference {
	return Reference(uint64(number) | uint64(generation)<<32)
}

func (x Reference) Number() uint32 {
	return uint32(x)
}

func (x Reference) Generation() uint16 {
	return uint16(x >> 32)
}

func (x Reference) String() string {
	res := []string{
		"obj_",
		strconv.FormatInt(int64(x.Number()), 10),
	}
	gen := x.Generation()
	if gen > 0 {
		res = append(res, "@", strconv.FormatUint(uint64(gen), 10))
	}
	return strings.Join(res, "")
}

// PDF implements the [Object] interface.
func (x Reference) PDF(w io.Writer) error {
	if x>>48 != 0 {
		return fmt.Errorf("invalid reference: 0x%016x", x)
	}

	_, err := fmt.Fprintf(w, "%d %d R", x.Number(), x.Generation())
	return err
}

// TODO(voss): either use this more widely, or merge it into the caller.
func writeObject(w io.Writer, obj Object) error {
	if obj == nil {
		_, err := w.Write([]byte("null"))
		return err
	}
	return obj.PDF(w)
}
