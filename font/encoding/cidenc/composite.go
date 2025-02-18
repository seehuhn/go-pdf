package cidenc

import (
	"iter"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/cid"
)

type Composite struct {
	wMode cmap.WritingMode
	codec *charcode.Codec
	info  map[charcode.Code]*codeInfo
}

type codeInfo struct {
	CID   cid.CID
	Width float64 // PDF glyph space units
	Text  string
}

func NewComposite() *Composite {
	return &Composite{}
}

// WritingMode indicates whether the font is for horizontal or vertical
// writing.
func (e *Composite) WritingMode() cmap.WritingMode {
	return e.wMode
}

// DecodeWidth reads one character code from the given string and returns
// the width of the corresponding glyph in PDF text space units (still to
// be multiplied by the font size) and the number of bytes read from the
// string.
//
// TODO(voss): remove
func (e *Composite) DecodeWidth(s pdf.String) (float64, int) {
	panic("not implemented") // TODO: Implement
}

// Codes iterates over the character codes in a PDF string.
func (e *Composite) Codes(s pdf.String) iter.Seq[*font.Code] {
	panic("not implemented") // TODO: Implement
}

func (e *Composite) get(c charcode.Code) *codeInfo {
	panic("not implemented") // TODO: Implement
}

func (e *Composite) codeToCID(c charcode.Code) cid.CID {
	panic("not implemented") // TODO: Implement
}

func (e *Composite) codeToNotdefCID(c charcode.Code) cid.CID {
	panic("not implemented") // TODO: Implement
}
