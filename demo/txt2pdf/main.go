// copyright (c) 2021  Jochen Voß

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
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

func encodeString(font *font.Font, s string) pdf.String {
	var res pdf.String
	col := 0
	for _, r := range s {
		if r == '\t' {
			for {
				res = append(res, ' ')
				col++
				if col%tabWidth == 0 {
					break
				}
			}
			continue
		}
		idx := font.CMap[r]
		c := font.Enc(idx)
		res = append(res, c...)
		col++
	}
	return res
}

func convert(inName, outName string) error {
	fmt.Println(inName, "->", outName)

	in, err := os.Open(inName)
	if err != nil {
		return err
	}
	defer in.Close()

	subset := newSubset()
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		subset.Add(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	_, err = in.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	out, err := pdf.Create(outName)
	if err != nil {
		return err
	}
	defer out.Close()

	err = out.SetInfo(pdf.Struct(&pdf.Info{
		Title:        inName,
		Producer:     "seehuhn.de/go/pdf/demo/txt2pdf",
		CreationDate: time.Now(),
	}))
	if err != nil {
		log.Fatal(err)
	}

	F1, err := builtin.Embed(out, "F", "Courier", subset.chars)
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

	scanner = bufio.NewScanner(in)
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

		line := encodeString(F1, scanner.Text())
		if len(line) > 0 {
			_ = line.PDF(page)
			fmt.Fprintln(page, " Tj T*")
		} else {
			fmt.Fprintln(page, "T*")
		}

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

	err = out.SetCatalog(pdf.Struct(&pdf.Catalog{
		Pages: pagesRef,
	}))
	if err != nil {
		return err
	}

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
