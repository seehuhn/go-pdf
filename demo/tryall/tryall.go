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
	"io"
	"log"
	"os"

	"seehuhn.de/go/pdf"
)

func getNames() <-chan string {
	fd, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	c := make(chan string)
	go func(c chan<- string) {
		scanner := bufio.NewScanner(fd)
		for scanner.Scan() {
			c <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			log.Println("cannot read more file names:", err)
		}

		fd.Close()
		close(c)
	}(c)
	return c
}

func doOneFile(fname string) error {
	r, err := pdf.Open(fname)
	if err != nil {
		return err
	}
	defer r.Close()

seqLoop:
	for {
		obj, _, err := r.ReadSequential()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		dict, ok := obj.(pdf.Dict)
		if !ok || dict["Type"] != pdf.Name("FontDescriptor") {
			continue
		}

		var stemV float64
		switch x := dict["StemV"].(type) {
		case pdf.Integer:
			stemV = float64(x)
		case pdf.Real:
			stemV = float64(x)
		default:
			continue seqLoop
		}
		if stemV > 1000 {
			continue
		}

		weight, ok := dict["FontWeight"].(pdf.Integer)
		if !ok {
			continue
		}

		stretch, ok := dict["FontStretch"].(pdf.Name)
		if !ok {
			stretch = "NA"
		} else {
			sMap := map[pdf.Name]pdf.Name{
				"UltraCondensed": "1",
				"ExtraCondensed": "2",
				"Condensed":      "3",
				"SemiCondensed":  "4",
				"Normal":         "5",
				"SemiExpanded":   "6",
				"Expanded":       "7",
				"ExtraExpanded":  "8",
				"UltraExpanded":  "9",
			}
			s2, ok := sMap[stretch]
			if ok {
				stretch = s2
			} else {
				fmt.Println("xxx", stretch)
			}
		}

		fmt.Printf("%g,%d,%s\n", stemV, weight, stretch)
	}

	return nil
}

func main() {
	total := 0
	errors := 0
	c := getNames()
	for fname := range c {
		total++
		err := doOneFile(fname)
		if err != nil {
			sz := "?????????? "
			fi, e2 := os.Stat(fname)
			if e2 == nil {
				sz = fmt.Sprintf("%10d ", fi.Size())
			}
			fmt.Println(sz, fname+":", err)
			errors++
		}
	}
	fmt.Println(total, "files,", errors, "errors")
}
