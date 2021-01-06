package pdf

import (
	"errors"
	"strconv"
)

var (
	// ErrWrongPassword indicates that authentication failed because the
	// correct password was not supplied.
	ErrWrongPassword = errors.New("authentication failed")

	errVersion = errors.New("unsupported PDF version")
)

// MalformedFileError indicates that the PDF file could not be parsed.
type MalformedFileError struct {
	Pos int64
	Err error
}

func (err *MalformedFileError) Error() string {
	middle := ""
	if err.Err != nil {
		middle = ": " + err.Err.Error()
	}
	tail := ""
	if err.Pos > 0 {
		tail = " (at byte " + strconv.FormatInt(err.Pos, 10) + ")"
	}
	return "not a valid PDF file" + middle + tail
}

func (err *MalformedFileError) Unwrap() error {
	return err.Err
}
