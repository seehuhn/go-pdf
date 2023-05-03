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

const (
	scannerBufSize = 1024
	regexpOverlap  = 64
)

type scanner struct {
	r       io.Reader
	filePos int64 // how far into the reader the start of buf is
	buf     []byte
	bufPos  int // current position within buf
	bufEnd  int // end of valid data within buf

	// GetInt is used to get the length of a stream, when the length is
	// specified as an indirect object.
	getInt func(Object) (Integer, error)

	enc         *encryptInfo
	encRef      Reference
	unencrypted map[Reference]bool // objects with no encryption
}

func newScanner(r io.Reader, getInt func(Object) (Integer, error),
	dec *encryptInfo) *scanner {
	return &scanner{
		r:      r,
		buf:    make([]byte, scannerBufSize),
		getInt: getInt,
		enc:    dec,
	}
}

// currentPos returns the current position in the file.
func (s *scanner) currentPos() int64 {
	return s.filePos + int64(s.bufPos)
}

func (s *scanner) ReadIndirectObject() (Object, Reference, error) {
	number, err := s.ReadInteger()
	if err != nil {
		return nil, 0, err
	}
	generation, err := s.ReadInteger()
	if err != nil {
		return nil, 0, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, 0, err
	}

	err = s.SkipString("obj")
	if err != nil {
		return nil, 0, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, 0, err
	}

	// TODO(voss): check for overflow
	ref := NewReference(uint32(number), uint16(generation))
	if s.unencrypted[ref] {
		// some objects are not encrypted, e.g. xref dictionaries
		s.enc = nil
	} else {
		s.encRef = ref
	}

	obj, err := s.ReadObject()
	if err != nil {
		return nil, 0, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, 0, err
	}

	if a, ok := obj.(Integer); ok {
		// Check whether this is the start of a reference to an indirect
		// object.
		buf, err := s.Peek(6)
		if err != nil {
			return nil, 0, err
		}
		if !bytes.Equal(buf, []byte("endobj")) {
			b, err := s.ReadInteger()
			if err != nil {
				return nil, 0, err
			}
			err = s.SkipWhiteSpace()
			if err != nil {
				return nil, 0, err
			}
			err = s.SkipString("R")
			if err != nil {
				return nil, 0, err
			}
			err = s.SkipWhiteSpace()
			if err != nil {
				return nil, 0, err
			}

			// TODO(voss): check for overflow
			obj = NewReference(uint32(a), uint16(b))
		}
	}

	err = s.SkipString("endobj")
	if err != nil {
		return nil, 0, err
	}

	return obj, ref, nil
}

