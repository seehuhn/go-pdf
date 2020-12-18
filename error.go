package pdflib

import "errors"

var (
	errOutOfRange = errors.New("file position out of range")

	// TODO(voss): include a byte offset in the error, so that users
	// get some meaningful feedback.
	errMalformed = errors.New("file is not a valid PDF file")

	errVersion = errors.New("unsupported PDF version")
)
