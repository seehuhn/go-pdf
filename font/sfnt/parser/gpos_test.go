// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package parser

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt"
)

func TestGpos(t *testing.T) {
	tt, err := sfnt.Open("../../truetype/ttf/SourceSerif4-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer tt.Close()

	pars := New(tt)
	info, err := pars.readGposInfo("latn", "ENG ")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(info)

	t.Error("fish")
}
