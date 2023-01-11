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

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"seehuhn.de/go/sfnt"
)

func main() {
	index := 1
	for _, fname := range os.Args[1:] {
		fd, err := os.Open(fname)
		if err != nil {
			log.Fatal(err)
		}
		info, err := sfnt.Read(fd)
		if err != nil {
			log.Fatal(err)
		}
		err = fd.Close()
		if err != nil {
			log.Println(err)
		}

		ext := filepath.Ext(fname)
		outName := fmt.Sprintf("font%05d%s", index, ext)
		index++

		fd, err = os.Create(outName)
		if err != nil {
			log.Fatal(err)
		}
		_, err = info.Write(fd)
		if err != nil {
			log.Fatal(err)
		}
		if err != nil {
			log.Println(err)
		}
	}
}
