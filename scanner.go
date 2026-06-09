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

	// maxScannerNestDepth caps the nesting depth of arrays and dictionaries
	// the scanner will accept.
	maxScannerNestDepth = 256
)

// Per-object size bounds, declared as variables so tests can substitute
// smaller values when exercising the bound checks.  In production, all
// are set well above any value seen in legitimate PDF input.
var (
	// maxStringBytes caps the byte length of string and hex-string objects.
	maxStringBytes = 16 * 1024 * 1024
	// maxNameBytes caps the byte length of name and number tokens.
	// PDF 1.7 requires at least 127 bytes for names.
	maxNameBytes = 4096
	// maxArrayLen caps the number of entries in an array.
	maxArrayLen = 1 << 20
	// maxDictLen caps the number of entries in a dictionary.
	maxDictLen = 64 << 10
)

// endstreamPat matches the spec-conformant terminator for a stream
// whose dictionary is missing /Length: a single EOL byte followed by
// the keyword endstream (PDF 7.3.8.2).  Used by
// [scanner.ReadStreamData] in the recovery path.
var endstreamPat = regexp.MustCompile(`[\r\n]endstream`)

type scanner struct {
	src     io.Reader
	filePos int64 // how far into the reader the start of buf is
	buf     []byte
	pos     int // current position within buf
	used    int // end of valid data within buf

	// fileReader, if set, is the underlying io.ReaderAt for the whole file.
	// This is used to create streamReader instances that can read at
	// absolute file positions.
	fileReader io.ReaderAt

	// GetInt is used to get the length of a stream, when the length is
	// specified as an indirect object.
	getInt getIntFn

	// scalarOnly, when set, makes ReadObject refuse composite objects
	// (dict, array, stream).  It is used while resolving an indirect
	// /Length, where the result must be a scalar integer.  Refusing
	// composites stops ReadObject before it can read a stream body, which
	// prevents the unbounded recursion a cyclic /Length would otherwise
	// cause.
	scalarOnly bool

	nestDepth int

	enc         *encryptInfo
	encRef      Reference
	unencrypted map[Reference]bool // objects with no encryption

	// err latches the first non-EOF error returned by src.Read.
	// Once set, refill returns it without re-reading from src.
	err error
}

type getIntFn func(Object) (Integer, error)

func newScanner(r io.Reader, getInt getIntFn, enc *encryptInfo) *scanner {
	return &scanner{
		src:    r,
		buf:    make([]byte, scannerBufSize),
		getInt: getInt,
		enc:    enc,
	}
}

// CurrentPos returns the current position in the file.
func (s *scanner) CurrentPos() int64 {
	return s.filePos + int64(s.pos)
}

func (s *scanner) ReadIndirectObject() (Native, Reference, error) {
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

	if number < 0 || number >= maxXRefSize || generation < 0 || generation > maxGeneration {
		return nil, 0, &MalformedFileError{
			Err: fmt.Errorf("invalid reference %d %d", number, generation),
		}
	}
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
		buf, err := s.PeekN(6)
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

			if a < 0 || a >= maxXRefSize || b < 0 || b > maxGeneration {
				return nil, 0, &MalformedFileError{
					Err: fmt.Errorf("invalid reference %d %d", a, b),
				}
			}
			obj = NewReference(uint32(a), uint16(b))
		}
	}

	err = s.SkipString("endobj")
	if err != nil {
		return nil, 0, err
	}

	return obj, ref, nil
}

