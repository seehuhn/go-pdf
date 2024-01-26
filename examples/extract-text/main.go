// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"flag"
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
)

func main() {
	flag.Parse()
	for _, fname := range flag.Args() {
		err := extractText(fname)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func extractText(fname string) error {
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()

	r, err := pdf.NewReader(fd, nil)
	if err != nil {
		return err
	}

	contents := reader.New(r)
	contents.Text = func(text string) error {
		fmt.Print(text)
		return nil
	}

	pages := pagetree.NewIterator(r)
	pageNo := 0
	pages.All()(func(_ pdf.Reference, pageDict pdf.Dict) bool {
		fmt.Println("Page", pageNo)
		fmt.Println()

		err := contents.ParsePage(pageDict)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println()

		pageNo++
		return true
	})
	return nil
}
