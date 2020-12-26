package pdflib

import (
	"errors"
	"strconv"
)

var (
	errOutOfRange = errors.New("file position out of range")
	errVersion    = errors.New("unsupported PDF version")
)

// MalformedFileError indicates that the PDF file could not be parsed.
type MalformedFileError struct {
	Pos int64
	Err error
}

func (err *MalformedFileError) Error() string {
	head := ""
	if err.Pos > 0 {
		head = strconv.FormatInt(err.Pos, 10) + ": "
	}
	tail := ""
	if err.Err != nil {
		tail = ": " + err.Err.Error()
	}
	return head + "not a valid PDF file" + tail
}
func (err *MalformedFileError) Unwrap() error {
	return err.Err
}
