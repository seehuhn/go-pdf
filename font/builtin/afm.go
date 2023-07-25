// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package builtin

import (
	"embed"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"
)

// Afm returns the font metrics for one of the built-in pdf fonts.
func (f Font) Afm() (*type1.Font, error) {
	fd, err := afmData.Open("afm/" + string(f) + ".afm")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	res, err := afm.Read(fd)
	if err != nil {
		return nil, err
	}

	return res, nil
}

//go:embed afm/*.afm
var afmData embed.FS
