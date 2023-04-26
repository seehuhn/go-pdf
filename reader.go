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

// Reader represents a pdf file opened for reading.  Use [Open] or
// [NewReader] to create a Reader.
type Reader struct {
	// Version is the PDF version used in this file.  This is specified in
	// the initial comment at the start of the file, and may be overridden by
	// the /Version entry in the document catalog.
	Version Version

	// The ID of the file.  This is either a slice of two byte slices (the
	// original ID of the file, and the ID of the current version), or nil if
	// the file does not specify an ID.
	ID [][]byte

	// Catalog is the document catalog for this file.
	Catalog *Catalog

	// Info is the document information dictionary for this file.
	// This is nil if the file does not contain a document information
	// dictionary.
	Info *Info

	// Errors is a list of errors encountered while opening the file.
	// This is only used if the ErrorHandling option is set to
	// ErrorHandlingReport.
	Errors []*MalformedFileError

	r          io.ReadSeeker
	needsClose bool

	xref map[uint32]*xRefEntry

	enc         *encryptInfo
	unencrypted map[Reference]bool
}

type ReaderOptions struct {
	// ReadPassword is a function that queries the user for a password for the
	// document with the given ID.  The function is called repeatedly, with
	// sequentially increasing values of try (starting at 0), until the correct
	// password is entered.  If the function returns the empty string, the
	// authentication attempt is aborted and an [AuthenticationError] is
	// reported to the caller.
	ReadPassword func(ID []byte, try int) string

	ErrorHandling ReaderErrorHandling
}

type ReaderErrorHandling int

const (
	// ErrorHandlingRecover means that the reader will try to recover from
	// errors and continue parsing the file.  This is the default.
	ErrorHandlingRecover = iota

	// ErrorHandlingReport means that the reader will try to recover from
	// errors and continue parsing the file, but will report errors to the
	// caller.
	ErrorHandlingReport

	// ErrorHandlingStop means that the reader will stop parsing the file as
	// soon as an error is encountered.
	ErrorHandlingStop
)

var defaultReaderOptions = &ReaderOptions{}

// Open opens the named PDF file for reading.  After use, [Reader.Close] must
// be called to close the file the Reader is reading from.
func Open(fname string) (*Reader, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	r, err := NewReader(fd, nil)
	if err != nil {
		fd.Close()
		return nil, err
	}
	r.needsClose = true
	return r, nil
}

