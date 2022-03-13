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
	"crypto/sha256"
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf/font/sfnt/table"
)

func tryFont(fname string) error {
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()

	header, err := table.ReadHeader(fd)
	if err != nil {
		return err
	}

	glyfData, err := header.ReadTableBytes(fd, "glyf")
	if table.IsMissing(err) {
		return nil
	} else if err != nil {
		return err
	}

	if len(glyfData) > 70000 {
		return nil
	}

	locaData, err := header.ReadTableBytes(fd, "loca")
	if err != nil {
		return err
	}

	hash := sha256.Sum256(glyfData)
	outName := fmt.Sprintf("glyf/%x.glyf", hash)
	err = os.WriteFile(outName, glyfData, 0644)
	if err != nil {
		return err
	}
	outName = fmt.Sprintf("glyf/%x.loca", hash)
	err = os.WriteFile(outName, locaData, 0644)
	if err != nil {
		return err
	}

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
