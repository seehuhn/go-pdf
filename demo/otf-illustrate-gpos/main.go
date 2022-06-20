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

package main

import (
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab/builder"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: otf-illustrate-gpos font.otf")
		os.Exit(1)
	}
	fontFileName := os.Args[1]

	fd, err := os.Open(fontFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	info, err := sfnt.Read(fd)
	if err != nil {
		log.Fatal(err)
	}

	if info.Gpos == nil {
		log.Fatal("font has no GPOS table")
	}

	explained := builder.ExplainGpos(info)
	for i, e := range explained {
		fmt.Printf("%d: %s\n", i, e)
	}
}
