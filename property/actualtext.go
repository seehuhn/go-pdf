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

package property

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 sections: 14.9

// ActualText represents an ActualText property list for marked content.
// This provides replacement text for content that should be used during
// text extraction, searching, and accessibility.
type ActualText struct {
	// MCID (optional) is the marked-content identifier for structure.
	MCID optional.UInt

	// Text is the replacement text string.
	Text string

	// SingleUse controls whether the property list is embedded directly
	// in the content stream (true) or as an indirect object via the
	// Properties resource dictionary (false).
	SingleUse bool
}

var _ List = (*ActualText)(nil)

// AsDirectDict returns the property list as a direct PDF dictionary
// if SingleUse is true.  Returns nil otherwise.
func (a *ActualText) AsDirectDict() pdf.Dict {
	if !a.SingleUse {
		return nil
	}
	dict := pdf.Dict{"ActualText": pdf.TextString(a.Text)}
	if mcid, ok := a.MCID.Get(); ok {
		dict["MCID"] = pdf.Integer(mcid)
	}
	return dict
}

// Equal reports whether two property lists are semantically equal.
func (a *ActualText) Equal(other List) bool {
	b, ok := other.(*ActualText)
	if !ok {
		return false
	}
	return a.Text == b.Text && a.MCID == b.MCID && a.SingleUse == b.SingleUse
}

// ExtractActualText extracts an ActualText property list from a PDF object.
// The dictionary must contain an ActualText key; otherwise an error is returned.
func ExtractActualText(c pdf.Cursor, obj pdf.Object, isDirect bool) (*ActualText, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}

	atObj, hasActualText := dict["ActualText"]
	if !hasActualText {
		return nil, errNoActualText
	}

	a := &ActualText{
		SingleUse: isDirect,
	}

	if s, ok := atObj.(pdf.String); ok {
		a.Text = string(s.AsTextString())
	}

	if mcid, ok := dict["MCID"].(pdf.Integer); ok && mcid >= 0 {
		a.MCID.Set(uint(mcid))
	}

	return a, nil
}

var errNoActualText = &pdf.MalformedFileError{
	Err: errors.New("not an ActualText property list"),
}

// Embed writes the property list to the PDF file.
// This implements the [pdf.Embedder] interface.
func (a *ActualText) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	dict := make(pdf.Dict)

	if mcid, ok := a.MCID.Get(); ok {
		dict["MCID"] = pdf.Integer(mcid)
	}

	dict["ActualText"] = pdf.TextString(a.Text)

	if a.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
