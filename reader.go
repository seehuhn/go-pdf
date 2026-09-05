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
	"math"
	"os"
	"slices"

	"seehuhn.de/go/pdf/internal/limits"
)

// ReaderOptions provides additional information for opening a PDF file.
type ReaderOptions struct {
	// Password is the password to use for opening encrypted PDF files.
	// The empty string is always tried first, even if Password is set.
	// If authentication fails, [Open] and [NewReader] return an
	// [AuthenticationError].
	Password string

	ErrorHandling ReaderErrorHandling
}

// ReaderErrorHandling specifies how the reader should handle errors.
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
//
// After construction, [Reader.Get] and [Reader.GetMeta] are safe for
// concurrent use from multiple goroutines.
type Reader struct {
	meta MetaInfo // immutable after construction

	r          io.ReaderAt // concurrent-safe by contract
	size       int64       // immutable after construction
	ownsReader bool        // true if Close should close r

	xref map[uint32]*xRefEntry // read-only after construction

	headerOffset int64 // byte position of '%' in '%PDF-'

	// startXRef is the header-relative offset of the newest cross-reference
	// section, and trailerSize the /Size entry of its trailer (0 if absent).
	// Both are needed to append an incremental update.
	startXRef   int64
	trailerSize int64

	// xrefOffsets holds the offsets of the file's cross-reference
	// sections, newest first, as visited by the Prev chain.  Read-only
	// after construction.
	xrefOffsets []int64

	enc         *encryptInfo       // read-only after construction
	unencrypted map[Reference]bool // read-only after construction

	// objstms caches decoded object streams, so that resolving the many
	// objects inside one ObjStm does not re-run its filter chain for every
	// lookup.  Methods of the cache are safe for concurrent use, matching
	// the concurrency guarantees of Get.
	objstms *objstmCache

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
		return nil, Wrap(err, fname)
	}
	fi, err := fd.Stat()
	if err != nil {
		fd.Close()
		return nil, Wrap(err, fname)
	}
	r, err := NewReader(fd, fi.Size(), opt)
	if err != nil {
		fd.Close()
		return nil, Wrap(err, fname)
	}
	r.ownsReader = true
	return r, nil
}

