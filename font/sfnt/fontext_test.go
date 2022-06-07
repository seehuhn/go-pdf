// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package sfnt_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/debug"
)

func TestGetFontInfo(t *testing.T) {
	font := debug.MakeSimpleFont()
	font.Trademark = "test trademark notice"
	font.Copyright = "(c) 2022 test copyright notice"

	fontInfo1 := font.GetFontInfo()

	cffFont1 := &cff.Font{
		FontInfo: fontInfo1,
		Outlines: font.Outlines.(*cff.Outlines),
	}
	buf := &bytes.Buffer{}
	err := cffFont1.Encode(buf)
	if err != nil {
		t.Fatal(err)
	}
	cffData := buf.Bytes()

	cffFont2, err := cff.Read(bytes.NewReader(cffData))
	if err != nil {
		t.Fatal(err)
	}
	fontInfo2 := cffFont2.FontInfo

	if d := cmp.Diff(fontInfo1, fontInfo2); d != "" {
		t.Errorf("font info differs: (-got +want)\n%s", d)
	}
}
