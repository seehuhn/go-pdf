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

	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

var seen = make(map[string]bool)
var count = make(map[uint16]int)

func tryFont(fname string) error {
	// tt, err := sfnt.Open(fname, nil)
	// if err != nil {
	// 	return err
	// }
	// defer tt.Close()

	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()

	header, err := table.ReadHeader(fd)
	if err != nil {
		return err
	}

	cmapData, err := header.ReadTableBytes(fd, "cmap")
	if err != nil {
		return err
	}

	subtables, err := cmap.LocateSubtables(cmapData)
	if err != nil {
		return err
	}
	for _, s := range subtables {
		format := uint16(s.Data[0])<<8 | uint16(s.Data[1])

		hash := sha256.New()
		hash.Write(s.Data)
		sum := string(hash.Sum(nil))
		if seen[string(sum)] {
			fmt.Println(format, len(s.Data), "seen")
			continue
		} else {
			seen[sum] = true
		}

		count[format]++
		outname := fmt.Sprintf("cmap/%02d-%05d.bin", format, count[format])
		err = os.WriteFile(outname, s.Data, 0644)
		if err != nil {
			return err
		}
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
	fmt.Println(count)
}
