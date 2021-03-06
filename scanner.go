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
	"regexp"
	"strconv"
)

const scannerBufSize = 1024

type scanner struct {
	r       io.Reader
	base    int64
	getInt  func(Object) (Integer, error)
	buf     []byte
	pos     int
	used    int
	skipped int64

	enc     *encryptInfo
	encRef  *Reference
	special map[Reference]bool // objects with no encryption
}

func newScanner(r io.Reader, base int64, getInt func(Object) (Integer, error),
	dec *encryptInfo) *scanner {
	return &scanner{
		r:      r,
		base:   base,
		buf:    make([]byte, scannerBufSize),
		getInt: getInt,
		enc:    dec,
	}
}

func (s *scanner) currentPos() int64 {
	return s.base + s.skipped + int64(s.pos)
}

func (s *scanner) bytesRead() int64 {
	return s.skipped + int64(s.pos)
}

func (s *scanner) ReadIndirectObject() (Object, *Reference, error) {
	number, err := s.ReadInteger()
	if err != nil {
		return nil, nil, err
	}
	generation, err := s.ReadInteger()
	if err != nil {
		return nil, nil, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, nil, err
	}

	err = s.SkipString("obj")
	if err != nil {
		return nil, nil, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, nil, err
	}

	ref := &Reference{int(number), uint16(generation)}
	if s.special[*ref] {
		// some objects are not encrypted, e.g. xref dictionaries
		s.enc = nil
	} else {
		s.encRef = ref
	}

	obj, err := s.ReadObject()
	if err != nil {
		return nil, nil, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, nil, err
	}

	if a, ok := obj.(Integer); ok {
		// Check whether this is the start of a reference to an indirect
		// object.
		buf, err := s.Peek(6)
		if err != nil {
			return nil, nil, err
		}
		if !bytes.Equal(buf, []byte("endobj")) {
			b, err := s.ReadInteger()
			if err != nil {
				return nil, nil, err
			}
			err = s.SkipString("R")
			if err != nil {
				return nil, nil, err
			}
			err = s.SkipWhiteSpace()
			if err != nil {
				return nil, nil, err
			}

			obj = &Reference{
				Number:     int(a),
				Generation: uint16(b),
			}
		}
	}

	err = s.SkipString("endobj")
	if err != nil {
		return nil, nil, err
	}

	return obj, ref, nil
}

func (s *scanner) ReadObject() (Object, error) {
	buf, err := s.Peek(5) // len("false") == 5
	if err == nil {
		// Below, we return `err` if we cannot detect an object.  Use
		// &MalformedFileError{} when there was no problem reading the input.
		if len(buf) < 5 {
			err = &MalformedFileError{Err: io.EOF}
		} else {
			err = &MalformedFileError{Pos: s.currentPos()}
		}
	}

	switch {
	case len(buf) == 0:
		// Test this first, so that we can use buf[0] in the following cases.
		return nil, err
	case bytes.HasPrefix(buf, []byte("null")):
		s.pos += 4
		return nil, nil
	case bytes.HasPrefix(buf, []byte("true")):
		s.pos += 4
		return Bool(true), nil
	case bytes.HasPrefix(buf, []byte("false")):
		s.pos += 5
		return Bool(false), nil
	case buf[0] == '/':
		return s.ReadName()
	case buf[0] >= '0' && buf[0] <= '9', buf[0] == '+', buf[0] == '-', buf[0] == '.':
		return s.ReadNumber()
		// It is the caller's responsibility to check whether this is the start
		// of a reference.

	case bytes.HasPrefix(buf, []byte("<<")):
		dict, err := s.ReadDict()
		if err != nil {
			return nil, err
		}

		// check whether this is the start of a stream
		err = s.SkipWhiteSpace()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return dict, nil
			}
			return nil, err
		}
		buf, _ = s.Peek(6) // len("stream") == 6
		if !bytes.HasPrefix(buf, []byte("stream")) {
			return dict, nil
		}
		return s.ReadStreamData(dict)
	case buf[0] == '(':
		s.pos++
		return s.ReadQuotedString()
	case buf[0] == '<':
		s.pos++
		return s.ReadHexString()
	case buf[0] == '[':
		s.pos++
		return s.ReadArray()
	}
	return nil, err
}

// ReadInteger reads an integer, optionally preceeded by white space.
func (s *scanner) ReadInteger() (Integer, error) {
	err := s.SkipWhiteSpace()
	if err != nil {
		return 0, err
	}

	first := true
	var res []byte
	err = s.ScanBytes(func(c byte) bool {
		if first && (c == '+' || c == '-') {
			res = append(res, c)
		} else if c >= '0' && c <= '9' {
			res = append(res, c)
		} else {
			return false
		}
		first = false
		return true
	})
	if err != nil && err != io.EOF {
		return 0, err
	}

	x, err := strconv.ParseInt(string(res), 10, 64)
	if err != nil {
		return 0, &MalformedFileError{
			Pos: s.currentPos(),
			Err: err,
		}
	}
	return Integer(x), nil
}

