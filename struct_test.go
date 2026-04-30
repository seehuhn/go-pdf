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

	// test that AsDict round-trips every valid Version
	for v := V1_0; v <= MaxVersion; v++ {
		a := &testStruct{V: v}
		aDict := AsDict(a)
		got, present := aDict["V"]
		if !present {
			t.Errorf("V missing from dict for version %s", v)
			continue
		}
		want, _ := v.ToString()
		if got != Name(want) {
			t.Errorf("version %s: got %v, want %s", v, got, want)
		}
	}

	// test that invalid versions are ignored by AsDict
	a := &testStruct{V: MaxVersion + 1}
	aDict := AsDict(a)
	if val, present := aDict["V"]; present {
		t.Errorf("expected null, got %v", val)
	}
}
