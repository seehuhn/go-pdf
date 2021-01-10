package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

func (r *Reader) findXRef() (int64, error) {
	pos, err := r.lastOccurence("startxref")
	if err != nil {
		return 0, err
	}
	s := r.scannerAt(pos + 9)

	xRefPos, err := s.ReadInteger()
	if err != nil {
		return 0, err
	}

	if xRefPos <= 0 || int64(xRefPos) >= r.size {
		return 0, &MalformedFileError{
			Pos: s.filePos(),
			Err: errors.New("invalid xref position"),
		}
	}

	return int64(xRefPos), nil
}

func (r *Reader) lastOccurence(pat string) (int64, error) {
	const chunkSize = 1024

	buf := make([]byte, chunkSize)
	k := int64(len(pat))
	pos := r.size
	for pos >= k {
		start := pos - chunkSize
		if start < 0 {
			start = 0
		}
		n, err := r.r.ReadAt(buf[:pos-start], start)
		if err != nil && err != io.EOF {
			return 0, err
		}

		idx := bytes.LastIndex(buf[:n], []byte(pat))
		if idx >= 0 {
			return start + int64(idx), nil
		}

		pos = start + k - 1
	}
	return 0, &MalformedFileError{
		Pos: 0,
		Err: errors.New("startxref not found"),
	}
}

func (r *Reader) readXRef() (map[int]*xRefEntry, Dict, error) {
	start, err := r.findXRef()
	if err != nil {
		return nil, nil, err
	}

	xref := make(map[int]*xRefEntry)
	trailer := Dict{}
	first := true
	seen := make(map[int64]bool)
	for {
		// avoid xref loops
		if seen[start] {
			break
		}
		seen[start] = true

		s := r.scannerAt(start)

		buf, err := s.Peek(4)
		if err != nil {
			return nil, nil, err
		}
		var dict Dict
		switch {
		case bytes.Equal(buf, []byte("xref")):
			dict, err = readOldStyleXRef(xref, s)

			if xRefStm, ok := dict["XRefStm"]; ok {
				zStart, ok := xRefStm.(Integer)
				if !ok {
					return nil, nil, &MalformedFileError{
						Err: errors.New("wrong type for XRefStm (expected Integer)"),
					}
				}
				s = r.scannerAt(int64(zStart))
				_, err = readNewStyleXRef(xref, s)
				if err != nil {
					return nil, nil, err
				}
			}
		default:
			dict, err = readNewStyleXRef(xref, s)
		}
		if err != nil {
			return nil, nil, err
		}

		if first {
			for _, key := range []Name{"Root", "Encrypt", "Info", "ID"} {
				val, ok := dict[key]
				if ok {
					trailer[key] = val
				}
			}
			first = false
		}

		prev := dict["Prev"]
		if prev == nil {
			break
		}
		prevStart, ok := prev.(Integer)
		if !ok || prevStart <= 0 || int64(prevStart) >= r.size {
			return nil, nil, &MalformedFileError{
				Pos: start,
				Err: fmt.Errorf("invalid /Prev value %s", format(prev)),
			}
		}
		start = int64(prevStart)
	}

	return xref, trailer, nil
}

func readOldStyleXRef(xref map[int]*xRefEntry, s *scanner) (Dict, error) {
	err := s.SkipString("xref")
	if err != nil {
		return nil, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, err
	}

	for {
		buf, err := s.Peek(1)
		if err != nil {
			return nil, err
		}
		if len(buf) == 0 || buf[0] < '0' || buf[0] > '9' {
			break
		}

		start, err := s.ReadInteger()
		if err != nil {
			return nil, err
		}
		length, err := s.ReadInteger()
		if err != nil {
			return nil, err
		}
		err = s.SkipWhiteSpace()
		if err != nil {
			return nil, err
		}

		err = decodeOldStyleSection(xref, s, int(start), int(start+length))
		if err != nil {
			return nil, err
		}
		err = s.SkipWhiteSpace()
		if err != nil {
			return nil, err
		}
	}

	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, err
	}
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

