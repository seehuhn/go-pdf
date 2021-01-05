package pdf

import (
	"errors"
	"io"
)

// Reader represents a pdf file opened for reading.
type Reader struct {
	size int64
	r    io.ReaderAt
	xref map[int]*xRefEntry

	HeaderVersion Version
	ID            []string
	Trailer       Dict

	encInfo *encryptInfo
	readPwd func() string
}

// NewReader creates a new Reader object.
func NewReader(data io.ReaderAt, size int64, readPwd func() string) (*Reader, error) {
	r := &Reader{
		size: size,
		r:    data,
	}

	s := r.scannerAt(0)
	version, err := s.readHeaderVersion()
	if err != nil {
		return nil, err
	}
	r.HeaderVersion = version

	xref, trailer, err := r.readXRef()
	if err != nil {
		return nil, err
	}
	r.xref = xref
	r.Trailer = trailer

	ID, ok := trailer["ID"].(Array)
	if ok {
		if len(ID) != 2 {
			return nil, &MalformedFileError{Err: errors.New("malformed ID array")}
		}
		for i := 0; i < 2; i++ {
			s, ok := ID[i].(String)
			if !ok {
				return nil, &MalformedFileError{Err: errors.New("malformed ID array")}
			}
			r.ID = append(r.ID, string(s))
		}
	}

	r.readPwd = readPwd
	if encObj, ok := trailer["Encrypt"]; ok {
		r.encInfo, err = r.checkEncrypt(encObj)
		if err != nil {
			return nil, err
		}

		// err = r.checkPwd()
		// if err != nil {
		// 	return nil, err
		// }
	}

	return r, nil
}

// Walk performs a depth-first walk through the object graph rooted at obj.
func (r *Reader) Walk(obj Object, seen map[Reference]bool, fn func(Object) error) error {
	switch x := obj.(type) {
	case Dict:
		for _, val := range x {
			err := r.Walk(val, seen, fn)
			if err != nil {
				return err
			}
		}
	case Array:
		for _, val := range x {
			err := r.Walk(val, seen, fn)
			if err != nil {
				return err
			}
		}
	case *Stream:
		for _, val := range x.Dict {
			err := r.Walk(val, seen, fn)
			if err != nil {
				return err
			}
		}
	case *Reference:
		if seen[*x] {
			return nil
		}
		seen[*x] = true

		val, err := r.Get(x)
		if err != nil {
			return err
		}
		err = r.Walk(val, seen, fn)
		if err != nil {
			return err
		}
	}
	return fn(obj)
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
	ind, err := s.ReadIndirectObject()
	if err != nil {
		return nil, err
	}

	if *ref != ind.Reference {
		return nil, &MalformedFileError{
			Pos: 0,
			Err: errors.New("xref corrupted"),
		}
	}

	return ind.Obj, nil
}

func (r *Reader) getFromObjectStream(number int, sRef *Reference) (Object, error) {
	if r.encInfo != nil && r.encInfo.StmF != nil {
		return nil, errors.New("StmF not implemented")
	}

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

	s := newScanner(stream.Decode(), r.GetInt)
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

		err = s.SkipWhiteSpace()
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
	if r.encInfo != nil && r.encInfo.StrF != nil {
		return "", errors.New("StrF not implemented")
	}
	candidate, err := r.Get(obj)
	if err != nil {
		return "", err
	}
	val, ok := candidate.(String)
	if !ok {
		return "", &MalformedFileError{
			Pos: r.errPos(obj),
			Err: errors.New("wrong type (expected String)"),
		}
	}
	return val, nil
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

func (r *Reader) scannerAt(pos int64) *scanner {
	return newScanner(io.NewSectionReader(r.r, pos, r.size-pos), r.GetInt)
}