// NewReader creates a new Reader object.
func NewReader(data io.ReadSeeker, opt *ReaderOptions) (*Reader, error) {
	if opt == nil {
		opt = defaultReaderOptions
	}

	r := &Reader{
		r:           data,
		unencrypted: make(map[Reference]bool),
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

	// Install a dummy xref table first, so that we don't crash if the trailer
	// dictionary tries to use indirect objects.
	r.xref = make(map[uint32]*xRefEntry)

	xref, trailer, err := r.readXRef()
	if err != nil {
		return nil, err
	}
	// Now we can install the real xref table.
	r.xref = xref

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

	IDObj := trailer["ID"]
	r.ID = getIDDirect(IDObj)
	if encObj, ok := trailer["Encrypt"]; ok {
		// If the file is encrypted, ID is guaranteed to be a direct object.
		if r.ID == nil {
			return nil, errors.New("file is encrypted, but has no ID")
		}
		if ref, ok := encObj.(Reference); ok {
			r.unencrypted[ref] = true
		}
		r.enc, err = r.parseEncryptDict(encObj, opt.ReadPassword)
		if err != nil {
			return nil, err
		}
	} else if r.ID == nil && IDObj != nil {
		// If the file is not encrypted, ID may be an indirect object.
		r.ID, err = r.getID(IDObj)
		if shouldExit(err) {
			return nil, err
		}
	}
	for _, id := range r.ID {
		if len(id) < 16 {
			err := &MalformedFileError{
				Err: errShortID,
			}
			if shouldExit(err) {
				return nil, err
			} else {
				r.ID = nil
				break
			}
		}
	}

	catalogDict, err := GetDict(r, trailer["Root"])
	if err != nil {
		return nil, err
	}
	r.Catalog = &Catalog{}
	err = r.DecodeDict(r.Catalog, catalogDict)
	if shouldExit(err) || (err != nil && r.Catalog.Pages == 0) {
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

// Close closes the file underlying the reader.  This call only has an effect
// if the Reader was created by [Open].
func (r *Reader) Close() error {
	if r.needsClose {
		err := r.r.(io.Closer).Close()
		if err != nil {
			return err
		}
	}
	return nil
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

// AuthenticateOwner tries to authentica the actions given in perm.  If a
// password is required, this calls the ReadPassword function specified in the
// [ReaderOptions] struct.  The return value is nil if the owner was
// authenticated (or if no authentication is required), and an object of type
// [AuthenticationError] if the required password was not supplied.
func (r *Reader) Authenticate(perm Perm) error {
	if r.enc == nil || r.enc.sec.OwnerAuthenticated {
		return nil
	}
	perm = perm & PermAll
	if perm&r.enc.UserPermissions == perm {
		return nil
	}
	_, err := r.enc.sec.GetKey(true)
	return err
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

func (r *Reader) getFromObjectStream(number uint32, sRef Reference) (Object, error) {
	// We need to be careful to avoid infinite loops, when reading an object
	// from an object stream requires opening other object streams first. This
	// could be either caused by the stream object being contained in another
	// object stream, or by the length of the stream object being contained in
	// another object stream.  (Both cases are forbidden by the PDF spec.)
	container, err := r.resolveNoObjStreams(sRef)
	if err != nil {
		return nil, err
	}
	objectStream, isStream := container.(*Stream)
	if !isStream {
		return nil, &MalformedFileError{
			Pos: r.errPos(sRef),
			Err: errors.New("wrong type for object stream"),
		}
	}

	contents, err := r.objStmScanner(objectStream, r.errPos(sRef))
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

	return obj, nil
}

// resolveNoObjStreams is like Resolve, but returns an error if the resolved
// object is inside an object stream.
func (r *Reader) resolveNoObjStreams(obj Object) (Object, error) {
	for {
		ref, isIndirect := obj.(Reference)
		if !isIndirect {
			return obj, nil
		}

		entry := r.xref[ref.Number()]
		if entry.IsFree() || entry.Generation != ref.Generation() {
			return nil, nil
		}
		if entry.InStream != 0 {
			return nil, errors.New("forbidden object inside object stream")
		}

		s, err := r.scannerFrom(entry.Pos)
		if err != nil {
			return nil, err
		}
		s.getInt = r.safeGetIntNoObjectStreams

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

// DecodeStream returns a reader for the decoded stream data.
// If numFilters is non-zero, only the first numFilters filters are decoded.
func (r *Reader) DecodeStream(x *Stream, numFilters int) (io.Reader, error) {
	var resolve func(Object) (Object, error)
	if r != nil {
		resolve = r.resolveNoObjStreams
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

func (r *Reader) getID(obj Object) ([][]byte, error) {
	arr, err := GetArray(r, obj)
	if err != nil {
		return nil, err
	}
	if len(arr) != 2 {
		return nil, &MalformedFileError{
			Err: errors.New("invalid ID"),
		}
	}
	id := make([][]byte, 2)
	for i, obj := range arr {
		s, err := GetString(r, obj)
		if err != nil {
			return nil, err
		}
		id[i] = []byte(s)
	}
	return id, nil
}

func getIDDirect(obj Object) [][]byte {
	if obj == nil {
		return nil
	}
	arr, ok := obj.(Array)
	if !ok || len(arr) != 2 {
		return nil
	}
	id := make([][]byte, 2)
	for i, obj := range arr {
		s, ok := obj.(String)
		if !ok {
			return nil
		}
		id[i] = []byte(s)
	}
	return id
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

// safeGetInt is like GetInt, but it restores the file position after reading.
func (r *Reader) safeGetInt(obj Object) (Integer, error) {
	if x, ok := obj.(Integer); ok {
		return x, nil
	}

	pos, err := r.r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	i, err := GetInt(r, obj)
	if err != nil {
		r.r.Seek(pos, io.SeekStart)
		return 0, err
	}

	_, err = r.r.Seek(pos, io.SeekStart)
	if err != nil {
		return 0, err
	}
	return i, nil
}

// safeGetIntNoObjectStreams is like safeGetInt, but it returns an error if the
// integer is stored inside an object stream.
func (r *Reader) safeGetIntNoObjectStreams(obj Object) (Integer, error) {
	if x, ok := obj.(Integer); ok {
		return x, nil
	}

	pos, err := r.r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	obj, err = r.resolveNoObjStreams(obj)
	if err != nil {
		return 0, err
	}

	i, isInt := obj.(Integer)
	if !isInt {
		return i, &MalformedFileError{
			// Pos: r.errPos(obj), // TODO(voss): how to get the position?
			Err: fmt.Errorf("wrong object type (expected Integer but got %T)", obj),
		}
	}

	_, err = r.r.Seek(pos, io.SeekStart)
	if err != nil {
		return 0, err
	}
	return i, nil
}

func (r *Reader) scannerFrom(pos int64) (*scanner, error) {
	s := newScanner(r.r, r.safeGetInt, r.enc)
	s.unencrypted = r.unencrypted

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

	if obj == nil {
		return x, nil
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
// desired type.  If the object is `null`, a zero object is returned witout
// error.  If the object is of the wrong type, an error is returned.
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
