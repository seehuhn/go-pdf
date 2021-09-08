// seehuhn.de/go/pdf - support for reading and writing PDF files
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
	"io"
	"os"
	"strconv"
)

// Reader represents a pdf file opened for reading. Use the function Open() or
// NewReader() to create a new Reader.
type Reader struct {
	// Version is the PDF version used in this file.  This is specified in
	// the initial comment at the start of the file, and may be overridden by
	// the /Version entry in the document catalog.
	Version Version

	// The ID of the file.  This is either a slice of two byte slices (the
	// original ID of the file, and the ID of the current version), or nil if
	// the file does not specify an ID.
	ID [][]byte

	size int64
	r    io.ReaderAt

	pos     int64
	objStm  *objStm
	level   int
	special map[Reference]bool

	xref    map[int]*xRefEntry
	trailer Dict
	catalog *Catalog

	enc *encryptInfo
}

// ReadPwdFunc describes a function which can be used to query the user for a
// password for the document with the given ID.  The first call for each
// authentication attempt has try == 0.  If the returned password was wrong,
// the function is called again, repeatedly, with sequentially increasing
// values of try.  If the ReadPwdFunc return the empty string, the
// authentication attempt is aborted and an AuthenticationError is reported to
// the caller.
type ReadPwdFunc func(ID []byte, try int) string

// Open opens the named PDF file for reading.  After use, Close() must be
// called to close the file the Reader is reading from.
func Open(fname string) (*Reader, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	fi, err := fd.Stat()
	if err != nil {
		return nil, err
	}
	return NewReader(fd, fi.Size(), nil)
}

// NewReader creates a new Reader object.
func NewReader(data io.ReaderAt, size int64, readPwd ReadPwdFunc) (*Reader, error) {
	r := &Reader{
		size:    size,
		r:       data,
		special: make(map[Reference]bool),
	}

	s := r.scannerAt(0)
	version, err := s.readHeaderVersion()
	if err != nil {
		return nil, err
	}
	r.Version = version

	xref, trailer, err := r.readXRef()
	if err != nil {
		return nil, err
	}
	r.xref = xref
	r.trailer = trailer

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
		if ref, ok := encObj.(*Reference); ok {
			r.special[*ref] = true
		}
		r.enc, err = r.parseEncryptDict(encObj, readPwd)
		if err != nil {
			return nil, err
		}
	}

	root := trailer["Root"]
	catalogDict, err := r.getDict(root)
	if err != nil {
		return nil, err
	}
	r.catalog = &Catalog{}
	err = catalogDict.Decode(r.catalog, r.Resolve)
	if err != nil {
		return nil, err
	}

	if r.catalog.Version > r.Version {
		// if unset, r.catalog.Version is zero and thus smaller than r.Version
		r.Version = r.catalog.Version
	}

	return r, nil
}

// AuthenticateOwner tries to authenticate the owner of a document. If a
// password is required, this calls the `readPwd()` function specified in the
// call to `NewReader()`.  The return value is nil if the owner was
// authenticated (or if no authentication is required), and ErrNoAuth if the
// required password was not supplied.
func (r *Reader) AuthenticateOwner() error {
	if r.enc == nil || r.enc.sec.OwnerAuthenticated {
		return nil
	}
	_, err := r.enc.sec.GetKey(true)
	return err
}

// Close closes the file underlying the reader.  This call only has an effect
// if the io.ReaderAt passed to NewReader() has a Close() method, or if the
// Reader was created using Open().  Otherwise, Close() has no effect and
// returns nil.
func (r *Reader) Close() error {
	closer, ok := r.r.(io.Closer)
	if ok {
		return closer.Close()
	}
	return nil
}

// GetCatalog returns the PDF Catalog for the file.
func (r *Reader) GetCatalog() (*Catalog, error) {
	return r.catalog, nil
}

