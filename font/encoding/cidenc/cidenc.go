// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidenc

import (
	"errors"
	"iter"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

// TODO(voss): add CIDEncoder implementations for the standard PDF CMaps.

// Info describes a character code known to a [CIDEncoder].
type Info struct {
	CID      cid.CID
	Width    float64  // PDF glyph space units
	VMetrics VMetrics // PDF glyph space units
	Text     string
}

// VMetrics holds the metrics of a glyph in vertical writing mode, in PDF
// glyph space units.
type VMetrics struct {
	// OffsY is the y coordinate of the glyph's vertical origin, measured from
	// its horizontal origin.
	OffsY float64

	// DeltaY is the vertical displacement to the following glyph.  This is
	// normally negative, since vertical text advances downward.
	DeltaY float64
}

// A CIDEncoder maps character codes to CIDs, glyph metrics and text content.
//
// Implementations are safe for concurrent use: any number of goroutines may
// read an encoder while another allocates codes.  Codes are only ever added,
// never changed or reassigned, so a code once returned stays valid for the
// life of the encoder.  Iterators hold no lock while yielding, so a caller
// may allocate codes from inside a loop over one.
//
// Each method sees the codes allocated when it was called.  Results of
// separate calls need not agree with each other, so a caller which combines
// them, for example to build a font dictionary out of MappedCodes, CMap and
// ToUnicode, must not allow codes to be allocated in between.
type CIDEncoder interface {
	// WritingMode indicates whether the encoding is for horizontal or vertical
	// writing.
	WritingMode() font.WritingMode

	// Codes iterates over the character codes in a PDF string.
	// For every code, valid or not, the iterator returns the information a
	// reader reports for it once the composite font writers have written
	// the font, based on the codes encoded so far.  Encoding a CID later
	// can change its metrics in the written font, and with them the result
	// for a code which selects that CID without having been encoded.
	// In vertical writing mode, VerticalAdvance is set from the
	// DeltaY value stored with the code, and the position vector is half
	// the width horizontally and the stored OffsY vertically.
	Codes(s pdf.String) iter.Seq[font.Code]

	// MappedCodes iterates over all codes known to the encoder.  The
	// [Info] pointer is only valid until the next iteration; a caller
	// which needs to keep the data must copy it.
	MappedCodes() iter.Seq2[charcode.Code, *Info]

	// Encode returns the code for a CID and text, allocating a new one on
	// first use and storing the text, the width and the vertical metrics
	// with it.  A repeat of the same CID and text yields the code already
	// allocated.
	//
	// The font dictionary holds one width and one set of vertical metrics
	// per CID, so every call for a CID must give the same metrics, and
	// those of CID 0 are fixed when the encoder is created.  An encoder
	// backed by a fixed CMap gives each CID a single code, and so also
	// holds a single text per CID.  Encode returns an error if a value
	// conflicts with the one already stored, rather than encoding a value
	// it cannot represent; a rejected call leaves the encoder unchanged.
	// In horizontal writing mode the vertical metrics are not used and
	// cannot conflict.
	Encode(cidVal cid.CID, text string, width float64, v VMetrics) (charcode.Code, error)

	// CID0Metrics returns the width and the vertical metrics of CID 0, as
	// given when the encoder was created.  These are the metrics of every
	// CID which has not been encoded.
	CID0Metrics() (width float64, v VMetrics)

	// CodesRemaining returns the number of unallocated codes.
	CodesRemaining() int

	// CMap returns a CMap object which describes the mapping
	// from character codes to CIDs.
	CMap(ros *cid.SystemInfo) *cmap.File

	Codec() *charcode.Codec

	// GetCode returns the code which [CIDEncoder.Encode] has returned for a
	// CID and text, without allocating one.  An encoder backed by a fixed
	// CMap ignores the text and returns the code the CMap gives the CID,
	// once the CID has been encoded; CID 0 counts as encoded from the
	// start, but has a code only if the CMap maps a code to it.
	GetCode(cid cid.CID, text string) (charcode.Code, bool)

	Width(code charcode.Code) float64

	// ToUnicode returns a ToUnicode CMap representing the text content
	// of the mapped codes.
	ToUnicode() *cmap.ToUnicodeFile
}

type key struct {
	cid  cid.CID
	text string
}

type codeInfo struct {
	CID      cid.CID
	Width    float64  // PDF glyph space units
	VMetrics VMetrics // PDF glyph space units
	Text     string
}

// ErrOverflow is returned by [CIDEncoder.Encode] once no further codes can be
// allocated.
var ErrOverflow = errors.New("too many glyphs")
