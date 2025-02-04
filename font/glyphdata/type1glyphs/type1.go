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

package type1glyphs

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/postscript/type1"
)

func Embed(w *pdf.Writer, tp glyphdata.Type, ref pdf.Reference, data *type1.Font) error {
	if tp != glyphdata.Type1 {
		return glyphdata.ErrWrongType
	}

	length1 := pdf.NewPlaceholder(w, 10)
	length2 := pdf.NewPlaceholder(w, 10)
	fontStmDict := pdf.Dict{
		"Length1": length1,
		"Length2": length2,
		"Length3": pdf.Integer(0),
	}
	fontStm, err := w.OpenStream(ref, fontStmDict, pdf.FilterCompress{})
	if err != nil {
		return fmt.Errorf("open Type1 stream: %w", err)
	}
	l1, l2, err := data.WritePDF(fontStm)
	if err != nil {
		return fmt.Errorf("write Type1 stream: %w", err)
	}
	err = length1.Set(pdf.Integer(l1))
	if err != nil {
		return fmt.Errorf("Type1 stream: length1: %w", err)
	}
	err = length2.Set(pdf.Integer(l2))
	if err != nil {
		return fmt.Errorf("Type1 stream: length2: %w", err)
	}
	err = fontStm.Close()
	if err != nil {
		return fmt.Errorf("close Type1 stream: %w", err)
	}

	return nil
}

func Extract(r pdf.Getter, tp glyphdata.Type, ref pdf.Object) (*type1.Font, error) {
	if tp != glyphdata.Type1 {
		return nil, glyphdata.ErrWrongType
	}

	stm, err := pdf.GetStream(r, ref)
	if err != nil {
		return nil, err
	} else if stm == nil {
		return nil, glyphdata.ErrNotFound
	}

	body, err := pdf.DecodeStream(r, stm, 0)
	if err != nil {
		return nil, err
	}

	return type1.Read(body)
}