// GetInfo returns the PDF Info dictionary for the file.
func (r *Reader) GetInfo() (*Info, error) {
	infoDict, err := r.getDict(r.trailer["Info"])
	if err != nil {
		return nil, err
	}
	info := &Info{}
	err = infoDict.Decode(info, r.Resolve)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// ReadSequential returns the objects in a PDF file in the order they are
// stored in the file.  When the end of file has been reached, io.EOF is
// returned.  The read position is not affected by other methods of the Reader,
// sequential access can safely be interspersed with calls to `Resolve()`.
//
// The function returns the next object in the file, together with a Reference
// which can be used to read the object using `Resolve()`.
//
// ReadSequential makes some effort to repair problems in corrupted or
// malformed PDF files.  In particular, it may still work when the `Resolve()`
// method fails with errors.
func (r *Reader) ReadSequential() (Object, *Reference, error) {
	s := r.scannerAt(r.pos)

	for {
		if r.objStm != nil && len(r.objStm.idx) > 0 {
			s2 := r.objStm.s
			err := s2.Discard(int64(r.objStm.idx[0].offs) - s2.bytesRead())
			if err != nil {
				return nil, nil, err
			}
			obj, err := s2.ReadObject()
			if err != nil {
				return nil, nil, err
			}
			ref := &Reference{
				Number:     r.objStm.idx[0].number,
				Generation: 0,
			}

			if len(r.objStm.idx) > 1 {
				r.objStm.idx = r.objStm.idx[1:]
			} else {
				r.objStm = nil
			}

			return obj, ref, nil
		}

		err := s.SkipWhiteSpace()
		if err != nil {
			return nil, nil, err
		}
		r.pos = s.currentPos()

		buf, _ := s.Peek(9)
		switch {
		case bytes.HasPrefix(buf, []byte("xref")):
			err = s.SkipAfter("trailer")
			if err != nil {
				return nil, nil, err
			}
			err = s.SkipWhiteSpace()
			if err != nil {
				return nil, nil, err
			}
			_, err = s.ReadDict()
			if err != nil {
				return nil, nil, err
			}
			err = s.SkipWhiteSpace()
			if err != nil {
				return nil, nil, err
			}
			continue

		case bytes.HasPrefix(buf, []byte("startxref")):
			err = s.SkipString("startxref")
			if err != nil {
				return nil, nil, err
			}
			_, err = s.ReadInteger()
			if err != nil {
				return nil, nil, err
			}
			continue

		case len(buf) == 0:
			return nil, nil, io.EOF

		case buf[0] < '0' || buf[0] > '9':
			// Some PDF files embed random data.  Try to skip to the
			// next object.
			err = s.skipToNextObject()
			if err != nil {
				return nil, nil, err
			}
		}

		obj, ref, err := s.ReadIndirectObject()
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				r.pos = s.currentPos()
				err = io.EOF
			}
			return nil, nil, err
		}
		if stm, ok := obj.(*Stream); ok && stm.Dict["Type"] == Name("XRef") {
			// skip xref streams when reading objects sequentially
			continue
		}
		if stm, ok := obj.(*Stream); ok && stm.Dict["Type"] == Name("ObjStm") {
			contents, err := r.objStmScanner(stm, r.pos)
			if err != nil {
				return nil, nil, err
			}
			r.objStm = contents
			r.pos = s.currentPos()
			continue
		}

		r.pos = s.currentPos()
		return obj, ref, nil
	}
}

// Resolve resolves references to indirect objects.
//
// If obj is of type *Reference, the function loads the corresponding object
// from the file and returns the result.  Otherwise, obj is returned unchanged.
func (r *Reader) Resolve(obj Object) (Object, error) {
	return r.doGet(obj, true)
}

func (r *Reader) doGet(obj Object, canStream bool) (Object, error) {
	ref, ok := obj.(*Reference)
	if !ok {
		return obj, nil
	}

	if r.xref == nil {
		return nil, &MalformedFileError{
			Pos: 0,
			Err: errors.New("cannot use references while reading xref table"),
		}
	}

	entry := r.xref[ref.Number]
	if entry.IsFree() || entry.Generation != ref.Generation {
		return nil, nil
	}

	if entry.InStream != nil {
		if !canStream {
			return nil, &MalformedFileError{
				Pos: 0,
				Err: errors.New("object streams inside streams not allowed"),
			}
		}

		return r.getFromObjectStream(ref.Number, entry.InStream)
	}

	s := r.scannerAt(entry.Pos)
	obj, fileRef, err := s.ReadIndirectObject()
	if err != nil {
		return nil, err
	}

	if *ref != *fileRef {
		return nil, &MalformedFileError{
			Pos: 0,
			Err: errors.New("xref corrupted"),
		}
	}

	return obj, nil
}

type objStm struct {
	s   *scanner
	idx []stmObj
}

type stmObj struct {
	number, offs int
}

func (r *Reader) objStmScanner(stream *Stream, errPos int64) (*objStm, error) {
	N, ok := stream.Dict["N"].(Integer)
	if !ok || N < 0 || N > 10000 {
		return nil, &MalformedFileError{
			Pos: errPos,
			Err: errors.New("no valid /N for ObjStm"),
		}
	}
	n := int(N)

	var dec *encryptInfo
	if r.enc != nil && !stream.isEncrypted {
		dec = r.enc
	}

	decoded, err := stream.Decode(r.Resolve)
	if err != nil {
		return nil, &MalformedFileError{
			Pos: errPos,
			Err: err,
		}
	}
	s := newScanner(decoded, 0, r.safeGetInt, dec)

	idx := make([]stmObj, n)
	for i := 0; i < n; i++ {
		no, err := s.ReadInteger()
		if err != nil {
			return nil, err
		}
		offs, err := s.ReadInteger()
		if err != nil {
			return nil, err
		}
		idx[i].number = int(no)
		idx[i].offs = int(offs)
	}

	pos := s.bytesRead()
	first, ok := stream.Dict["First"].(Integer)
	if !ok || first < Integer(pos) {
		return nil, &MalformedFileError{
			Pos: errPos,
			Err: errors.New("no valid /First for ObjStm"),
		}
	}
	for i := range idx {
		idx[i].offs += int(first)
	}

	return &objStm{s: s, idx: idx}, nil
}

