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

package cff

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt"
)

func TestReadCFF(t *testing.T) {
	in, err := os.Open("Atkinson-Hyperlegible-BoldItalic-102.cff")
	if err != nil {
		t.Fatal(err)
	}

	cff, err := Read(in)
	if err != nil {
		t.Fatal(err)
	}

	err = in.Close()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(cff.Info.FontName)
}

func TestRewriteOtf(t *testing.T) {
	otf, err := sfnt.Open("../opentype/otf/Atkinson-Hyperlegible-BoldItalic-102.otf", nil)
	if err != nil {
		t.Fatal(err)
	}

	cffRaw, err := otf.Header.ReadTableBytes(otf.Fd, "CFF ")
	if err != nil {
		t.Fatal(err)
	}

	r := bytes.NewReader(cffRaw)
	cff, err := Read(r)
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	err = cff.Encode(buf)
	if err != nil {
		t.Fatal(err)
	}
	blob := buf.Bytes()

	err = os.WriteFile("debug.cff", blob, 0644)
	if err != nil {
		t.Fatal(err)
	}

	exOpt := &sfnt.ExportOptions{
		Replace: map[string][]byte{
			"CFF ": blob,
		},
	}

	out, err := os.Create("debug.otf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = otf.Export(out, exOpt)
	if err != nil {
		t.Fatal(err)
	}
	err = out.Close()
	if err != nil {
		t.Fatal(err)
	}
}
