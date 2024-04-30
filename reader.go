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
	"math"
	"os"
)

// ReaderOptions provides additional information for opening a PDF file.
type ReaderOptions struct {
	// ReadPassword is a function that queries the user for a password for the
	// document with the given ID.  The function is called repeatedly, with
	// sequentially increasing values of try (starting at 0), until the correct
	// password is entered.  If the function returns the empty string, the
	// authentication attempt is aborted and [AuthenticationError] is reported
	// to the caller.
	ReadPassword func(ID []byte, try int) string

	ErrorHandling ReaderErrorHandling
}

type ReaderErrorHandling int

const (
	// ErrorHandlingRecover means that the reader will try to recover from
	// errors and continue parsing the file.  This is the default.
	//
	// This guarantees that the reader will return a valid Catalog object,
	// with a non-null Pages field.
	ErrorHandlingRecover = iota

	// ErrorHandlingReport means that the reader will try to recover from
	// errors and continue parsing the file, but will report errors to the
	// caller.
	//
	// This mode tolerates more errors than ErrorHandlingRecover does.
	// In particular, it is not guaranteed that there are any pages in the
	// document.
	ErrorHandlingReport

	// ErrorHandlingStop means that the reader will stop parsing the file as
	// soon as an error is encountered.
	ErrorHandlingStop
)

// Reader represents a pdf file opened for reading.  Use [Open] or
// [NewReader] to create a Reader.
type Reader struct {
	meta MetaInfo

	r      io.ReadSeeker
	closeR bool

	xref map[uint32]*xRefEntry

	enc         *encryptInfo
	unencrypted map[Reference]bool

	// Errors is a list of errors encountered while opening the file.
	// This is only used if the ErrorHandling option is set to
	// ErrorHandlingReport.
	Errors []*MalformedFileError
}

// Open opens the named PDF file for reading.  After use, [Reader.Close] must
// be called to close the file the Reader is reading from.
func Open(fname string, opt *ReaderOptions) (*Reader, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	r, err := NewReader(fd, opt)
	if err != nil {
		fd.Close()
		return nil, err
	}
	r.closeR = true
	return r, nil
}

// NewReader creates a new Reader object.
func NewReader(data io.ReadSeeker, opt *ReaderOptions) (*Reader, error) {
	if opt == nil {
		opt = &ReaderOptions{}
	}

	r := &Reader{
		r:           data,
		unencrypted: make(map[Reference]bool),
	}

	s, err := r.scannerFrom(0, false)
	if err != nil {
		return nil, err
	}
	version, err := s.readHeaderVersion()
	if err != nil {
		// TODO(voss): A PDF processor shall attempt to read any PDF file, even
		// if the PDF file’s version is more recent than that for which the PDF
		// processor was created.
		return nil, err
	}
	r.meta.Version = version

	// Install a dummy xref table first, so that we don't crash if an invalid
	// trailer dictionary attempts to use indirect objects.
	r.xref = make(map[uint32]*xRefEntry)

	xref, trailer, err := r.readXRef()
	if err != nil {
		return nil, Wrap(err, "xref")
	}
	r.xref = xref // Now we can install the real xref table.

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
	r.meta.ID = getIDDirect(IDObj)
	if encObj, ok := trailer["Encrypt"]; ok {
		// If the file is encrypted, ID is guaranteed to be a direct object.
		if r.meta.ID == nil {
			return nil, errors.New("file is encrypted, but no ID found")
		}
		if ref, ok := encObj.(Reference); ok {
			r.unencrypted[ref] = true
		}
		r.enc, err = r.parseEncryptDict(encObj, opt.ReadPassword)
		if err != nil {
			err = Wrap(err, "encryption dictionary")
			if shouldExit(err) {
				return nil, err
			}
		}
	} else if r.meta.ID == nil && IDObj != nil {
		// If the file is not encrypted, ID may be an indirect object.
		r.meta.ID, err = r.getID(IDObj)
		if shouldExit(err) {
			return nil, err
		}
	}
	if version >= V2_0 {
		for _, id := range r.meta.ID {
			if len(id) < 16 {
				err := &MalformedFileError{Err: errInvalidID}
				if shouldExit(err) {
					return nil, err
				}
				r.meta.ID = nil
				break
			}
		}
	}

	catalogDict, err := GetDict(r, trailer["Root"])
	if err != nil {
		err = Wrap(err, "document catalog")
		if shouldExit(err) {
			return nil, err
		}
	}
	r.meta.Catalog = &Catalog{}
	err = DecodeDict(r, r.meta.Catalog, catalogDict)
	if shouldExit(err) {
		return nil, err
	} else if r.meta.Catalog.Pages == 0 {
		err := &MalformedFileError{
			Err: errors.New("no pages in PDF document catalog"),
		}
		if opt.ErrorHandling == ErrorHandlingReport {
			r.Errors = append(r.Errors, err)
		} else {
			return nil, err
		}
	}
	if r.meta.Catalog.Version > r.meta.Version {
		r.meta.Version = r.meta.Catalog.Version
	}

	if r.meta.Version == V1_0 {
		r.meta.ID = nil
	}

	infoDict, err := GetDict(r, trailer["Info"])
	if shouldExit(err) {
		return nil, err
	}
	if infoDict != nil {
		r.meta.Info = &Info{}
		err = DecodeDict(r, r.meta.Info, infoDict)
		if shouldExit(err) {
			return nil, err
		}
	}

	// remove xref-related information from trailer dictionary
	delete(trailer, "Size")
	delete(trailer, "Prev")
	delete(trailer, "Type")
	delete(trailer, "Index")
	delete(trailer, "W")
	r.meta.Trailer = trailer

	return r, nil
}

