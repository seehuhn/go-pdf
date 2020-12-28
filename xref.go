package pdflib

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

func (r *Reader) findXRef() (int64, int64, error) {
	endChunk := int64(1024)
	if endChunk > r.size {
		endChunk = r.size
	}
	buf := make([]byte, endChunk)
	n, err := r.r.ReadAt(buf, r.size-endChunk)
	if err != nil {
		return 0, 0, err
	}

	idx := bytes.LastIndex(buf[:n], []byte("startxref"))
	if idx < 0 {
		return 0, 0, &MalformedFileError{
			Pos: r.size,
			Err: errors.New("startxref not found"),
		}
	}
	markerPos := r.size - endChunk + int64(idx)

	idx += 9
	for isSpace[buf[idx]] && idx < n {
		idx++
	}

	var xrefPos int64
	for idx < n && buf[idx] >= '0' && buf[idx] <= '9' {
		xrefPos = xrefPos*10 + int64(buf[idx]-'0')
		idx++
	}
	if xrefPos <= 0 || xrefPos >= markerPos {
		return 0, 0, &MalformedFileError{
			Pos: markerPos,
			Err: errors.New("invalid xref position"),
		}
	}

	return xrefPos, markerPos, nil
}

func (r *Reader) readXRef() (map[int64]*xrefEntry, error) {
	start, end, err := r.findXRef()
	if err != nil {
		return nil, err
	}

	xref := make(map[int64]*xrefEntry)
	for {
		xRefData := io.NewSectionReader(r.r, start, end-start)
		s := newScanner(xRefData)
		xRefData = nil

		buf, err := s.Peek(4)
		if err != nil {
			return nil, err
		}
		var dict Dict
		switch {
		case bytes.Equal(buf, []byte("xref")):
			dict, err = readOldStyleXRef(xref, s)
		case bytes.HasPrefix(buf, []byte("<<")):
			dict, err = readNewStyleXRef(xref, s)
		default:
			return nil, &MalformedFileError{
				Pos: start,
				Err: errors.New("xref data not found"),
			}
		}

		prev := dict["Prev"]
		if prev == nil {
			break
		}
		prevStart, ok := prev.(Integer)
		if !ok || prevStart <= 0 || int64(prevStart) >= start {
			return nil, &MalformedFileError{
				Pos: start,
				Err: fmt.Errorf("invalid /Prev value %s", format(prev)),
			}
		}
		start, end = int64(prevStart), start
	}

	return xref, nil
}

func readOldStyleXRef(xref map[int64]*xrefEntry, s *scanner) (Dict, error) {
	err := s.SkipString("xref")
	if err != nil {
		return nil, err
	}
	err = s.SkipWhiteSpace()
	if err != nil {
		return nil, err
	}

	for {
		done, err := s.HasPrefix("trailer")
		if err != nil {
			return nil, err
		}
		if done {
			break
		}

		start, err := s.ReadInteger()
		if err != nil {
			return nil, err
		}
		err = s.SkipWhiteSpace()
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

		err = decodeOldStyleSection(xref, s, int64(start), int64(start+length))
		if err != nil {
			return nil, err
		}
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

func decodeOldStyleSection(xref map[int64]*xrefEntry, s *scanner, start, end int64) error {
	for i := start; i < end; i++ {
		if xref[i] != nil {
			s.Discard(20)
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
			return err
		}
		c := buf[17]
		switch c {
		case 'f':
			xref[i] = &xrefEntry{
				Pos:        -1,
				Generation: uint16(b),
			}
		case 'n':
			xref[i] = &xrefEntry{
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

func readNewStyleXRef(xref map[int64]*xrefEntry, s *scanner) (Dict, error) {
	obj, err := s.ReadIndirectObject()
	if err != nil {
		return nil, err
	}
	stream, ok := obj.Obj.(*Stream)
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

func checkNewStyleDict(dict Dict) ([]int, []*xrefSubSection, error) {
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
		if !ok || i < 3 && (wi < 0 || wi > 7) {
			return nil, nil, &MalformedFileError{}
		}
		w = append(w, int(wi))
	}

	Index := dict["Index"]
	var ss []*xrefSubSection
	if Index == nil {
		ss = append(ss, &xrefSubSection{0, int64(size)})
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
			ss = append(ss, &xrefSubSection{int64(start), int64(size)})
		}
	}
	return w, ss, nil
}

func decodeNewStyleXref(xref map[int64]*xrefEntry, r io.Reader, w []int, ss []*xrefSubSection) error {
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
				xref[i] = &xrefEntry{
					Pos:        -1,
					Generation: uint16(b),
				}
			case 1:
				// used object, not compressed
				// a = byte offset of the object
				// b = generation number
				xref[i] = &xrefEntry{
					Pos:        a,
					Generation: uint16(b),
				}
			case 2:
				// used object, compressed
				// a = object number of the compressed stream (generation number 0)
				// b = index within the stream
				xref[i] = &xrefEntry{
					Pos: b,
					InStream: &Reference{
						Index:      a,
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

type xrefSubSection struct {
	Start, Size int64
}

type xrefEntry struct {
	Pos        int64 // -1 indicates unused/deleted objects
	Generation uint16
	InStream   *Reference
}