func (r *Reader) getFromObjectStream(number int, sRef *Reference) (Object, error) {
	container, err := r.doGet(sRef, false)
	if err != nil {
		return nil, err
	}
	stream, ok := container.(*Stream)
	if !ok {
		return nil, &MalformedFileError{
			Pos: r.errPos(sRef),
			Err: errors.New("wrong type for object stream"),
		}
	}

	contents, err := r.objStmScanner(stream, r.errPos(sRef))
	if err != nil {
		return nil, err
	}

	found := false
	for _, info := range contents.idx {
		if info.number == number {
			err = contents.s.Discard(int64(info.offs) - contents.s.bytesRead())
			if err != nil {
				return nil, err
			}
			found = true
			break
		}
	}
	if !found {
		return nil, &MalformedFileError{
			Pos: r.errPos(sRef),
			Err: errors.New("object missing from stream"),
		}
	}

	return contents.s.ReadObject()
}

// getDict resolves references to indirect objects and makes sure the resulting
// object is a dictionary.
func (r *Reader) getDict(obj Object) (Dict, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return nil, err
	}
	val, ok := candidate.(Dict)
	if !ok {
		return nil, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("wrong type (expected Dict)"),
		}
	}
	return val, nil
}

// getInt resolves references to indirect objects and makes sure the resulting
// object is an Integer.
func (r *Reader) getInt(obj Object) (Integer, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return 0, err
	}
	val, ok := candidate.(Integer)
	if !ok {
		return 0, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("wrong type (expected Integer)"),
		}
	}
	return val, nil
}

// getName resolves references to indirect objects and makes sure the resulting
// object is a Name.
func (r *Reader) getName(obj Object) (Name, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return "", err
	}
	val, ok := candidate.(Name)
	if !ok {
		return "", &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("wrong type (expected Name)"),
		}
	}
	return val, nil
}

// getString resolves references to indirect objects and makes sure the resulting
// object is a String.
func (r *Reader) getString(obj Object) (String, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return nil, err
	}
	val, ok := candidate.(String)
	if !ok {
		return nil, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("wrong type (expected String)"),
		}
	}
	return val, nil
}

func (r *Reader) safeGetInt(obj Object) (Integer, error) {
	if x, ok := obj.(Integer); ok {
		return x, nil
	}

	if r.level > 2 {
		return 0, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("length in ObjStm with Length in ... exceeded"),
		}
	}
	r.level++
	val, err := r.getInt(obj)
	r.level--
	return val, err
}

func (r *Reader) scannerAt(pos int64) *scanner {
	var enc *encryptInfo
	if r.enc != nil {
		enc = r.enc
	}
	s := newScanner(io.NewSectionReader(r.r, pos, r.size-pos), pos,
		r.safeGetInt, enc)
	s.special = r.special
	return s
}

func (r *Reader) errPos(obj Object) int64 {
	ref, ok := obj.(*Reference)
	if !ok {
		return 0
	}
	if r.xref == nil {
		return 0
	}

	number := ref.Number
	gen := ref.Generation
	for {
		entry := r.xref[number]
		if entry.IsFree() || entry.Generation != gen {
			return 0
		}

		if entry.InStream == nil {
			return entry.Pos
		}
		number = entry.InStream.Number
		gen = entry.InStream.Generation
	}
}

// Version represent the version of PDF standard used in a file.
type Version int

// Constants for the known PDF versions.
const (
	_ Version = iota
	V1_0
	V1_1
	V1_2
	V1_3
	V1_4
	V1_5
	V1_6
	V1_7
	tooHighVersion
)

// ParseVersion parses a PDF version string.
func ParseVersion(verString string) (Version, error) {
	switch verString {
	case "1.0":
		return V1_0, nil
	case "1.1":
		return V1_1, nil
	case "1.2":
		return V1_2, nil
	case "1.3":
		return V1_3, nil
	case "1.4":
		return V1_4, nil
	case "1.5":
		return V1_5, nil
	case "1.6":
		return V1_6, nil
	case "1.7":
		return V1_7, nil
	}
	return 0, errVersion
}

// ToString returns the string representation of ver, e.g. "1.7".
// If ver does not correspond to a supported PDF version, and error is
// returned.
func (ver Version) ToString() (string, error) {
	if ver >= V1_0 && ver <= V1_7 {
		return "1." + string([]byte{byte(ver - V1_0 + '0')}), nil
	}
	return "", errVersion
}

func (ver Version) String() string {
	versionString, err := ver.ToString()
	if err != nil {
		versionString = "pdf.Version(" + strconv.Itoa(int(ver)) + ")"
	}
	return versionString
}