// ReadNumber reads an integer or real number.
func (s *scanner) ReadNumber() (Object, error) {
	hasDot := false
	first := true
	var res []byte
	err := s.ScanBytes(func(c byte) bool {
		if !hasDot && c == '.' {
			hasDot = true
			res = append(res, c)
		} else if first && (c == '+' || c == '-') {
			res = append(res, c)
		} else if c >= '0' && c <= '9' {
			res = append(res, c)
		} else {
			return false
		}
		first = false
		return true
	})
	if err != nil && err != io.EOF {
		return nil, err
	}

	if hasDot {
		x, err := strconv.ParseFloat(string(res), 64)
		if err != nil {
			return nil, &MalformedFileError{Err: err}
		}
		return Real(x), nil
	}

	x, err := strconv.ParseInt(string(res), 10, 64)
	if err != nil {
		return nil, &MalformedFileError{Err: err}
	}
	return Integer(x), nil
}

// ReadQuotedString reads a ()-delimited string, starting after the opening
// bracket.
func (s *scanner) ReadQuotedString() (String, error) {
	var res []byte
	parentCount := 0
	escape := false
	ignoreLF := false
	isOctal := 0
	octalVal := byte(0)
	err := s.ScanBytes(func(c byte) bool {
		if ignoreLF {
			ignoreLF = false
			if c == '\n' {
				return true
			}
		}
		if isOctal > 0 {
			octalVal = octalVal*8 + (c - '0')
			isOctal--
			if isOctal == 0 {
				res = append(res, octalVal)
			}
			return true
		}
		if escape {
			escape = false
			switch c {
			case '\n':
				return true
			case '\r':
				ignoreLF = true
				return true
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			case 'b':
				c = '\b'
			case 'f':
				c = '\f'
			}
			if c >= '0' && c <= '7' {
				isOctal = 2
				octalVal = c - '0'
				return true
			}
		} else if c == '\\' {
			escape = true
			return true
		} else if c == '(' {
			parentCount++
		} else if c == ')' {
			if parentCount > 0 {
				parentCount--
			} else {
				return false
			}
		}
		res = append(res, c)
		return true
	})
	if err != nil {
		return nil, err
	}

	err = s.SkipString(")")
	if err != nil {
		return nil, err
	}

	if s.enc != nil && s.encRef != nil {
		res, err = s.enc.DecryptBytes(s.encRef, res)
		if err != nil {
			return nil, err
		}
	}

	return String(res), nil
}

// ReadHexString reads a <>-delimited string, starting after the opening
// angled bracket.
func (s *scanner) ReadHexString() (String, error) {
	var res []byte
	var hexVal byte
	first := true
	err := s.ScanBytes(func(c byte) bool {
		var d byte
		if c >= '0' && c <= '9' {
			d = c - '0'
		} else if c >= 'A' && c <= 'F' {
			d = c - 'A' + 10
		} else if c >= 'a' && c <= 'f' {
			d = c - 'a' + 10
		} else if c == '>' {
			return false
		} else {
			return true
		}
		if first {
			hexVal = d
		} else {
			res = append(res, 16*hexVal+d)
		}
		first = !first
		return true
	})
	if err != nil {
		return nil, err
	}
	if !first {
		res = append(res, 16*hexVal)
	}

	err = s.SkipString(">")
	if err != nil {
		return nil, err
	}

	if s.enc != nil && s.encRef != nil {
		res, err = s.enc.DecryptBytes(s.encRef, res)
		if err != nil {
			return nil, err
		}
	}

	return String(res), nil
}

// ReadName reads a PDF name object.
func (s *scanner) ReadName() (Name, error) {
	err := s.SkipString("/")
	if err != nil {
		return "", err
	}

	hex := 0
	var hexByte byte
	var res []byte
	err = s.ScanBytes(func(c byte) bool {
		if hex > 0 {
			var val byte
			if c >= '0' && c <= '9' {
				val = c - '0'
			} else if c >= 'A' && c <= 'F' {
				val = c - 'A' + 10
			} else if c >= 'a' && c <= 'f' {
				val = c - 'a' + 10
			}
			hexByte = 16*hexByte + val
			hex--
			if hex == 0 {
				res = append(res, hexByte)
			}
		} else if c == '#' {
			hexByte = 0
			hex = 2
		} else if isSpace[c] || isDelimiter[c] {
			return false
		} else {
			res = append(res, c)
		}
		return true
	})
	if err != nil && err != io.EOF {
		return "", err
	}

	return Name(res), nil
}

