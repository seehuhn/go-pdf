package pdflib

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// Reader represents a pdf file opened for reading.
type Reader struct {
	size int64
	pos  int64
	r    io.ReaderAt

	HeaderVersion PDFVersion
	Trailer       *PDFDict
}

// NewReader creates a new Reader object.
func NewReader(data io.ReaderAt, size int64) (*Reader, error) {
	file := &Reader{
		size: size,
		r:    data,
	}
	err := file.checkHeader()
	if err != nil {
		return nil, err
	}

	pos, err := file.findXRef()
	if err != nil {
		return nil, err
	}
	_, dict, err := file.expectXRefAndTrailer(pos)
	if err != nil {
		return nil, err
	}
	file.Trailer = dict

	return file, nil
}

func (f *Reader) checkHeader() error {
	pos, err := f.expect(0, "%PDF-1.")
	if err != nil {
		return errMalformed
	}
	_, val, err := f.expectInteger(pos)
	if err != nil {
		return errMalformed
	}
	if val < 0 || val > 7 {
		return errVersion
	}
	f.HeaderVersion = PDFVersion(val)
	return nil
}

// get reads and returns a byte range from the file.
func (f *Reader) get(from, to int64, shrinkIfNeeded bool) ([]byte, error) {
	if from < 0 || from > f.size || to < from {
		return nil, errOutOfRange
	}
	if to > f.size {
		if shrinkIfNeeded {
			to = f.size
		} else {
			return nil, errOutOfRange
		}
	}
	buf := make([]byte, to-from)
	n, err := f.r.ReadAt(buf, from)
	if err == io.EOF && int64(n) == to-from {
		err = nil
	} else if err != nil {
		f.pos = -1
		return nil, err
	}
	f.pos = to
	return buf, nil
}

func (f *Reader) expect(pos int64, pattern string) (int64, error) {
	end := pos + int64(len(pattern))
	buf, err := f.get(pos, end, true)
	if err != nil {
		return 0, err
	}
	if !bytes.Equal(buf, []byte(pattern)) {
		return pos, errMalformed
	}
	return end, nil
}

func (f *Reader) expectWord(pos int64, word string) (int64, error) {
	n := len(word)
	end := pos + int64(n)
	buf, err := f.get(pos, end+1, true)
	if err != nil {
		return 0, err
	}
	if !bytes.HasPrefix(buf, []byte(word)) {
		return pos, errMalformed
	}
	if len(buf) > n &&
		(buf[n] >= 'A' && buf[n] <= 'Z' || buf[n] >= 'a' && buf[n] <= 'z') {
		return pos, errMalformed
	}
	return end, nil
}

func (f *Reader) expectBytes(pos int64, cont func(byte) bool) (int64, error) {
	blockSize := 32
	var buf []byte
	start := pos
	used := 0
gatherLoop:
	for {
		var err error
		if used >= len(buf) {
			start += int64(len(buf))
			used = 0

			buf, err = f.get(start, start+int64(blockSize), true)
			if err != nil {
				return 0, err
			} else if len(buf) == 0 {
				// EOF reached
				break gatherLoop
			}
		}
		// now we have used < len(buf)

		if !cont(buf[used]) {
			break gatherLoop
		}
		used++
	}

	return start + int64(used), nil
}

func (f *Reader) expectEOL(pos int64) (int64, error) {
	buf, err := f.get(pos, pos+2, true)
	if err != nil {
		return 0, err
	}
	if len(buf) == 0 || (buf[0] != '\r' && buf[0] != '\n') {
		return 0, errMalformed
	}
	if len(buf) > 1 && buf[0] == '\r' && buf[1] == '\n' {
		return pos + 2, nil
	}
	return pos + 1, nil
}

func (f *Reader) expectWhiteSpaceMaybe(pos int64) (int64, error) {
	isComment := false
	return f.expectBytes(pos, func(c byte) bool {
		if isComment {
			if c == '\r' || c == '\n' {
				isComment = false
			}
		} else {
			if c == '%' {
				isComment = true
			} else if !isSpace[c] {
				return false
			}
		}
		return true
	})
}

func (f *Reader) expectBool(pos int64) (int64, PDFBool, error) {
	var res PDFBool
	pos, err := f.expectWord(pos, "false")
	if err == errMalformed {
		pos, err = f.expectWord(pos, "true")
		if err == nil {
			res = PDFBool(true)
		}
	}
	return pos, res, err
}

