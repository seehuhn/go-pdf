package tounicode

import "seehuhn.de/go/pdf"

type CharCode uint32

// Info describes a mapping from character codes to Unicode characters sequences.
type Info struct {
	CodeSpace []CodeSpaceRange
	Singles   []Single
	Ranges    []Range

	Name       pdf.Name
	Registry   pdf.String
	Ordering   pdf.String
	Supplement pdf.Integer
}

type CodeSpaceRange struct {
	First CharCode
	Last  CharCode
}

type Single struct {
	Code CharCode
	Text string
}

type Range struct {
	First CharCode
	Last  CharCode
	Text  []string
}
