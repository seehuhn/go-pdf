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

package simple

import (
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/afm"
	pst1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
)

// EmbedType1 loads a Type 1 font from a file and embeds it into a PDF file.
// Both ".pfa" and ".pfb" files are supported.
// If afmname is not empty, the corresponding ".afm" file is loaded as well
// and is used to provide additional font metrics, ligatures, and kerning
// information.
//
// ResName, if not empty, is the default PDF resource name to use for the
// embedded font inside PDF content streams.  Normally, this should be left
// empty.
func EmbedType1(w pdf.Putter, fname string, afmname string, resName pdf.Name) (font.Layouter, error) {
	sfnt, err := LoadType1(fname, afmname)
	if err != nil {
		return nil, err
	}
	return sfnt.Embed(w, &font.Options{ResName: resName})
}

// LoadType1 loads a Type 1 font from a file as a simple PDF font.
// Both ".pfa" and ".pfb" files are supported.
// If afmname is not empty, the corresponding ".afm" file is loaded as well
// and is used to provide additional font metrics, ligatures, and kerning
// information.
func LoadType1(fname, afmname string) (font.Embedder, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	psfont, err := pst1.Read(fd)
	if err != nil {
		return nil, err
	}

	var metric *afm.Info
	if afmname != "" {
		afmFd, err := os.Open(afmname)
		if err != nil {
			return nil, err
		}
		defer afmFd.Close()

		metric, err = afm.Read(afmFd)
		if err != nil {
			return nil, err
		}
	}

	return type1.New(psfont, metric)
}
