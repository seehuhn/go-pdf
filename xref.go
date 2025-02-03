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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"strconv"
)

func (r *Reader) findXRef(size int64) (int64, error) {
	pos, err := r.lastOccurence("startxref", size)
	if err != nil {
		return 0, err
	}
	s, err := r.scannerFrom(pos+9, false)
	if err != nil {
		return 0, err
	}

	xRefPos, err := s.ReadInteger()
	if err != nil {
		return 0, Wrap(err, fmt.Sprintf("byte %d", pos+9))
	}

	if xRefPos <= 0 || int64(xRefPos) >= size {
		return 0, &MalformedFileError{
			Err: fmt.Errorf("invalid xref position %d", xRefPos),
			Loc: []string{fmt.Sprintf("byte %d", pos+9)},
		}
	}

	return int64(xRefPos), nil
}

func (r *Reader) readXRef() (map[uint32]*xRefEntry, Dict, error) {
	size, err := getSize(r.r)
	if err != nil {
		return nil, nil, err
	}

	start, err := r.findXRef(size)
	if err != nil {
		return nil, nil, err
	}

	xref := make(map[uint32]*xRefEntry)
	trailer := Dict{}
	first := true
	seen := make(map[int64]bool)
	for {
		// avoid xref loops
		if seen[start] {
			break
		}
		seen[start] = true

		s, err := r.scannerFrom(start, false)
		if err != nil {
			return nil, nil, err
		}

		buf, err := s.Peek(4)
		if err != nil {
			return nil, nil, err
		}
		var dict Dict
		var ref Reference
		switch {
		case bytes.Equal(buf, []byte("xref")):
			dict, err = readXRefTable(xref, s)
			if err != nil {
				return nil, nil, Wrap(err, fmt.Sprintf("table at byte %d", start))
			}

			if xRefStm, ok := dict["XRefStm"]; ok {
				zStart, ok := xRefStm.(Integer)
				if !ok {
					return nil, nil, &MalformedFileError{
						Err: errInvalidXref,
					}
				}
				s, err = r.scannerFrom(int64(zStart), false)
				if err != nil {
					return nil, nil, err
				}
				_, ref, err = r.readXRefStream(xref, s)
				if err != nil {
					return nil, nil, Wrap(err, "XRefStm")
				}
			}
		default:
			dict, ref, err = r.readXRefStream(xref, s)
			if err != nil {
				return nil, nil, Wrap(err, fmt.Sprintf("stream at byte %d", start))
			}
		}
		if ref != 0 {
			r.unencrypted[ref] = true
		}

		if first {
			for key, val := range dict {
				if key == "Root" || key == "Encrypt" || key == "Info" || key == "ID" ||
					key.isSecondClassName() ||
					key.isThirdClassName() {
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
		if !ok || prevStart <= 0 || int64(prevStart) >= size {
			return nil, nil, &MalformedFileError{
				Err: errors.New("invalid /Prev"),
				Loc: []string{fmt.Sprintf("xref at byte %d", start)},
			}
		}
		start = int64(prevStart)
	}

	return xref, trailer, nil
}

func readXRefTable(xref map[uint32]*xRefEntry, s *scanner) (Dict, error) {
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
			return nil, Wrap(err, fmt.Sprintf("byte %d", s.currentPos()))
		}
		length, err := s.ReadInteger()
		if err != nil {
			return nil, Wrap(err, fmt.Sprintf("byte %d", s.currentPos()))
		}

		if start < 0 || length < 0 || start > math.MaxUint32 || start+length > math.MaxUint32 {
			return nil, &MalformedFileError{
				Err: errInvalidXref,
				Loc: []string{fmt.Sprintf("byte %d", s.currentPos())},
			}
		}

		err = s.SkipWhiteSpace()
		if err != nil {
			return nil, err
		}

		err = decodeXRefSection(xref, s, uint32(start), uint32(start+length))
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

func decodeXRefSection(xref map[uint32]*xRefEntry, s *scanner, start, end uint32) error {
	offByOne := uint32(0)
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
				Err: io.ErrUnexpectedEOF,
			}
		}

		a, err := strconv.ParseInt(string(buf[:10]), 10, 64)
		if err != nil {
			return err
		}
		b, err := strconv.ParseUint(string(buf[11:16]), 10, 16)
		if err != nil {
			// fix an error seen in some PDF files
			if bytes.HasPrefix(buf, []byte("0000000000 65536 ")) {
				b = 65535
				buf[17] = 'f'
			} else {
				return err
			}
		}

		// fix an error seen in some PDF files
		if i == start && start == 1 && a == 0 && b == 65535 {
			offByOne = 1
		}

		c := buf[17]
		switch c {
		case 'f':
			xref[i-offByOne] = &xRefEntry{
				Pos:        -1,
				Generation: uint16(b),
			}
		case 'n':
			xref[i-offByOne] = &xRefEntry{
				Pos:        a,
				Generation: uint16(b),
			}
		default:
			return &MalformedFileError{
				Err: errInvalidXref,
				Loc: []string{fmt.Sprintf("byte %d", s.currentPos())},
			}
		}

		if buf[19] == '\n' || buf[19] == '\r' {
			s.bufPos += 20
		} else {
			// Some mal-formed PDF files use one-byte line endings.
			// Try to fix this up ...
			s.bufPos += 19
		}
	}
	return nil
}