func (s *scanner) ReadObject() (Native, error) {
	buf, err := s.PeekN(5) // len("false") == 5
	if err != nil {
		return nil, err
	}

	switch {
	case len(buf) == 0:
		// Test this first, so that we can use buf[0] in the following cases.
		return nil, &MalformedFileError{Err: io.EOF}
	case bytes.HasPrefix(buf, []byte("null")):
		s.pos += 4
		return nil, nil
	case bytes.HasPrefix(buf, []byte("true")):
		s.pos += 4
		return Boolean(true), nil
	case bytes.HasPrefix(buf, []byte("false")):
		s.pos += 5
		return Boolean(false), nil
	case buf[0] == '/':
		return s.ReadName()
	case buf[0] >= '0' && buf[0] <= '9', buf[0] == '+', buf[0] == '-', buf[0] == '.':
		return s.ReadNumber()
		// It is the caller's responsibility to check whether this is the start
		// of a reference.

	case bytes.HasPrefix(buf, []byte("<<")):
		if s.scalarOnly {
			return nil, &MalformedFileError{Err: errors.New("integer expected, got composite object")}
		}
		dict, err := s.ReadDict()
		if err != nil {
			return nil, err
		}

		// check whether this is the start of a stream
		err = s.SkipWhiteSpace()
		if err != nil && err != io.EOF {
			return nil, err
		}
		buf, _ = s.PeekN(6) // len("stream") == 6
		if !bytes.HasPrefix(buf, []byte("stream")) {
			return dict, nil
		}
		return s.ReadStreamData(dict)
	case buf[0] == '(':
		s.pos++
		return s.ReadString()
	case buf[0] == '<':
		s.pos++
		return s.ReadHexString()
	case buf[0] == '[':
		if s.scalarOnly {
			return nil, &MalformedFileError{Err: errors.New("integer expected, got composite object")}
		}
		s.pos++
		return s.ReadArray()
	}

	return nil, &MalformedFileError{
		Err: fmt.Errorf("unexpected character %q", buf[0]),
	}
}

// ReadInteger reads an integer, optionally preceded by white space.
func (s *scanner) ReadInteger() (Integer, error) {
	err := s.SkipWhiteSpace()
	if err != nil {
		return 0, err
	}

	first := true
	overflow := false
	var res []byte
	err = s.ScanBytes(func(b byte) bool {
		if first && (b == '+' || b == '-') {
			// ok
		} else if b >= '0' && b <= '9' {
			// ok
		} else {
			return false
		}
		first = false
		if len(res) < maxNameBytes {
			res = append(res, b)
		} else {
			overflow = true
		}
		return true
	})
	if err != nil && err != io.EOF {
		return 0, err
	}
	if overflow {
		return 0, &MalformedFileError{Err: errors.New("integer too long")}
	}

	x, err := strconv.ParseInt(string(res), 10, 64)
	if err != nil {
		return 0, &MalformedFileError{Err: err}
	}
	return Integer(x), nil
}

// ReadNumber reads an integer or real number.
func (s *scanner) ReadNumber() (Native, error) {
	hasDot := false
	first := true
	overflow := false
	var res []byte
	err := s.ScanBytes(func(b byte) bool {
		if !hasDot && b == '.' {
			hasDot = true
		} else if first && (b == '+' || b == '-') {
			// ok
		} else if b >= '0' && b <= '9' {
			// ok
		} else {
			return false
		}
		first = false
		if len(res) < maxNameBytes {
			res = append(res, b)
		} else {
			overflow = true
		}
		return true
	})
	if err != nil && err != io.EOF {
		return nil, err
	}
	if overflow {
		return nil, &MalformedFileError{Err: errors.New("number too long")}
	}

	if !hasDot {
		if x, err := strconv.ParseInt(string(res), 10, 64); err == nil {
			return Integer(x), nil
		}
		// fall through: very large integer literals are returned as Real
	}

	x, err := strconv.ParseFloat(string(res), 64)
	if err != nil {
		return nil, &MalformedFileError{Err: err}
	}
	return Real(x), nil
}

