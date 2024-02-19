// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

	"golang.org/x/text/language"
)

func TestCatalog(t *testing.T) {
	pRef := NewReference(1, 2)

	// test a round-trip
	cat0 := &Catalog{
		Pages: pRef,
	}
	d1 := AsDict(cat0)
	if len(d1) != 2 {
		t.Errorf("wrong Catalog dict: %s", Format(d1))
	}
	cat1 := &Catalog{}
	err := DecodeDict(nil, cat1, d1)
	if err != nil {
		t.Error(err)
	} else if *cat0 != *cat1 {
		t.Errorf("Catalog wrongly decoded: %v", cat1)
	}
}

func TestCatalogReadMissingPages(t *testing.T) {
	ref := NewReference(123, 0)
	catalogDict := Dict{
		"MetaData": ref,
	}
	catalog := &Catalog{}
	err := DecodeDict(nil, catalog, catalogDict)
	if err == nil {
		t.Errorf("missing Pages not detected")
	}
	if catalog.MetaData != ref {
		t.Errorf("wrong MetaData: %v", catalog.MetaData)
	}
}

func TestCatalogWriteMissingPages(t *testing.T) {
	catalog := &Catalog{}
	dict := AsDict(catalog)
	if _, present := dict["Pages"]; present {
		t.Errorf("missing Pages not ignored")
	}
}

func TestInfo(t *testing.T) {
	// test missing struct
	var info0 *Info
	d1 := AsDict(info0)
	if d1 != nil {
		t.Error("wrong dict for nil Info struct")
	}

	// test empty struct
	info0 = &Info{}
	d1 = AsDict(info0)
	if d1 == nil || len(d1) != 0 {
		t.Errorf("wrong dict for empty Info struct: %#v", d1)
	}

	// test regular fields
	info0.Author = "Jochen Voß"
	d1 = AsDict(info0)
	if len(d1) != 1 {
		t.Errorf("wrong dict for empty Info struct: %s", Format(d1))
	}
	info1 := &Info{}
	err := DecodeDict(nil, info1, d1)
	if err != nil {
		t.Error(err)
	} else if info0.Author != info1.Author || info0.CreationDate != info1.CreationDate {
		t.Errorf("Catalog wrongly decoded: %v", info1)
	}

	// test custom fields
	d1 = Dict{
		"grumpy": TextString("bärbeißig"),
		"funny":  TextString("\000\001\002 \\<>'\")("),
	}
	err = DecodeDict(nil, info1, d1)
	if err != nil {
		t.Error(err)
	}
	d2 := AsDict(info1)
	if len(d1) != len(d2) {
		t.Errorf("wrong d2: %s", Format(d2))
	}
	for key, val := range d1 {
		if d2[key].(String).AsTextString() != val.(String).AsTextString() {
			t.Errorf("wrong d2[%s]: %s", key, Format(d2[key]))
		}
	}
}

func TestStructVersion(t *testing.T) {
	type testStruct struct {
		Dummy Object `pdf:"optional"`
		V     Version
		Other Version `pdf:"optional"`
	}

	res := &testStruct{}

	// test normal operation
	for v := V1_0; v < tooHighVersion; v++ {
		a := &testStruct{V: v}
		aDict := AsDict(a)
		err := DecodeDict(nil, res, aDict)
		if err != nil {
			t.Error(err)
		}
		if a.V != res.V {
			t.Errorf("wrong version: %s != %s", a.V, res.V)
		}
	}

	// test that invalid versions are ignored
	a := &testStruct{V: tooHighVersion}
	aDict := AsDict(a)
	val, present := aDict["V"]
	if present {
		t.Errorf("expected null, got %v", val)
	}

	// test that missing versions in a Dict are detected
	aDict = Dict{}
	err := DecodeDict(nil, res, aDict)
	if err == nil {
		t.Errorf("missing version not detected")
	}

	// test that invalid versions in a Dict are detected
	aDict = Dict{"V": Name("9.9")}
	err = DecodeDict(nil, res, aDict)
	if err == nil {
		t.Errorf("invalid version not detected")
	}
	aDict = Dict{"V": Number(1.7)}
	err = DecodeDict(nil, res, aDict)
	if err == nil {
		t.Errorf("invalid type not detected")
	}
}

func TestDecodeVersion(t *testing.T) {
	for _, version := range []Object{Name("1.5"), Real(1.5), String("1.5")} {
		res := &Catalog{}
		dict := Dict{"Version": version, "Pages": Reference(0)}
		err := DecodeDict(nil, res, dict)
		if err != nil {
			t.Error(err)
			continue
		}
		if res.Version != V1_5 {
			t.Errorf("wrong version: %s", res.Version)
		}
	}
}

func TestDecodeLanguageTag(t *testing.T) {
	type testStruct struct {
		Lang language.Tag
	}

	d1 := Dict{"Lang": TextString("en-GB")}
	s1 := &testStruct{}
	err := DecodeDict(nil, s1, d1)
	if err != nil {
		t.Error(err)
	}
	if s1.Lang != language.BritishEnglish {
		t.Errorf("wrong language: %s", s1.Lang)
	}
}

func TestEncodeLanguageTag(t *testing.T) {
	type testStruct struct {
		Lang language.Tag
	}

	s2 := &testStruct{
		Lang: language.BrazilianPortuguese,
	}
	d2 := AsDict(s2)
	if s, ok := d2["Lang"].(String); !ok || s.AsTextString() != "pt-BR" {
		t.Errorf("wrong language: %s", d2["Lang"])
	}
}

func TestEmptyLanguageTag(t *testing.T) {
	type testStruct struct {
		Lang language.Tag
	}

	s3 := &testStruct{}
	d3 := AsDict(s3)
	if _, present := d3["Lang"]; present {
		t.Errorf("empty language tag not ignored, got %q", d3["Lang"])
	}
}
