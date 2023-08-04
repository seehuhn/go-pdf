// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package type1

import (
	"fmt"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
)

type embedded struct {
	*fontInfo

	w       pdf.Putter
	ref     pdf.Reference
	resName pdf.Name

	enc    cmap.SimpleEncoder
	closed bool
}

func (f *embedded) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	res := &embedded{
		fontInfo: f.fontInfo,
		w:        w,
		ref:      w.Alloc(),
		resName:  resName,
		enc:      cmap.NewSimpleEncoder(),
	}

	w.AutoClose(res)

	return res, nil
}

func (e *embedded) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	return append(s, e.enc.Encode(gid, rr))
}

func (f *embedded) ResourceName() pdf.Name {
	return f.resName
}

func (f *embedded) Reference() pdf.Reference {
	return f.ref
}

func (f *embedded) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.enc.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.resName, f.afm.FontInfo.FontName)
	}
	f.enc = cmap.NewFrozenSimpleEncoder(f.enc)

	encoding := make([]string, 256)
	for i, gid := range f.enc.Encoding() {
		encoding[i] = f.names[gid]
	}

	t1 := &PDFFont{
		PSFont:   f.afm,
		ResName:  f.resName,
		Encoding: encoding,
	}
	return t1.Embed(f.w, f.ref)
}
