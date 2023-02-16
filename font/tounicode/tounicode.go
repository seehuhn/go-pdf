package tounicode

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/sfnt/type1"
)

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf
// https://www.adobe.com/content/dam/acom/en/devnet/acrobat/pdfs/5411.ToUnicode.pdf

// Info describes a mapping from character codes to Unicode characters
// sequences.
type Info struct {
	CodeSpace []CodeSpaceRange
	Singles   []Single
	Ranges    []Range

	Name pdf.Name
	ROS  *type1.CIDSystemInfo
}

func (info *Info) containsCode(code cmap.CID) bool {
	for _, r := range info.CodeSpace {
		if r.First <= code && code <= r.Last {
			return true
		}
	}
	return false
}

func (info *Info) containsRange(first, last cmap.CID) bool {
	for _, r := range info.CodeSpace {
		if r.First <= first && last <= r.Last {
			return true
		}
	}
	return false
}

type CodeSpaceRange struct {
	First cmap.CID
	Last  cmap.CID
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
	Code  cmap.CID
	UTF16 []uint16
}

type Range struct {
	First cmap.CID
	Last  cmap.CID
	UTF16 [][]uint16
}
