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

package xobject

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/image"
)

func Extract(x *pdf.Extractor, obj pdf.Object) (graphics.XObject, error) {
	stm, err := pdf.GetStream(x.R, obj)
	if err != nil {
		return nil, err
	}
	err = pdf.CheckDictType(x.R, stm.Dict, "XObject")
	if err != nil {
		return nil, err
	}

	subtype, err := pdf.GetName(x.R, stm.Dict["Subtype"])
	if err != nil {
		return nil, err
	}

	switch subtype {
	case "Image":
		img, err := image.ExtractDict(x, stm)
		return img, err
	case "Form":
		form, err := form.Extract(x, stm)
		return form, err
	case "PS":
		ps, err := extractPostScript(x, stm)
		return ps, err
	default:
		return nil, pdf.Errorf("unsupported XObject Subtype %q", subtype)
	}
}
