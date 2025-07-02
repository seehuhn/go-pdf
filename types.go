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
	"slices"
	"strconv"
	"strings"
)

// Native represents an object in a PDF file. Thus must be one of the
// following:
//
//   - [Array]
//   - [Boolean]
//   - [Dict]
//   - [Integer]
//   - [Name]
//   - [Operator]
//   - [Real]
//   - [Reference]
//   - [*Stream]
//   - [String]
//   - [*Placeholder]
//
// The [Object] interface is a more general interface which can be used to
// construct more complex types out of the native PDF data types.
type Native interface {
	Object

	// isNative does nothing.  This is a marker method to help the compiler
	// tell apart Native objects from other objects.
	isNative()
}

// Object represents an object in a PDF file.
type Object interface {
	// AsPDF returns the representation of the using built-in PDF data types.
	//
	// Only the returned top-level object is required to use a native type.  In
	// case this object contains other objects, AsPDF must be called recursively
	// to convert these objects to native types as well.
	//
	// The output options can be used to control how the object is formatted.
	AsPDF(OutputOptions) Native
}

// OutputOptions is a bit-mask which controls how [Object] values are formatted.
type OutputOptions uint32

// HasAny returns true if any of the given output options are set.
func (o OutputOptions) HasAny(opt OutputOptions) bool {
	return o&opt != 0
}

// These constants give the supported output options.
// The values are bit masks which can be combined using bitwise OR.
const (
	// OptContentStream is set if we are inside a content stream.
	OptContentStream OutputOptions = 1 << iota

	// OptDictTypes adds optional dictionary /Type arguments.
	OptDictTypes

	// OptTrimStandardFonts omits the font descriptor and glyph width
	// information from font dictionaries for the 14 standard PDF fonts, where
	// possible.  This can reduce file size but may affect compatibility with
	// some PDF readers.
	OptTrimStandardFonts

	// optObjStm allows the use of object streams.
	optObjStm

	// OptPretty makes the output more human-readable.
	OptPretty

	// OptTextStringUtf8 enables UTF-8 encoding for text strings.
	OptTextStringUtf8

	// optXRefStream allows to use an xref stream instead of an xref table.
	optXRefStream
)

func defaultOutputOptions(v Version) OutputOptions {
	var opt OutputOptions
	if v >= V1_5 {
		opt |= optObjStm
		opt |= optXRefStream
	}
	if v >= V2_0 {
		opt |= OptTextStringUtf8
	}
	return opt
}