func (r *Reader) readXRefStream(xref map[uint32]*xRefEntry, s *scanner) (Dict, Reference, error) {
	obj, ref, err := s.ReadIndirectObject()
	if err != nil {
		return nil, 0, err
	}
	stream, ok := obj.(*Stream)
	if !ok {
		return nil, 0, &MalformedFileError{
			Err: fmt.Errorf("found %T instead of xref stream", obj),
		}
	}
	dict := stream.Dict

	w, ss, err := checkXRefStreamDict(dict)
	if err != nil {
		return nil, 0, err
	}
	decoded, err := DecodeStream(r, stream, 0)
	if err != nil {
		return nil, 0, err
	}
	err = decodeXRefStream(xref, decoded, w, ss)
	if err != nil {
		return nil, 0, err
	}

	return dict, ref, nil
}

func checkXRefStreamDict(dict Dict) ([]int, []*xRefSubSection, error) {
	size, ok := dict["Size"].(Integer)
	if !ok || size < 0 || size > math.MaxUint32 {
		return nil, nil, &MalformedFileError{
			Err: errInvalidXref,
		}
	}
	W, ok := dict["W"].(Array)
	if !ok || len(W) != 3 {
		return nil, nil, &MalformedFileError{
			Err: errInvalidXref,
		}
	}
	var w []int
	for _, Wi := range W {
		wi, ok := Wi.(Integer)
		if !ok || wi < 0 || wi > 8 {
			return nil, nil, &MalformedFileError{
				Err: errInvalidXref,
			}
		}
		w = append(w, int(wi))
	}
	if w[0]+w[1]+w[2] == 0 {
		return nil, nil, &MalformedFileError{
			Err: errInvalidXref,
		}
	}

	Index := dict["Index"]
	var ss []*xRefSubSection
	if Index == nil {
		ss = append(ss, &xRefSubSection{0, uint32(size)})
	} else {
		ind, ok := Index.(Array)
		if !ok || len(ind)%2 != 0 {
			return nil, nil, &MalformedFileError{}
		}
		for i := 0; i < len(ind); i += 2 {
			start, ok1 := ind[i].(Integer)
			size, ok2 := ind[i+1].(Integer)
			if !ok1 || !ok2 {
				return nil, nil, &MalformedFileError{
					Err: errInvalidXref,
				}
			} else if start < 0 || size <= 0 || start > math.MaxUint32 || start+size > math.MaxUint32 {
				return nil, nil, &MalformedFileError{
					Err: errInvalidXref,
				}
			}
			ss = append(ss, &xRefSubSection{uint32(start), uint32(size)})
		}
	}
	return w, ss, nil
}

