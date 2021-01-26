package pdf

import (
	"errors"
	"strconv"
)

var (
	// ErrNoAuth indicates that authentication failed because the
	// correct password was not supplied.
	ErrNoAuth = errors.New("authentication failed")

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

// VersionError is returned when trying to use a feature in a PDF file which is
// not supported by the PDF version used.
type VersionError struct {
	Earliest  Version
	Operation string
}

func (err *VersionError) Error() string {
	return (err.Operation + " requires PDF version " +
		err.Earliest.String() + " or newer")
}
