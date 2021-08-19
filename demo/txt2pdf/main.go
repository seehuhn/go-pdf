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
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/pages"
)

const tabWidth = 4

type subset struct {
	chars map[rune]bool
}

func newSubset() *subset {
	return &subset{
		chars: make(map[rune]bool),
	}
}

func (ccc *subset) Add(s string) {
	for _, r := range s {
		if unicode.IsGraphic(r) {
			ccc.chars[r] = true
		}
	}
}

func convert(inName, outName string) error {
	fmt.Println(inName, "->", outName)

	in, err := os.Open(inName)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := pdf.Create(outName)
	if err != nil {
		return err
	}
	defer out.Close()

	out.SetInfo(&pdf.Info{
		Title:        inName,
		Producer:     "seehuhn.de/go/pdf/demo/txt2pdf",
		CreationDate: time.Now(),
	})

	F1, err := builtin.Embed(out, "F", "Courier")
	if err != nil {
		return err
	}

	pageTree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{F1.Name: F1.Ref},
		},
		MediaBox: pages.A4,
	})

	var page *pages.Page
	numLines := int((pages.A4.URy - 144) / 12)
	pageLines := 0

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		if page == nil {
			page, err = pageTree.AddPage(nil)
			if err != nil {
				return err
			}
			page.Println("BT")
			page.Println("/F 12 Tf")
			page.Println("12 TL")
			page.Printf("72 %f Td\n", page.URy-72-10)
		}

		line := scanner.Text()
		var rr []rune
		if strings.Contains(line, "\t") {
			col := 0
			for _, r := range line {
				if r == '\t' {
					for {
						rr = append(rr, ' ')
						col++
						if col%tabWidth == 0 {
							break
						}
					}
				} else {
					rr = append(rr, r)
				}
			}
		} else {
			rr = []rune(line)
		}
		glyphs := F1.Layout(rr)
		F1.Draw(page, glyphs)

		fmt.Fprintln(page, " T*")

		pageLines++
		if pageLines >= numLines {
			fmt.Fprintln(page, "ET")
			err = page.Close()
			if err != nil {
				return err
			}
			page = nil
			pageLines = 0
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if page != nil {
		fmt.Fprintln(page, "ET")
		err = page.Close()
		if err != nil {
			return err
		}
	}

	pagesRef, err := pageTree.Flush()
	if err != nil {
		return err
	}

	out.SetCatalog(&pdf.Catalog{
		Pages: pagesRef,
	})

	return nil
}

func main() {
	flag.Parse()

	for _, inName := range flag.Args() {
		baseName := strings.TrimSuffix(inName, ".txt")
		var outName string
		for i := 1; ; i++ {
			if i == 1 {
				outName = baseName + ".pdf"
			} else {
				outName = fmt.Sprintf("%s-%d.pdf", baseName, i)
			}
			_, err := os.Stat(outName)
			if os.IsNotExist(err) {
				break
			} else if err != nil {
				log.Fatal(err)
			}
		}
		err := convert(inName, outName)
		if err != nil {
			log.Fatal(err)
		}
	}
}