// ReadArray reads an array, starting after the opening "[".
func (s *scanner) ReadArray() (Array, error) {
	var array Array
	integersSeen := 0
	for {
		err := s.SkipWhiteSpace()
		if err != nil {
			return nil, err
		}

		buf, err := s.Peek(1)
		if err != nil {
			return nil, err
		}
		if buf[0] == ']' {
			break
		}
		if integersSeen >= 2 && buf[0] == 'R' {
			s.pos++
			k := len(array)
			a := int(array[k-2].(Integer))
			b := uint16(array[k-1].(Integer))
			array = append(array[:k-2], &Reference{a, b})
			integersSeen = 0
			continue
		}

		obj, err := s.ReadObject()
		if err != nil {
			return nil, err
		}

		if _, isInt := obj.(Integer); isInt {
			integersSeen++
		} else {
			integersSeen = 0
		}

		array = append(array, obj)
	}
	s.pos++ // we have already seen the closing "]"

	return array, nil
}

// ReadDict reads a PDF dictionary.
func (s *scanner) ReadDict() (Dict, error) {
	err := s.SkipString("<<")
	if err != nil {
		return nil, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, err
	}

	dict := make(map[Name]Object)
	for {
		var key Name
		key, err = s.ReadName()
		if _, ok := err.(*MalformedFileError); ok {
			break
		} else if err != nil {
			return nil, err
		}

		err = s.SkipWhiteSpace()
		if err != nil {
			return nil, err
		}

		var val Object
		val, err = s.ReadObject()
		if err != nil {
			return nil, err
		}
		err = s.SkipWhiteSpace()
		if err != nil {
			return nil, err
		}

		// If we found an integer, check whether this is a reference to an
		// indirect object.
		if a, isInt := val.(Integer); isInt {
			buf, err := s.Peek(1)
			if err != nil {
				return nil, err
			}
			if len(buf) == 0 {
				return nil, &MalformedFileError{
					Pos: s.currentPos(),
					Err: io.ErrUnexpectedEOF,
				}
			}
			if buf[0] != '/' && buf[0] != '>' {
				errPos := s.currentPos()

				b, err := s.ReadInteger()
				if err != nil {
					return nil, err
				}

				err = s.SkipWhiteSpace()
				if err != nil {
					return nil, err
				}

				buf, err := s.Peek(1)
				if err != nil {
					return nil, err
				}
				if buf[0] != 'R' {
					return nil, &MalformedFileError{
						Pos: errPos,
						Err: errors.New("expected /Name but found Integer"),
					}
				}
				s.pos++
				err = s.SkipWhiteSpace()
				if err != nil {
					return nil, err
				}

				val = &Reference{int(a), uint16(b)}
			}
		}

		dict[key] = val
	}
	err = s.SkipString(">>")
	if err != nil {
		return nil, err
	}

	return dict, nil
}

// ReadStreamData reads the data of a PDF Stream, starting after the Dict.
func (s *scanner) ReadStreamData(dict Dict) (*Stream, error) {
	length, err := s.getInt(dict["Length"])
	if err != nil {
		return nil, err
	} else if length < 0 {
		return nil, &MalformedFileError{
			Pos: s.currentPos(),
			Err: errors.New("stream with negative length"),
		}
	}

	err = s.SkipString("stream")
	if err != nil {
		return nil, err
	}

	buf, err := s.Peek(2)
	if err != nil {
		return nil, err
	}
	if len(buf) >= 1 && buf[0] == '\n' {
		s.pos++
	} else if len(buf) >= 2 && buf[0] == '\r' && buf[1] == '\n' {
		s.pos += 2
	} else if len(buf) >= 1 && buf[0] == '\r' {
		// not allowed by the spec, but seen in the wild
		s.pos++
	} else {
		return nil, &MalformedFileError{}
	}

	start := s.bytesRead()
	l := int64(length)

	var streamData io.Reader
	if origReader, ok := s.r.(io.ReaderAt); ok {
		streamData = io.NewSectionReader(origReader, start, l)
		err = s.Discard(l)
		if err != nil {
			return nil, err
		}
	} else {
		// the spec does not allow streams inside streams
		return nil, &MalformedFileError{}
	}

	isEncrypted := false
	if s.enc != nil {
		streamData, err = s.enc.DecryptStream(s.encRef, streamData)
		if err != nil {
			return nil, err
		}
		isEncrypted = true
	}

	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, err
	}

	err = s.SkipString("endstream")
	if err != nil {
		return nil, err
	}

	return &Stream{
		Dict:        dict,
		R:           streamData,
		isEncrypted: isEncrypted,
	}, nil
}

