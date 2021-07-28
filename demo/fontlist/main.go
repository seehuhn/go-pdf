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

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/parser"
)

func tryFont(fname string) error {
	// fmt.Println(fname)
	tt, err := sfnt.Open(fname)
	if err != nil {
		return err
	}
	defer tt.Close()

	if !tt.IsTrueType() || !tt.HasTables("GSUB") {
		return nil
	}

	p := parser.New(tt)
	info, err := p.ReadGsubInfo("latn", "ENG ")
	if err != nil {
		return err
	}

	_ = info

	// cmap, err := tt.SelectCmap()
	// if err != nil {
	// 	return err
	// }

	// s := "a nai\u0308ve, affluent Ba\u0308r"
	// var glyphs []font.GlyphID
	// for _, r := range s {
	// 	gid, ok := cmap[r]
	// 	if !ok {
	// 		return errors.New("missing glyph")
	// 	}
	// 	glyphs = append(glyphs, gid)
	// }
	// l1 := len(glyphs)

	// glyphs = info.Substitute(glyphs)
	// l2 := len(glyphs)

	// fmt.Println(l1, l2)

	return nil
}

func main() {
	fd, err := os.Open("all-fonts")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		fname := scanner.Text()
		err = tryFont(fname)
		if err != nil {
			fmt.Println(fname+":", err)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("main loop failed:", err)
	}
}
