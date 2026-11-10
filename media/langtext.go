// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
	Text pdf.TextString
}

// MultiLangText provides text in multiple languages, used to give alternative
// descriptions for media that cannot be played and title-bar text for floating
// windows.
type MultiLangText []LangText

// extractMultiLangText reads a multi-language text array.  Pairs consist of a
// language identifier followed by the corresponding text.  A trailing
// unpaired element is ignored.
func extractMultiLangText(c pdf.Cursor, obj pdf.Object) (MultiLangText, error) {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil {
		return nil, err
	}
	if len(arr) < 2 {
		return nil, nil
	}
	var out MultiLangText
	for i := 0; i+1 < len(arr); i += 2 {
		lang, err := pdf.Optional(c.TextString(arr[i]))
		if err != nil {
			return nil, err
		}
		text, err := pdf.Optional(c.TextString(arr[i+1]))
		if err != nil {
			return nil, err
		}
		out = append(out, LangText{Lang: lang.String(), Text: text})
	}
	return out, nil
}

// toArray converts the multi-language text to a PDF array.
func (m MultiLangText) toArray() pdf.Array {
	arr := make(pdf.Array, 0, 2*len(m))
	for _, lt := range m {
		arr = append(arr, pdf.Text(lt.Lang), lt.Text)
	}
	return arr
}
