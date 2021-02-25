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
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"

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

	root, err := r.GetCatalog()
	if err != nil {
		return err
	}
	catalog := &pdf.Catalog{}
	// Ignore errors, to get at least partial information in case of malformed
	// PDF files.
	_ = root.AsStruct(catalog, r.Resolve)
	pagesObj, err := r.Resolve(catalog.Pages)
	if err != nil {
		return err
	}
	pages, ok := pagesObj.(pdf.Dict)
	if !ok {
		return errors.New("/Pages object has wrong type")
	}
	countObj, err := r.Resolve(pages["Count"])
	if err != nil {
		return err
	}

	_ = countObj
	// fmt.Println(count, fname)

	for {
		obj, _, err := r.ReadSequential()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		stream, ok := obj.(*pdf.Stream)
		if !ok {
			continue
		}
		filters, err := stream.Filters(r.Resolve)
		if err != nil {
			return err
		}
		for _, fi := range filters {
			if fi.Name != "FlateDecode" {
				continue
			}

			useColors := false
			if pred, ok := fi.Parms["Predictor"].(pdf.Integer); ok && pred > 1 {
				useColors = true
			}

			var keys []pdf.Name
			for key, val := range fi.Parms {
				if key == "Columns" {
					continue
				}
				if key == "BitsPerComponent" && val == pdf.Integer(8) {
					continue
				}
				if key == "Colors" && (val == pdf.Integer(1) || !useColors) {
					continue
				}
				keys = append(keys, key)
			}
			if len(keys) == 0 {
				continue
			}
			sort.Slice(keys, func(i int, j int) bool {
				return keys[i] < keys[j]
			})

			buf := &bytes.Buffer{}
			for _, name := range keys {
				val := fi.Parms[name]
				if val == nil {
					continue
				}
				if buf.Len() > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(string(name) + ":")
				val.PDF(buf)
			}
			fmt.Println(buf.String())
		}

		// dict, ok := obj.(pdf.Dict)
		// if !ok {
		// 	continue
		// }
		// if dict["Type"] == pdf.Name("Font") {
		// 	fmt.Println(dict["Subtype"])
		// }
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