func decodeXRefStream(xref map[uint32]*xRefEntry, r io.Reader, w []int, ss []*xRefSubSection) error {
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
					Pos:      b,
					InStream: NewReference(uint32(a), 0),
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

func (r *Reader) lastOccurence(pat string, size int64) (int64, error) {
	const chunkSize = 1024

	buf := make([]byte, chunkSize)
	k := int64(len(pat))
	pos := size
	for pos >= k {
		start := pos - chunkSize
		if start < 0 {
			start = 0
		}
		_, err := r.r.Seek(start, io.SeekStart)
		if err != nil {
			return 0, err
		}
		n, err := r.r.Read(buf[:pos-start])
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
		Err: fmt.Errorf("pattern %q not found", pat),
	}
}

// setXRef adds a new entry to the xref table.
// If the entry is already present, an error is returned.
func (pdf *Writer) setXRef(ref Reference, entry *xRefEntry) error {
	_, seen := pdf.xref[ref.Number()]
	if seen {
		return errDuplicateRef
	}
	if pdf.nextRef <= ref.Number() {
		pdf.nextRef = ref.Number() + 1
	}
	pdf.xref[ref.Number()] = entry
	return nil
}

func (pdf *Writer) writeXRefTable(xRefDict Dict) error {
	_, err := fmt.Fprintf(pdf.w, "xref\n0 %d\n", pdf.nextRef)
	if err != nil {
		return err
	}
	for i := uint32(0); i < pdf.nextRef; i++ {
		entry := pdf.xref[i]
		if entry != nil && entry.InStream != 0 {
			return errors.New("cannot use xref tables with object streams")
		}
		if entry != nil && entry.Pos >= 0 {
			_, err = fmt.Fprintf(pdf.w, "%010d %05d n\r\n",
				entry.Pos, entry.Generation)
		} else {
			// free object
			_, err = pdf.w.Write([]byte("0000000000 65535 f\r\n"))
		}
		if err != nil {
			return err
		}
	}

	_, err = pdf.w.Write([]byte("trailer\n"))
	if err != nil {
		return err
	}
	err = Format(pdf.w, pdf.outputOptions, xRefDict)
	if err != nil {
		return err
	}
	_, err = pdf.w.Write([]byte{'\n'})
	return err
}

func (pdf *Writer) writeXRefStream(xRefDict Dict) error {
	ref := pdf.Alloc()
	// no more object allocations after this point

	xRefDict["Type"] = Name("XRef")
	xRefDict["Size"] = Integer(pdf.nextRef)

	maxField2 := int64(0)
	maxField3 := uint16(0)
	for i := uint32(0); i < pdf.nextRef; i++ {
		entry := pdf.xref[i]
		if entry == nil {
			continue
		}
		var f2 int64
		var f3 uint16
		if entry.InStream != 0 {
			f2 = int64(entry.InStream.Number())
			f3 = uint16(entry.Pos)
		} else if entry.Pos >= 0 {
			f2 = entry.Pos
			f3 = entry.Generation
		} else {
			gen := entry.Generation
			if gen == 65535 {
				gen = 0
			}
			f2 = 0
			f3 = gen
		}
		if f2 > maxField2 {
			maxField2 = f2
		}
		if f3 > maxField3 {
			maxField3 = f3
		}
	}
	w2 := (bits.Len64(uint64(maxField2)) + 7) / 8
	w3 := (bits.Len16(maxField3) + 7) / 8
	W := Array{Integer(1), Integer(w2), Integer(w3)}
	xRefDict["W"] = W

	// Compress the xref stream in memory, to make sure we know the size of the
	// stream before writing the xref stream object.
	filter := FilterFlate{
		"Predictor": Integer(12),
		"Columns":   Integer(1 + w2 + w3),
	}
	xRefBuf := &bytes.Buffer{}
	wxRaw, err := filter.Encode(pdf.meta.Version, withDummyClose{xRefBuf})
	if err != nil {
		return err
	}
	wx := bufio.NewWriter(wxRaw)
	for i := uint32(0); i < pdf.nextRef; i++ {
		entry := pdf.xref[i]
		if entry == nil {
			err := wx.WriteByte(0)
			if err != nil {
				return err
			}
			err = encodeInt64(wx, 0, w2)
			if err != nil {
				return err
			}
			err = encodeInt16(wx, 0, w3)
			if err != nil {
				return err
			}
		} else if entry.Pos < 0 {
			err := wx.WriteByte(0)
			if err != nil {
				return err
			}
			err = encodeInt64(wx, 0, w2)
			if err != nil {
				return err
			}
			err = encodeInt16(wx, entry.Generation, w3)
			if err != nil {
				return err
			}
		} else if entry.InStream == 0 {
			err := wx.WriteByte(1)
			if err != nil {
				return err
			}
			err = encodeInt64(wx, uint64(entry.Pos), w2)
			if err != nil {
				return err
			}
			err = encodeInt16(wx, entry.Generation, w3)
			if err != nil {
				return err
			}
		} else {
			err := wx.WriteByte(2)
			if err != nil {
				return err
			}
			err = encodeInt64(wx, uint64(entry.InStream.Number()), w2)
			if err != nil {
				return err
			}
			err = encodeInt16(wx, uint16(entry.Pos), w3)
			if err != nil {
				return err
			}
		}
	}
	err = wx.Flush()
	if err != nil {
		return err
	}
	err = wxRaw.Close()
	if err != nil {
		return err
	}
	xRefData := xRefBuf.Bytes()

	name, parms, err := filter.Info(pdf.meta.Version)
	if err != nil {
		return err
	}
	xRefDict["Filter"] = name
	xRefDict["DecodeParms"] = parms
	xRefDict["Length"] = Integer(len(xRefData))

	swx, err := pdf.OpenStream(ref, xRefDict)
	if err != nil {
		return err
	}
	_, err = swx.Write(xRefData)
	if err != nil {
		return err
	}
	err = swx.Close()
	return err
}

func encodeInt64(data io.ByteWriter, x uint64, w int) error {
	for i := w - 1; i >= 0; i-- {
		err := data.WriteByte(byte(x >> (i * 8)))
		if err != nil {
			return err
		}
	}
	return nil
}

func encodeInt16(data io.ByteWriter, x uint16, w int) error {
	for i := w - 1; i >= 0; i-- {
		err := data.WriteByte(byte(x >> (i * 8)))
		if err != nil {
			return err
		}
	}
	return nil
}

type xRefSubSection struct {
	Start, Size uint32
}

type xRefEntry struct {
	InStream   Reference
	Pos        int64
	Generation uint16
}

func (entry *xRefEntry) IsFree() bool {
	return entry == nil || entry.Pos < 0
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
