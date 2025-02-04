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

package opentypeglyphs

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/sfnt"
)

func Embed(w *pdf.Writer, tp glyphdata.Type, ref pdf.Reference, data *sfnt.Font) error {
	switch tp {
	case glyphdata.OpenTypeCFFSimple, glyphdata.OpenTypeCFF:
		if !data.IsCFF() {
			return glyphdata.ErrInvalidFont
		}
		if tp == glyphdata.OpenTypeCFFSimple && data.AsCFF().IsCIDKeyed() {
			return glyphdata.ErrInvalidFont
		}

		fontStmDict := pdf.Dict{
			"Subtype": pdf.Name("OpenType"),
		}
		fontStm, err := w.OpenStream(ref, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return fmt.Errorf("open OpenType/CFF stream: %w", err)
		}
		err = data.WriteOpenTypeCFFPDF(fontStm)
		if err != nil {
			return fmt.Errorf("write OpenType/CFF stream: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return fmt.Errorf("close OpenType/CFF stream: %w", err)
		}

	case glyphdata.TrueType:
		if !data.IsGlyf() {
			return glyphdata.ErrInvalidFont
		}

		length1 := pdf.NewPlaceholder(w, 10)
		fontStmDict := pdf.Dict{
			"Length1": length1,
		}
		fontStm, err := w.OpenStream(ref, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return fmt.Errorf("open TrueType stream: %w", err)
		}
		l1, err := data.WriteTrueTypePDF(fontStm)
		if err != nil {
			return fmt.Errorf("write TrueType stream: %w", err)
		}
		err = length1.Set(pdf.Integer(l1))
		if err != nil {
			return fmt.Errorf("TrueType stream: length1: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return fmt.Errorf("close TrueType stream: %w", err)
		}

	case glyphdata.OpenTypeGlyf:
		if !data.IsGlyf() {
			return glyphdata.ErrInvalidFont
		}

		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("OpenType"),
		}
		fontStm, err := w.OpenStream(ref, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return fmt.Errorf("open OpenType stream: %w", err)
		}
		_, err = data.WriteTrueTypePDF(fontStm)
		if err != nil {
			return fmt.Errorf("write OpenType stream: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return fmt.Errorf("close OpenType stream: %w", err)
		}

	default:
		return glyphdata.ErrWrongType
	}

	return nil
}

func Extract(r pdf.Getter, tp glyphdata.Type, ref pdf.Object) (*sfnt.Font, error) {
	stm, err := pdf.GetStream(r, ref)
	if err != nil {
		return nil, err
	} else if stm == nil {
		return nil, glyphdata.ErrNotFound
	}

	fontData, err := pdf.DecodeStream(r, stm, 0)
	if err != nil {
		return nil, err
	}

	data, err := sfnt.Read(fontData)
	if err != nil {
		return nil, err
	}

	switch tp {
	case glyphdata.OpenTypeCFF:
		if !data.IsCFF() {
			return nil, glyphdata.ErrInvalidFont
		}
	case glyphdata.OpenTypeCFFSimple:
		if !data.IsCFF() || data.AsCFF().IsCIDKeyed() {
			return nil, glyphdata.ErrInvalidFont
		}
	case glyphdata.TrueType, glyphdata.OpenTypeGlyf:
		if !data.IsGlyf() {
			return nil, glyphdata.ErrInvalidFont
		}
	default:
		return nil, glyphdata.ErrWrongType
	}

	return data, nil
}
