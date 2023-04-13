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

	cache *lruCache
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
		cache:     newCache(100), // TODO(voss): make this configurable?
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

	xref, trailer, err := r.readXRef()
	if err != nil {
		return nil, err
	}
	r.xref = xref

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
	catalogDict, err := r.GetDict(root)
	if err != nil {
		return nil, err
	}
	r.Catalog = &Catalog{}
	err = catalogDict.Decode(r.Catalog, r.Resolve)
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

// GetInfo reads the PDF /Info dictionary for the file.
// If no Info dictionary is present, nil is returned.
func (r *Reader) GetInfo() (*Info, error) {
	if r.infoObj == nil {
		return nil, nil
	}
	infoDict, err := r.GetDict(r.infoObj)
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

// Resolve resolves references to indirect objects.
//
// If obj is of type [Reference], the function loads the corresponding object
// from the file and returns the result.  Otherwise, obj is returned unchanged.
func (r *Reader) Resolve(obj Object) (Object, error) {
	return r.doGet(obj, true)
}

func (r *Reader) doGet(obj Object, canStream bool) (Object, error) {
	origRef, ok := obj.(Reference)
	if !ok {
		return obj, nil
	}
	if r.xref == nil {
		return nil, &MalformedFileError{
			Pos: 0,
			Err: errors.New("cannot resolve references while reading xref table"),
		}
	}
	cachedObj, ok := r.cache.Get(origRef)
	if ok {
		return cachedObj, nil
	}

	count := 0
	for {
		ref, ok := obj.(Reference)
		if !ok {
			break
		}
		count++
		if count > 8 {
			return nil, &MalformedFileError{
				Pos: 0,
				Err: errors.New("too many indirection levels"),
			}
		}

		entry := r.xref[ref.Number()]
		if entry.IsFree() || entry.Generation != ref.Generation() {
			obj = nil
			break
		}

		if entry.InStream != 0 {
			if !canStream {
				return nil, &MalformedFileError{
					Pos: 0,
					Err: errors.New("streams inside streams not allowed"),
				}
			}

			var err error
			obj, err = r.getFromObjectStream(ref.Number(), entry.InStream)
			if err != nil {
				return nil, err
			}
		} else {
			s, err := r.scannerFrom(entry.Pos)
			if err != nil {
				return nil, err
			}
			var fileRef Reference
			obj, fileRef, err = s.ReadIndirectObject()
			if err != nil {
				return nil, err
			} else if ref != fileRef {
				return nil, &MalformedFileError{
					Pos: 0,
					Err: errors.New("xref corrupted"),
				}
			}
		}
	}

	if _, isStream := obj.(*Stream); !isStream {
		r.cache.Put(origRef, obj)
	}

	return obj, nil
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

	const readAhead = 7

	a := m - readAhead
	if a < 0 {
		a = 0
	}
	b := a + (2*readAhead + 1)
	if b > len(contents.idx) {
		b = len(contents.idx)
	}
	var res Object
	for i := a; i < b; i++ {
		info := contents.idx[i]
		key := NewReference(info.number, 0)
		if r.cache.Has(key) {
			if i == m {
				panic("should not happen")
			}
			continue
		}

		delta := int64(info.offs) - contents.s.currentPos()
		if delta != -1 { // allow for the separating white space to be included in both adjacent objects
			err = contents.s.Discard(delta)
			if err != nil {
				return nil, err
			}
		}
		obj, err := contents.s.ReadObject()
		if err != nil {
			return nil, err
		}
		if i == m {
			res = obj
		}
		r.cache.Put(key, obj)
	}

	return res, nil
}

// GetInt resolves references to indirect objects and makes sure the resulting
// object is an Integer.
func (r *Reader) GetInt(obj Object) (Integer, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return 0, err
	}
	val, ok := candidate.(Integer)
	if !ok {
		return 0, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("wrong object type (expected Integer)"),
		}
	}
	return val, nil
}

// GetDict resolves references to indirect objects and makes sure the resulting
// object is a dictionary.
func (r *Reader) GetDict(obj Object) (Dict, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return nil, err
	}
	val, ok := candidate.(Dict)
	if !ok {
		return nil, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: fmt.Errorf("wrong object type: expected Dict, got %T", candidate),
		}
	}

	return val, nil
}

// GetStream resolves references to indirect objects and makes sure the resulting
// object is a dictionary.
func (r *Reader) GetStream(obj Object) (*Stream, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return nil, err
	}
	val, ok := candidate.(*Stream)
	if !ok {
		return nil, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: fmt.Errorf("wrong object type: expected Stream, got %T", candidate),
		}
	}

	return val, nil
}

// GetArray resolves references to indirect objects and makes sure the resulting
// object is an array.
func (r *Reader) GetArray(obj Object) (Array, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return nil, err
	}
	val, ok := candidate.(Array)
	if !ok {
		return nil, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: fmt.Errorf("wrong object type: expected Array, got %T", candidate),
		}
	}

	return val, nil
}

// GetName resolves references to indirect objects and makes sure the resulting
// object is a Name.
func (r *Reader) GetName(obj Object) (Name, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return "", err
	}
	val, ok := candidate.(Name)
	if !ok {
		return "", &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("wrong object type (expected Name)"),
		}
	}
	return val, nil
}

// GetString resolves references to indirect objects and makes sure the
// resulting object is a String.
func (r *Reader) GetString(obj Object) (String, error) {
	candidate, err := r.Resolve(obj)
	if err != nil {
		return nil, err
	}
	val, ok := candidate.(String)
	if !ok {
		return nil, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("wrong object type (expected String)"),
		}
	}
	return val, nil
}

// GetRectangle resolves references to indirect objects and makes sure the
// resulting object is a PDF rectangle object.
// If the object is null, nil is returned.
func (r *Reader) GetRectangle(obj Object) (*Rectangle, error) {
	if obj == nil {
		return nil, nil
	}

	candidate, err := r.Resolve(obj)
	if err != nil {
		return nil, err
	}

	val, ok := candidate.(Array)
	if !ok && len(val) != 4 {
		return nil, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: fmt.Errorf("wrong object type: expected Rectangle, got %T", candidate),
		}
	}

	return val.AsRectangle()
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

	val, err := r.GetInt(obj)
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
		resolve = r.Resolve
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
	for {
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
