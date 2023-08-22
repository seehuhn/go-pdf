// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

// Package cid provides support for embedding CID fonts into PDF documents.
package cid

import (
	"os"

	"golang.org/x/text/language"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/truetype"
)

// Inside the PDF documents on my laptop, the following encoding CMaps are used
// for CIDFonts.  The numbers are the number of occurences of the encoding:
//
//   3110 /Identity-H
//      6 /UniCNS-UTF16-H
//      5 /Identity-V
//      5 /UniGB-UCS2-H
//      4 /UniKS-UCS2-H
//      3 /UniJIS-UCS2-H
//      2 /90msp-RKSJ-H
//      2 /KSCms-UHC-H
//      2 /UniGB-UTF16-H
//      2 (indirect reference to embedded CMap)
//      1 /ETenms-B5-H
//      1 /GBK-EUC-H

// EmbedFile loads a font from a file and embeds it into a PDF file.
// At the moment, only TrueType and OpenType fonts are supported.
func EmbedFile(w pdf.Putter, fname string, resName pdf.Name, loc language.Tag) (font.Embedded, error) {
	font, err := LoadFont(fname, loc)
	if err != nil {
		return nil, err
	}
	return font.Embed(w, resName)
}

// Embed creates a PDF CIDFont and embeds it into a PDF file.
// At the moment, only TrueType and OpenType fonts are supported.
func Embed(w pdf.Putter, info *sfnt.Font, resName pdf.Name, loc language.Tag) (font.Embedded, error) {
	f, err := Font(info, loc)
	if err != nil {
		return nil, err
	}
	return f.Embed(w, resName)
}

// LoadFont loads a font from a file as a PDF CIDFont.
// At the moment, only TrueType and OpenType fonts are supported.
//
// CIDFonts lead to larger PDF files than simple fonts, but there is no limit
// on the number of distinct glyphs which can be accessed.
func LoadFont(fname string, loc language.Tag) (font.Font, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	info, err := sfnt.Read(fd)
	if err != nil {
		return nil, err
	}
	return Font(info, loc)
}

// Font creates a PDF CIDFont.
//
// CIDFonts lead to larger PDF files than simple fonts, but there is no limit
// on the number of distinct glyphs which can be accessed.
func Font(info *sfnt.Font, loc language.Tag) (font.Font, error) {
	if info.IsCFF() {
		opt := &cff.FontOptions{
			Language: loc,
		}
		return cff.NewComposite(info, opt)
	}
	return truetype.NewComposite(info, loc)
}
