package pdflib

import "io"

// PDFObject represents an object in a PDF file.
type PDFObject interface {
	PDFWrite(w io.Writer) error
}

// PDFBool represents a boolean value in a PDF file.
type PDFBool bool

// PDFWrite implements the PDFObject interface
func (b PDFBool) PDFWrite(w io.Writer) error {
	panic("not implemented")
}

// PDFInt represents an integer constant in a PDF file.
type PDFInt int64

// PDFWrite implements the PDFObject interface
func (b PDFInt) PDFWrite(w io.Writer) error {
	panic("not implemented")
}

// PDFReal represents an real number in a PDF file.
type PDFReal float64

// PDFWrite implements the PDFObject interface
func (b PDFReal) PDFWrite(w io.Writer) error {
	panic("not implemented")
}

// PDFString represents a string constant in a PDF file.
type PDFString string

// PDFWrite implements the PDFObject interface
func (b PDFString) PDFWrite(w io.Writer) error {
	panic("not implemented")
}

// PDFName represents a name in a PDF file.
type PDFName string

// PDFWrite implements the PDFObject interface
func (b PDFName) PDFWrite(w io.Writer) error {
	panic("not implemented")
}

// PDFArray represent an array in a PDF file.
type PDFArray []PDFObject

// PDFWrite implements the PDFObject interface
func (d PDFArray) PDFWrite(w io.Writer) error {
	panic("not implemented")
}

// PDFDict represent a Dictionary object in a PDF file.
type PDFDict map[PDFName]PDFObject

// PDFWrite implements the PDFObject interface
func (d PDFDict) PDFWrite(w io.Writer) error {
	panic("not implemented")
}

// PDFReference represents an indirect object in a PDF file.
type PDFReference struct {
	no, gen int64
}

// PDFWrite implements the PDFObject interface
func (d PDFReference) PDFWrite(w io.Writer) error {
	panic("not implemented")
}
