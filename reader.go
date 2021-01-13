package pdf

import (
	"errors"
	"io"
	"os"
)

// Reader represents a pdf file opened for reading.
type Reader struct {
	size int64
	r    io.ReaderAt
	xref map[int]*xRefEntry

	PDFVersion Version
	Catalog    *Catalog
	Info       *Info

	ID  [][]byte
	enc *encryptInfo
}

// NewReader creates a new Reader object.
func NewReader(data io.ReaderAt, size int64, readPwd func(needOwner bool) string) (*Reader, error) {
	r := &Reader{
		size: size,
		r:    data,
	}

	s := r.scannerAt(0)
	version, err := s.readHeaderVersion()
	if err != nil {
		return nil, err
	}
	r.PDFVersion = version

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
		r.enc, err = r.parseEncryptDict(encObj)
		if err != nil {
			return nil, err
		}
		// TODO(voss): set this in a different way?
		r.enc.sec.getPasswd = readPwd
	}

	rootDict, err := r.GetDict(trailer["Root"])
	if err != nil {
		return nil, err
	}
	r.Catalog = &Catalog{}
	rootDict.AsStruct(r.Catalog, r, r.errPos(trailer["Root"]))

	if r.Catalog.Version != "" {
		var v2 Version
		switch r.Catalog.Version {
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
				Pos: r.errPos(trailer["Root"]),
				Err: errVersion,
			}
		}
		if v2 > r.PDFVersion {
			r.PDFVersion = v2
		}
	}

	obj, err := r.Get(trailer["Info"])
	if err != nil {
		return nil, err
	}
	if infoDict, ok := obj.(Dict); ok {
		r.Info = &Info{}
		// We ignore errors here, so that we still recover the values we did
		// read successfully.
		infoDict.AsStruct(r.Info, r, r.errPos(trailer["Info"]))
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

// Close closes the underlying file of the reader.  This call only has an
// effect if the io.ReaderAt passed to NewReader() has a Close() method, or if
// the Reader was created using Open().  Otherwise, Close() has no effect and
// returns nil.
func (r *Reader) Close() error {
	closer, ok := r.r.(io.Closer)
	if ok {
		return closer.Close()
	}
	return nil
}

// Walk performs a depth-first walk through the object graph rooted at obj.
//
// TODO(voss): remove
func (r *Reader) Walk(obj Object, seen map[Reference]Object,
	leaf func(Object) (Object, error),
	root func(Object, *Reference) (*Reference, error)) (Object, error) {
	obj, err := r.doWalk(obj, seen, leaf, root)
	if err == nil && root != nil {
		obj, err = root(obj, nil)
	}
	return obj, err
}

func (r *Reader) doWalk(obj Object, seen map[Reference]Object,
	leaf func(Object) (Object, error),
	root func(Object, *Reference) (*Reference, error)) (Object, error) {
	switch x := obj.(type) {
	case Dict:
		res := Dict{}
		for _, key := range x.sortedKeys() {
			repl, err := r.doWalk(x[key], seen, leaf, root)
			if err != nil {
				return nil, err
			}
			res[key] = repl
		}
		return res, nil
	case Array:
		var res Array
		for _, val := range x {
			repl, err := r.doWalk(val, seen, leaf, root)
			if err != nil {
				return nil, err
			}
			res = append(res, repl)
		}
		return res, nil
	case *Stream:
		res := &Stream{
			Dict: make(Dict),
			R:    x.R,
		}
		for _, key := range x.sortedKeys() {
			repl, err := r.doWalk(x.Dict[key], seen, leaf, root)
			if err != nil {
				return nil, err
			}
			res.Dict[key] = repl
		}
		return res, nil
	case *Reference:
		if other, ok := seen[*x]; ok {
			return other, nil
		}
		res, err := leaf(x)
		if err != nil {
			return nil, err
		}
		seen[*x] = res

		ind, err := r.Get(x)
		if err != nil {
			return nil, err
		}
		subTree, err := r.doWalk(ind, seen, leaf, root)
		if err != nil {
			return nil, err
		}
		if root != nil {
			ref, _ := res.(*Reference)
			_, err = root(subTree, ref)
			if err != nil {
				return nil, err
			}
		}

		return res, nil
	}
	return leaf(obj)
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

	first, ok := stream.Dict["First"].(Integer)
	if !ok {
		return nil, &MalformedFileError{
			Pos: r.errPos(sRef),
			Err: errors.New("malformed object stream (no /First)"),
		}
	}

	var dec *encryptInfo
	if r.enc != nil && !stream.isEncrypted {
		dec = r.enc
	}
	s := newScanner(stream.Decode(), r.safeGetInt, dec)
	for {
		err := s.SkipWhiteSpace()
		if err != nil {
			return nil, err
		}
		if s.filePos() >= int64(first) {
			return nil, &MalformedFileError{
				Pos: r.errPos(sRef),
				Err: errors.New("object missing from stream"),
			}
		}
		no, err := s.ReadInteger()
		if err != nil {
			return nil, err
		}

		offs, err := s.ReadInteger()
		if err != nil {
			return nil, err
		}

		if int(no) == number {
			objPos := int64(first + offs)
			err = s.Discard(objPos - s.filePos())
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return s.ReadObject()
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

	ref, ok := obj.(*Reference)
	if !ok {
		return 0, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("wrong type (expected Integer)"),
		}
	}

	if r.xref == nil {
		return 0, &MalformedFileError{
			Pos: 0,
			Err: errors.New("cannot use references while reading xref table"),
		}
	}

	entry := r.xref[ref.Number]
	if entry.IsFree() || entry.Generation != ref.Generation {
		return 0, &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("missing integer (not in xref table)"),
		}
	}
	s := r.scannerAt(entry.Pos)
	return s.readIndirectInteger()
}

func (r *Reader) scannerAt(pos int64) *scanner {
	var dec *encryptInfo
	if r.enc != nil {
		dec = r.enc
	}
	return newScanner(io.NewSectionReader(r.r, pos, r.size-pos),
		r.safeGetInt, dec)
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
