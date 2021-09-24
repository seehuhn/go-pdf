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
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"seehuhn.de/go/pdf"
)

func doOneFile(fname string) error {
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	fi, err := fd.Stat()
	if err != nil {
		return err
	}
	r, err := pdf.NewReader(fd, fi.Size(), readPwd)
	if err != nil {
		return err
	}
	defer r.Close()

	for {
		obj, _, err := r.ReadSequential()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if dict, ok := obj.(pdf.Dict); ok {
			if dict["Type"] != pdf.Name("Font") || dict["Subtype"] != pdf.Name("CIDFontType2") {
				continue
			}
			desc := "<missing>"
			if CIDSystemInfo, ok := dict["CIDSystemInfo"]; ok {
				CIDSystemInfo, err = r.Resolve(CIDSystemInfo)
				if err != nil {
					return err
				}
				d := CIDSystemInfo.(pdf.Dict)
				registry := string(d["Registry"].(pdf.String))
				ordering := string(d["Ordering"].(pdf.String))
				supplement := int(d["Supplement"].(pdf.Integer))
				desc = fmt.Sprintf("%s-%s-%d", registry, ordering, supplement)
			}
			// fmt.Println(desc)
			_ = desc
		}
	}

	return nil
}

func main() {
	passwords := flag.String("p", "", "list of passwords for authetication")
	flag.Parse()

	if *passwords != "" {
		err := loadPasswordFile(*passwords)
		if err != nil {
			log.Fatal(err)
		}
	}

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

var passwords = map[string]string{}

func loadPasswordFile(fname string) error {
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		ff := strings.Fields(scanner.Text())
		if len(ff) != 2 {
			return errors.New("malformed password file")
		}
		passwords[ff[0]] = ff[1]
	}
	return scanner.Err()
}

func readPwd(ID []byte, try int) string {
	if try != 0 {
		return ""
	}
	hex := fmt.Sprintf("%x", ID)
	return passwords[hex]
}

func getNames() <-chan string {
	fd, err := os.Open(flag.Arg(0))
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
