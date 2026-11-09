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

package cmap

// IdentityH and IdentityV are the predefined Identity-H and Identity-V
// CMaps, which map two-byte codes to the CID of the same value.  They
// must not be modified.
var (
	IdentityH = mustPredefined("Identity-H")
	IdentityV = mustPredefined("Identity-V")
)

func mustPredefined(name string) *File {
	c, err := Predefined(name)
	if err != nil {
		panic(err)
	}
	return c
}
