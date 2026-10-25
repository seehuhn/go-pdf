// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package action

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// TestLaunchFileOptional checks that a Launch action reads without F where
// another entry locates the file, that a malformed F is dropped, and that an
// action left with nothing to locate the file is rejected on both read and
// write.
func TestLaunchFileOptional(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	win := pdf.Dict{"F": pdf.String("app.exe")}

	valid := map[string]pdf.Dict{
		"with Win":           {"S": pdf.Name("Launch"), "Win": win},
		"with Win and bad F": {"S": pdf.Name("Launch"), "Win": win, "F": pdf.Integer(5)},
		"with Mac":           {"S": pdf.Name("Launch"), "Mac": pdf.Dict{}},
		"with Unix":          {"S": pdf.Name("Launch"), "Unix": pdf.Array{}},
	}
	for name, dict := range valid {
		a, err := Decode(c, dict, true)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if f := a.(*Launch).F; f != nil {
			t.Errorf("%s: F = %v, want nil", name, f)
		}
	}

	invalid := map[string]pdf.Dict{
		"empty":              {"S": pdf.Name("Launch")},
		"with bad F":         {"S": pdf.Name("Launch"), "F": pdf.Integer(5)},
		"with dangling Mac":  {"S": pdf.Name("Launch"), "Mac": pdf.NewReference(99, 0)},
		"with dangling Unix": {"S": pdf.Name("Launch"), "Unix": pdf.NewReference(99, 0)},
		"with null Mac":      {"S": pdf.Name("Launch"), "Mac": nil},
	}
	for name, dict := range invalid {
		if _, err := Decode(c, dict, true); !pdf.IsMalformed(err) {
			t.Errorf("%s: got %v, want malformed error", name, err)
		}
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	if _, err := (&Launch{}).Encode(rm); err == nil {
		t.Error("encode without file succeeded")
	}
}

// TestLaunchMacUnixRoundTrip checks that the undefined Mac and Unix entries
// survive a read-write-read cycle.
func TestLaunchMacUnixRoundTrip(t *testing.T) {
	values := map[string]pdf.Object{
		"dict":    pdf.Dict{"Key": pdf.Name("Value")},
		"array":   pdf.Array{pdf.Integer(1), pdf.String("x")},
		"integer": pdf.Integer(7),
		"string":  pdf.String("param"),
	}
	for name, value := range values {
		for _, key := range []pdf.Name{"Mac", "Unix"} {
			t.Run(name+" "+string(key), func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
				c := pdf.NewCursor(w)
				a, err := Decode(c, pdf.Dict{"S": pdf.Name("Launch"), key: value}, true)
				if err != nil {
					t.Fatal(err)
				}
				testActionRoundTrip(t, pdf.V1_7, a)
			})
		}
	}
}

// TestLaunchMacUnixStream checks that a Mac or Unix value which is a stream
// can be read and written to a different file.
func TestLaunchMacUnixStream(t *testing.T) {
	data := []byte("launch parameters")
	for _, key := range []pdf.Name{"Mac", "Unix"} {
		t.Run(string(key), func(t *testing.T) {
			src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			ref := src.Alloc()
			stm, err := src.OpenStream(ref, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stm.Write(data); err != nil {
				t.Fatal(err)
			}
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}

			a, err := Decode(pdf.NewCursor(src), pdf.Dict{"S": pdf.Name("Launch"), key: ref}, true)
			if err != nil {
				t.Fatal(err)
			}

			dst, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(dst)
			encoded, err := a.Encode(rm)
			if err != nil {
				t.Fatal(err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			dstRef, ok := encoded.(pdf.Dict)[key].(pdf.Reference)
			if !ok {
				t.Fatalf("%s written as %T, want reference", key, encoded.(pdf.Dict)[key])
			}
			got, err := pdf.CursorAt(pdf.NewExtractor(dst), nil).ReadAll(dstRef, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, data) {
				t.Errorf("stream data %q, want %q", got, data)
			}

			// the written file reads back to a Launch action again
			b, err := Decode(pdf.NewCursor(dst), encoded, true)
			if err != nil {
				t.Fatal(err)
			}
			l := b.(*Launch)
			if key == "Mac" && l.Mac == nil || key == "Unix" && l.Unix == nil {
				t.Errorf("%s lost on second read", key)
			}
		})
	}
}
