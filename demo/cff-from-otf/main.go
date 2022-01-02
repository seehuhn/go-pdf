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

// Extract CFF data from OpenType font files.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"seehuhn.de/go/pdf/font/sfnt"
)

func main() {
	args := os.Args[1:]
	for _, fname := range args {
		outName := filepath.Base(strings.TrimSuffix(fname, ".otf") + ".cff")
		fmt.Println(fname, "->", outName)

		otf, err := sfnt.Open(fname, nil)
		if err != nil {
			log.Fatalf("%s: %v", fname, err)
		}

		cffData, err := otf.Header.ReadTableBytes(otf.Fd, "CFF ")
		if err != nil {
			log.Fatalf("%s: %v", fname, err)
		}

		err = otf.Close()
		if err != nil {
			log.Fatalf("%s: %v", fname, err)
		}

		err = os.WriteFile(outName, cffData, 0644)
		if err != nil {
			log.Fatalf("%s: %v", outName, err)
		}
	}
}
