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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/widths"
	"seehuhn.de/go/sfnt/glyph"
)

type CIDFont struct {
	codec *charcode.Codec
	dec   map[uint32]*codeInfo

	wMode cmap.WritingMode

	widths map[cmap.CID]float64
	dw     float64
}

type codeInfo struct {
	CID    cmap.CID
	NotDef cmap.CID // CID to use if glyph is missing from the font
	Text   []rune
}

func (f *CIDFont) WritingMode() cmap.WritingMode {
	return f.wMode
}

func (f *CIDFont) ForeachWidth(s pdf.String, yield func(width float64, isSpace bool)) {
	panic("not implemented")
}

// CodeAndWidth converts a glyph ID (corresponding to the given text) into
// a PDF character code The character code is appended to s. The function
// returns the new string s, the width of the glyph in PDF text space units
// (still to be multiplied by the font size), and a value indicating
// whether PDF word spacing adjustment applies to this glyph.
//
// As a side effect, this function may allocate codes for the given
// glyph/text combination in the font's encoding.
func (f *CIDFont) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	panic("not implemented") // TODO: Implement
}

func getCIDFont(r pdf.Getter, dicts *font.Dicts) (*CIDFont, error) {
	fontDict := dicts.FontDict
	cidFontDict := dicts.CIDFontDict

	encoding, err := cmap.ExtractNew(r, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}

	toUni, _ := cmap.ExtractToUnicodeNew(r, fontDict["ToUnicode"])

	// Fix the code space range.
	var cs charcode.CodeSpaceRange
	cs = append(cs, encoding.CodeSpaceRange...)
	cs = append(cs, toUni.CodeSpaceRange...)
	codec, err := charcode.NewCodec(cs)
	if err != nil {
		// In case the two code spaces are not compatible, try to use only the
		// code space from the encoding.
		cs = append(cs[:0], encoding.CodeSpaceRange...)
		codec, err = charcode.NewCodec(cs)
	}
	if err != nil {
		return nil, err
	}

	dec := make(map[uint32]*codeInfo)
	hasMapping := make(map[uint32]bool)

	// First add the CID mappings
	for _, entry := range encoding.CIDSingles {
		code, k, ok := codec.Decode(entry.Code)
		if !ok || k != len(entry.Code) {
			continue
		}
		dec[code] = &codeInfo{
			CID: entry.Value,
		}
		hasMapping[code] = true
	}
	for _, entry := range encoding.CIDRanges {
		for code, cid := range entry.All(codec) {
			dec[code] = &codeInfo{
				CID: cid,
			}
			hasMapping[code] = true
		}
	}

	// Add the notdef mappings
	for _, entry := range encoding.NotdefSingles {
		code, k, ok := codec.Decode(entry.Code)
		if !ok || k != len(entry.Code) {
			continue
		}

		d := dec[code]
		if hasMapping[code] {
			d.NotDef = entry.Value
		} else {
			d.CID = entry.Value
		}
		dec[code] = d
	}
	for _, entry := range encoding.NotdefRanges {
		for code := range entry.All(codec) {
			d := dec[code]
			if hasMapping[code] {
				d.NotDef = entry.Value
			} else {
				d.CID = entry.Value
			}
			dec[code] = d
		}
	}

	// Add the ToUnicode mappings
	for _, entry := range toUni.Singles {
		code, k, ok := codec.Decode(entry.Code)
		if !ok || k != len(entry.Code) {
			continue
		}

		d := dec[code]
		d.Text = entry.Value
		dec[code] = d
	}
	for _, entry := range toUni.Ranges {
		for code, text := range entry.All(codec) {
			d := dec[code]
			d.Text = text
			dec[code] = d
		}
	}

	ww, dw, err := widths.DecodeComposite(r, cidFontDict["W"], cidFontDict["DW"])
	if err != nil {
		return nil, err
	}

	res := &CIDFont{
		codec:  codec,
		dec:    dec,
		wMode:  encoding.WMode,
		widths: ww,
		dw:     dw,
	}

	return res, nil
}
