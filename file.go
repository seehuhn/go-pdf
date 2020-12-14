package pdflib

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// File represents an open pdf file.
type File struct {
	Size int64
	Pos  int64

	fd            io.ReadSeeker
	headerVersion int
}

// NewFile creates a new File object.
func NewFile(data io.ReadSeeker) (*File, error) {
	file, err := newFile(data)
	if err != nil {
		return nil, err
	}
	err = file.checkHeader()
	if err != nil {
		return nil, err
	}
	return file, nil
}

func newFile(data io.ReadSeeker) (*File, error) {
	pos, err := data.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	return &File{
		Size: pos,
		Pos:  pos,

		fd:            data,
		headerVersion: -1,
	}, nil
}

func (f *File) checkHeader() error {
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
	f.headerVersion = int(val)
	return nil
}

// Get reads and returns a byte range from the file.
func (f *File) Get(from, to int64, shrinkIfNeeded bool) ([]byte, error) {
	if from < 0 || from > f.Size || to < from {
		return nil, errOutOfRange
	}
	if to > f.Size {
		if shrinkIfNeeded {
			to = f.Size
		} else {
			return nil, errOutOfRange
		}
	}
	if f.Pos != from {
		_, err := f.fd.Seek(from, io.SeekStart)
		if err != nil {
			f.Pos = -1
			return nil, err
		}
	}
	buf := make([]byte, to-from)
	_, err := io.ReadFull(f.fd, buf)
	if err != nil {
		f.Pos = -1
		return nil, err
	}
	f.Pos = to
	return buf, nil
}

func (f *File) expect(pos int64, pattern string) (int64, error) {
	// TODO(voss): change this to only match whole tokens?
	end := pos + int64(len(pattern))
	buf, err := f.Get(pos, end, true)
	if err != nil {
		return 0, err
	}
	if bytes.Equal(buf, []byte(pattern)) {
		return end, nil
	}
	return pos, errMalformed
}

func (f *File) expectBytes(pos int64, cont func(byte) bool) (int64, error) {
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

			buf, err = f.Get(start, start+int64(blockSize), true)
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

func (f *File) expectEOL(pos int64) (int64, error) {
	buf, err := f.Get(pos, pos+2, true)
	if err != nil {
		return 0, err
	}
	if len(buf) == 0 || (buf[0] != 0x0D && buf[0] != 0x0A) {
		return 0, errMalformed
	}
	if len(buf) > 1 && buf[0] == 0x0D && buf[1] == 0x0A {
		return pos + 2, nil
	}
	return pos + 1, nil
}

func (f *File) expectWhiteSpaceMaybe(pos int64) (int64, error) {
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

func (f *File) expectInteger(pos int64) (int64, int64, error) {
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

func (f *File) expectNumericOrReference(pos int64) (int64, PDFObject, error) {
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

	p3, err = f.expect(p3, "R")
	if err == errMalformed {
		return p2, PDFInt(x1), nil
	} else if err != nil {
		return 0, nil, err
	}

	return p3, PDFReference{x1, x2}, nil
}

func (f *File) expectName(pos int64) (int64, PDFName, error) {
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

func (f *File) expectBool(pos int64) (int64, PDFBool, error) {
	panic("not implemented")
}

func (f *File) expectString(pos int64) (int64, PDFString, error) {
	panic("not implemented")
}

func (f *File) expectArray(pos int64) (int64, PDFArray, error) {
	panic("not implemented")
}

func (f *File) expectDict(pos int64) (int64, PDFDict, error) {
	pos, err := f.expect(pos, "<<")
	if err != nil {
		return pos, nil, err
	}
	dict := make(PDFDict)
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

		dict[key] = val
	}
	pos, err = f.expect(pos, ">>")
	if err != nil {
		return 0, nil, err
	}

	return pos, dict, nil
}

func (f *File) expectDictOrStream(pos int64) (int64, PDFObject, error) {
	pos, dict, err := f.expectDict(pos)
	if err != nil {
		return pos, nil, err
	}

	p2, err := f.expectWhiteSpaceMaybe(pos)
	if err != nil {
		return 0, nil, err
	}
	p2, err = f.expect(p2, "stream")
	if err == errMalformed {
		// just a dict, no stream
		return pos, dict, nil
	}

	panic("not implemented")
}

func (f *File) expectObject(pos int64) (int64, PDFObject, error) {
	head, err := f.Get(pos, pos+2, true)
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
	case bytes.Equal(head, []byte("<<")): // needs to come before string
		return f.expectDictOrStream(pos)
	case head[0] == '(', head[0] == '<':
		return f.expectString(pos)
	case head[0] == '[':
		return f.expectArray(pos)
	case head[0] >= '0' && head[0] <= '9', head[0] == '+', head[0] == '-', head[0] == '.':
		return f.expectNumericOrReference(pos)
	}
	return pos, nil, errMalformed
}

func (f *File) expectXRef(pos int64) (int64, error) {
	pos, err := f.expect(pos, "xref")
	if err != nil {
		return pos, err
	}
	pos, err = f.expectEOL(pos)
	if err != nil {
		return pos, err
	}

	for {
		var start, length int64

		pos, start, err = f.expectInteger(pos)
		if err == errMalformed {
			break
		} else if err != nil {
			return 0, err
		}

		pos, err = f.expect(pos, " ")
		if err != nil {
			return pos, err
		}

		pos, length, err = f.expectInteger(pos)
		if err == errMalformed {
			break
		}

		pos, err = f.expectEOL(pos)
		if err != nil {
			return pos, err
		}

		fmt.Println("xref", start, length)
		pos += 20 * length
	}

	return pos, nil
}

func (f *File) expectTrailer(pos int64) (int64, error) {
	pos, err := f.expect(pos, "trailer")
	if err != nil {
		return pos, err
	}
	pos, err = f.expectWhiteSpaceMaybe(pos)
	if err != nil {
		return 0, err
	}
	pos, _, err = f.expectDict(pos)
	if err != nil {
		return 0, err
	}
	return pos, nil
}

func (f *File) findXRef() (int64, error) {
	pos, err := f.findStartXRef()
	if err != nil {
		return 0, err
	}

	pos, err = f.expect(pos, "startxref")
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

func (f *File) findStartXRef() (int64, error) {
	for sz := int64(32); sz <= 1024; sz *= 2 {
		if sz > f.Size {
			sz = f.Size
		}

		buf, err := f.Get(f.Size-sz, f.Size, false)
		if err != nil {
			return 0, err
		}

		idx := bytes.LastIndex(buf, []byte("startxref"))
		if idx >= 0 {
			return f.Size - sz + int64(idx), nil
		}

		if sz == f.Size {
			break
		}
	}
	return 0, errMalformed
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
