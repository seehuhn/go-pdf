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

// Object represents an object in a PDF file.  There are nine basic types of
// PDF objects, which implement this interface: [Array], [Boolean], [Dict],
// [Integer], [Name], [Real], [Reference], [*Stream], and [String].
// The PDF "null" object is represented by the Go value `nil`.
// Custom types can be constructed out of these basic types, by implementing
// the Object interface.
type Object interface {
	// PDF writes the PDF file representation of the object to w.
	PDF(w io.Writer) error
}

// Boolean represents a boolean value in a PDF file.
type Boolean bool

// PDF implements the [Object] interface.
func (x Boolean) PDF(w io.Writer) error {
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
		scanner.bufPos++
		s, err = scanner.ReadQuotedString()
	} else if b[0] == '<' {
		scanner.bufPos++
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

	var pretty bool
	if wenc, ok := w.(*posWriter); ok {
		if wenc.enc != nil {
			enc, err := wenc.enc.EncryptBytes(wenc.ref, l)
			if err != nil {
				return err
			}
			l = enc
		} else {
			pretty = wenc.pretty
		}
	}

	if pretty {
		good := 0
		bad := 0
		for _, c := range l {
			isPrint := c >= 0x20 && c <= 0x7e || c == '\n' || c == '\r' || c == '\t'
			if isPrint {
				good++
			} else {
				bad++
			}
		}
		if good < 9*bad { // use hex encoding
			_, err := fmt.Fprintf(w, "<%x>", l)
			return err
		}
	}

	const bufLen = 8
	var buf [bufLen]byte
	var used int

	var finalErr error
	need := func(need int) {
		if finalErr != nil {
			used = 0
		} else if used+need > bufLen {
			_, err := w.Write(buf[:used])
			if err != nil {
				finalErr = err
			}
			used = 0
		}
	}

	numClosingParentheses := 0
	for _, c := range l {
		if c == ')' {
			numClosingParentheses++
		}
	}

	parenthesisLevel := 0
	need(1)
	buf[used] = '('
	used++
	for i, c := range l {
		switch c {
		case '\r':
			need(2)
			buf[used] = '\\'
			buf[used+1] = 'r'
			used += 2
		case '\n':
			if i > 0 && l[i-1] == '\r' || i < len(l)-1 && l[i+1] == '\r' {
				need(2)
				buf[used] = '\\'
				buf[used+1] = 'n'
				used += 2
			} else {
				need(1)
				buf[used] = c
				used++
			}
		case '(':
			if parenthesisLevel < numClosingParentheses {
				parenthesisLevel++
				need(1)
				buf[used] = c
				used++
			} else {
				need(2)
				buf[used] = '\\'
				buf[used+1] = '('
				used += 2
			}
		case ')':
			numClosingParentheses--
			if parenthesisLevel > 0 {
				parenthesisLevel--
				need(1)
				buf[used] = c
				used++
			} else {
				need(2)
				buf[used] = '\\'
				buf[used+1] = ')'
				used += 2
			}
		case '\\':
			need(2)
			buf[used] = '\\'
			buf[used+1] = '\\'
			used += 2
		default:
			need(1)
			buf[used] = c
			used++
		}
	}
	need(1)
	buf[used] = ')'
	used++

	need(bufLen) // flush the buffer

	return finalErr
}

