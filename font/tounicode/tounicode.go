package tounicode

import (
	"fmt"
	"unicode/utf16"

	"seehuhn.de/go/pdf"
)

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

func (c CodeSpaceRange) String() string {
	var format string
	if c.Last >= 1<<24 {
		format = "%08x"
	} else if c.Last >= 1<<16 {
		format = "%06x"
	} else if c.Last >= 1<<8 {
		format = "%04x"
	} else {
		format = "%02x"
	}
	return fmt.Sprintf("<"+format+"> <"+format+">", c.First, c.Last)
}

type Single struct {
	Code CharCode
	Text string
}

func (bfc Single) String() string {
	var format string
	if bfc.Code >= 1<<24 {
		format = "%08x"
	} else if bfc.Code >= 1<<16 {
		format = "%06x"
	} else if bfc.Code >= 1<<8 {
		format = "%04x"
	} else {
		format = "%02x"
	}

	var text []byte
	for _, x := range utf16.Encode([]rune(bfc.Text)) {
		text = append(text, byte(x>>8), byte(x))
	}
	return fmt.Sprintf("<"+format+"> <%02X>", bfc.Code, text)
}

type Range struct {
	First CharCode
	Last  CharCode
	Text  []string
}