func (f *Reader) expectInteger(pos int64) (int64, int64, error) {
	var res []byte
	first := true
	p2, err := f.expectBytes(pos, func(c byte) bool {
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
	if err != nil {
		return 0, 0, err
	}

	x, err := strconv.ParseInt(string(res), 10, 64)
	if err != nil {
		return pos, 0, errMalformed
	}
	return p2, x, nil
}

func (f *Reader) expectNumericOrReference(pos int64) (int64, PDFObject, error) {
	var res []byte
	hasDot := false
	first := true
	p2, err := f.expectBytes(pos, func(c byte) bool {
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
	if err != nil {
		return 0, nil, err
	}

	if hasDot {
		x, err := strconv.ParseFloat(string(res), 64)
		if err != nil {
			return pos, nil, errMalformed
		}
		return p2, PDFReal(x), nil
	}

	x1, err := strconv.ParseInt(string(res), 10, 64)
	if err != nil {
		return pos, nil, errMalformed
	}

	p3, err := f.expectWhiteSpaceMaybe(p2)
	if err != nil {
		return 0, nil, err
	}

	p3, x2, err := f.expectInteger(p3)
	if err == errMalformed {
		return p2, PDFInt(x1), nil
	} else if err != nil {
		return 0, nil, err
	}

	p3, err = f.expectWhiteSpaceMaybe(p3)
	if err != nil {
		return 0, nil, err
	}

	p3, err = f.expectWord(p3, "R")
	if err == errMalformed {
		return p2, PDFInt(x1), nil
	} else if err != nil {
		return 0, nil, err
	}

	return p3, &PDFReference{x1, x2}, nil
}

func (f *Reader) expectName(pos int64) (int64, PDFName, error) {
	pos, err := f.expect(pos, "/")
	if err != nil {
		return pos, "", err
	}

	var res []byte
	hex := 0
	var hexByte byte
	pos, err = f.expectBytes(pos, func(c byte) bool {
		if hex > 0 {
			var val byte
			if c >= '0' && c <= '9' {
				val = c - '0'
			} else if c >= 'A' && c <= 'F' {
				val = c - 'A'
			} else if c >= 'a' && c <= 'f' {
				val = c - 'a'
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
	if err != nil {
		return 0, "", err
	}

	return pos, PDFName(res), nil
}

func (f *Reader) expectQuotedString(pos int64) (int64, PDFString, error) {
	pos, err := f.expect(pos, "(")
	if err != nil {
		return pos, "", err
	}

	var res []byte
	parentCount := 0
	escape := false
	ignoreLF := false
	isOctal := 0
	octalVal := byte(0)
	pos, err = f.expectBytes(pos, func(c byte) bool {
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
		} else if c == '\r' {
			c = '\n'
			ignoreLF = true
		}
		res = append(res, c)
		return true
	})
	if err != nil {
		return pos, "", err
	}

	pos, err = f.expect(pos, ")")
	if err != nil {
		return pos, "", err
	}
	return pos, PDFString(res), nil
}

func (f *Reader) expectHexString(pos int64) (int64, PDFString, error) {
	pos, err := f.expect(pos, "<")
	if err != nil {
		return pos, "", err
	}

	var res []byte
	var hexVal byte
	first := true
	pos, err = f.expectBytes(pos, func(c byte) bool {
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
		return pos, "", err
	}
	if !first {
		res = append(res, 16*hexVal)
	}

	pos, err = f.expect(pos, ">")
	if err != nil {
		return pos, "", err
	}
	return pos, PDFString(res), nil
}

func (f *Reader) expectArray(pos int64) (int64, PDFArray, error) {
	pos, err := f.expect(pos, "[")
	if err != nil {
		return pos, nil, err
	}

	var array PDFArray
	for {
		pos, err = f.expectWhiteSpaceMaybe(pos)
		if err != nil {
			return 0, nil, err
		}

		var obj PDFObject
		pos, obj, err = f.expectObject(pos)
		if err == errMalformed {
			break
		} else if err != nil {
			return 0, nil, err
		}

		array = append(array, obj)
	}

	pos, err = f.expect(pos, "]")
	if err != nil {
		return pos, nil, err
	}
	return pos, array, nil
}

func (f *Reader) expectDict(pos int64) (int64, *PDFDict, error) {
	pos, err := f.expect(pos, "<<")
	if err != nil {
		return pos, nil, err
	}
	dict := &PDFDict{Data: make(map[PDFName]PDFObject)}
	for {
		pos, err = f.expectWhiteSpaceMaybe(pos)
		if err != nil {
			return 0, nil, err
		}

		var key PDFName
		pos, key, err = f.expectName(pos)
		if err == errMalformed {
			break
		} else if err != nil {
			return 0, nil, err
		}

		pos, err = f.expectWhiteSpaceMaybe(pos)
		if err != nil {
			return 0, nil, err
		}

		var val PDFObject
		pos, val, err = f.expectObject(pos)
		if err != nil {
			return 0, nil, err
		}

		dict.Data[key] = val
	}
	pos, err = f.expect(pos, ">>")
	if err != nil {
		return 0, nil, err
	}

	return pos, dict, nil
}

func (f *Reader) expectStream(pos int64) (int64, *PDFStream, error) {
	p2, ref, err := f.expectObjectLabel(pos)
	if err != nil {
		return p2, nil, err
	}

	p2, err = f.expectWhiteSpaceMaybe(p2)
	if err != nil {
		return 0, nil, err
	}

	p2, dict, err := f.expectDict(p2)
	if err == errMalformed {
		return pos, nil, errMalformed
	} else if err != nil {
		return 0, nil, err
	}

	p2, stream, err := f.expectStreamTail(p2, dict)
	if err == errMalformed {
		return pos, nil, errMalformed
	} else if err != nil {
		return 0, nil, err
	}
	stream.Ref = ref

	p2, err = f.expectWhiteSpaceMaybe(p2)
	if err != nil {
		return 0, nil, err
	}

	p2, err = f.expectWord(p2, "endobj")
	if err == errMalformed {
		return pos, nil, errMalformed
	} else if err != nil {
		return 0, nil, err
	}

	return p2, stream, nil
}

func (f *Reader) expectStreamTail(pos int64, dict *PDFDict) (int64, *PDFStream, error) {
	length, ok := dict.Data[PDFName("Length")].(PDFInt)
	if !ok {
		return pos, nil, errMalformed
	}

	p2, err := f.expectWhiteSpaceMaybe(pos)
	if err != nil {
		return 0, nil, err
	}

	p2, err = f.expectWord(p2, "stream")
	if err == errMalformed {
		return pos, nil, errMalformed
	} else if err != nil {
		return 0, nil, err
	}

	buf, err := f.get(p2, p2+2, true)
	if err != nil {
		return 0, nil, err
	}
	if len(buf) >= 1 && buf[0] == '\n' {
		p2++
	} else if len(buf) >= 2 && buf[0] == '\r' && buf[1] == '\n' {
		p2 += 2
	} else {
		return pos, nil, errMalformed
	}

	start := p2

	p2 += int64(length)
	if p2 >= f.size {
		return pos, nil, errMalformed
	}
	end := p2

	p2, err = f.expectWhiteSpaceMaybe(p2)
	if err != nil {
		return 0, nil, err
	}

	p2, err = f.expectWord(p2, "endstream")
	if err == errMalformed {
		return pos, nil, errMalformed
	} else if err != nil {
		return 0, nil, err
	}

	res := &PDFStream{
		PDFDict: *dict,
		R:       io.NewSectionReader(f.r, start, end-start),
	}
	return p2, res, nil
}

func (f *Reader) expectDictOrStream(pos int64) (int64, PDFObject, error) {
	pos, dict, err := f.expectDict(pos)
	if err != nil {
		return pos, nil, err
	}

	p2, err := f.expectWhiteSpaceMaybe(pos)
	if err != nil {
		return 0, nil, err
	}

	p3, stream, err := f.expectStreamTail(p2, dict)
	if err == errMalformed {
		// just a dict, it seems ...
		return pos, dict, nil
	} else if err != nil {
		return 0, nil, err
	}
	return p3, stream, err
}

func (f *Reader) expectObject(pos int64) (int64, PDFObject, error) {
	head, err := f.get(pos, pos+2, true)
	if err != nil {
		return 0, nil, err
	} else if len(head) == 0 {
		// we have reached EOF
		return pos, nil, errMalformed
	}

	switch {
	case bytes.Equal(head, []byte("tr")), bytes.Equal(head, []byte("fa")):
		return f.expectBool(pos)
	case head[0] == '/':
		return f.expectName(pos)
	case bytes.Equal(head, []byte("<<")): // this must come before hex strings
		return f.expectDictOrStream(pos)
	case head[0] == '(':
		return f.expectQuotedString(pos)
	case head[0] == '<':
		return f.expectHexString(pos)
	case head[0] == '[':
		return f.expectArray(pos)
	case head[0] >= '0' && head[0] <= '9', head[0] == '+', head[0] == '-', head[0] == '.':
		return f.expectNumericOrReference(pos)
	}
	return pos, nil, errMalformed
}

// read expressions like "12 0 obj"
func (f *Reader) expectObjectLabel(pos int64) (int64, *PDFReference, error) {
	p2, x, err := f.expectInteger(pos)
	if err != nil {
		return p2, nil, err
	}

	p2, err = f.expectWhiteSpaceMaybe(p2)
	if err != nil {
		return 0, nil, err
	}

	p2, y, err := f.expectInteger(p2)
	if err == errMalformed {
		return pos, nil, errMalformed
	} else if err != nil {
		return 0, nil, err
	}

	p2, err = f.expectWhiteSpaceMaybe(p2)
	if err != nil {
		return 0, nil, err
	}

	p2, err = f.expectWord(p2, "obj")
	if err == errMalformed {
		return pos, nil, errMalformed
	} else if err != nil {
		return 0, nil, err
	}

	return p2, &PDFReference{x, y}, nil
}

func (f *Reader) expectXRefAndTrailer(pos int64) (int64, *PDFDict, error) {
	pos, err := f.expectWord(pos, "xref")
	if err == errMalformed {
		pos, stream, err := f.expectStream(pos)
		return pos, &stream.PDFDict, err
	} else if err != nil {
		return pos, nil, err
	}

	pos, err = f.expectEOL(pos)
	if err != nil {
		return pos, nil, err
	}

	for {
		var start, length int64

		pos, start, err = f.expectInteger(pos)
		if err == errMalformed {
			break
		} else if err != nil {
			return 0, nil, err
		}

		pos, err = f.expect(pos, " ")
		if err != nil {
			return pos, nil, err
		}

		pos, length, err = f.expectInteger(pos)
		if err == errMalformed {
			break
		}

		pos, err = f.expectEOL(pos)
		if err != nil {
			return pos, nil, err
		}

		fmt.Println("xref", start, length)
		pos += 20 * length
		// TODO(voss): read the xref information
	}

	pos, dict, err := f.expectTrailer(pos)
	if err != nil {
		return 0, nil, err
	}

	return pos, dict, nil
}

func (f *Reader) expectTrailer(pos int64) (int64, *PDFDict, error) {
	pos, err := f.expectWord(pos, "trailer")
	if err != nil {
		return pos, nil, err
	}
	pos, err = f.expectWhiteSpaceMaybe(pos)
	if err != nil {
		return 0, nil, err
	}
	pos, dict, err := f.expectDict(pos)
	if err != nil {
		return 0, nil, err
	}
	return pos, dict, nil
}

func (f *Reader) findXRef() (int64, error) {
	pos := int64(-1)
	for sz := int64(32); sz <= 1024; sz *= 2 {
		if sz > f.size {
			sz = f.size
		}

		buf, err := f.get(f.size-sz, f.size, false)
		if err != nil {
			return 0, err
		}

		idx := bytes.LastIndex(buf, []byte("startxref"))
		if idx >= 0 {
			pos = f.size - sz + int64(idx)
			break
		}

		if sz == f.size {
			break
		}
	}
	if pos < 0 {
		return 0, errMalformed
	}

	pos, err := f.expectWord(pos, "startxref")
	if err != nil {
		return 0, err
	}

	pos, err = f.expectEOL(pos)
	if err != nil {
		return 0, err
	}

	_, val, err := f.expectInteger(pos)
	return val, err
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

// PDFVersion represent the version of PDF standard used in a file.
type PDFVersion int

// Constants for the known PDF versions.
const (
	V1_0 PDFVersion = iota
	V1_1
	V1_2
	V1_3
	V1_4
	V1_5
	V1_6
	V1_7
)
