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
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cff"
)

func isSubset(name string) bool {
	return len(name) > 7 && name[6] == '+'
}

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

		Font, ok := obj.(pdf.Dict)
		if !ok || Font["Type"] != pdf.Name("Font") {
			continue
		}
		fdObj, _ := r.Resolve(Font["FontDescriptor"])
		if fdObj == nil {
			// built-in font?
			continue
		}
		FontDescriptor, ok := fdObj.(pdf.Dict)
		if !ok {
			return errors.New("silly font")
		}
		stmObj, _ := r.Resolve(FontDescriptor["FontFile3"])
		if stmObj == nil {
			continue
		}
		FontFile3, ok := stmObj.(*pdf.Stream)
		if !ok {
			return errors.New("funny font")
		}

		fontType, _ := Font["Subtype"].(pdf.Name)
		subType, _ := FontFile3.Dict["Subtype"].(pdf.Name)
		baseFont, _ := Font["BaseFont"].(pdf.Name)

		r, err := FontFile3.Decode(r.Resolve)
		if err != nil {
			return err
		}
		buf, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		cff, err := cff.Read(bytes.NewReader(buf))
		if err != nil {
			return err
		}

		fmt.Println(
			isSubset(string(baseFont)), isSubset(cff.FontName), fontType, subType)
	}

	return nil
}

func main() {
	passwords := flag.String("p", "", "file with passwords for authentication")
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
