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

package pdfenc

//go:generate go run ./generate.go

// An Encoding is a mapping from single byte codes to glyph names.
type Encoding struct {
	Encoding [256]string
	Has      map[string]bool
}

// Standard is the Adobe Standard Encoding for Latin text.
//
// See Appendix D.2 of PDF 32000-1:2008.
var Standard = Encoding{
	Encoding: standardEncoding,
	Has:      standardEncodingHas,
}

// WinAnsi is the PDF version of the standard Microsoft Windows specific
// encoding for Latin text in Western writing systems.
//
// See Appendix D.2 of PDF 32000-1:2008.
var WinAnsi = Encoding{
	Encoding: winAnsiEncoding,
	Has:      winAnsiEncodingHas,
}

// MacRoman is the PDF version of the MacOS standard encoding for Latin
// text in Western writing systems.
//
// See Appendix D.2 of PDF 32000-1:2008.
var MacRoman = Encoding{
	Encoding: macRomanEncoding,
	Has:      macRomanEncodingHas,
}

// MacRomanAlt is sometimes used when constructing character encodings
// for TrueType fonts in PDF files.
//
// This differs from [MacRoman] by the addition of 15 characters and
// the replacement of the currency glyph with the Euro symbol.
var MacRomanAlt = Encoding{
	Encoding: macRomanAlt,
	Has:      macRomanAltHas,
}

// MacExpert is an encoding which contains more obscure characters.
//
// See Appendix D.4 of PDF 32000-1:2008.
var MacExpert = Encoding{
	Encoding: macExpertEncoding,
	Has:      macExpertEncodingHas,
}

// Symbol is the built-in encoding for the Symbol font.
//
// See Appendix D.5 of PDF 32000-1:2008.
var Symbol = Encoding{
	Encoding: symbolEncoding,
	Has:      symbolEncodingHas,
}

// ZapfDingbats is the built-in encoding of the ZapfDingbats font.
//
// See Appendix D.6 of PDF 32000-1:2008.
var ZapfDingbats = Encoding{
	Encoding: zapfDingbatsEncoding,
	Has:      zapfDingbatsEncodingHas,
}

// PDFDoc is an encoding for text strings in a PDF document outside the
// document's content streams.
//
// See Appendix D.2 of PDF 32000-1:2008.
var PDFDoc = Encoding{
	Encoding: pdfDocEncoding,
	Has:      pdfDocEncodingHas,
}
