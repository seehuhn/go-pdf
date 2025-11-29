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
	"time"

	"golang.org/x/text/language"
)

func TestStructEncodeTextString(t *testing.T) {
	for _, testText := range []TextString{"", "hello", "Grüß Gott", "こんにちは"} {
		type testStructType struct {
			S TextString
		}
		testStructVal := &testStructType{S: testText}
		testDict := AsDict(testStructVal)

		if testDict["S"] != testText {
			t.Errorf("wrong string: %q != %q", testDict["S"], testText)
		}
	}
}

func TestStructDecodeTextString(t *testing.T) {
	type testCase struct {
		name string
		in   Object
		out  TextString
	}
	cases := []testCase{
		{"ASCII TextString", TextString("hello"), TextString("hello")},
		{"latin1 TextString", TextString("Grüß Gott"), TextString("Grüß Gott")},
		{"empty TextString", TextString(""), TextString("")},
		{"ASCII String", pdfDocEncodeMust("hello"), TextString("hello")},
		{"latin1 String", pdfDocEncodeMust("Grüß Gott"), TextString("Grüß Gott")},
		{"empty String", String(""), TextString("")},
		{"nil", nil, TextString("")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testDict := Dict{"S": c.in}

			type testStructType struct {
				S TextString `pdf:"optional"`
			}
			var testStructVal testStructType
			err := DecodeDict(nil, &testStructVal, testDict)
			if err != nil {
				t.Fatal(err)
			}
			if testStructVal.S != c.out {
				t.Errorf("wrong text string: %s", testStructVal.S)
			}
		})
	}
}

func TestStructEncodeDate(t *testing.T) {
	for _, timeVal := range []time.Time{
		time.Now().Local(),
		time.Now().UTC(),
		time.Date(2021, 1, 2, 3, 4, 5, 0, time.FixedZone("CET", 3600)),
		time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		testDate := Date(timeVal)
		type testStructType struct {
			T Date
		}
		testStructVal := &testStructType{T: testDate}
		testDict := AsDict(testStructVal)

		if testDict["T"] != testDate {
			t.Errorf("wrong date: %q != %q", testDict["S"], testDate)
		}
	}
}

func TestStructDecodeDate(t *testing.T) {
	type testCase struct {
		name string
		in   Object
		out  time.Time
	}
	now := time.Now().Round(time.Second)
	nowDate := Date(now)
	cases := []testCase{
		{"Date", nowDate, now},
		{"String", String("D:199812231952-08'00"), time.Date(1998, 12, 23, 19, 52, 0, 0, time.FixedZone("UTC-8", -8*60*60))},
		{"TextString", TextString("D:199812231952-08'00"), time.Date(1998, 12, 23, 19, 52, 0, 0, time.FixedZone("UTC-8", -8*60*60))},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testDict := Dict{"T": c.in}

			type testStructType struct {
				T Date `pdf:"optional"`
			}
			var testStructVal testStructType
			err := DecodeDict(nil, &testStructVal, testDict)
			if err != nil {
				t.Fatal(err)
			}

			got := time.Time(testStructVal.T)
			if !got.Equal(c.out) {
				t.Errorf("wrong date: %s -> %s -> %s",
					Date(c.out), AsString(c.in), testStructVal.T)
			}
		})
	}
}

func TestStructEncodeLanguageTag(t *testing.T) {
	type testStruct struct {
		Lang language.Tag
	}

	s2 := &testStruct{
		Lang: language.BrazilianPortuguese,
	}
	d2 := AsDict(s2)
	if s, ok := d2["Lang"].(asTextStringer); !ok || s.AsTextString() != "pt-BR" {
		t.Errorf("wrong language: %s != %s", s.AsTextString(), "pt-BR")
	}
}

func TestStructDecodeLanguageTag(t *testing.T) {
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

func TestStructEmptyLanguageTag(t *testing.T) {
	type testStruct struct {
		Lang language.Tag
	}

	s3 := &testStruct{}
	d3 := AsDict(s3)
	if _, present := d3["Lang"]; present {
		t.Errorf("empty language tag not ignored, got %q", d3["Lang"])
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
	aDict = Dict{"V": Integer(17)}
	err = DecodeDict(nil, res, aDict)
	if err == nil {
		t.Errorf("invalid type not detected")
	}
}

func TestDecodeVersion(t *testing.T) {
	// We support various ways to specify the version number.
	// The first one (Name) is correct as per the PDF spec, the others are
	// invalid cases which can be found in the wild.
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

func pdfDocEncodeMust(s string) String {
	res, ok := PDFDocEncode(s)
	if !ok {
		panic("encoding failed")
	}
	return res
}