// ReadString reads a ()-delimited string, starting after the opening bracket.
func (s *scanner) ReadString() (String, error) {
	var res []byte
	bracketLevel := 1 // we are already inside the opening "("
	ignoreLF := false
	for {
		if len(res) >= maxStringBytes {
			return nil, &MalformedFileError{
				Err: errors.New("string too long"),
			}
		}
		b, err := s.ReadByte()
		if err != nil {
			return nil, err
		}
		if ignoreLF {
			ignoreLF = false
			if b == '\n' {
				continue
			}
		}
		switch b {
		case '(':
			bracketLevel++
			res = append(res, b)
		case ')':
			bracketLevel--
			if bracketLevel == 0 {
				if s.enc != nil && s.encRef != 0 {
					res, err = s.enc.DecryptBytes(s.encRef, res)
					if err != nil {
						return nil, err
					}
				}
				return String(res), nil
			}
			res = append(res, b)
		case '\\':
			esc, err := s.ReadByte()
			if err != nil {
				return nil, err
			}
			switch esc {
			case 'n':
				res = append(res, '\n')
			case 'r':
				res = append(res, '\r')
			case 't':
				res = append(res, '\t')
			case 'b':
				res = append(res, '\b')
			case 'f':
				res = append(res, '\f')
			case '\n':
				// line continuation
			case '\r':
				// line continuation; ignore an immediately following LF
				ignoreLF = true
			case '0', '1', '2', '3', '4', '5', '6', '7':
				oct := esc - '0'
				for range 2 {
					buf, err := s.PeekN(1)
					if err != nil && err != io.EOF {
						return nil, err
					}
					if len(buf) == 0 || buf[0] < '0' || buf[0] > '7' {
						break
					}
					oct = oct*8 + (buf[0] - '0')
					s.pos++
				}
				res = append(res, oct)
			default:
				res = append(res, esc)
			}
		case '\r':
			// unescaped CR or CR+LF, normalised to LF per PDF 7.3.4.2
			res = append(res, '\n')
			ignoreLF = true
		default:
			res = append(res, b)
		}
	}
}

// ReadHexString reads a <>-delimited string, starting after the opening
// angled bracket.
func (s *scanner) ReadHexString() (String, error) {
	var res []byte
	var hexVal byte
	first := true
	tooLong := false
	err := s.ScanBytes(func(b byte) bool {
		var d byte
		if b >= '0' && b <= '9' {
			d = b - '0'
		} else if b >= 'A' && b <= 'F' {
			d = b - 'A' + 10
		} else if b >= 'a' && b <= 'f' {
			d = b - 'a' + 10
		} else if b == '>' {
			return false
		} else {
			return true
		}
		if first {
			hexVal = d
		} else {
			if len(res) >= maxStringBytes {
				tooLong = true
				return false
			}
			res = append(res, 16*hexVal+d)
		}
		first = !first
		return true
	})
	if err != nil {
		return nil, err
	}
	if tooLong {
		return nil, &MalformedFileError{
			Err: errors.New("hex string too long"),
		}
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

	var res []byte
	for {
		buf, err := s.PeekN(1)
		if err != nil && err != io.EOF {
			return "", err
		}
		if len(buf) == 0 {
			break
		}
		if len(res) >= maxNameBytes {
			return "", &MalformedFileError{
				Err: errors.New("name too long"),
			}
		}
		b := buf[0]
		if b == '#' {
			if b, ok := s.tryHex(); ok {
				res = append(res, b)
				continue
			}
			// PDF 7.3.5: when "#" is not followed by two hex digits,
			// treat the "#" as a literal character.
			res = append(res, '#')
		} else if class[b] != regular {
			break
		} else {
			res = append(res, b)
		}
		s.pos++
	}

	return Name(res), nil
}

// tryHex peeks at "#" and the two bytes after it. If both are valid hex
// digits it consumes all three bytes and returns the decoded byte. Otherwise
// it returns (0, false) without consuming.
func (s *scanner) tryHex() (byte, bool) {
	buf, _ := s.PeekN(3)
	if len(buf) != 3 {
		return 0, false
	}
	hi := hexDigit(buf[1])
	lo := hexDigit(buf[2])
	if hi == 255 || lo == 255 {
		return 0, false
	}
	s.pos += 3
	return hi<<4 | lo, true
}

func hexDigit(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	}
	return 255
}

