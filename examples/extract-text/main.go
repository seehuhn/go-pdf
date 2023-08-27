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
	"seehuhn.de/go/pdf/content"
	"seehuhn.de/go/pdf/pagetree"
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

	numPages, err := pagetree.NumPages(r)
	if err != nil {
		return err
	}

	for pageNo := 0; pageNo < numPages; pageNo++ {
		fmt.Println()
		fmt.Println("Page", pageNo)
		fmt.Println()
		pageDict, err := pagetree.GetPage(r, pageNo)
		if err != nil {
			return err
		}
		content.ForAllText(r, pageDict, func(ctx *content.Context, s string) error {
			fmt.Print(s)
			return nil
		})
	}
	return nil
}
