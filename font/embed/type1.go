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

package embed

import (
	"os"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/postscript/afm"
	pst1 "seehuhn.de/go/postscript/type1"
)

// Type1File loads and embeds a Type 1 font.
// The file `psname` can be either an .pfb or .pfa file.
// The file `afmname` is the corresponding .afm file.
// Both `psname` and `afmname` are optional, but at least one of them must be given.
func Type1File(psname, afmname string) (font.Layouter, error) {
	var psFont *pst1.Font
	var metrics *afm.Metrics
	if psname != "" {
		fd, err := os.Open(psname)
		if err != nil {
			return nil, err
		}
		psFont, err = pst1.Read(fd)
		if err != nil {
			fd.Close()
			return nil, err
		}
		err = fd.Close()
		if err != nil {
			return nil, err
		}
	}
	if afmname != "" {
		fd, err := os.Open(afmname)
		if err != nil {
			return nil, err
		}
		metrics, err = afm.Read(fd)
		if err != nil {
			fd.Close()
			return nil, err
		}
		err = fd.Close()
		if err != nil {
			return nil, err
		}
	}
	return Type1Font(psFont, metrics)
}

// Type1Font embeds a Type 1 font.
// The `psFont` and `metrics` parameters are optional, but at least one of them must be given.
func Type1Font(psFont *pst1.Font, metrics *afm.Metrics) (font.Layouter, error) {
	return type1.New(psFont, metrics)
}
