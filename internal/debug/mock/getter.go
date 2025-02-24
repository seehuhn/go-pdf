// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package mock

import "seehuhn.de/go/pdf"

// Getter is a [pdf.Getter] which returns no objects.
var Getter = getter{}

type getter struct{}

func (r getter) GetMeta() *pdf.MetaInfo {
	m := &pdf.MetaInfo{
		Version: pdf.V2_0,
	}
	return m
}

func (r getter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	return nil, nil
}