func decodeOldStyleSection(xref map[int]*xRefEntry, s *scanner, start, end int) error {
	// TODO(voss): use xrefSubSection?
	for i := start; i < end; i++ {
		if xref[i] != nil {
			err := s.Discard(20)
			if err != nil {
				return err
			}
			continue
		}

		buf, err := s.Peek(20)
		if err != nil {
			return err
		}
		if len(buf) < 20 {
			return &MalformedFileError{
				Pos: s.filePos(),
				Err: io.ErrUnexpectedEOF,
			}
		}

		a, err := strconv.ParseInt(string(buf[:10]), 10, 64)
		if err != nil {
			return err
		}
		b, err := strconv.ParseUint(string(buf[11:16]), 10, 16)
		if err != nil {
			// fix a common error in some PDF files
			if bytes.HasPrefix(buf, []byte("0000000000 65536 ")) {
				b = 65535
				buf[17] = 'f'
			} else {
				return err
			}
		}
		c := buf[17]
		switch c {
		case 'f':
			xref[i] = &xRefEntry{
				Pos:        -1,
				Generation: uint16(b),
			}
		case 'n':
			xref[i] = &xRefEntry{
				Pos:        a,
				Generation: uint16(b),
			}
		default:
			return &MalformedFileError{
				Pos: s.filePos(),
				Err: errors.New("malformed xref table"),
			}
		}

		s.pos += 20
	}
	return nil
}

func readNewStyleXRef(xref map[int]*xRefEntry, s *scanner) (Dict, error) {
	obj, _, err := s.ReadIndirectObject()
	if err != nil {
		return nil, err
	}
	stream, ok := obj.(*Stream)
	if !ok {
		return nil, &MalformedFileError{
			Pos: s.filePos(),
			Err: errors.New("invalid xref stream"),
		}
	}
	dict := stream.Dict

	w, ss, err := checkNewStyleDict(dict)
	if err != nil {
		return nil, err
	}
	err = decodeNewStyleXref(xref, stream.Decode(), w, ss)
	if err != nil {
		return nil, err
	}

	return dict, nil
}

func checkNewStyleDict(dict Dict) ([]int, []*xRefSubSection, error) {
	size, ok := dict["Size"].(Integer)
	if !ok {
		return nil, nil, &MalformedFileError{}
	}
	W, ok := dict["W"].(Array)
	if !ok || len(W) < 3 {
		return nil, nil, &MalformedFileError{}
	}
	var w []int
	for i, Wi := range W {
		wi, ok := Wi.(Integer)
		if !ok || i < 3 && (wi < 0 || wi > 8) {
			return nil, nil, &MalformedFileError{}
		}
		w = append(w, int(wi))
	}

	Index := dict["Index"]
	var ss []*xRefSubSection
	if Index == nil {
		ss = append(ss, &xRefSubSection{0, int(size)})
	} else {
		ind, ok := Index.(Array)
		if !ok || len(ind)%2 != 0 {
			return nil, nil, &MalformedFileError{}
		}
		for i := 0; i < len(ind); i += 2 {
			start, ok1 := ind[i].(Integer)
			size, ok2 := ind[i+1].(Integer)
			if !ok1 || !ok2 {
				return nil, nil, &MalformedFileError{}
			}
			ss = append(ss, &xRefSubSection{int(start), int(size)})
		}
	}
	return w, ss, nil
}

func decodeNewStyleXref(xref map[int]*xRefEntry, r io.Reader, w []int, ss []*xRefSubSection) error {
	wTotal := 0
	for _, wi := range w {
		wTotal += wi
	}
	buf := make([]byte, wTotal)

	w0 := w[0]
	w1 := w[1]
	w2 := w[2]
	for _, sec := range ss {
		for i := sec.Start; i < sec.Start+sec.Size; i++ {
			_, err := io.ReadFull(r, buf)
			if err != nil {
				return err
			}

			if xref[i] != nil {
				continue
			}

			tp := decodeInt(buf[:w0])
			if w1 == 0 {
				tp = 1
			}
			a := decodeInt(buf[w0 : w0+w1])
			b := decodeInt(buf[w0+w1 : w0+w1+w2])
			switch tp {
			case 0:
				// free/deleted object
				// a = next free object
				// b = generation number to be used if the object is resurrected
				xref[i] = &xRefEntry{
					Pos:        -1,
					Generation: uint16(b),
				}
			case 1:
				// used object, not compressed
				// a = byte offset of the object
				// b = generation number
				xref[i] = &xRefEntry{
					Pos:        a,
					Generation: uint16(b),
				}
			case 2:
				// used object, compressed
				// a = object number of the compressed stream (generation number 0)
				// b = index within the stream
				xref[i] = &xRefEntry{
					Pos: b,
					InStream: &Reference{
						Number:     int(a),
						Generation: 0,
					},
				}
			}
		}
	}
	return nil
}

func decodeInt(buf []byte) (res int64) {
	for _, x := range buf {
		res = res<<8 | int64(x)
	}
	return res
}

type xRefSubSection struct {
	Start, Size int
}

type xRefEntry struct {
	InStream   *Reference
	Pos        int64
	Generation uint16
}

func (entry *xRefEntry) IsFree() bool {
	return entry == nil || entry.Pos < 0
}
