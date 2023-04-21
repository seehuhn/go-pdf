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
	"fmt"
	"io"
	"regexp"
	"strconv"
)

type FileInfo struct {
	R             io.ReadSeeker
	FileSize      int64
	PDFStart      int64
	PDFEnd        int64
	HeaderVersion string
	Sections      []*FileSection
}

// TODO(voss): add start and end offsets
type FileSection struct {
	XRefPos       int64
	TrailerPos    int64
	StartXRefPos  int64
	EOFPos        int64
	Objects       []*FileObject
	Catalog       *FileObject
	ObjectStreams []*FileObject
}

type FileObject struct {
	Reference
	ObjStart int64
	ObjEnd   int64
	Broken   bool
	Type     string
	SubType  Name
}

// SequentialScan reads a PDF file sequentially, extracting information
// about the file structure and the location of indirect objects.
// This can be used to attempt to read damaged PDF files, in particular
// in cases where the cross-reference table is missing or corrupt.
func SequentialScan(r io.ReadSeeker) (*FileInfo, error) {
	fi := &FileInfo{R: r}
	err := fi.locateObjects()
	if err != nil {
		return nil, err
	}

	err = fi.checkObjects()
	if err != nil {
		return nil, err
	}

	return fi, nil
}

type getIntFn func(Object) (Integer, error)

func (fi *FileInfo) Read(objInfo *FileObject) (Object, error) {
	var getInt getIntFn
	if objInfo.Type == "Stream" {
		getInt = fi.makeSafeGetInt()
	}
	obj, _, err := fi.doRead(objInfo, getInt)
	return obj, err
}

func (fi *FileInfo) doRead(objInfo *FileObject, getInt getIntFn) (Object, int64, error) {
	if objInfo == nil {
		return nil, 0, nil
	}

	// safe the current file position and restore it later
	prevPos, err := fi.R.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, 0, err
	}
	defer fi.R.Seek(prevPos, io.SeekStart)

	_, err = fi.R.Seek(objInfo.ObjStart, io.SeekStart)
	if err != nil {
		return nil, 0, err
	}
	s := newScanner(fi.R, getInt, nil)
	s.filePos = objInfo.ObjStart

	x, ref, err := s.ReadIndirectObject()
	if err != nil {
		return nil, 0, err
	}

	if ref != objInfo.Reference {
		panic("unreachable") // TODO(voss): remove
	}

	return x, s.currentPos(), nil
}

func (fi *FileInfo) MakeReader(opt *ReaderOptions) (*Reader, error) {
	// TODO(voss): unify as much code as possible with NewReader

	if opt == nil {
		opt = defaultReaderOptions
	}

	r := &Reader{
		r:           fi.R,
		unencrypted: make(map[Reference]bool),
	}

	version, err := ParseVersion(fi.HeaderVersion)
	if err != nil {
		return nil, err
	}
	r.Version = version

	r.xref = fi.makeXRef()

	trailer, err := fi.getTrailer()
	if err != nil {
		return nil, err
	}

	ID, ok := trailer["ID"].(Array)
	if ok && len(ID) >= 2 {
		for i := 0; i < 2; i++ {
			s, ok := ID[i].(String)
			if !ok {
				break
			}
			r.ID = append(r.ID, []byte(s))
		}
		if len(r.ID) != 2 {
			r.ID = nil
		}
	}

	if encObj, ok := trailer["Encrypt"]; ok {
		if ref, ok := encObj.(Reference); ok {
			r.unencrypted[ref] = true
		}
		r.enc, err = r.parseEncryptDict(encObj, opt.ReadPassword)
		if err != nil {
			return nil, err
		}
		// TODO(voss): extract the permission bits
	}

	shouldExit := func(err error) bool {
		if err == nil {
			return false
		}
		if opt.ErrorHandling == ErrorHandlingReport {
			if e, ok := err.(*MalformedFileError); ok {
				r.Errors = append(r.Errors, e)
				return false
			}
		}
		return opt.ErrorHandling != ErrorHandlingRecover
	}

	catalogDict, err := GetDict(r, trailer["Root"])
	if err != nil {
		return nil, err
	}
	r.Catalog = &Catalog{}
	err = r.DecodeDict(r.Catalog, catalogDict)
	if shouldExit(err) || r.Catalog.Pages == 0 {
		return nil, err
	}
	if r.Catalog.Version > r.Version {
		// if unset, r.catalog.Version is zero and thus smaller than r.Version
		r.Version = r.Catalog.Version
	}

	infoDict, err := GetDict(r, trailer["Info"])
	if shouldExit(err) {
		return nil, err
	}
	if infoDict != nil {
		r.Info = &Info{}
		err = r.DecodeDict(r.Info, infoDict)
		if shouldExit(err) {
			return nil, err
		}
	}

	return r, nil
}

