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

package gtab_test

import (
	"testing"

	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/gtab"
	"seehuhn.de/go/pdf/locale"
)

func TestGpos(t *testing.T) {
	tt, err := sfnt.Open("../../truetype/ttf/SourceSerif4-Regular.ttf", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tt.Close()

	pars, err := gtab.New(tt.Header, tt.Fd, locale.EnGB)
	if err != nil {
		t.Fatal(err)
	}
	info, err := pars.ReadGposTable()
	if err != nil {
		t.Fatal(err)
	}
	_ = info // TODO(voss): do some checks
}