func (s *scanner) readHeaderVersion() (Version, error) {
	err := s.SkipString("%PDF-")
	if err != nil {
		if e, ok := err.(*MalformedFileError); ok {
			// Give a clearer message if this is not a PDF file.
			e.Err = errors.New("PDF header not found")
		}
		return 0, err
	}

	var buf []byte
	err = s.ScanBytes(func(c byte) bool {
		if c >= '0' && c <= '9' || c == '.' {
			buf = append(buf, c)
			return true
		}
		return false
	})
	if err != nil {
		return 0, err
	}

	ver, err := ParseVersion(string(buf))
	if err != nil {
		return 0, &MalformedFileError{
			Pos: 5,
			Err: err,
		}
	}

	return ver, nil
}

// Refill discards the read part of the buffer and reads as much new data as
// possible.  Once the end of file is reached, s.used will be smaller than the
// buffer size, but no error will be returned.
func (s *scanner) refill() error {
	s.skipped += int64(s.pos)
	copy(s.buf, s.buf[s.pos:s.used])
	s.used -= s.pos
	s.pos = 0

	n, err := io.ReadFull(s.r, s.buf[s.used:])
	s.used += n

	if s.used > 0 || err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}

	return err
}

// Peek returns a view of the next n bytes of input.  The function panics, if n
// is larger than scannerBufSize.  On EOF, short buffers without an error code
// will be returned.
func (s *scanner) Peek(n int) ([]byte, error) {
	if n > scannerBufSize {
		panic("peek window too large")
	}

	var err error
	if s.pos+n > s.used {
		err = s.refill()
	}

	if s.pos+n > s.used {
		return s.buf[s.pos:s.used], err
	}

	return s.buf[s.pos : s.pos+n], nil
}

func (s *scanner) Discard(n int64) error {
	if n < 0 {
		panic("negative offset for Discard()")
	}
	unread := int64(s.used - s.pos)
	if n <= unread {
		s.pos += int(n)
		return nil
	}

	n -= unread
	s.skipped += int64(s.used)
	s.pos = 0
	s.used = 0

	n, err := io.CopyN(io.Discard, s.r, n)
	s.skipped += n
	return err
}

// ScanBytes iterates over the bytes of s until `accept()` returns false.  The
// scanner position after the call returns is the byte for which `accept()`
// returned false, the next read will start with this byte.
func (s *scanner) ScanBytes(accept func(c byte) bool) error {
	empty := true
	for {
		for s.pos < s.used {
			if !accept(s.buf[s.pos]) {
				return nil
			}
			s.pos++
			empty = false
		}
		err := s.refill()
		if err == io.EOF && !empty {
			return nil
		}
		if s.used == 0 {
			if err == nil {
				err = io.EOF
			}
			return err
		}
	}
}

func (s *scanner) SkipWhiteSpace() error {
	isComment := false
	return s.ScanBytes(func(c byte) bool {
		if isComment {
			if c == '\r' || c == '\n' {
				isComment = false
			}
		} else if c == '%' {
			isComment = true
		} else {
			return isSpace[c]
		}
		return true
	})
}

func (s *scanner) SkipString(pat string) error {
	patBytes := []byte(pat)
	n := len(patBytes)
	buf, err := s.Peek(n)
	if err != nil {
		return err
	}
	if !bytes.Equal(buf, patBytes) {
		return &MalformedFileError{
			Pos: s.currentPos(),
			Err: fmt.Errorf("expected %q but found %q", pat, string(buf)),
		}
	}
	s.pos += n
	return nil
}

func (s *scanner) SkipAfter(pat string) error {
	patBytes := []byte(pat)
	n := len(patBytes)
	if n > scannerBufSize {
		panic("SkipAfter target string too long")
	}

	for {
		idx := bytes.Index(s.buf[s.pos:s.used], patBytes)
		if idx >= 0 {
			s.pos += idx + n
			return nil
		}
		s.pos = s.used
		err := s.refill()
		if err != nil {
			return err
		}
		if s.used == 0 {
			return io.EOF
		}
	}
}

func (s *scanner) skipToNextObject() error {
	pat := regexp.MustCompile(`^\d{1,12}\s+\d{1,12}\s+obj\s`)
	for {
		afterEOL := false
		err := s.ScanBytes(func(c byte) bool {
			if afterEOL {
				if c >= '0' && c <= '9' {
					return false
				}
				afterEOL = isSpace[c]
			} else if c == '\r' || c == '\n' {
				afterEOL = true
			}
			return true
		})
		if err != nil {
			return err
		}

		buf, err := s.Peek(32)
		if err != nil {
			return err
		} else if len(buf) == 0 {
			return io.EOF
		}
		if pat.Find(buf) != nil {
			return nil
		}
	}
}

var (
	isSpace = map[byte]bool{
		0:  true,
		9:  true,
		10: true,
		12: true,
		13: true,
		32: true,
	}
	isDelimiter = map[byte]bool{
		'(': true,
		')': true,
		'<': true,
		'>': true,
		'[': true,
		']': true,
		'{': true,
		'}': true,
		'/': true,
		'%': true,
	}
)
