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
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
)

func main() {
	flag.Parse()
	fname := flag.Arg(0)
	err := extractText(fname)
	if err != nil {
		log.Fatal(err)
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

	yTop := 680.0

	contents := reader.New(r, nil)
	var oldX float64
	var oldY float64
	var line []byte
	contents.Text = func(text string) error {
		x, y := contents.GetTextPositionDevice()
		if y < 100 || y > yTop {
			return nil
		}
		if y < oldY-5 || y > oldY+5 {
			if len(line) > 0 {
				out(line)
				line = line[:0]
			}
			oldX = 0
			oldY = y
		}

		if oldX != 0 && x > oldX+12 {
			line = append(line, ' ')
		}
		oldX = x

		line = append(line, text...)
		return nil
	}

	for _, pageNo := range []int{678, 679} {
		_, pageDict, err := pagetree.GetPage(r, pageNo)
		if err != nil {
			return err
		}

		oldX = 0
		oldY = 0

		err = contents.ParsePage(pageDict, matrix.Identity)
		if err != nil {
			return err
		}
		if len(line) > 0 {
			out(line)
			line = line[:0]
		}

		yTop = 725.0
	}

	sort.Strings(names)
	fmt.Println("var IsSymbol = map[string]bool{")
	for _, name := range names {
		fmt.Printf("\t%q: true,\n", name)
	}
	fmt.Println("}")
	fmt.Println()
	fmt.Println("var SymbolEncoding = [256]string{")
	for i, name := range enc {
		if name == "" {
			name = ".notdef"
		}
		fmt.Printf("\t%q,\t// 0o%03o = %d\n", name, i, i)
	}
	fmt.Println("}")

	return nil
}

func out(line []byte) {
	_, sz := utf8.DecodeRune(line) // skip the first rune
	ff := strings.Fields(string(line[sz:]))
	if len(ff) != 2 {
		panic(ff)
	}
	name := ff[0]
	code, err := strconv.ParseInt(ff[1], 8, 32)
	if err != nil {
		panic(err)
	}
	names = append(names, name)
	enc[code] = name
}

var names []string
var enc [256]string
