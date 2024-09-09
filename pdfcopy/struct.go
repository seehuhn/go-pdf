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

package pdfcopy

import "seehuhn.de/go/pdf"

// CopyStruct copies a struct from the source file to the target file.
// For this to work, `data` must be a pointer to a struct, and must be
// a valid argument to [pdf.AsDict].  Otherwise, the function panics.
//
// Once Go supports methods with type parameters, this function can be turned
// into a method on [Copier].
func CopyStruct[T any](c *Copier, data *T) (*T, error) {
	oldDict := pdf.AsDict(data)
	newDict, err := c.CopyDict(oldDict)
	if err != nil {
		return nil, err
	}
	newData := new(T)
	err = pdf.DecodeDict(c.r, newData, newDict)
	if err != nil {
		return nil, err
	}
	return newData, nil
}