func (s *scanner) ReadObject() (Object, error) {
	buf, err := s.Peek(5) // len("false") == 5
	if err != nil {
		return nil, err
	}

	switch {
	case len(buf) == 0:
		// Test this first, so that we can use buf[0] in the following cases.
		return nil, &MalformedFileError{Err: io.EOF}
	case bytes.HasPrefix(buf, []byte("null")):
		s.bufPos += 4
		return nil, nil
	case bytes.HasPrefix(buf, []byte("true")):
		s.bufPos += 4
		return Bool(true), nil
	case bytes.HasPrefix(buf, []byte("false")):
		s.bufPos += 5
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
		if err != nil && err != io.EOF {
			return nil, err
		}
		buf, _ = s.Peek(6) // len("stream") == 6
		if !bytes.HasPrefix(buf, []byte("stream")) {
			return dict, nil
		}
		return s.ReadStreamData(dict)
	case buf[0] == '(':
		s.bufPos++
		return s.ReadQuotedString()
	case buf[0] == '<':
		s.bufPos++
		return s.ReadHexString()
	case buf[0] == '[':
		s.bufPos++
		return s.ReadArray()
	}

	return nil, &MalformedFileError{
		Err: fmt.Errorf("unexpected character %q", buf[0]),
	}
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
			Err: err,
			Loc: []string{"ReadInteger"},
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

	if s.enc != nil && s.encRef != 0 {
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

	if s.enc != nil && s.encRef != 0 {
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
			s.bufPos++
			k := len(array)
			// TODO(voss): check for overflow
			a := uint32(array[k-2].(Integer))
			b := uint16(array[k-1].(Integer))
			array = append(array[:k-2], NewReference(a, b))
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
	s.bufPos++ // we have already seen the closing "]"

	return array, nil
}

// ReadDict reads a PDF dictionary.
func (s *scanner) ReadDict() (dict Dict, err error) {
	defer func() {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		if err == io.ErrUnexpectedEOF {
			err = &MalformedFileError{
				Err: errors.New("unexpected EOF while reading Dict"),
			}
		} else if err != nil {
			err = wrap(err, fmt.Sprintf("byte %d", s.currentPos()))
		}
	}()

	err = s.SkipString("<<")
	if err != nil {
		return nil, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, err
	}

	dict = make(map[Name]Object)
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
				return nil, &MalformedFileError{Err: io.ErrUnexpectedEOF}
			}
			if buf[0] != '/' && buf[0] != '>' {
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
					return nil, &MalformedFileError{Err: errors.New("expected /Name but found Integer")}
				}
				s.bufPos++
				err = s.SkipWhiteSpace()
				if err != nil {
					return nil, err
				}

				// TODO(voss): check for overflow
				val = NewReference(uint32(a), uint16(b))
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
func (s *scanner) ReadStreamData(dict Dict) (stm *Stream, err error) {
	defer func() {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		if err == io.ErrUnexpectedEOF {
			err = &MalformedFileError{
				Err: errors.New("unexpected EOF while reading Stream"),
			}
		} else if err != nil {
			err = wrap(err, fmt.Sprintf("byte %d", s.currentPos()))
		}
	}()

	length, err := s.getInt(dict["Length"])
	if err != nil {
		return nil, wrap(err, "reading Length")
	} else if length < 0 {
		return nil, &MalformedFileError{
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
		s.bufPos++
	} else if len(buf) >= 2 && buf[0] == '\r' && buf[1] == '\n' {
		s.bufPos += 2
	} else if len(buf) >= 1 && buf[0] == '\r' {
		// not allowed by the spec, but seen in the wild
		s.bufPos++
	} else {
		return nil, &MalformedFileError{
			Err: errors.New("stream does not start with newline"),
		}
	}

	start := s.currentPos()
	l := int64(length)

	var streamData io.Reader
	if origReader, ok := s.r.(io.ReadSeeker); ok {
		streamData = &streamReader{
			r:   origReader,
			pos: start,
			end: start + l,
		}
		err = s.Discard(l)
		if err != nil {
			return nil, err
		}
	} else {
		// Does not happen in valid PDF files.
		// TODO(voss): can this be reached at all?
		return nil, &MalformedFileError{
			Err: errors.New("cannot seek"),
		}
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
			e.Err = errNoPDF
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
			Err: err,
			Loc: []string{"PDF header"},
		}
	}

	return ver, nil
}

// Refill discards the read part of the buffer and reads as much new data as
// possible.  Once the end of file is reached, s.bufEnd will be smaller than the
// buffer size, but no error will be returned.
func (s *scanner) refill() error {
	// move the remaining data to the beginning of the buffer
	s.filePos += int64(s.bufPos)
	copy(s.buf, s.buf[s.bufPos:s.bufEnd])
	s.bufEnd -= s.bufPos
	s.bufPos = 0

	// try to read more data
	n, err := io.ReadFull(s.r, s.buf[s.bufEnd:])
	s.bufEnd += n

	if n > 0 || err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	return err
}

// Peek returns a view of the next n bytes of input from the scanner's buffer.
// If n is larger than scannerBufSize, the function panics.
// If an EOF is encountered before n bytes can be read, the function returns
// the remaining bytes without an error code.
func (s *scanner) Peek(n int) ([]byte, error) {
	if n > scannerBufSize {
		panic("peek window too large")
	}

	var err error
	if s.bufPos+n > s.bufEnd {
		err = s.refill()
	}

	if s.bufPos+n > s.bufEnd {
		return s.buf[s.bufPos:s.bufEnd], err
	}

	return s.buf[s.bufPos : s.bufPos+n], nil
}

func (s *scanner) Discard(n int64) error {
	if n < 0 {
		panic(fmt.Sprintf("negative discard offset %d", n))
	}
	unread := int64(s.bufEnd - s.bufPos)
	if n <= unread {
		s.bufPos += int(n)
		return nil
	}

	n -= unread
	s.filePos += int64(s.bufEnd)
	s.bufPos = 0
	s.bufEnd = 0

	n, err := io.CopyN(io.Discard, s.r, n)
	s.filePos += n
	return err
}

// ScanBytes iterates over the bytes of s until `accept()` returns false. The
// scanner position after the call returns is the byte for which `accept()`
// returned false; the next read will start with this byte.
// If the end of the input is reached before `accept()` returns false,
// `ScanBytes` returns io.EOF.
func (s *scanner) ScanBytes(accept func(c byte) bool) error {
	empty := true
	for {
		for s.bufPos < s.bufEnd {
			if !accept(s.buf[s.bufPos]) {
				return nil
			}
			s.bufPos++
			empty = false
		}
		err := s.refill()
		if err == io.EOF && !empty {
			return nil
		}
		if s.bufEnd == 0 {
			if err == nil {
				err = io.EOF
			}
			return err
		}
	}
}

// SkipWhiteSpace skips over all whitespace characters until the next
// non-whitespace character. If the end of the input is reached, the function
// returns io.EOF.
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

// SkipString skips over the given string.
// If the string is not found, the function returns a MalformedFileError.
func (s *scanner) SkipString(pat string) error {
	patBytes := []byte(pat)
	n := len(patBytes)
	buf, err := s.Peek(n)
	if err != nil {
		return err
	}
	if !bytes.Equal(buf, patBytes) {
		return &MalformedFileError{
			Err: fmt.Errorf("expected %q but found %q", pat, string(buf)),
		}
	}
	s.bufPos += n
	return nil
}

func (s *scanner) SkipAfter(pat string) error {
	patBytes := []byte(pat)
	n := len(patBytes)
	if n > scannerBufSize {
		panic("SkipAfter target string too long")
	}

	for {
		idx := bytes.Index(s.buf[s.bufPos:s.bufEnd], patBytes)
		if idx >= 0 {
			s.bufPos += idx + n
			return nil
		}
		s.bufPos = s.bufEnd
		err := s.refill()
		if err != nil {
			return err
		}
		if s.bufEnd == 0 {
			return io.EOF
		}
	}
}

// find returns the next non-overlapping occurrence of the regular expression pat
// in the file. It returns the position of the match, and the submatches as
// returned by regexp.FindStringSubmatch.
func (s *scanner) find(pat *regexp.Regexp) (int64, []string, error) {
	for {
		// search for a match in the current buffer
		m := pat.FindSubmatchIndex(s.buf[s.bufPos:s.bufEnd])
		if m != nil {
			matchPos := s.filePos + int64(s.bufPos+m[0])

			// found a match
			res := make([]string, len(m)/2)
			for i := range res {
				a, b := m[2*i], m[2*i+1]
				if a >= 0 && b > a {
					res[i] = string(s.buf[s.bufPos+a : s.bufPos+b])
				}
			}

			s.bufPos += m[1]
			return matchPos, res, nil
		}

		// There are no more matches in the current buffer, so we read more data.
		// We need to be prepared for a partial match at the end of the buffer.
		nextPos := s.bufEnd - regexpOverlap
		if nextPos > s.bufPos {
			s.bufPos = nextPos
		}
		endBefore := s.bufEnd
		err := s.refill()
		if err != nil {
			return 0, nil, err
		}
		endAfter := s.bufEnd
		if endBefore < scannerBufSize && endBefore == endAfter {
			return 0, nil, io.EOF
		}
	}
}

type streamReader struct {
	r   io.ReadSeeker
	pos int64
	end int64
}

func (r *streamReader) Read(buf []byte) (int, error) {
	if r.pos >= r.end {
		return 0, io.EOF
	}
	if int64(len(buf)) > r.end-r.pos {
		buf = buf[:r.end-r.pos]
	}

	prevPos, err := r.r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	_, err = r.r.Seek(r.pos, io.SeekStart)
	if err != nil {
		return 0, err
	}
	n, err := r.r.Read(buf)
	r.pos += int64(n)
	if err != nil {
		return n, err
	}

	_, err = r.r.Seek(prevPos, io.SeekStart)
	if err != nil {
		return n, err
	}

	return n, nil
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
