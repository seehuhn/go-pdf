// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package reader

import (
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/widths"
)

type CIDFont struct {
	codec *charcode.Codec
	cmap  *cmap.File
	toUni *cmap.ToUnicodeFile
	dec   map[charcode.Code]*font.Code

	notdef *font.Code

	widths map[cmap.CID]float64
	dw     float64
}

func (r *Reader) readCompositeFont(info *font.Dicts, toUni *cmap.ToUnicodeFile) (F FontFromFile, err error) {
	fontDict := info.FontDict
	cidFontDict := info.CIDFontDict

	encoding, err := cmap.Extract(r.R, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}

	// Fix the code space range.
	var csr charcode.CodeSpaceRange
	csr = append(csr, encoding.CodeSpaceRange...)
	csr = append(csr, toUni.CodeSpaceRange...)
	codec, err := charcode.NewCodec(csr)
	if err != nil {
		// In case the two code spaces are not compatible, try to use only the
		// code space from the encoding.
		csr = append(csr[:0], encoding.CodeSpaceRange...)
		codec, err = charcode.NewCodec(csr)
	}
	if err != nil {
		return nil, err
	}

	ww, dw, err := widths.DecodeComposite(r.R, cidFontDict["W"], cidFontDict["DW"])
	if err != nil {
		return nil, err
	}

	cid0Width, ok := ww[0]
	if !ok {
		cid0Width = dw
	}
	notdef := &font.Code{
		Text:  string([]rune{unicode.ReplacementChar}),
		Width: cid0Width,
	}

	res := &CIDFont{
		codec: codec,
		cmap:  encoding,
		toUni: toUni,
		dec:   make(map[charcode.Code]*font.Code),

		notdef: notdef,

		widths: ww,
		dw:     dw,
	}

	return res, nil
}

func (f *CIDFont) WritingMode() font.WritingMode {
	return f.cmap.WMode
}

func (f *CIDFont) Decode(s pdf.String) (*font.Code, int) {
	code, k, ok := f.codec.Decode(s)
	if !ok {
		return f.notdef, k
	}

	if ci, ok := f.dec[code]; ok {
		return ci, k
	}

	CID1 := f.cmap.LookupCID(s[:k])
	CID2 := f.cmap.LookupNotdefCID(s[:k])
	if CID1 == 0 {
		CID1 = CID2
		CID2 = 0
	}

	w, ok := f.widths[CID1]
	if !ok {
		w = f.dw
	}

	var text string
	if f.toUni != nil {
		text, _ = f.toUni.Lookup(s[:k])
	} else {
		// TODO(voss): try the ToUnicode CMaps for the Adobe standard
		// character collections.
	}

	res := &font.Code{
		CID:    CID1,
		Notdef: CID2,
		Text:   text,
		Width:  w,
	}
	f.dec[code] = res
	return res, k
}

func (f *CIDFont) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	ci, k := f.Decode(s)
	return ci.Width, k
}

func (f *CIDFont) FontData() interface{} {
	panic("not implemented") // TODO: Implement
}

func (f *CIDFont) Key() pdf.Reference {
	panic("not implemented") // TODO: Implement
}
