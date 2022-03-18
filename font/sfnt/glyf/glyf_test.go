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

package glyf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-test/deep"
)

func FuzzGlyf(f *testing.F) {
	names, err := filepath.Glob("../../../demo/try-all-fonts/glyf/*.glyf")
	if err != nil {
		f.Fatal(err)
	}
	for _, name := range names {
		glyfData, err := os.ReadFile(name)
		if err != nil {
			f.Error(err)
			continue
		}
		locaName := strings.TrimSuffix(name, ".glyf") + ".loca"
		locaData, err := os.ReadFile(locaName)
		if err != nil {
			f.Error(err)
			continue
		}
		locaFormat := int16(0)
		if len(glyfData) > 0xFFFF {
			locaFormat = 1
		}
		f.Add(glyfData, locaData, locaFormat)
	}

	f.Fuzz(func(t *testing.T, glyfData, locaData []byte, locaFormat int16) {
		enc := &Encoded{
			GlyfData:   glyfData,
			LocaData:   locaData,
			LocaFormat: locaFormat,
		}
		info, err := Decode(enc)
		if err != nil {
			return
		}

		enc2 := info.Encode()

		info2, err := Decode(enc2)
		if err != nil {
			t.Fatal(err)
		}

		different := false
		for _, diff := range deep.Equal(info, info2) {
			fmt.Println(diff)
			different = true
		}
		if different {
			fmt.Println("A.g", enc.GlyfData)
			fmt.Println("A.l", enc.LocaData)
			fmt.Println("B.g", enc2.GlyfData)
			fmt.Println("B.l", enc2.LocaData)
			fmt.Println(info)
			fmt.Println(info2)
			t.Error("not equal")
		}
	})
}
