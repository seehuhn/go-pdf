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

package font

// IsStandard determines whether a font is one of the 14 standard PDF Type 1 fonts.
// This is based solely on the font's PostScript name.
//
// TODO(voss): use the [stdmtx] package instead.
var IsStandard = map[string]bool{
	"Courier":               true,
	"Courier-Bold":          true,
	"Courier-BoldOblique":   true,
	"Courier-Oblique":       true,
	"Helvetica":             true,
	"Helvetica-Bold":        true,
	"Helvetica-BoldOblique": true,
	"Helvetica-Oblique":     true,
	"Times-Roman":           true,
	"Times-Bold":            true,
	"Times-BoldItalic":      true,
	"Times-Italic":          true,
	"Symbol":                true,
	"ZapfDingbats":          true,
}

// IsStandardNonSymbolic determines whether a font is one of the 12 standard
// non-symbolic fonts.
//
// TODO(voss): use the [stdmtx] package instead.
var IsStandardNonSymbolic = map[string]bool{
	"Courier":               true,
	"Courier-Bold":          true,
	"Courier-BoldOblique":   true,
	"Courier-Oblique":       true,
	"Helvetica":             true,
	"Helvetica-Bold":        true,
	"Helvetica-BoldOblique": true,
	"Helvetica-Oblique":     true,
	"Times-Roman":           true,
	"Times-Bold":            true,
	"Times-BoldItalic":      true,
	"Times-Italic":          true,
}
