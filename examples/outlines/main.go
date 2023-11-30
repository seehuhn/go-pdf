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
	"fmt"
	"log"
	"os"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/outline"
)

func main() {
	r, err := pdf.Open(os.Args[1], nil)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	outlines, err := outline.Read(r)
	if err != nil {
		log.Fatal(err)
	}

	traverseTree(r, outlines, 0)
}

func traverseTree(r pdf.Getter, tree *outline.Tree, level int) {
	fmt.Printf("%s%q\n", strings.Repeat("\t", level), tree.Title)
	for _, child := range tree.Children {
		traverseTree(r, child, level+1)
	}
}
