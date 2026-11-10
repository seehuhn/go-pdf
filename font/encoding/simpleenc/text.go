// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package simpleenc

import (
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/type1/names"
)

// ToUnicode returns the ToUnicode CMap for the codes allocated so far, or nil
// if every code means what its glyph name already implies.
//
// The glyph names are read against the font name given to [NewSimple], which is
// the same name [Simple.Encode] used: a code left out here because its text is
// implied must be one the implication holds for.
func (t *Simple) ToUnicode() *cmap.ToUnicodeFile {
	t.mu.RLock()
	defer t.mu.RUnlock()

	m := make(map[charcode.Code]string)
	for k, c := range t.code {
		glyphName := t.glyphName[k.gid]
		implied := names.ToUnicode(glyphName, t.fontName)
		if k.text != implied {
			m[charcode.Code(c)] = k.text
		}
	}

	if len(m) == 0 {
		return nil
	}

	tuInfo, _ := cmap.NewToUnicodeFile(charcode.Simple, m)
	return tuInfo
}
