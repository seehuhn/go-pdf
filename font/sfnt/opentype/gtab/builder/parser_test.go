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

package builder

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/font/debug"
)

func TestParser(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()
	lookups, err := parse(fontInfo, `
	GSUB_1: A->B, M->N
	GSUB_1: A-C -> B-D, M->N, N->O
	GSUB_1: A->X, B->X, C->X, M->X, N->X
	GSUB_2: A -> "AA", B -> "AA", C -> "ABAAC"
	GSUB_3: A -> "BCD"
	GSUB_4: -marks A A -> A
	`)
	if err != nil {
		t.Fatal(err)
	}

	explain := ExplainGsub(fontInfo, lookups)
	fmt.Println(explain)

	// t.Error("fish")
}