func (fi *FileInfo) locateObjects() error {
	r := fi.R

	size, err := getSize(r)
	if err != nil {
		return err
	}
	fi.FileSize = size

	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	s := newScanner(r, nil, nil)

	pos, m, err := s.find(startRegexp)
	if err == io.EOF {
		return ErrNoPDF
	} else if err != nil {
		return err
	}
	fi.PDFStart = pos
	fi.HeaderVersion = m[1]

	section := &FileSection{}

	used := false
	inTrailer := false
	finish := func() {
		if used {
			fi.Sections = append(fi.Sections, section)
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
				ObjStart:  pos,
				Reference: NewReference(uint32(n), uint16(g)),
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

	err = s.SkipWhiteSpace()
	if err != nil && err != io.EOF {
		return err
	}
	fi.PDFEnd = s.currentPos()

	if len(fi.Sections) == 0 {
		return &MalformedFileError{
			Err: errors.New("no PDF content found in file"),
		}
	}

	return nil
}

func (fi *FileInfo) checkObjects() error {
	for _, section := range fi.Sections {
		for _, objInfo := range section.Objects {
			x, endPos, err := fi.doRead(objInfo, dummyGetInt)
			if err != nil {
				if _, isBroken := err.(*MalformedFileError); isBroken {
					objInfo.Broken = true
					continue
				}
				return err
			}
			objInfo.ObjEnd = endPos

			switch o := x.(type) {
			case Array:
				objInfo.Type = "Array"
			case Bool:
				objInfo.Type = "Bool"
			case Dict:
				objInfo.Type = "Dict"
				if t, ok := o["Type"].(Name); ok {
					objInfo.SubType = t

					if t == "Catalog" {
						_, hasPages := o["Pages"]
						if section.Catalog == nil || hasPages {
							section.Catalog = objInfo
						}
					}
				}
			case Integer:
				objInfo.Type = "Integer"
			case Name:
				objInfo.Type = "Name"
			case Real:
				objInfo.Type = "Real"
			case Reference:
				objInfo.Type = "Reference"
			case *Stream:
				objInfo.Type = "Stream"
				if t, ok := o.Dict["Type"].(Name); ok {
					objInfo.SubType = t

					if t == "ObjStm" {
						// TODO(voss): what to do if the generation number is not 0?
						_, hasFirst := o.Dict["First"]
						if hasFirst {
							section.ObjectStreams = append(section.ObjectStreams, objInfo)
						}
					}
				}
			case String:
				objInfo.Type = "String"
			}
		}
	}
	return nil
}

func (fi *FileInfo) findObject(ref Reference) *FileObject {
	// If an object is defined repeatedly, we use the last definition.
	for i := len(fi.Sections) - 1; i >= 0; i-- {
		sec := fi.Sections[i]
		for j := len(sec.Objects) - 1; j >= 0; j-- {
			obj := sec.Objects[j]
			if obj.Reference == ref {
				return obj
			}
		}
	}
	return nil
}

func (fi *FileInfo) makeSafeGetInt() getIntFn {
	var getInt func(obj Object) (Integer, error)

	seen := make(map[Reference]bool)
	getInt = func(obj Object) (Integer, error) {
		for {
			ref, isReference := obj.(Reference)
			if !isReference {
				break
			}

			if seen[ref] || len(seen) > 8 {
				return 0, errors.New("circular reference")
			}
			seen[ref] = true

			x, _, err := fi.doRead(fi.findObject(ref), getInt)
			if err != nil {
				return 0, err
			}
			obj = x
		}

		switch x := obj.(type) {
		case Integer:
			return x, nil
		case nil:
			return 0, errors.New("expected integer, got null")
		default:
			return 0, fmt.Errorf("expected integer, got %T", obj)
		}
	}
	return getInt
}

func (fi *FileInfo) makeXRef() map[uint32]*xRefEntry {
	// TODO(voss): locate objects in object streams.
	xref := make(map[uint32]*xRefEntry)
	for _, section := range fi.Sections {
		for _, obj := range section.Objects {
			if obj.Broken {
				continue
			}
			ref := obj.Reference
			if entry, ok := xref[ref.Number()]; ok && ref.Generation() < entry.Generation {
				continue
			}

			xref[obj.Reference.Number()] = &xRefEntry{
				Pos:        obj.ObjStart,
				Generation: obj.Reference.Generation(),
			}
		}
	}
	return xref
}

func (fi *FileInfo) getTrailer() (Dict, error) {
	for j := len(fi.Sections) - 1; j >= 0; j-- {
		sect := fi.Sections[j]

		// method 1: Try to find a cross-reference stream.  If there are several,
		// use the last one.
		var xrefStream *FileObject
		for _, obj := range sect.Objects {
			if obj.Type == "Stream" && obj.SubType == "XRef" {
				xrefStream = obj
			}
		}
		if xrefStream != nil {
			xref, err := fi.Read(xrefStream)
			if err == nil {
				stm, ok := xref.(*Stream)
				if ok && stm.Dict["Root"] != nil {
					return stm.Dict, nil
				}
			}
		}

		// method 2: Try to find a trailer dictionary.
		trailer, err := fi.readTrailer(sect)
		if err == nil {
			return trailer, nil
		}

		// TODO(voss): method 3: Try to collect all the pieces to build
		// our own trailer dictionary.
	}
	return nil, errors.New("no trailer found")
}

func (fi *FileInfo) readTrailer(sect *FileSection) (Dict, error) {
	if sect.TrailerPos == 0 {
		return nil, errors.New("no trailer found in section")
	}

	// safe the current file position and restore it later
	prevPos, err := fi.R.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	defer fi.R.Seek(prevPos, io.SeekStart)

	_, err = fi.R.Seek(sect.TrailerPos, io.SeekStart)
	if err != nil {
		return nil, err
	}
	s := newScanner(fi.R, dummyGetInt, nil)
	s.filePos = sect.TrailerPos

	err = s.SkipString("trailer")
	if err != nil {
		return nil, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, err
	}
	return s.ReadDict()
}

// countLeadingSpaces returns the number of leading whitespace characters in s.
func countLeadingSpaces(s string) int64 {
	var n int64
	for n < int64(len(s)) && isSpace[s[n]] {
		n++
	}
	return n
}

// dummyGetInt is a dummy function that implements the getInt function.
// It can be used if we don't care about the the contents of streams,
// but only want to read the stream dictionary.
func dummyGetInt(o Object) (Integer, error) {
	if i, ok := o.(Integer); ok {
		return i, nil
	}
	return 0, nil
}

var (
	ErrNoPDF = errors.New("PDF header not found")
)

var (
	startRegexp = regexp.MustCompile(`%PDF-([12]\.[0-9])[^0-9]`)

	whiteSpacePat = `[\000\011\014 ]+`
	eolPat        = `(?:\r\n|\r|\n|^)`
	objectPat     = `([0-9]+)` + whiteSpacePat + `([0-9]+)` + whiteSpacePat + `obj`
	markerPat     = eolPat + `(` + objectPat + `|xref|trailer|startxref|%%EOF)\b`
	markerRegexp  = regexp.MustCompile(markerPat)
)
