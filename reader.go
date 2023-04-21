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
	"errors"
	"fmt"
	"io"
	"os"
)

// Reader represents a pdf file opened for reading. Use the functions [Open] or
// [NewReader] to create a new Reader.
type Reader struct {
	// Version is the PDF version used in this file.  This is specified in
	// the initial comment at the start of the file, and may be overridden by
	// the /Version entry in the document catalog.
	Version Version

	// The ID of the file.  This is either a slice of two byte slices (the
	// original ID of the file, and the ID of the current version), or nil if
	// the file does not specify an ID.
	ID [][]byte

	Catalog *Catalog

	infoObj   Object // the /Info entry of the trailer dictionary
	enc       *encryptInfo
	cleartext map[Reference]bool

	xref map[uint32]*xRefEntry

	r    io.ReadSeeker
	size int64

	level int
}

type ReaderOptions struct {
	// ReadPassword is a function that queries the user for a password for the
	// document with the given ID.  The function is called repeatedly, with
	// sequentially increasing values of try (starting at 0), until the correct
	// password is entered.  If the function returns the empty string, the
	// authentication attempt is aborted and an [AuthenticationError] is
	// reported to the caller.
	ReadPassword func(ID []byte, try int) string
}

var defaultReaderOptions = &ReaderOptions{}

// Open opens the named PDF file for reading.  After use, [Reader.Close] must
// be called to close the file the Reader is reading from.
func Open(fname string) (*Reader, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	return NewReader(fd, nil)
}

// NewReader creates a new Reader object.
func NewReader(data io.ReadSeeker, opt *ReaderOptions) (*Reader, error) {
	if opt == nil {
		opt = defaultReaderOptions
	}

	size, err := getSize(data)
	if err != nil {
		return nil, err
	}

	r := &Reader{
		size:      size,
		r:         data,
		cleartext: make(map[Reference]bool),
	}

	s, err := r.scannerFrom(0)
	if err != nil {
		return nil, err
	}
	version, err := s.readHeaderVersion()
	if err != nil {
		return nil, err
	}
	r.Version = version

	// Install a temporary xref table, so that we don't crash if the trailer
	// dictionary refers to indirect objects.
	r.xref = make(map[uint32]*xRefEntry)

	xref, trailer, err := r.readXRef()
	if err != nil {
		return nil, err
	}
	r.xref = xref // this is the real xref table

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
			r.cleartext[ref] = true
		}
		r.enc, err = r.parseEncryptDict(encObj, opt.ReadPassword)
		if err != nil {
			return nil, err
		}
		// TODO(voss): extract the permission bits
	}

	root := trailer["Root"]
	catalogDict, err := GetDict(r, root)
	if err != nil {
		return nil, err
	}
	r.Catalog = &Catalog{}
	err = r.DecodeDict(r.Catalog, catalogDict)
	if err != nil {
		return nil, err
	}

	if r.Catalog.Version > r.Version {
		// if unset, r.catalog.Version is zero and thus smaller than r.Version
		r.Version = r.Catalog.Version
	}

	r.infoObj = trailer["Info"]

	return r, nil
}

// AuthenticateOwner tries to authenticate the owner of a document. If a
// password is required, this calls the ReadPassword function specified in the
// [ReaderOptions] struct.  The return value is nil if the owner was
// authenticated (or if no authentication is required), and an object of type
// [AuthenticationError] if the required password was not supplied.
func (r *Reader) AuthenticateOwner() error {
	if r.enc == nil || r.enc.sec.OwnerAuthenticated {
		return nil
	}
	_, err := r.enc.sec.GetKey(true)
	return err
}

// Close closes the file underlying the reader.  This call only has an effect
// if the io.ReadSeeker passed to [NewReader] has a Close method, or if the
// Reader was created using [Open].  Otherwise, Close has no effect and
// returns nil.
//
// TODO(voss): don't unconditionally close the underlying file
func (r *Reader) Close() error {
	closer, ok := r.r.(io.Closer)
	if ok {
		return closer.Close()
	}
	return nil
}

