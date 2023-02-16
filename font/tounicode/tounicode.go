package tounicode

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cmap"
)

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf
// https://www.adobe.com/content/dam/acom/en/devnet/acrobat/pdfs/5411.ToUnicode.pdf

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

func (info *Info) ContainsCode(code cmap.CharCode) bool {
	for _, r := range info.CodeSpace {
		if r.First <= code && code <= r.Last {
			return true
		}
	}
	return false
}

func (info *Info) ContainsRange(first, last cmap.CharCode) bool {
	for _, r := range info.CodeSpace {
		if r.First <= first && last <= r.Last {
			return true
		}
	}
	return false
}

type CodeSpaceRange struct {
	First cmap.CharCode
	Last  cmap.CharCode
}

func (c CodeSpaceRange) String() string {
	var format string
	if c.Last >= 1<<24 {
		format = "%08X"
	} else if c.Last >= 1<<16 {
		format = "%06X"
	} else if c.Last >= 1<<8 {
		format = "%04X"
	} else {
		format = "%02X"
	}
	return fmt.Sprintf("<"+format+"> <"+format+">", c.First, c.Last)
}

type Single struct {
	Code cmap.CharCode
	Text string
}

type Range struct {
	First cmap.CharCode
	Last  cmap.CharCode
	Text  []string
}