// ReadArray reads an array, starting after the opening "[".
func (s *scanner) ReadArray() (array Array, err error) {
	defer func() {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		if err == io.ErrUnexpectedEOF {
			err = &MalformedFileError{
				Err: errors.New("unexpected EOF while reading Array"),
			}
		} else if err != nil {
			err = Wrap(err, fmt.Sprintf("byte %d", s.CurrentPos()))
		}
	}()

	if s.nestDepth >= maxScannerNestDepth {
		return nil, &MalformedFileError{
			Err: errors.New("nesting depth exceeded"),
		}
	}
	s.nestDepth++
	defer func() { s.nestDepth-- }()

	// At this point we already have read the opening "[",
	// so we don't want to return nil.
	array = Array{}

	integersSeen := 0
	for {
		err = s.SkipWhiteSpace()
		if err != nil {
			return nil, err
		}

		var buf []byte
		buf, err = s.PeekN(1)
		if err != nil {
			return nil, err
		}
		if buf[0] == ']' {
			break
		}
		if integersSeen >= 2 && buf[0] == 'R' {
			s.pos++
			k := len(array)
			a := array[k-2].(Integer)
			b := array[k-1].(Integer)
			if a < 0 || a >= maxXRefSize || b < 0 || b > maxGeneration {
				array = append(array[:k-2], nil)
			} else {
				array = append(array[:k-2], NewReference(uint32(a), uint16(b)))
			}
			integersSeen = 0
			continue
		}

		var obj Native
		obj, err = s.ReadObject()
		if err != nil {
			return nil, err
		}

		if _, isInt := obj.(Integer); isInt {
			integersSeen++
		} else {
			integersSeen = 0
		}

		if len(array) >= maxArrayLen {
			return nil, &MalformedFileError{
				Err: errors.New("array too long"),
			}
		}
		array = append(array, obj)
	}
	s.pos++ // we have already seen the closing "]"

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
			err = Wrap(err, fmt.Sprintf("byte %d", s.CurrentPos()))
		}
	}()

	if s.nestDepth >= maxScannerNestDepth {
		return nil, &MalformedFileError{
			Err: errors.New("nesting depth exceeded"),
		}
	}
	s.nestDepth++
	defer func() { s.nestDepth-- }()

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
		if IsMalformed(err) {
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
			buf, err := s.PeekN(1)
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

				buf, err := s.PeekN(1)
				if err != nil {
					return nil, err
				}
				if buf[0] != 'R' {
					return nil, &MalformedFileError{Err: errors.New("expected /Name but found Integer")}
				}
				s.pos++
				err = s.SkipWhiteSpace()
				if err != nil {
					return nil, err
				}

				if a < 0 || a >= maxXRefSize || b < 0 || b > maxGeneration {
					val = nil
				} else {
					val = NewReference(uint32(a), uint16(b))
				}
			}
		}

		if _, exists := dict[key]; !exists && len(dict) >= maxDictLen {
			return nil, &MalformedFileError{
				Err: errors.New("dictionary too large"),
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
			err = Wrap(err, fmt.Sprintf("byte %d", s.CurrentPos()))
		}
	}()

	// /Length is required, but real-world PDFs (and fuzz mutations) omit
	// it.  Treat a missing entry as "unknown" and recover by scanning
	// ahead for endstream below.
	_, hasLength := dict["Length"]
	length, err := s.getInt(dict["Length"])
	if err != nil {
		return nil, Wrap(err, "reading Length")
	} else if length < 0 {
		return nil, &MalformedFileError{
			Err: errors.New("stream with negative length"),
		}
	}

	err = s.SkipString("stream")
	if err != nil {
		return nil, err
	}

	buf, err := s.PeekN(2)
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
		return nil, &MalformedFileError{
			Err: errors.New("stream does not start with newline"),
		}
	}

	origReader := s.fileReader
	if origReader == nil {
		return nil, &MalformedFileError{
			Err: errors.New("cannot read stream data"),
		}
	}
	start := s.CurrentPos()

	var crypt *filterCrypt
	if s.enc != nil {
		crypt = &filterCrypt{enc: s.enc, ref: s.encRef}
	}

	var l int64
	if hasLength {
		l = int64(length)
		err = s.Discard(l)
		if err != nil {
			return nil, err
		}
		err = s.SkipWhiteSpace()
		if err != nil {
			return nil, err
		}
		err = s.SkipString("endstream")
		if err != nil {
			return nil, err
		}
	} else {
		// recover by scanning forward for a spec-conformant
		// EOL+endstream (PDF 7.3.8.2); matching only when preceded by
		// an EOL byte avoids cutting streams whose content contains
		// the substring "endstream"
		eolPos, _, err := s.Find(endstreamPat)
		if err != nil {
			return nil, err
		}
		l = eolPos - start
		l = trimTrailingEOL(origReader, start, l)
	}

	return &Stream{
		Dict:   dict,
		data:   origReader,
		start:  start,
		length: l,
		crypt:  crypt,
	}, nil
}