// Format writes the textual representation of one or more objects to the given writer.
// The output does not include any leading or trailing whitespace.
//
// The exact format written depends on the output options.
func Format(w io.Writer, opt OutputOptions, objects ...Object) error {
	var err error

	if opt.HasAny(OptPretty) {
		// Separate objects by spaces.
		for i, obj := range objects {
			if i > 0 {
				_, err = io.WriteString(w, " ")
				if err != nil {
					return err
				}
			}
			_, err = doFormat(w, obj, opt, false)
			if err != nil {
				return err
			}
		}
	} else {
		// Avoid spaces between objects as much as possible.
		needSep := false
		for _, obj := range objects {
			needSep, err = doFormat(w, obj, opt, needSep)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// doFormat writes the textual representation of a single object, with optional
// leading white space.
//
// The argument `needSep` indicates whether the function should write a
// separator before the object, in case the output starts with an alphanumeric
// character. The first return value indicates whether a separator is needed
// after the object, if the following object starts with a regular character.
func doFormat(w io.Writer, obj Object, opt OutputOptions, needSep bool) (bool, error) {
	var native Native
	if obj != nil {
		native = obj.AsPDF(opt)
	}

	switch x := native.(type) {
	case nil:
		if needSep {
			_, err := io.WriteString(w, " ")
			if err != nil {
				return false, err
			}
		}
		_, err := io.WriteString(w, "null")
		return true, err

	case Array:
		if x == nil {
			if needSep {
				_, err := io.WriteString(w, " ")
				if err != nil {
					return false, err
				}
			}
			_, err := io.WriteString(w, "null")
			return true, err
		}

		_, err := io.WriteString(w, "[")
		if err != nil {
			return false, err
		}
		err = Format(w, opt, x...)
		if err != nil {
			return false, err
		}
		_, err = io.WriteString(w, "]")
		return false, err

	case Boolean:
		if needSep {
			_, err := io.WriteString(w, " ")
			if err != nil {
				return false, err
			}
		}
		if x {
			_, err := io.WriteString(w, "true")
			return true, err
		} else {
			_, err := io.WriteString(w, "false")
			return true, err
		}

	case Dict:
		err := formatDict(w, opt, x)
		return false, err

	case Integer:
		if needSep {
			_, err := io.WriteString(w, " ")
			if err != nil {
				return false, err
			}
		}
		s := strconv.FormatInt(int64(x), 10)
		_, err := io.WriteString(w, s)
		return true, err

	case Name:
		return true, formatName(w, x)

	case Operator:
		if !opt.HasAny(OptContentStream) {
			return false, errors.New("operator outside content stream")
		}
		if needSep {
			_, err := io.WriteString(w, " ")
			if err != nil {
				return false, err
			}
		}
		_, err := io.WriteString(w, string(x))
		return true, err

	case Real:
		if needSep {
			_, err := io.WriteString(w, " ")
			if err != nil {
				return false, err
			}
		}
		s := strconv.FormatFloat(float64(x), 'f', -1, 64)
		if !strings.Contains(s, ".") {
			s = s + "."
		}
		_, err := io.WriteString(w, s)
		return true, err

	case Reference:
		if needSep {
			_, err := io.WriteString(w, " ")
			if err != nil {
				return false, err
			}
		}
		_, err := fmt.Fprintf(w, "%d %d R", x.Number(), x.Generation())
		return true, err

	case *Stream:
		return true, errors.New("direct stream objects are not allowed")

	case String:
		err := formatString(w, x, opt)
		return false, err

	case *Placeholder:
		if needSep {
			_, err := io.WriteString(w, " ")
			if err != nil {
				return false, err
			}
		}

		// method 1: If the value is already known, we can just write it to the
		// file.
		if x.value != nil {
			_, err := w.Write(x.value)
			return true, err
		}

		// method 2: If we can seek, write whitespace now and replace this with
		// the actual value later.
		if _, ok := x.pdf.origW.(io.WriteSeeker); ok {
			x.pos = append(x.pos, x.pdf.w.pos)
			_, err := w.Write(bytes.Repeat([]byte{' '}, x.size))
			return true, err
		}

		// method 3: If all else fails, use an indirect reference.
		if x.ref == 0 {
			x.ref = x.pdf.Alloc()
		}
		return doFormat(w, x.ref, opt, false)

	default:
		panic(fmt.Sprintf("Format: invalid PDF object type %T", x))
	}
}

func formatName(w io.Writer, name Name) error {
	l := []byte(name)

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

func formatString(w io.Writer, s String, opt OutputOptions) error {
	l := []byte(s)

	pretty := opt.HasAny(OptPretty)
	if wenc, ok := w.(*posWriter); ok {
		if wenc.enc != nil {
			enc, err := wenc.enc.EncryptBytes(wenc.ref, l)
			if err != nil {
				return err
			}
			l = enc
			pretty = false
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

func formatDict(w io.Writer, opt OutputOptions, dict Dict) error {
	_, err := io.WriteString(w, "<<")
	if err != nil {
		return err
	}
	keys := dict.SortedKeys()

	if opt.HasAny(OptPretty) {
		_, err = io.WriteString(w, "\n")
		if err != nil {
			return err
		}
		for _, name := range keys {
			val := dict[name]
			if val == nil {
				continue
			}

			err := formatName(w, name)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, " ")
			if err != nil {
				return err
			}
			_, err = doFormat(w, val, opt, false)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, "\n")
			if err != nil {
				return err
			}
		}
	} else {
		for _, name := range keys {
			val := dict[name]
			if val == nil {
				continue
			}

			err := formatName(w, name)
			if err != nil {
				return err
			}
			_, err = doFormat(w, val, opt, true)
			if err != nil {
				return err
			}
		}
	}
	_, err = io.WriteString(w, ">>")
	return err
}

// Boolean represents a boolean value in a PDF file.
type Boolean bool

func (x Boolean) isNative() {}

func (x Boolean) AsPDF(opt OutputOptions) Native {
	return x
}

// Integer represents an integer constant in a PDF file.
type Integer int64

func (x Integer) isNative() {}

func (x Integer) AsPDF(opt OutputOptions) Native {
	return x
}

// Real represents an real number in a PDF file.
type Real float64

func (x Real) isNative() {}

func (x Real) AsPDF(opt OutputOptions) Native {
	return x
}

// String represents a raw string in a PDF file.  The character set and encoding,
// if any, is determined by the context.
type String []byte

func (x String) isNative() {}

func (x String) AsPDF(opt OutputOptions) Native {
	return x
}

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
	switch b[0] {
	case '(':
		scanner.bufPos++
		s, err = scanner.ReadQuotedString()
	case '<':
		scanner.bufPos++
		s, err = scanner.ReadHexString()
	default:
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

// Name represents a name object in a PDF file.
type Name string

func (x Name) isNative() {}

func (x Name) AsPDF(opt OutputOptions) Native {
	return x
}

func (x Name) isSecondClassName() bool {
	k := len(x)
	if k > 5 {
		k = 5
	}
	for _, c := range x[:k] {
		if c == ':' || c == '_' {
			return true
		}
	}
	return false
}

func (x Name) isThirdClassName() bool {
	return len(x) >= 2 && x[0] == 'X' && x[1] == 'X'
}

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

// Operator represents a PDF content stream operator.
type Operator string

func (x Operator) isNative() {}

func (x Operator) AsPDF(opt OutputOptions) Native {
	return x
}

// Array represent an array of objects in a PDF file.
type Array []Object

func (x Array) isNative() {}

func (x Array) String() string {
	res := []string{}
	res = append(res, "Array")
	res = append(res, strconv.FormatInt(int64(len(x)), 10)+" elements")
	return "<" + strings.Join(res, ", ") + ">"
}

func (x Array) AsPDF(opt OutputOptions) Native {
	return x
}

// Dict represent a Dictionary object in a PDF file.
//
// Entries which map to a nil value are equivalent to missing entries,
// and are not included in PDF output.
type Dict map[Name]Object

func (d Dict) isNative() {}

func (d Dict) String() string {
	res := []string{}
	tp, ok := d["Type"].(Name)
	if ok {
		res = append(res, string(tp)+" Dict")
	} else {
		res = append(res, "Dict")
	}
	if len(d) != 1 {
		res = append(res, strconv.FormatInt(int64(len(d)), 10)+" entries")
	} else {
		res = append(res, "1 entry")
	}
	return "<" + strings.Join(res, ", ") + ">"
}

func (d Dict) AsPDF(opt OutputOptions) Native {
	return d
}

// Clone makes a shallow copy of the dictionary.
func (d Dict) Clone() Dict {
	if d == nil {
		return nil
	}

	rea := make(Dict, len(d))
	for k, v := range d {
		rea[k] = v
	}
	return rea
}

// SortedKeys returns the keys of the dictionary in deterministic order.
func (d Dict) SortedKeys() []Name {
	keys := make([]Name, 0, len(d))

	// put some special keys first
	for _, key := range []Name{"Type", "Subtype"} { // this much match the loop below
		if _, ok := d[key]; ok {
			keys = append(keys, key)
		}
	}

	// collect and sort the remaining keys
	base := len(keys)
	for k := range d {
		if k != Name("Type") && k != Name("Subtype") {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys[base:])

	return keys
}

// Stream represent a stream object in a PDF file.
// Use the [DecodeStream] function to access the contents of the stream.
type Stream struct {
	Dict
	R io.Reader

	isEncrypted bool
}

func (x *Stream) isNative() {}

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

func (x *Stream) AsPDF(opt OutputOptions) Native {
	return x
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

func (x Reference) isNative() {}

func (x Reference) AsPDF(opt OutputOptions) Native {
	return x
}

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

// IsDirect returns true if the object foes not contain any references to
// indirect objects.
//
// If the object contains custom implementations of the [Object] interface, the
// `IsDirect` method of these objects is called recursively.  If no `IsDirect`
// method is present, the function panics.
//
// TODO(voss): remove?
func IsDirect(obj Object) bool {
	native := obj.AsPDF(0)

	switch x := native.(type) {
	case Boolean, Integer, Real, Name, String, nil:
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

	pdf *Writer
	pos []int64
	ref Reference
}

func (x *Placeholder) isNative() {}

// NewPlaceholder creates a new placeholder for a value which is not yet known.
// The argument size must be an upper bound to the length of the replacement
// text.  Once the value becomes known, it can be filled in using the
// [Placeholder.Set] method.
func NewPlaceholder(pdf *Writer, size int) *Placeholder {
	return &Placeholder{
		size: size,
		pdf:  pdf,
	}
}

func (x *Placeholder) AsPDF(opt OutputOptions) Native {
	return x
}

// Set fills in the value of the placeholder object.  This should be called
// as soon as possible after the value becomes known.
func (x *Placeholder) Set(val Native) error {
	if x == nil {
		return nil
	}

	if x.ref != 0 {
		err := x.pdf.Put(x.ref, val)
		if err != nil {
			return fmt.Errorf("Placeholder.Set: %w", err)
		}
		return nil
	}

	if x.value != nil {
		return errors.New("Placeholder.Set: value already set")
	}

	// format the value
	buf := &bytes.Buffer{}
	_, err := doFormat(buf, val, 0, false)
	if err != nil {
		return fmt.Errorf("Placeholder.Set: %w", err)
	} else if buf.Len() > x.size {
		return errors.New("Placeholder: replacement text too long")
	}
	x.value = make([]byte, buf.Len())
	copy(x.value, buf.Bytes())

	if len(x.pos) == 0 {
		return nil
	}

	// Replace all previously written placeholders with the final value.
	x.pdf.w.Flush()
	fill := x.pdf.origW.(io.WriteSeeker)
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
	if err != nil {
		return err
	}

	x.pos = nil
	return nil
}

// AsString formats a PDF object as a string, in the same way as the
// it would be written to a PDF file.
func AsString(obj Object) string {
	buf := &bytes.Buffer{}
	err := Format(buf, OptPretty, obj)
	if err != nil {
		panic(err) // TODO(voss): unreachable?
	}
	return buf.String()
}
