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
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"seehuhn.de/go/pdf/font/sfnt"
)

func tryFont(fname string) error {
	if !strings.HasSuffix(fname, ".otf") {
		return nil
	}

	tt, err := sfnt.Open(fname, nil)
	if err != nil {
		return err
	}
	defer tt.Close()

	if !tt.IsOpenType() {
		return nil
	}

	if !tt.HasTables("glyf") {
		return nil
	}

	fmt.Println(len(tt.Width), fname)

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