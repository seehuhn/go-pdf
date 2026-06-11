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

package acroform

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.4.3

// VariableText holds the entries shared by fields whose appearance is generated
// from a text value: the default appearance, justification, and rich-text
// attributes. Terminal field types embed it.
type VariableText struct {
	// DefaultAppearance (optional) is the default appearance string used in
	// formatting the field's variable text. It is a content stream fragment
	// establishing the text font, size, and colour; at a minimum it sets the
	// font and size with a Tf operator. A font size of zero requests automatic
	// sizing. An empty value indicates that the field has no default appearance
	// string.
	//
	// This corresponds to the /DA entry.
	DefaultAppearance string

	// Align specifies the justification of the field's variable text. The zero
	// value is [pdf.TextAlignLeft].
	//
	// This corresponds to the /Q entry.
	Align pdf.TextAlign

	// DefaultStyle (optional) is a rich-text default style string. It applies
	// only to fields with the [FieldRichText] flag set.
	//
	// This corresponds to the /DS entry.
	DefaultStyle string

	// RichValue (optional) is a rich-text value, a stream or text string. The
	// library treats this value as opaque.
	//
	// This corresponds to the /RV entry.
	RichValue pdf.Object
}

// GetVariableText returns the variable-text attributes.
func (v *VariableText) GetVariableText() *VariableText { return v }

// VariableTextField is the interface satisfied by terminal field types that
// carry variable-text attributes — the default appearance, justification, and
// rich-text entries. It lets these attributes be read from a [Field] without a
// type switch on the concrete field type:
//
//	if vt, ok := f.(acroform.VariableTextField); ok {
//		da := vt.GetVariableText().DefaultAppearance
//	}
type VariableTextField interface {
	Field
	GetVariableText() *VariableText
}

// fillDict adds the VariableText entries to the given PDF dictionary.
func (v *VariableText) fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	w := rm.Out

	if v.DefaultAppearance != "" {
		dict["DA"] = pdf.String(v.DefaultAppearance)
	}

	if v.Align != pdf.TextAlignLeft {
		if v.Align < pdf.TextAlignLeft || v.Align > pdf.TextAlignRight {
			return fmt.Errorf("invalid text alignment %d", v.Align)
		}
		dict["Q"] = pdf.Integer(v.Align)
	}

	if v.DefaultStyle != "" {
		if err := pdf.CheckVersion(w, "field DS entry", pdf.V1_5); err != nil {
			return err
		}
		dict["DS"] = pdf.TextString(v.DefaultStyle)
	}

	if v.RichValue != nil {
		if err := pdf.CheckVersion(w, "field RV entry", pdf.V1_5); err != nil {
			return err
		}
		dict["RV"] = v.RichValue
	}

	return nil
}
