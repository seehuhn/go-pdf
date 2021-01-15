package pdf

import (
	"errors"
	"strconv"
)

var (
	// ErrWrongPassword indicates that authentication failed because the
	// correct password was not supplied.
	ErrWrongPassword = errors.New("authentication failed")

	errVersion   = errors.New("unsupported PDF version")
	errCorrupted = errors.New("corrupted ciphertext")
	errNoDate    = errors.New("not a valid date string")
)

// MalformedFileError indicates that the PDF file could not be parsed.
type MalformedFileError struct {
	Err error
	Pos int64
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

type errorReader struct {
	err error
}

func (e *errorReader) Read([]byte) (int, error) {
	return 0, e.err
}
