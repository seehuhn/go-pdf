package pdf

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
)

// Reader represents a pdf file opened for reading.
type Reader struct {
	size int64
	r    io.ReaderAt

	pos     int64
	objStm  *objStm
	level   int
	special map[Reference]bool

	xref    map[int]*xRefEntry
	trailer Dict

	Version Version

	ID  [][]byte
	enc *encryptInfo
}

// NewReader creates a new Reader object.
//
// TODO(voss): remove the needOwner argument, add an .AuthenticateOwner method
// instead.
func NewReader(data io.ReaderAt, size int64, readPwd func(needOwner bool) string) (*Reader, error) {
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
		r.enc, err = r.parseEncryptDict(encObj)
		if err != nil {
			return nil, err
		}
		// TODO(voss): set this in a different way?
		r.enc.sec.getPasswd = readPwd
	}

	root := trailer["Root"]
	catalog, err := r.GetDict(root)
	if err == nil {
		catVer, ok := catalog["Version"].(Name)
		if ok {
			var v2 Version
			switch catVer {
			case "1.4":
				v2 = V1_4
			case "1.5":
				v2 = V1_5
			case "1.6":
				v2 = V1_6
			case "1.7":
				v2 = V1_7
			default:
				return nil, &MalformedFileError{
					Pos: r.errPos(root),
					Err: errVersion,
				}
			}
			if v2 > r.Version {
				r.Version = v2
			}
		}
	}

	return r, nil
}

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

// Catalog returns the PDF Catalog dictionary for the file.
func (r *Reader) Catalog() (Dict, error) {
	return r.GetDict(r.trailer["Root"])
}

// Info returns the PDF Catalog dictionary for the file.
func (r *Reader) Info() (Dict, error) {
	return r.GetDict(r.trailer["Info"])
}

func (r *Reader) Read() (Object, *Reference, error) {
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

		// Try to fix up the xref information, so that after a sequential
		// read of the file, random access works even if the xref tables
		// had been corrupted.
		// TODO(voss): remove this?
		entry := r.xref[ref.Number]
		if entry != nil && (entry.Pos < 0 || entry.Generation <= ref.Generation) {
			r.xref[ref.Number] = &xRefEntry{
				Pos:        r.pos,
				Generation: ref.Generation,
			}
		}

		r.pos = s.currentPos()
		return obj, ref, nil
	}
}

// Get resolves references to indirect objects.
//
// If obj is of type *Reference, the function loads the corresponding object
// from the file and returns the result.  Otherwise, obj is returned unchanged.
func (r *Reader) Get(obj Object) (Object, error) {
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

	decoded, err := stream.Decode()
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

func (r *Reader) getFromObjectStream(number int, cRef *Reference) (Object, error) {
	// TODO(voss): fudge up the error position on return
	container, err := r.doGet(cRef, false)
	if err != nil {
		return nil, err
	}
	stream, ok := container.(*Stream)
	if !ok {
		return nil, &MalformedFileError{
			Pos: r.errPos(cRef),
			Err: errors.New("wrong type for object stream"),
		}
	}

	contents, err := r.objStmScanner(stream, r.errPos(cRef))
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
			Pos: r.errPos(cRef),
			Err: errors.New("object missing from stream"),
		}
	}

	return contents.s.ReadObject()
}

// GetDict resolves references to indirect objects and makes sure the resulting
// object is a dictionary.
func (r *Reader) GetDict(obj Object) (Dict, error) {
	candidate, err := r.Get(obj)
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

// GetInt resolves references to indirect objects and makes sure the resulting
// object is an Integer.
func (r *Reader) GetInt(obj Object) (Integer, error) {
	candidate, err := r.Get(obj)
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

// GetName resolves references to indirect objects and makes sure the resulting
// object is a Name.
func (r *Reader) GetName(obj Object) (Name, error) {
	candidate, err := r.Get(obj)
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

// GetString resolves references to indirect objects and makes sure the resulting
// object is a String.
func (r *Reader) GetString(obj Object) (String, error) {
	candidate, err := r.Get(obj)
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
	val, err := r.GetInt(obj)
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
	V1_0 Version = iota + 1
	V1_1
	V1_2
	V1_3
	V1_4
	V1_5
	V1_6
	V1_7
	tooHighVersion
)

func (ver Version) String() string {
	if ver >= V1_0 && ver <= V1_7 {
		return "1." + string([]byte{byte(ver - V1_0 + '0')})
	}
	return "pdf.Version(" + strconv.Itoa(int(ver)) + ")"
}
