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

package pdf

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestTextString_Get(t *testing.T) {
	tests := []struct {
		name    string
		input   Object
		want    TextString
		wantErr bool
	}{
		{
			name:  "PDFDocEncoded string",
			input: String("Hello, World!"),
			want:  "Hello, World!",
		},
		{
			name:  "UTF-16BE string",
			input: String("\xFE\xFF\x00H\x00e\x00l\x00l\x00o"),
			want:  "Hello",
		},
		{
			name:  "UTF-8 string",
			input: String("\xEF\xBB\xBFHello"),
			want:  "Hello",
		},
		{
			name:  "Empty string",
			input: String(""),
			want:  "",
		},
		{
			name:  "String with special characters",
			input: String("Line 1\nLine 2\tTabbed"),
			want:  "Line 1\nLine 2\tTabbed",
		},
		{
			name:    "Invalid object type",
			input:   Integer(42),
			wantErr: true,
		},
		{
			name:  "Nil object",
			input: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTextString(nil, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetTextString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("GetTextString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTextString_AsPDF(t *testing.T) {
	tests := []struct {
		name string
		in   TextString
		opt  OutputOptions
		want Native
	}{
		{
			name: "ASCII string",
			in:   "hello",
			opt:  0,
			want: String("hello"),
		},
		{
			name: "Non-PDFDocEncoding char",
			in:   "Ā",
			opt:  0,
			want: String("\xfe\xff\x01\x00"),
		},
		{
			name: "Chinese characters",
			in:   "中文",
			opt:  0,
			want: String("\xfe\xff\x4e\x2d\x65\x87"),
		},
		{
			name: "ASCII with UTF-8", // still encoded as PDFDocEncoding, though
			in:   "hello",
			opt:  OptTextStringUtf8,
			want: String("hello"),
		},
		{
			name: "Non-PDFDocEncoding with UTF-8",
			in:   "Ā",
			opt:  OptTextStringUtf8,
			want: String("\xef\xbb\xbfĀ"),
		},
		{
			name: "Chinese with UTF-8",
			in:   "中文",
			opt:  OptTextStringUtf8,
			want: String("\xef\xbb\xbf中文"),
		},
		{
			name: "Empty string",
			in:   "",
			opt:  0,
			want: String(""),
		},
		{
			name: "String with control characters",
			in:   "\x00\x01\x02",
			opt:  0,
			want: String("\xfe\xff\x00\x00\x00\x01\x00\x02"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.AsPDF(tt.opt)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AsPDF(%q, %v) = %v, want %v", tt.in, tt.opt, got, tt.want)
			}
		})
	}
}

func TestTextString_Roundtrip(t *testing.T) {
	tests := []struct {
		name string
		text TextString
	}{
		{
			name: "Empty string",
			text: "",
		},
		{
			name: "ASCII string",
			text: "hello",
		},
		{
			name: "String with umlauts",
			text: "ein Bär",
		},
		{
			name: "Romanian string",
			text: "o țesătură",
		},
		{
			name: "Chinese string",
			text: "中文",
		},
		{
			name: "Japanese string",
			text: "日本語",
		},
		{
			name: "Control characters",
			text: "\x00\x09\n\x0c\r",
		},
		{
			name: "Mixed ASCII and non-ASCII",
			text: "Hello, 世界!",
		},
		{
			name: "Emoji",
			text: "😀🌍🚀",
		},
		{
			name: "utf8 marker",
			text: TextString(pdfDocDecode(utf8Marker)),
		},
		{
			name: "utf16 marker",
			text: TextString(pdfDocDecode(utf16Marker)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, opt := range []OutputOptions{0, OptTextStringUtf8} {
				enc := tt.text.AsPDF(opt).(String)
				out := enc.AsTextString()
				if out != tt.text {
					t.Errorf("Roundtrip failed for %q with option %v:\nencoded: % x\ndecoded: %q",
						tt.text, opt, enc, out)
				}
			}
		})
	}
}

func TestDateString(t *testing.T) {
	PST := time.FixedZone("PST", -8*60*60)
	cases := []time.Time{
		time.Date(1998, 12, 23, 19, 52, 0, 0, PST),
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2020, 12, 24, 16, 30, 12, 0, time.FixedZone("", 90*60)),
	}
	for _, t1 := range cases {
		enc := Date(t1).AsPDF(0).(String)
		out, err := enc.AsDate()
		if err != nil {
			t.Error(err)
		} else if t2 := time.Time(out); !t1.Equal(t2) {
			fmt.Println(t1, string(enc), out)
			t.Errorf("wrong time: %s != %s", t2, t1)
		}
	}
}

func TestDecodeDate(t *testing.T) {
	cases := []string{
		"D:19981223195200-08'00'",
		"D:20000101000000Z",
		"D:20201224163012+01'30'",
		"D:20010809191510 ", // trailing space, seen in some PDF files
	}
	for i, test := range cases {
		in := String(test)
		_, err := in.AsDate()
		if err != nil {
			t.Errorf("%d %q %s\n", i, test, err)
		}
	}
}

func TestRectangle1(t *testing.T) {
	type testCase struct {
		in  string
		out *Rectangle
	}
	cases := []testCase{
		{"[0 0 0 0]", &Rectangle{0, 0, 0, 0}},
		{"[1 2 3 4]", &Rectangle{1, 2, 3, 4}},
		{"[1.0 2.0 3.0 4.0]", &Rectangle{1, 2, 3, 4}},
		{"[1.1 2.2 3.3 4.4]", &Rectangle{1.1, 2.2, 3.3, 4.4}},
		{"[1.11 2.22 3.33 4.44]", &Rectangle{1.11, 2.22, 3.33, 4.44}},
		{"[1 2.222 3.333 4.4444]", &Rectangle{1, 2.222, 3.333, 4.4444}},
	}
	for _, test := range cases {
		t.Run(test.in, func(t *testing.T) {
			r := strings.NewReader(test.in)
			s := newScanner(r, nil, nil)
			obj, err := s.ReadObject()
			if err != nil {
				t.Fatal(err)
			}

			rect, err := asRectangle(nil, obj.(Array))

			if err != nil {
				t.Errorf("Decode(%q) returned error %v", test.in, err)
			}
			if !rect.NearlyEqual(test.out, 1e-6) {
				t.Errorf("Decode(%q) = %v, want %v", test.in, rect, *test.out)
			}
		})
	}
}

func TestRectangle2(t *testing.T) {
	cases := []*Rectangle{
		{0, 0, 0, 0},
		{1, 2, 3, 4},
		{0.5, 1.5, 2.5, 3.5},
		{0.5005, 1.5005, 2.5005, 3.5005},
		{1.0 / 3.0, 1.5, 2.5, 3.5},
	}
	for _, test := range cases {
		t.Run(test.String(), func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := Format(buf, 0, test)
			if err != nil {
				t.Fatal(err)
			}

			s := newScanner(buf, nil, nil)
			obj, err := s.ReadObject()
			if err != nil {
				t.Fatal(err)
			}

			rect, err := asRectangle(nil, obj.(Array))
			if err != nil {
				t.Errorf("Decode(%q) returned error %v", test.String(), err)
			}

			if !rect.NearlyEqual(test, .5e-2) {
				t.Errorf("Decode(%q) = %v, want %v", test.String(), rect, test)
			}
		})
	}
}

func TestCatalog(t *testing.T) {
	pRef := NewReference(1, 2)

	// test a round-trip
	cat0 := &Catalog{
		Pages: pRef,
	}
	d1 := AsDict(cat0)
	if len(d1) != 2 {
		t.Errorf("wrong Catalog dict: %s", AsString(d1))
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
		"Metadata": ref,
	}
	catalog := &Catalog{}
	err := DecodeDict(nil, catalog, catalogDict)
	if err == nil {
		t.Errorf("missing Pages not detected")
	}
	if catalog.Metadata != ref {
		t.Errorf("wrong Metadata: %v", catalog.Metadata)
	}
}

func TestCatalogWriteMissingPages(t *testing.T) {
	catalog := &Catalog{}
	dict := AsDict(catalog)
	if _, present := dict["Pages"]; present {
		t.Errorf("missing Pages not ignored")
	}
}

func TestDocumentInfoRegular(t *testing.T) {
	// test missing struct
	var info1 *Info
	d1 := AsDict(info1)
	if d1 != nil {
		t.Error("wrong dict for nil Info struct")
	}

	// test empty struct
	info1 = &Info{}
	d1 = AsDict(info1)
	if d1 == nil || len(d1) != 0 {
		t.Errorf("wrong dict for empty Info struct: %#v", d1)
	}

	// test all regular fields
	now := time.Now()
	info1 = &Info{
		Title:        "Test Title",
		Author:       "Jochen Voß",
		Subject:      "unit testing",
		Keywords:     "tests, go, DecodeDict",
		Creator:      "TestDocumentInfoRegular",
		Producer:     "seehuhn.de/go/pdf",
		CreationDate: Date(now.Add(-1 * time.Second)),
		ModDate:      Date(now),
		Trapped:      "Unknown",
	}
	d1 = AsDict(info1)
	info2 := &Info{}
	err := DecodeDict(nil, info2, d1)

	if info1.CreationDate.String() != info2.CreationDate.String() {
		t.Errorf("wrong CreationDate: %s != %s", info1.CreationDate, info2.CreationDate)
	}
	info1.CreationDate = Date{}
	info2.CreationDate = Date{}
	if info1.ModDate.String() != info2.ModDate.String() {
		t.Errorf("wrong ModDate: %s != %s", info1.ModDate, info2.ModDate)
	}
	info1.ModDate = Date{}
	info2.ModDate = Date{}

	if err != nil {
		t.Error(err)
	} else if d := cmp.Diff(info1, info2); d != "" {
		t.Errorf("wrong Info: %s", d)
	}
}

func TestDocumentInfoCustom(t *testing.T) {
	// test custom fields
	d1 := Dict{
		"grumpy": TextString("bärbeißig"),
		"funny":  TextString("\000\001\002 \\<>'\")("),
	}

	info := &Info{}
	err := DecodeDict(nil, info, d1)
	if err != nil {
		t.Error(err)
	}
	if len(info.Custom) != 2 {
		t.Errorf("wrong Extra: %v", info.Custom)
	}

	d2 := AsDict(info)
	if len(d1) != len(d2) {
		t.Fatalf("wrong d2: %s", AsString(d2))
	}
	for key, val := range d1 {
		if d2[key] != val {
			t.Errorf("wrong d2[%s]: %s", key, AsString(d2[key]))
		}
	}
}
