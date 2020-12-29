package pdflib

import (
	"bytes"
	"errors"
	"io"
)

// Reader represents a pdf file opened for reading.
type Reader struct {
	size int64
	r    io.ReaderAt

	HeaderVersion PDFVersion
	Trailer       *Dict

	xref map[int]*xRefEntry
}

// NewReader creates a new Reader object.
func NewReader(data io.ReaderAt, size int64) (*Reader, error) {
	r := &Reader{
		size: size,
		r:    data,
	}

	version, err := r.readHeaderVersion()
	if err != nil {
		return nil, err
	}
	r.HeaderVersion = version

	xref, err := r.readXRef()
	if err != nil {
		return nil, err
	}
	r.xref = xref

	return r, nil
}

func (r *Reader) readHeaderVersion() (PDFVersion, error) {
	var buf [16]byte
	n, err := r.r.ReadAt(buf[:], 0)
	if err != nil && err != io.EOF {
		return -1, err
	}

	if !bytes.HasPrefix(buf[:n], []byte("%PDF-1.")) || n < 8 {
		return -1, &MalformedFileError{
			Pos: 0,
			Err: errors.New("PDF header not found"),
		}
	}

	version := PDFVersion(buf[7]) - '0'
	if version < 0 || version >= tooLargeVersion ||
		n > 8 && buf[8] >= '0' && buf[8] <= '9' {
		return -1, &MalformedFileError{Pos: 7, Err: errVersion}
	}

	return version, nil
}

// PDFVersion represent the version of PDF standard used in a file.
type PDFVersion int

// Constants for the known PDF versions.
const (
	V1_0 PDFVersion = iota
	V1_1
	V1_2
	V1_3
	V1_4
	V1_5
	V1_6
	V1_7
	tooLargeVersion
)
