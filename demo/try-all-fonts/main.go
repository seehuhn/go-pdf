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
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

var out *csv.Writer

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
	cmapTable, err := cmap.Decode(cmapData)
	if err != nil {
		return err
	}
	good := false
	for key := range cmapTable {
		if key.Language != 0 {
			fmt.Println("xxx", key, fname)
		} else {
			good = true
		}
	}
	if !good {
		fmt.Println("yyy", fname)
	}

	// nameData, err := header.ReadTableBytes(fd, "name")
	// if err != nil {
	// 	return err
	// }

	// tableReader := func(name string) (*io.SectionReader, error) {
	// 	rec := header.Find(name)
	// 	if rec == nil {
	// 		return nil, &table.ErrNoTable{Name: name}
	// 	}
	// 	return io.NewSectionReader(fd, int64(rec.Offset), int64(rec.Length)), nil
	// }

	// os2Fd, err := tableReader("OS/2")
	// if table.IsMissing(err) {
	// 	return nil
	// } else if err != nil {
	// 	return err
	// }
	// os2Info, err := os2.Read(os2Fd)
	// if err != nil {
	// 	return err
	// }

	// nameInfo, err := name.Decode(nameData)
	// if err != nil {
	// 	return err
	// }

	// t := nameInfo.Tables.Get()
	// if t == nil {
	// 	return errors.New("no name table")
	// }

	// if !os2Info.IsBold && strings.Contains(t.Subfamily, "Bold") {
	// 	fmt.Println(t)
	// }

	return nil
}

func main() {
	fd, err := os.Open("all-fonts")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	outFd, err := os.Create("x.csv")
	if err != nil {
		log.Fatal(err)
	}
	out = csv.NewWriter(outFd)

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
	out.Flush()
	err = outFd.Close()
	if err != nil {
		log.Fatal(err)
	}
}