// AsTextString interprets x as a PDF "text string" and returns
// the corresponding utf-8 encoded string.
func (x String) AsTextString() string {
	if isUTF16(string(x)) {
		return utf16Decode(x[2:])
	} else if isUTF8(string(x)) {
		return string(x[3:])
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
	// TODO(voss): for PDF >=2.0 we should use UTF-8 encoding here
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

// TODO(voss): remove this function
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

// ReadAll reads the content of a stream and returns it as a byte slice.
func ReadAll(r Getter, s *Stream) ([]byte, error) {
	in, err := DecodeStream(r, s, 0)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Reference represents a reference to an indirect object in a PDF file.
// The lowest 32 bits represent the object number, the next 16 bits the
// generation number.
type Reference uint64

// NewReference creates a new reference object.
func NewReference(number uint32, generation uint16) Reference {
	return Reference(uint64(number) | uint64(generation)<<32)
}

// NewInternalReference creates a new reference object which is guaranteed
// not to clash with an existing reference in the PDF file.
//
// TODO(voss): can we get rid of this?
func NewInternalReference(number uint32) Reference {
	return Reference(uint64(number) | internalReferenceFlag)
}

const internalReferenceFlag = 1 << 48

// Number returns the object number of the reference.
func (x Reference) Number() uint32 {
	return uint32(x)
}

// Generation returns the generation number of the reference.
func (x Reference) Generation() uint16 {
	return uint16(x >> 32)
}

// IsInternal returns true if the reference is an internal reference.
func (x Reference) IsInternal() bool {
	return x>>48 != 0
}

func (x Reference) String() string {
	if x&internalReferenceFlag != 0 {
		return fmt.Sprintf("int_%d", x&0x000_0000_ffff_ffff)
	}
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

// IsDirect returns true if the object foes not contain any references to
// indirect objects.
//
// If the object contains custom implementations of the [Object] interface, the
// `IsDirect` method of these objects is called recursively.  If no `IsDirect`
// method is present, the function panics.
func IsDirect(obj Object) bool {
	switch x := obj.(type) {
	case Boolean, Integer, Real, Number, Name, String, nil:
		return true
	case Reference:
		return x == 0
	case Array:
		for _, elem := range x {
			if !IsDirect(elem) {
				return false
			}
		}
		return true
	case Dict:
		for _, elem := range x {
			if !IsDirect(elem) {
				return false
			}
		}
		return true
	case *Stream:
		return false
	case interface{ IsDirect() bool }:
		return x.IsDirect()
	default:
		panic(fmt.Sprintf("IsDirect: unknown type %T", obj))
	}
}

// A Placeholder is a space reserved in a PDF file that can later be filled
// with a value.  One common use case is to store the length of compressed
// content in a PDF stream dictionary.  To create Placeholder objects,
// use the [Writer.NewPlaceholder] method.
type Placeholder struct {
	value []byte
	size  int

	pdf Putter
	pos []int64
	ref Reference
}

// NewPlaceholder creates a new placeholder for a value which is not yet known.
// The argument size must be an upper bound to the length of the replacement
// text.  Once the value becomes known, it can be filled in using the
// [Placeholder.Set] method.
func NewPlaceholder(pdf Putter, size int) *Placeholder {
	return &Placeholder{
		size: size,
		pdf:  pdf,
	}
}

// PDF implements the [Object] interface.
func (x *Placeholder) PDF(w io.Writer) error {
	// method 1: If the value is already known, we can just write it to the
	// file.
	if x.value != nil {
		_, err := w.Write(x.value)
		return err
	}

	// method 2: If we can seek, write whitespace for now and fill in
	// the actual value later.
	if pdf, ok := x.pdf.(*Writer); ok {
		if _, ok := pdf.origW.(io.WriteSeeker); ok {
			x.pos = append(x.pos, pdf.w.pos)

			buf := bytes.Repeat([]byte{' '}, x.size)
			_, err := w.Write(buf)
			return err
		}
	}

	// method 3: If all else fails, use an indirect reference.
	x.ref = x.pdf.Alloc()
	buf := &bytes.Buffer{}
	err := x.ref.PDF(buf)
	if err != nil {
		return err
	}
	x.value = buf.Bytes()
	_, err = w.Write(x.value)
	return err
}

// Set fills in the value of the placeholder object.  This should be called
// as soon as possible after the value becomes known.
func (x *Placeholder) Set(val Object) error {
	if x.ref != 0 {
		pdf := x.pdf
		err := pdf.Put(x.ref, val)
		if err != nil {
			return fmt.Errorf("Placeholder.Set: %w", err)
		}
		return nil
	}

	buf := bytes.NewBuffer(make([]byte, 0, x.size))
	err := val.PDF(buf)
	if err != nil {
		return err
	}
	if buf.Len() > x.size {
		return errors.New("Placeholder: replacement text too long")
	}
	x.value = buf.Bytes()

	if len(x.pos) == 0 {
		return nil
	}

	pdf := x.pdf.(*Writer)

	pdf.w.Flush()

	fill := pdf.origW.(io.WriteSeeker)
	currentPos, err := fill.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	for _, pos := range x.pos {
		_, err = fill.Seek(pos, io.SeekStart)
		if err != nil {
			return err
		}
		_, err = fill.Write(x.value)
		if err != nil {
			return err
		}
	}

	_, err = fill.Seek(currentPos, io.SeekStart)
	return err
}

// Format formats a PDF object as a string, in the same way as the
// it would be written to a PDF file.
func Format(obj Object) string {
	if obj == nil {
		return "null"
	}
	buf := &bytes.Buffer{}
	err := obj.PDF(buf)
	if err != nil {
		panic(err) // unreachable
	}
	return buf.String()
}
