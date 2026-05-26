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

package media

import (
	"seehuhn.de/go/pdf"
)

// LangText is a single language-tagged string in a [MultiLangText] array.
type LangText struct {
	// Lang is a language identifier.  An empty value denotes the default
	// text used when no language matches.
	Lang string

	// Text is the text string in the given language.
	Text string
}

// MultiLangText provides text in multiple languages, used to give alternative
// descriptions for media that cannot be played and title-bar text for floating
// windows.
type MultiLangText []LangText

// extractMultiLangText reads a multi-language text array.  Pairs consist of a
// language identifier followed by the corresponding text.  A trailing
// unpaired element is ignored.
func extractMultiLangText(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) (MultiLangText, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil {
		return nil, err
	}
	if len(arr) < 2 {
		return nil, nil
	}
	var out MultiLangText
	for i := 0; i+1 < len(arr); i += 2 {
		lang, err := pdf.Optional(pdf.GetTextString(x.R, arr[i]))
		if err != nil {
			return nil, err
		}
		text, err := pdf.Optional(pdf.GetTextString(x.R, arr[i+1]))
		if err != nil {
			return nil, err
		}
		out = append(out, LangText{Lang: string(lang), Text: string(text)})
	}
	return out, nil
}

// toArray converts the multi-language text to a PDF array.
func (m MultiLangText) toArray() pdf.Array {
	arr := make(pdf.Array, 0, 2*len(m))
	for _, lt := range m {
		arr = append(arr, pdf.TextString(lt.Lang), pdf.TextString(lt.Text))
	}
	return arr
}
