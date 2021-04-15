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
	"seehuhn.de/go/pdf/font/sfnt/table"
)

func tryFont(fname string) error {
	tt, err := sfnt.Open(fname)
	if err != nil {
		return err
	}
	defer tt.Close()

	_, err = tt.ReadGsubLigInfo("DEU ", "latn")
	if table.IsMissing(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// cmap, fd, err := tt.ReadCmapTable()
	// if err != nil {
	// 	return err
	// }
	// for _, encRec := range cmap.EncodingRecords {
	// 	_, err := fd.Seek(int64(encRec.SubtableOffset), io.SeekStart)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	var format uint16
	// 	err = binary.Read(fd, binary.BigEndian, &format)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if format != 12 {
	// 		continue
	// 	}

	// 	cmap, err := encRec.LoadCmap(fd, func(i int) rune { return rune(i) })
	// 	if err != nil {
	// 		return err
	// 	}
	// 	fmt.Println(len(cmap))
	// }

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