// ReadInfo reads the PDF /Info dictionary for the file.
// If no Info dictionary is present, nil is returned.
func (r *Reader) ReadInfo() (*Info, error) {
	if r.infoObj == nil {
		return nil, nil
	}
	infoDict, err := GetDict(r, r.infoObj)
	if err != nil {
		return nil, err
	}
	info := &Info{}
	err = r.DecodeDict(info, infoDict)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// Get reads an indirect object from the PDF file.  If the object is not
// present, nil is returned without an error.
func (r *Reader) Get(ref Reference) (Object, error) {
	entry := r.xref[ref.Number()]
	if entry.IsFree() || entry.Generation != ref.Generation() {
		return nil, nil
	}

	if entry.InStream != 0 {
		return r.getFromObjectStream(ref.Number(), entry.InStream)
	} else {
		s, err := r.scannerFrom(entry.Pos)
		if err != nil {
			return nil, err
		}
		var fileRef Reference
		obj, fileRef, err := s.ReadIndirectObject()
		if err != nil {
			return nil, err
		}
		if ref != fileRef {
			return nil, &MalformedFileError{
				Pos: 0,
				Err: errors.New("xref corrupted"),
			}
		}
		return obj, nil
	}
}

// resolveNoStream is like Resolve, but returns nil if the resolved object is
// inside an object stream.
func (r *Reader) resolveNoStream(obj Object) (Object, error) {
	for {
		ref, isIndirect := obj.(Reference)
		if !isIndirect {
			return obj, nil
		}

		entry := r.xref[ref.Number()]
		if entry.IsFree() || entry.Generation != ref.Generation() || entry.InStream != 0 {
			return nil, nil
		}

		s, err := r.scannerFrom(entry.Pos)
		if err != nil {
			return nil, err
		}
		var fileRef Reference
		next, fileRef, err := s.ReadIndirectObject()
		if err != nil {
			return nil, err
		} else if ref != fileRef {
			return nil, &MalformedFileError{
				Pos: 0,
				Err: errors.New("xref corrupted"),
			}
		}

		obj = next
	}
}

type objStm struct {
	s   *scanner
	idx []stmObj
}

type stmObj struct {
	number uint32
	offs   int
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

	dec := r.enc
	if stream.isEncrypted {
		// Objects in encrypted streams are not encrypted again.
		dec = nil
	}

	decoded, err := r.DecodeStream(stream, 0)
	if err != nil {
		return nil, &MalformedFileError{
			Pos: errPos,
			Err: err,
		}
	}
	s := newScanner(decoded, r.safeGetInt, dec)

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
		// TODO(voss): check for overflow
		idx[i].number = uint32(no)
		idx[i].offs = int(offs)
	}

	pos := s.currentPos()
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

func (r *Reader) getFromObjectStream(number uint32, sRef Reference) (Object, error) {
	container, err := r.resolveNoStream(sRef)
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

	m := -1
	for i, info := range contents.idx {
		if info.number == number {
			m = i
			break
		}
	}
	if m < 0 {
		return nil, &MalformedFileError{
			Pos: r.errPos(sRef),
			Err: errors.New("object missing from stream"),
		}
	}

	info := contents.idx[m]

	delta := int64(info.offs) - contents.s.currentPos()
	if delta < 0 {
		return nil, nil
	}
	err = contents.s.Discard(delta)
	if err != nil {
		return nil, err
	}

	obj, err := contents.s.ReadObject()
	if err != nil {
		return nil, err
	}
	if _, isStream := obj.(*Stream); isStream {
		// Streams inside object streams are not allowed.
		return nil, nil
	}

	return obj, nil
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
	pos, err := r.r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	defer r.r.Seek(pos, io.SeekStart)

	val, err := GetInt(r, obj)
	if err != nil {
		return 0, err
	}
	r.level--
	return val, nil
}

// DecodeStream returns a reader for the decoded stream data.
// If numFilters is non-zero, only the first numFilters filters are decoded.
func (r *Reader) DecodeStream(x *Stream, numFilters int) (io.Reader, error) {
	var resolve func(Object) (Object, error)
	if r != nil {
		resolve = r.resolveNoStream
	}
	filters, err := x.Filters(resolve)
	if err != nil {
		return nil, err
	}

	out := x.R
	for i, fi := range filters {
		if numFilters > 0 && i >= numFilters {
			break
		}
		filter, err := fi.getFilter()
		if err != nil {
			return nil, err
		}
		out, err = filter.Decode(out)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Reader) scannerFrom(pos int64) (*scanner, error) {
	s := newScanner(r.r, r.safeGetInt, r.enc)
	s.cleartext = r.cleartext

	_, err := r.r.Seek(pos, io.SeekStart)
	if err != nil {
		return nil, err
	}
	s.filePos = pos

	return s, nil
}

func (r *Reader) errPos(obj Object) int64 {
	ref, ok := obj.(Reference)
	if !ok {
		return 0
	}
	if r.xref == nil {
		return 0
	}

	number := ref.Number()
	gen := ref.Generation()
	for i := 0; i < 8; i++ {
		entry := r.xref[number]
		if entry.IsFree() || entry.Generation != gen {
			return 0
		}

		if entry.InStream == 0 {
			return entry.Pos
		}
		number = entry.InStream.Number()
		gen = entry.InStream.Generation()
	}
	return 0
}

func getSize(r io.Seeker) (int64, error) {
	cur, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	_, err = r.Seek(cur, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return size, nil
}

type Getter interface {
	Get(Reference) (Object, error)
}

// Resolve resolves references to indirect objects.
//
// If obj is a [Reference], the function reads the corresponding object from
// the file and returns the result.  If obj is not a [Reference], it is
// returned unchanged.  The function recursively follows chains of references
// until it resolves to a non-reference object.
//
// If a reference loop is encountered, the function returns an error of type
// [MalformedFileError].
func Resolve(r Getter, obj Object) (Object, error) {
	count := 0
	for {
		ref, isReference := obj.(Reference)
		if !isReference {
			break
		}
		count++
		if count > 16 {
			return nil, &MalformedFileError{
				Pos: 0,
				Err: errors.New("too many levels of indirection"),
			}
		}

		var err error
		obj, err = r.Get(ref)
		if err != nil {
			return nil, err
		}
	}

	return obj, nil
}

func resolveAndCast[T Object](r Getter, obj Object) (x T, err error) {
	obj, err = Resolve(r, obj)
	if err != nil {
		return x, err
	}

	var isCorrectType bool
	x, isCorrectType = obj.(T)
	if isCorrectType {
		return x, nil
	}

	return x, &MalformedFileError{
		// Pos: r.errPos(obj), // TODO(voss): how to get the position?
		Err: fmt.Errorf("wrong object type (expected %T but got %T)", x, obj),
	}
}

// Helper functions for getting objects of a specific type.  Each of these
// functions calls Resolve on the object before attempting to convert it to the
// desired type.  If the object is `null`, nil is returned. If the object is of
// the wrong type, an error is returned.
var (
	GetArray  = resolveAndCast[Array]
	GetBool   = resolveAndCast[Bool]
	GetDict   = resolveAndCast[Dict]
	GetInt    = resolveAndCast[Integer]
	GetName   = resolveAndCast[Name]
	GetReal   = resolveAndCast[Real]
	GetStream = resolveAndCast[*Stream]
	GetString = resolveAndCast[String]
)
