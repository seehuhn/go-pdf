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

package pdf

import (
	"testing"
)

func TestCatalog(t *testing.T) {
	pRef := &Reference{Number: 1, Generation: 2}

	// test a round-trip
	cat0 := &Catalog{
		Pages: pRef,
	}
	d1 := Struct(cat0)
	if len(d1) != 2 {
		t.Errorf("wrong Catalog dict: %s", format(d1))
	}
	cat1 := &Catalog{}
	d1.AsStruct(cat1, nil)
	if *cat0 != *cat1 {
		t.Errorf("Catalog wrongly decoded: %v", cat1)
	}
}

func TestInfo(t *testing.T) {
	// test missing struct
	var info0 *Info
	d1 := Struct(info0)
	if d1 != nil {
		t.Error("wrong dict for nil Info struct")
	}

	// test empty struct
	info0 = &Info{}
	d1 = Struct(info0)
	if d1 == nil || len(d1) != 0 {
		t.Errorf("wrong dict for empty Info struct: %#v", d1)
	}

	// test regular fields
	info0.Author = "Jochen Voß"
	d1 = Struct(info0)
	if len(d1) != 1 {
		t.Errorf("wrong dict for empty Info struct: %s", format(d1))
	}
	info1 := &Info{}
	d1.AsStruct(info1, nil)
	if info0.Author != info1.Author || info0.CreationDate != info1.CreationDate {
		t.Errorf("Catalog wrongly decoded: %v", info1)
	}

	// test custom fields
	d1 = Dict{
		"grumpy": TextString("bärbeißig"),
		"funny":  TextString("\000\001\002 \\<>'\")("),
	}
	d1.AsStruct(info1, nil)
	d2 := Struct(info1)
	if len(d1) != len(d2) {
		t.Errorf("wrong d2: %s", format(d2))
	}
	for key, val := range d1 {
		if d2[key].(String).AsTextString() != val.(String).AsTextString() {
			t.Errorf("wrong d2[%s]: %s", key, format(d2[key]))
		}
	}
}
