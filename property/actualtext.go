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
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 sections: 14.9

// ActualText represents an ActualText property list for marked content.
// This provides replacement text for content that should be used during
// text extraction, searching, and accessibility.
//
// PDF 2.0 section 14.9
type ActualText struct {
	// MCID (optional) is the marked-content identifier for structure.
	MCID optional.UInt

	// Text is the replacement text string.
	Text string

	// SingleUse controls whether the property list is embedded as a direct
	// object in the Properties resource dictionary (true) or as an indirect
	// object (false).
	SingleUse bool
}

var _ List = (*ActualText)(nil)

func (a *ActualText) Keys() []pdf.Name {
	var keys []pdf.Name
	keys = append(keys, "ActualText")
	if _, ok := a.MCID.Get(); ok {
		keys = append(keys, "MCID")
	}
	return keys
}

func (a *ActualText) Get(key pdf.Name) (*ResolvedObject, error) {
	switch key {
	case "MCID":
		if v, ok := a.MCID.Get(); ok {
			return &ResolvedObject{obj: pdf.Integer(v), x: nil}, nil
		}
	case "ActualText":
		return &ResolvedObject{obj: pdf.TextString(a.Text), x: nil}, nil
	}
	return nil, ErrNoKey
}

func (a *ActualText) IsDirect() bool {
	return a.SingleUse
}

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

// ExtractActualText extracts an ActualText property list from a PDF object.
func ExtractActualText(x *pdf.Extractor, obj pdf.Object) (*ActualText, error) {
	dict, err := x.GetDictTyped(obj, "")
	if err != nil {
		return nil, err
	}

	result := &ActualText{}

	if mcid, ok := dict["MCID"].(pdf.Integer); ok && mcid >= 0 && uint64(mcid) <= math.MaxUint {
		result.MCID.Set(uint(mcid))
	}

	if actualTextObj, ok := dict["ActualText"]; ok {
		text, err := pdf.Optional(pdf.GetTextString(x.R, actualTextObj))
		if err != nil {
			return nil, err
		}
		result.Text = string(text)
	}

	return result, nil
}