// Close closes the Reader.
//
// This call only has an effect if the Reader was created by [Open].
func (r *Reader) Close() error {
	if r.closeR {
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
	if r.enc == nil || r.enc.sec.ownerAuthenticated {
		return nil
	}
	_, err := r.enc.sec.GetKey(true)
	return err
}

// Authenticate tries to authenticate the actions given in perm.  If a
// password is required, this calls the ReadPassword function specified in the
// [ReaderOptions] struct.  The return value is nil if the owner was
// authenticated (or if no authentication is required), and an object of type
// [AuthenticationError] if the required password was not supplied.
func (r *Reader) Authenticate(perm Perm) error {
	if r.enc == nil || r.enc.sec.key != nil {
		return nil
	}
	perm &= PermAll
	if perm&r.enc.UserPermissions == perm {
		return nil
	}
	_, err := r.enc.sec.GetKey(false)
	return err
}

func (r *Reader) GetMeta() *MetaInfo {
	return &r.meta
}

// Get reads an indirect object from the PDF file.  If the object is not
// present, nil is returned without an error.
//
// The argument canObjStm specifies whether the object may be read from an
// object stream.  Normally, this should be set to true.  If canObjStm is false
// and the object is in an object stream, an error is returned.
func (r *Reader) Get(ref Reference, canObjStm bool) (_ Object, err error) {
	if ref.IsInternal() {
		panic("internal reference") // TODO(voss): return an error instead?
	}

	defer func() {
		if err != nil {
			err = Wrap(err, "object "+ref.String())
		}
	}()

	entry := r.xref[ref.Number()]
	if entry.IsFree() || entry.Generation != ref.Generation() {
		return nil, nil
	}

	if entry.InStream != 0 {
		if !canObjStm {
			return nil, &MalformedFileError{
				Err: errors.New("object in object stream"),
				Loc: []string{"object " + ref.String()},
			}
		}
		return r.getFromObjectStream(ref.Number(), entry.InStream)
	} else {
		s, err := r.scannerFrom(entry.Pos, canObjStm)
		if err != nil {
			return nil, err
		}
		obj, fileRef, err := s.ReadIndirectObject()
		if err != nil {
			return nil, err
		}
		if ref != fileRef {
			return nil, &MalformedFileError{
				Err: errors.New("xref corrupted"),
				Loc: []string{"object " + ref.String() + "*"},
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
	container, err := resolve(r, sRef, false)
	if err != nil {
		return nil, err
	}
	objectStream, isStream := container.(*Stream)
	if !isStream {
		return nil, &MalformedFileError{
			Err: fmt.Errorf("got %T instead object stream", container),
			Loc: []string{"object " + sRef.String()},
		}
	}

	contents, err := r.objStmScanner(objectStream)
	if err != nil {
		return nil, Wrap(err, "object stream "+sRef.String())
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
			Err: fmt.Errorf("object %d not found", number),
			Loc: []string{"object stream " + sRef.String()},
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

func (r *Reader) getID(obj Object) ([][]byte, error) {
	arr, err := GetArray(r, obj)
	if err != nil {
		return nil, err
	}
	if len(arr) != 2 {
		return nil, &MalformedFileError{
			Err: errInvalidID,
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

// getIDDirect tries to extract the ID from an object without resolving
// references to indirect objects.  If the object is not a valid ID, or if it
// contains indirect references, nil is returned.
//
// This is only used before the encryption dictionary is parsed.
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

func (r *Reader) objStmScanner(stream *Stream) (_ *objStm, err error) {
	defer func() {
		if err != nil {
			err = Wrap(err, "decoding ObjStm")
		}
	}()

	N, ok := stream.Dict["N"].(Integer)
	if !ok || N < 0 || N > 10000 {
		return nil, &MalformedFileError{Err: errors.New("no valid /N")}
	}
	n := int(N)

	dec := r.enc
	if stream.isEncrypted {
		// Objects in encrypted streams are not encrypted again.
		dec = nil
	}

	decoded, err := DecodeStream(r, stream, 0)
	if err != nil {
		return nil, err
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
		if no < 0 || no > math.MaxUint32 || offs < 0 || offs > math.MaxInt {
			return nil, &MalformedFileError{Err: errors.New("invalid object number or offset")}
		}
		idx[i].number = uint32(no)
		idx[i].offs = int(offs)
	}

	pos := s.currentPos()
	first, ok := stream.Dict["First"].(Integer)
	if !ok || first < Integer(pos) {
		return nil, &MalformedFileError{Err: errors.New("no valid /First")}
	}
	for i := range idx {
		// TODO(voss): check for overflow
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

	i, err := GetInteger(r, obj)
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

	obj, err = resolve(r, obj, false)
	if err != nil {
		return 0, err
	}

	x, isInt := obj.(Integer)
	if !isInt {
		return x, &MalformedFileError{
			Err: fmt.Errorf("expected Integer but got %T", obj),
		}
	}

	_, err = r.r.Seek(pos, io.SeekStart)
	if err != nil {
		return 0, err
	}
	return x, nil
}

func (r *Reader) scannerFrom(pos int64, canObjStm bool) (*scanner, error) {
	getInt := r.safeGetInt
	if !canObjStm {
		getInt = r.safeGetIntNoObjectStreams
	}
	s := newScanner(r.r, getInt, r.enc)
	s.unencrypted = r.unencrypted

	_, err := r.r.Seek(pos, io.SeekStart)
	if err != nil {
		return nil, err
	}
	s.filePos = pos

	return s, nil
}
