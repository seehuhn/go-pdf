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

// Package composite provides convenience functions for embedding fonts into
// PDF files.
package composite

import (
	"os"

	"golang.org/x/text/language"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/truetype"
)

// EmbedOpenType loads a font from a file and embeds it into a PDF file.
// Both TrueType and OpenType fonts are supported.
//
// ResName, if not empty, is the default PDF resource name to use for the
// embedded font inside PDF content streams.  Normally, this should be left
// empty.
func EmbedOpenType(w pdf.Putter, fname string, resName pdf.Name, loc language.Tag) (font.Embedded, error) {
	font, err := LoadOpenType(fname, loc)
	if err != nil {
		return nil, err
	}
	return font.Embed(w, resName)
}

// EmbedOld creates a PDF CIDFont and embeds it into a PDF file.
//
// Deprecated: use EmbedOpenType instead, or open code the functionality.
//
// TODO(voss): remove
func EmbedOld(w pdf.Putter, info *sfnt.Font, resName pdf.Name, loc language.Tag) (font.Layouter, error) {
	opt := &font.Options{
		Language: loc,
	}
	var f font.Embedder
	var err error
	if info.IsCFF() {
		f, err = cff.NewComposite(info, opt)
	} else {
		f, err = truetype.NewComposite(info, opt)
	}
	if err != nil {
		return nil, err
	}

	return f.Embed(w, resName)
}

// LoadOpenType loads a font from a file as a simple PDF font.
// Both TrueType and OpenType fonts are supported.
func LoadOpenType(fname string, loc language.Tag) (font.Embedder, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	info, err := sfnt.Read(fd)
	if err != nil {
		return nil, err
	}

	opt := &font.Options{
		Language: loc,
	}
	if info.IsCFF() {
		return cff.NewComposite(info, opt)
	}
	return truetype.NewComposite(info, opt)
}
