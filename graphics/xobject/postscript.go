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
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// postscript represents the long-deprecated PostScript XObject.
type postScript struct {
	WriteTo func(w io.Writer) error
}

var _ graphics.XObject = (*postScript)(nil)

func extractPostScript(x *pdf.Extractor, stm *pdf.Stream) (*postScript, error) {
	err := pdf.CheckDictType(x.R, stm.Dict, "XObject")
	if err != nil {
		return nil, err
	}

	subtype, err := pdf.GetName(x.R, stm.Dict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "PS" {
		return nil, fmt.Errorf("unexpected Subtype %q != PS", subtype)
	}

	draw := func(w io.Writer) error {
		r, err := pdf.GetStreamReader(x.R, stm)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		if err != nil {
			r.Close()
			return err
		}
		return r.Close()
	}
	return &postScript{WriteTo: draw}, nil
}

func (ps *postScript) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"Type":    pdf.Name("XObject"),
		"Subtype": pdf.Name("PS"),
	}
	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}
	err = ps.WriteTo(stm)
	if err != nil {
		stm.Close()
		return nil, zero, err
	}
	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

func (ps *postScript) Subtype() pdf.Name {
	return "PS"
}
