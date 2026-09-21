// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package extract

import (
	"sync"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// StandardFontFallback returns a reading instance of the standard 14 font
// that name refers to, according to [standard.ByName], or of Helvetica when
// the name matches none of them.
//
// [Resources] installs this function on every resource dictionary it
// decodes, and [Form] and [Pattern] on the empty dictionary they substitute
// for a missing Resources entry, so that a Tf operator naming a font the
// dictionary lacks still shows its text.  The instances are shared and must
// not be modified.
func StandardFontFallback(name pdf.Name) font.Instance {
	f, ok := standard.ByName(string(name))
	if !ok {
		f = standard.Helvetica
	}
	return fallbackFonts()[f]
}

// fallbackFonts returns the shared reading instances, built on first use.
var fallbackFonts = sync.OnceValue(func() map[standard.Font]font.Instance {
	m := make(map[standard.Font]font.Instance, len(standard.All))
	for _, f := range standard.All {
		m[f] = standardFallback(f)
	}
	return m
})

// standardFallback builds a reading instance of f equivalent to the one
// decoding a font dictionary which names f and nothing else would produce:
// same descriptor, same widths, same text for every code.
func standardFallback(f standard.Font) font.Instance {
	psName := f.PostScriptName()
	stdInfo := stdmtx.Metrics[psName]
	d := &dict.Type1{
		PostScriptName: psName,
		Descriptor:     standardDescriptor(psName, stdInfo),
		// the font's built-in encoding, spelled out so that codes map to
		// glyph names, and through them to text, without the font program
		Encoding: func(code byte) string { return stdInfo.Encoding[code] },
	}
	standardWidths(d.Width[:], d.Encoding, stdInfo)
	return d.MakeFont()
}
