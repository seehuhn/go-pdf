// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"errors"
	"io"
	"regexp"
	"strconv"
)

type FileInfo struct {
	StartPos      int64
	Size          int64
	HeaderVersion string
	Sections      []*FileSection
}

type FileSection struct {
	StartPos      int64
	XRefPos       int64
	TrailerPos    int64
	StartXRefPos  int64
	EOFPos        int64
	Objects       []*FileObject
	Catalog       *FileObject
	ObjectStreams []*FileObject
}

type FileObject struct {
	Pos        int64
	End        int64
	Number     uint32
	Generation uint16
	Broken     bool
	Type       string
	SubType    Name
}

// SequentialScan reads a PDF file sequentially, extracting information
// about the file structure and the location of indirect objects.
// This can be used to attempt to read damaged PDF files, in particular
// in cases where the cross-reference table is missing or corrupt.
func SequentialScan(r io.ReadSeeker) (*FileInfo, error) {
	ss := &seqScanner{r: r}
	err := ss.init()
	if err != nil {
		return nil, err
	}

	err = ss.CheckObjects()
	if err != nil {
		return nil, err
	}

	return ss.info, nil
}

type seqScanner struct {
	r    io.ReadSeeker
	info *FileInfo
}

func (ss *seqScanner) init() error {
	r := ss.r

	_, err := r.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	s := newScanner(r, nil, nil)

	info := &FileInfo{}

	pos, m, err := s.find(startRegexp)
	if err == io.EOF {
		return ErrNoPDF
	} else if err != nil {
		return err
	}
	info.StartPos = pos
	info.HeaderVersion = m[1]

	section := &FileSection{}

	used := false
	inTrailer := false
	finish := func() {
		if used {
			info.Sections = append(info.Sections, section)
		}
		inTrailer = false
		used = false
		section = &FileSection{}
	}

scanLoop:
	for {
		pos, m, err = s.find(markerRegexp)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		pos += countLeadingSpaces(m[0])

		switch {
		case m[2] != "":
			// We found an indirect object.
			// m is of the form ["\n1 0 obj" "1 0 obj" "1" "0"]
			n, err := strconv.ParseUint(m[2], 10, 32)
			if err != nil {
				continue scanLoop
			}
			g, err := strconv.ParseUint(m[3], 10, 16)
			if err != nil {
				continue scanLoop
			}

			if inTrailer {
				finish()
			}
			obj := &FileObject{
				Pos:        pos,
				Number:     uint32(n),
				Generation: uint16(g),
			}
			section.Objects = append(section.Objects, obj)
			used = true
		case m[1] == "xref":
			section.XRefPos = pos
			inTrailer = true
			used = true
		case m[1] == "trailer":
			section.TrailerPos = pos
			inTrailer = true
			used = true
		case m[1] == "startxref":
			section.StartXRefPos = pos
			inTrailer = true
			used = true
		case m[1] == "%%EOF":
			section.EOFPos = pos
			finish()
		default:
			panic("unreachable")
		}
	}
	finish()

	info.Size, err = getSize(r)
	if err != nil {
		return err
	}

	ss.info = info
	return nil
}

func (ss *seqScanner) CheckObjects() error {
	for _, section := range ss.info.Sections {
		for _, obj := range section.Objects {
			x, endPos, err := ss.readObject(obj)
			if err != nil {
				if _, isBroken := err.(*MalformedFileError); isBroken {
					obj.Broken = true
					continue
				}
				return err
			}
			obj.End = endPos

			switch o := x.(type) {
			case Array:
				obj.Type = "Array"
			case Bool:
				obj.Type = "Bool"
			case Dict:
				obj.Type = "Dict"
				if t, ok := o["Type"].(Name); ok {
					obj.SubType = t

					if t == "Catalog" {
						_, hasPages := o["Pages"]
						if section.Catalog == nil || hasPages {
							section.Catalog = obj
						}
					}
				}
			case Integer:
				obj.Type = "Integer"
			case Name:
				obj.Type = "Name"
			case Real:
				obj.Type = "Real"
			case Reference:
				obj.Type = "Reference"
			case *Stream:
				obj.Type = "Stream"
				if t, ok := o.Dict["Type"].(Name); ok {
					obj.SubType = t

					if t == "ObjStm" {
						// TODO(voss): what to do if the generation number is not 0?
						_, hasFirst := o.Dict["First"]
						if hasFirst {
							section.ObjectStreams = append(section.ObjectStreams, obj)
						}
					}
				}
			case String:
				obj.Type = "String"
			}
		}
	}
	return nil
}

func (ss *seqScanner) readObject(obj *FileObject) (Object, int64, error) {
	_, err := ss.r.Seek(obj.Pos, io.SeekStart)
	if err != nil {
		return nil, 0, err
	}
	dummyGetInt := func(o Object) (Integer, error) {
		if i, ok := o.(Integer); ok {
			return i, nil
		}
		return 0, nil // We ignore stream data for now, so the length doesn't matter.
	}
	s := newScanner(ss.r, dummyGetInt, nil)
	s.filePos = obj.Pos
	x, ref, err := s.ReadIndirectObject()
	if err != nil {
		return nil, 0, err
	}
	if ref != NewReference(obj.Number, obj.Generation) {
		panic("unreachable") // TODO(voss): remove
	}
	return x, s.currentPos(), nil
}

// countLeadingSpaces returns the number of leading whitespace characters in s.
func countLeadingSpaces(s string) int64 {
	var n int64
	for n < int64(len(s)) && isSpace[s[n]] {
		n++
	}
	return n
}

var (
	ErrNoPDF = errors.New("PDF header not found")
)

var (
	startRegexp = regexp.MustCompile(`%PDF-([12]\.[0-9])[^0-9]`)

	whiteSpacePat = `[\000\011\014 ]+`
	eolPat        = `(?:\r|\n|\r\n)`
	objectPat     = `([0-9]+)` + whiteSpacePat + `([0-9]+)` + whiteSpacePat + `obj`
	markerPat     = eolPat + `(` + objectPat + `|xref|trailer|startxref|%%EOF)\b`
	markerRegexp  = regexp.MustCompile(markerPat)
)
