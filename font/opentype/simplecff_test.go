// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package opentype_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/sfnt"
)

func TestEmbedSimple(t *testing.T) {
	// step 1: embed a font instance into a simple PDF file
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	fontData := makefont.OpenType()
	fontInstance, err := opentype.New(fontData, nil)
	if err != nil {
		t.Fatal(err)
	}

	ref, _, err := pdf.ResourceManagerEmbed(rm, fontInstance)
	if err != nil {
		t.Fatal(err)
	}

	// make sure a few glyphs are included and encoded
	fontInstance.Layout(nil, 12, "Hello")

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// step 2: read back the font and verify that everything is as expected
	dict, err := type1.ExtractDict(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	if dict.Ref != ref {
		t.Errorf("wrong object reference: expected %v, got %v", ref, dict.Ref)
	}
	if dict.PostScriptName != fontData.PostScriptName() {
		t.Errorf("wrong PostScript name: expected %v, got %v",
			fontData.PostScriptName(), dict.PostScriptName)
	}
	if len(dict.SubsetTag) != 6 {
		t.Errorf("wrong subset tag: %q", dict.SubsetTag)
	}

	// TODO(voss): more tests

	if dict.GetFont == nil {
		t.Fatal("GetFont method is nil")
	}
	F, err := dict.GetFont()
	if err != nil {
		t.Fatal(err)
	} else if F == nil {
		t.Fatal("GetFont method returned nil")
	}
	_, ok := F.(*sfnt.Font)
	if !ok {
		t.Errorf("wrong font type: %T", F)
	}
}
