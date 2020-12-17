package pdflib

import "io"

// PDFObject represents an object in a PDF file.
type PDFObject interface{}

// PDFBool represents a boolean value in a PDF file.
type PDFBool bool

// PDFInt represents an integer constant in a PDF file.
type PDFInt int64

// PDFReal represents an real number in a PDF file.
type PDFReal float64

// PDFString represents a string constant in a PDF file.
type PDFString string

// PDFName represents a name in a PDF file.
type PDFName string

// PDFArray represent an array in a PDF file.
type PDFArray []PDFObject

// PDFDict represent a Dictionary object in a PDF file.
type PDFDict struct {
	Data map[PDFName]PDFObject
	Ref  *PDFReference
}

// PDFStream represent a stream object in a PDF file.
type PDFStream struct {
	PDFDict
	R io.Reader
}

// PDFReference represents an indirect object in a PDF file.
type PDFReference struct {
	no, gen int64
}
