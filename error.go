package pdflib

import "errors"

var (
	errOutOfRange = errors.New("file position out of range")
	errMalformed  = errors.New("file is not a valid PDF file")
	errVersion    = errors.New("unsupported PDF version")
)