// trimTrailingEOL returns length with any single trailing \n, \r, or
// \r\n removed.  The bytes before "endstream" are an EOL per spec
// (PDF 7.3.8.2) and must not be considered part of the stream
// content.
func trimTrailingEOL(r io.ReaderAt, start, length int64) int64 {
	if length <= 0 {
		return length
	}
	var probe [2]byte
	readAt := start + length - int64(len(probe))
	readLen := len(probe)
	if readAt < start {
		readAt = start
		readLen = int(length)
	}
	n, _ := r.ReadAt(probe[:readLen], readAt)
	if n == 0 {
		return length
	}
	switch probe[n-1] {
	case '\n':
		length--
		if n >= 2 && probe[n-2] == '\r' {
			length--
		}
	case '\r':
		length--
	}
	return length
}

func (s *scanner) ReadHeaderVersion() (Version, error) {
	err := s.SkipString("%PDF-")
	if err != nil {
		var e *MalformedFileError
		if errors.As(err, &e) {
			e.Err = errNoPDF
		}
		return 0, err
	}

	var buf []byte
	err = s.ScanBytes(func(b byte) bool {
		if b >= '0' && b <= '9' || b == '.' {
			buf = append(buf, b)
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

// PeekN returns a view of the next n bytes of input from the scanner's buffer.
// If n is larger than scannerBufSize, the function panics.
// If an EOF is encountered before n bytes can be read, the function returns
// the remaining bytes without an error code.
func (s *scanner) PeekN(n int) ([]byte, error) {
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

// ReadByte returns the next byte from the input, consuming it. It returns
// io.EOF if no more input is available.
func (s *scanner) ReadByte() (byte, error) {
	buf, err := s.PeekN(1)
	if err != nil && err != io.EOF {
		return 0, err
	}
	if len(buf) == 0 {
		return 0, io.EOF
	}
	c := buf[0]
	s.pos++
	return c, nil
}

func (s *scanner) Discard(n int64) error {
	if n < 0 {
		panic(fmt.Sprintf("negative discard offset %d", n))
	}
	unread := int64(s.used - s.pos)
	if n <= unread {
		s.pos += int(n)
		return nil
	}

	n -= unread
	s.filePos += int64(s.used)
	s.pos = 0
	s.used = 0

	n, err := io.CopyN(io.Discard, s.src, n)
	s.filePos += n
	return err
}

// ScanBytes iterates over the bytes of s until `accept()` returns false. The
// scanner position after the call returns is the byte for which `accept()`
// returned false; the next read will start with this byte.
// If the end of the input is reached before `accept()` returns false,
// `ScanBytes` returns io.EOF.
func (s *scanner) ScanBytes(accept func(b byte) bool) error {
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

// SkipWhiteSpace skips over all whitespace characters until the next
// non-whitespace character. If the end of the input is reached, the function
// returns io.EOF.
func (s *scanner) SkipWhiteSpace() error {
	isComment := false
	return s.ScanBytes(func(b byte) bool {
		if isComment {
			if b == '\r' || b == '\n' {
				isComment = false
			}
		} else if b == '%' {
			isComment = true
		} else {
			return class[b] == space
		}
		return true
	})
}

// SkipString skips over the given string.
// If the string is not found, the function returns a MalformedFileError.
func (s *scanner) SkipString(pat string) error {
	patBytes := []byte(pat)
	n := len(patBytes)
	buf, err := s.PeekN(n)
	if err != nil {
		return err
	}
	if !bytes.Equal(buf, patBytes) {
		return &MalformedFileError{
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

// Find returns the next non-overlapping occurrence of the regular expression pat
// in the file. It returns the position of the match, and the submatches as
// returned by regexp.FindStringSubmatch.
func (s *scanner) Find(pat *regexp.Regexp) (int64, []string, error) {
	for {
		// search for a match in the current buffer
		m := pat.FindSubmatchIndex(s.buf[s.pos:s.used])
		if m != nil {
			matchPos := s.filePos + int64(s.pos+m[0])

			// found a match
			res := make([]string, len(m)/2)
			for i := range res {
				a, b := m[2*i], m[2*i+1]
				if a >= 0 && b > a {
					res[i] = string(s.buf[s.pos+a : s.pos+b])
				}
			}

			s.pos += m[1]
			return matchPos, res, nil
		}

		// There are no more matches in the current buffer, so we read more data.
		// We need to be prepared for a partial match at the end of the buffer.
		nextPos := s.used - regexpOverlap
		if nextPos > s.pos {
			s.pos = nextPos
		}
		endBefore := s.used
		err := s.refill()
		if err != nil {
			return 0, nil, err
		}
		endAfter := s.used
		if endBefore < scannerBufSize && endBefore == endAfter {
			return 0, nil, io.EOF
		}
	}
}

// refill discards the read part of the buffer and reads as much new data as
// possible.  Once the end of file is reached, s.used will be smaller than the
// buffer size, but no error will be returned.  Non-EOF errors from src.Read
// are latched in s.err and returned on every subsequent call.
func (s *scanner) refill() error {
	if s.err != nil {
		return s.err
	}

	// move the remaining data to the beginning of the buffer
	s.filePos += int64(s.pos)
	copy(s.buf, s.buf[s.pos:s.used])
	s.used -= s.pos
	s.pos = 0

	// try to read more data
	n, err := io.ReadFull(s.src, s.buf[s.used:])
	s.used += n

	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	} else if err != nil {
		s.err = err
		if n > 0 {
			err = nil
		}
	}
	return err
}

type streamReader struct {
	r     io.ReaderAt
	start int64
	pos   int64
	end   int64
}

func (r *streamReader) Read(buf []byte) (int, error) {
	if r.pos >= r.end {
		return 0, io.EOF
	}
	if int64(len(buf)) > r.end-r.pos {
		buf = buf[:r.end-r.pos]
	}
	n, err := r.r.ReadAt(buf, r.pos)
	r.pos += int64(n)
	if err == io.EOF {
		if n > 0 {
			err = nil
		}
	}
	return n, err
}

func (r *streamReader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = r.start + offset
	case io.SeekCurrent:
		abs = r.pos + offset
	case io.SeekEnd:
		abs = r.end + offset
	default:
		return 0, errors.New("invalid whence")
	}

	if abs < r.start {
		abs = r.start
	} else if abs > r.end {
		abs = r.end
	}

	r.pos = abs
	return abs - r.start, nil
}

type characterClass byte

const (
	regular characterClass = iota
	space
	delimiter
)

// class classifies each byte per PDF 7.2.3 (whitespace + delimiters).
// regular is the zero value, so unlisted bytes are regular by default.
var class = [256]characterClass{
	0:   space,
	9:   space,
	10:  space,
	12:  space,
	13:  space,
	32:  space,
	'(': delimiter,
	')': delimiter,
	'<': delimiter,
	'>': delimiter,
	'[': delimiter,
	']': delimiter,
	'{': delimiter,
	'}': delimiter,
	'/': delimiter,
	'%': delimiter,
}
