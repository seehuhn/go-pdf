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

// Keys returns the dictionary keys used by this property list.
// This implements the [List] interface.
func (a *ActualText) Keys() []pdf.Name {
	var keys []pdf.Name
	keys = append(keys, "ActualText")
	if _, ok := a.MCID.Get(); ok {
		keys = append(keys, "MCID")
	}
	return keys
}

// Get retrieves the value for a given key.
// This implements the [List] interface.
func (a *ActualText) Get(key pdf.Name) (pdf.Object, error) {
	switch key {
	case "MCID":
		if v, ok := a.MCID.Get(); ok {
			return pdf.Integer(v), nil
		}
	case "ActualText":
		return pdf.TextString(a.Text), nil
	}
	return nil, ErrNoKey
}

// IsDirect reports whether this property list can be embedded directly
// into a content stream.
// This implements the [List] interface.
func (a *ActualText) IsDirect() bool {
	return a.SingleUse
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

	ref := rm.AllocSelf()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