// NewReader creates a new Reader object.
func NewReader(data io.ReaderAt, size int64, opt *ReaderOptions) (*Reader, error) {
	if opt == nil {
		opt = &ReaderOptions{}
	}

	r := &Reader{
		r:           data,
		size:        size,
		unencrypted: make(map[Reference]bool),
	}

	// search the first 1024 bytes for the %PDF- signature
	headerOffset, err := findHeaderOffset(data, size)
	if err != nil {
		return nil, err
	}
	r.headerOffset = headerOffset

	s, err := r.scannerFrom(headerOffset, false)
	if err != nil {
		return nil, err
	}
	version, err := s.ReadHeaderVersion()
	if err != nil {
		var e *MalformedFileError
		if errors.As(err, &e) && e.Err == errNoPDF {
			return nil, err // no PDF header at all
		}
		// A PDF processor shall attempt to read any PDF file, even if the
		// header announces a version more recent than the one for which
		// the processor was written: continue with the newest known
		// version.
		version = V2_0
		if opt.ErrorHandling == ErrorHandlingReport {
			r.Errors = append(r.Errors, &MalformedFileError{
				Err: fmt.Errorf("unsupported PDF header version: %w", err),
				Loc: []string{"PDF header"},
			})
		}
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
	r.objstms = newObjstmCache()

	if err := r.decodeTrailerCore(trailer, opt); err != nil {
		return nil, err
	}

	// remove xref-related information from trailer dictionary
	delete(trailer, "Type")
	delete(trailer, "Size")
	delete(trailer, "Index")
	delete(trailer, "Prev")
	delete(trailer, "W")
	r.meta.Trailer = trailer

	return r, nil
}

// decodeTrailerCore initialises the encryption context, document ID,
// document catalog and document info dictionary of r from a trailer
// dictionary.  It contains the parts of the file-opening sequence shared by
// NewReader and (*FileInfo).MakeReader; the two differ in how they obtain
// the trailer and how they clean it up afterwards.
func (r *Reader) decodeTrailerCore(trailer Dict, opt *ReaderOptions) error {
	shouldExit := func(err error) bool {
		if err == nil {
			return false
		}
		if opt.ErrorHandling == ErrorHandlingReport {
			var e *MalformedFileError
			if errors.As(err, &e) {
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
			return errors.New("file is encrypted, but no ID found")
		}
		if ref, ok := encObj.(Reference); ok {
			r.unencrypted[ref] = true
		}
		var perm Perm
		var err error
		r.enc, perm, err = r.parseEncryptDict(encObj, opt.Password)
		if err != nil {
			// An /Encrypt entry we cannot parse means we cannot decrypt
			// the file's streams, so the rest of the document would be
			// unreadable.  Surface the error regardless of
			// ErrorHandling — the file is unsalvageable in any mode.
			// AuthenticationError is returned unwrapped so callers can
			// detect it cleanly via errors.As.
			var authErr *AuthenticationError
			if errors.As(err, &authErr) {
				return err
			}
			return Wrap(err, "encryption dictionary")
		}
		r.meta.Permissions = perm
		r.meta.Encryption = r.enc.publicInfo()
	} else {
		r.meta.Permissions = PermAll
	}
	if r.meta.ID == nil && IDObj != nil {
		// If the file is not encrypted, ID may be an indirect object.
		var err error
		r.meta.ID, err = r.getID(IDObj)
		if shouldExit(err) {
			return err
		}
	}
	if r.meta.Version >= V2_0 {
		for _, id := range r.meta.ID {
			if len(id) < 16 {
				err := &MalformedFileError{Err: errInvalidID}
				if shouldExit(err) {
					return err
				}
				r.meta.ID = nil
				break
			}
		}
	}

	catalogDict, err := NewCursor(r).Dict(trailer["Root"])
	if err != nil {
		err = Wrap(err, "document catalog")
		if shouldExit(err) {
			return err
		}
	}

	// mark the catalog metadata stream as exempt from document-level
	// encryption when /EncryptMetadata is false — the spec exempts it
	// from default StmF encryption in that case.  Must happen before
	// DecodeCatalog reads the stream below.
	if metaRef, ok := catalogDict["Metadata"].(Reference); ok && metaRef != 0 {
		if r.enc != nil && r.enc.sec.unencryptedMetadata {
			r.unencrypted[metaRef] = true
		}
	}

	x := NewExtractor(r)
	r.meta.Catalog, err = Decode(CursorAt(x, nil), catalogDict, DecodeCatalog)
	if shouldExit(err) {
		return err
	} else if r.meta.Catalog == nil || r.meta.Catalog.Pages == 0 {
		err := &MalformedFileError{
			Err: errors.New("no pages in PDF document catalog"),
		}
		if opt.ErrorHandling == ErrorHandlingReport {
			r.Errors = append(r.Errors, err)
			if r.meta.Catalog == nil {
				r.meta.Catalog = &Catalog{}
			}
		} else {
			return err
		}
	}
	if r.meta.Catalog.Version > r.meta.Version {
		r.meta.Version = r.meta.Catalog.Version
	}

	// /EncryptMetadata=false exempts only the catalog metadata stream;
	// record that on the typed value so a rewrite preserves the policy
	if r.enc != nil && r.enc.sec.unencryptedMetadata && r.meta.Catalog.Metadata != nil {
		r.meta.Catalog.Metadata.Plaintext = true
	}

	if r.meta.Version == V1_0 {
		r.meta.ID = nil
	}

	r.meta.Info, err = Decode(CursorAt(x, nil), trailer["Info"], ExtractInfo)
	if shouldExit(err) {
		return err
	}
	return nil
}

// Close closes the Reader.
//
// This call only has an effect if the Reader was created by [Open].
func (r *Reader) Close() error {
	if r.objstms != nil {
		r.objstms.clear()
	}
	if r.ownsReader {
		err := r.r.(io.Closer).Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// GetMeta returns meta information about the file.
// This implements the [Getter] interface.
func (r *Reader) GetMeta() *MetaInfo {
	return &r.meta
}

// XRefOffsets returns the byte offsets of the file's cross-reference
// sections, newest first.  Offsets count from the start of the data, so
// they are comparable with file lengths; in a file with bytes before the
// %PDF- header they exceed the values written after startxref and Prev
// by that header offset.  A hybrid-reference file's cross-reference
// stream belongs to the table whose trailer names it and is not listed
// separately.
func (r *Reader) XRefOffsets() []int64 {
	return slices.Clone(r.xrefOffsets)
}

// Get reads an indirect object from the PDF file.  If the object is not
// present, nil is returned without an error.
//
// The argument canObjStm specifies whether the object may be read from an
// object stream.  Normally, this should be set to true.  If canObjStm is false
// and the object is in an object stream, an error is returned.
func (r *Reader) Get(ref Reference, canObjStm bool) (Native, error) {
	return r.get(ref, canObjStm, false)
}

// get implements [Reader.Get].  If scalarOnly is set, the object is read in
// scalar-only mode, in which composite objects are refused.  This is used to
// resolve an indirect stream /Length, where the value must be a scalar
// integer; refusing composites prevents unbounded recursion on a cyclic
// /Length.
func (r *Reader) get(ref Reference, canObjStm, scalarOnly bool) (_ Native, err error) {
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
		getInt := safeGetInteger(lengthGetter{r}, true)
		return getFromObjStm(r, ref.Number(), entry.InStream, getInt, r.objstms)
	}

	s, err := r.scannerFrom(entry.Pos+r.headerOffset, canObjStm)
	if err != nil {
		return nil, err
	}
	s.scalarOnly = scalarOnly
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

// lengthGetter wraps a Reader so that Get reads objects in scalar-only mode.
// It is used to resolve an indirect stream /Length: the value must be a scalar
// integer, and refusing composite objects prevents a cyclic /Length from
// causing unbounded recursion.
type lengthGetter struct{ *Reader }

func (g lengthGetter) Get(ref Reference, canObjStm bool) (Native, error) {
	return g.Reader.get(ref, canObjStm, true)
}

func getFromObjStm(r Getter, number uint32, sRef Reference, getInt getIntFn, cache *objstmCache) (obj Native, err error) {
	// We need to be careful to avoid infinite loops, in case reading from an
	// object stream requires opening other object streams first.  This could
	// be either caused by the stream object being contained in another object
	// stream, or by the length of the stream object being contained in another
	// object stream.  (Both cases are forbidden by the PDF spec.)
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

	data, idx, err := cache.load(r, sRef, objectStream, getInt)
	if err != nil {
		return nil, Wrap(err, "object stream "+sRef.String())
	}

	offs, ok := lookupObjStm(idx, number)
	if !ok {
		return nil, &MalformedFileError{
			Err: fmt.Errorf("object %d not found", number),
			Loc: []string{"object stream " + sRef.String()},
		}
	}
	if offs < 0 || offs > len(data) {
		return nil, &MalformedFileError{
			Err: fmt.Errorf("invalid object offset %d", offs),
			Loc: []string{"object stream " + sRef.String()},
		}
	}

	// The cached data is immutable, so a fresh scanner over the object's
	// section gives every lookup an independent view.  Strings inside object
	// streams are never encrypted (PDF 32000-2, 7.5.7), so no encryptInfo is
	// needed.
	s := newScanner(bytes.NewReader(data[offs:]), getInt, nil)
	return s.ReadObject()
}

// lookupObjStm returns the byte offset of the given object in a decoded
// object stream.
func lookupObjStm(idx []stmObj, number uint32) (int, bool) {
	for _, info := range idx {
		if info.number == number {
			return info.offs, true
		}
	}
	return 0, false
}

// load decodes an object stream and parses its index.  Decoded streams are
// cached, because documents commonly keep most of their objects in one
// ObjStm, and re-running the filter chain for every single lookup makes
// resolving N objects cost N full decodes.
func (c *objstmCache) load(r Getter, sRef Reference, stream *Stream, getInt getIntFn) ([]byte, []stmObj, error) {
	if d := c.get(sRef); d != nil {
		return d.data, d.idx, nil
	}

	N, ok := stream.Dict["N"].(Integer)
	if !ok || N < 0 || N > 10000 {
		return nil, nil, &MalformedFileError{Err: errors.New("no valid /N")}
	}
	n := int(N)

	data, err := ReadAll(r, nil, stream, int64(limits.MaxObjStmBytes))
	if err != nil {
		return nil, nil, err
	}
	s := newScanner(bytes.NewReader(data), getInt, nil)

	idx := make([]stmObj, n)
	for i := range n {
		no, err := s.ReadInteger()
		if err != nil {
			return nil, nil, err
		}
		offs, err := s.ReadInteger()
		if err != nil {
			return nil, nil, err
		}
		if no < 0 || no > math.MaxUint32 || offs < 0 || offs > math.MaxInt {
			return nil, nil, &MalformedFileError{Err: errors.New("invalid object number or offset")}
		}
		idx[i].number = uint32(no)
		idx[i].offs = int(offs)
	}

	pos := s.CurrentPos()
	first, ok := stream.Dict["First"].(Integer)
	firstInt := int(first)
	if !ok || first < Integer(pos) || first != Integer(firstInt) {
		return nil, nil, &MalformedFileError{Err: errors.New("no valid /First")}
	}
	for i := range idx {
		x := idx[i].offs + firstInt
		if x < idx[i].offs { // check for integer overflow
			return nil, nil, &MalformedFileError{Err: errors.New("invalid object offset")}
		}
		idx[i].offs = x
	}

	c.put(sRef, data, idx)
	return data, idx, nil
}

func (r *Reader) getID(obj Object) ([][]byte, error) {
	arr, err := NewCursor(r).Array(obj)
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
		s, err := NewCursor(r).String(obj)
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
// This is only used until the encryption dictionary has been parsed.
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

type stmObj struct {
	number uint32
	offs   int
}

// safeGetInteger returns a function that reads an integer from a getter.
//
// If canObjStm is false, the function will return an error if the object is in
// an object stream.  This is used to avoid infinite recursion when reading
// object streams.
func safeGetInteger(r Getter, canObjStm bool) getIntFn {
	return func(obj Object) (Integer, error) {
		if x, ok := obj.(Integer); ok {
			return x, nil
		}
		if canObjStm {
			return getInteger(r, obj)
		}
		return getIntegerNoObjStm(r, obj)
	}
}

// findHeaderOffset searches the first 1024 bytes of the file for the
// %PDF- signature and returns its byte offset.
func findHeaderOffset(data io.ReaderAt, size int64) (int64, error) {
	n := min(size, 1024)
	buf := make([]byte, n)
	_, err := data.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return 0, err
	}
	idx := bytes.Index(buf, []byte("%PDF-"))
	if idx < 0 {
		return 0, &MalformedFileError{Err: errNoPDF}
	}
	return int64(idx), nil
}

func (r *Reader) scannerFrom(pos int64, canObjStm bool) (*scanner, error) {
	getInt := safeGetInteger(lengthGetter{r}, canObjStm)
	sr := io.NewSectionReader(r.r, pos, r.size-pos)
	s := newScanner(sr, getInt, r.enc)
	s.fileReader = r.r
	s.unencrypted = r.unencrypted
	s.filePos = pos
	return s, nil
}
