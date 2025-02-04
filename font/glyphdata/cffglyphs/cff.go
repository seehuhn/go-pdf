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

package cffglyphs

import (
	"bytes"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/sfnt/cff"
)

func Embed(w *pdf.Writer, tp glyphdata.Type, ref pdf.Reference, data *cff.Font) error {
	switch tp {
	case glyphdata.CFFSimple:
		if data.IsCIDKeyed() {
			return glyphdata.ErrInvalidFont
		}

		fontStmDict := pdf.Dict{
			"Subtype": pdf.Name("Type1C"),
		}
		fontStm, err := w.OpenStream(ref, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return fmt.Errorf("open CFF stream: %w", err)
		}
		err = data.Write(fontStm)
		if err != nil {
			return fmt.Errorf("write CFF stream: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return fmt.Errorf("close CFF stream: %w", err)
		}

	case glyphdata.CFF:
		fontStmDict := pdf.Dict{
			"Subtype": pdf.Name("CIDFontType0C"),
		}
		fontFileStream, err := w.OpenStream(ref, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return fmt.Errorf("open CFF stream: %w", err)
		}
		err = data.Write(fontFileStream)
		if err != nil {
			return fmt.Errorf("write CFF stream: %w", err)
		}
		err = fontFileStream.Close()
		if err != nil {
			return fmt.Errorf("close CFF stream: %w", err)
		}

	default:
		return glyphdata.ErrWrongType
	}

	return nil
}

func Extract(r pdf.Getter, tp glyphdata.Type, ref pdf.Object) (*cff.Font, error) {
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
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	data, err := cff.Read(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	switch tp {
	case glyphdata.CFFSimple:
		if data.IsCIDKeyed() {
			return nil, glyphdata.ErrInvalidFont
		}
	case glyphdata.CFF:
		// pass
	default:
		return nil, glyphdata.ErrWrongType
	}
	return data, nil
}
