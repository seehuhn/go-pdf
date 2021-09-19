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
	"fmt"
	"io"
	"log"
	"os"

	"seehuhn.de/go/pdf"
)

var passwords = map[string]string{
	"acac29b4192fd923c24fe6042479b2a9": "test",
	"c13cb3f802bb2d44b74cb449d64665a7": "Arlo",
	"ce78fac52b4b1c9ae79788e36217ca99": "EDIT",
	"fa71d689cb967d41b249a20f14d5269d": "201206616",
}

func readPwd(ID []byte, try int) string {
	if try != 0 {
		return ""
	}
	hex := fmt.Sprintf("%x", ID)
	return passwords[hex]
}

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
			if dict["Type"] != pdf.Name("Font") {
				continue
			}
			toUnicode, ok := dict["ToUnicode"]
			if !ok {
				continue
			}
			cmapObj, err := r.Resolve(toUnicode)
			if err != nil {
				return err
			}
			cmap := cmapObj.(*pdf.Stream)

			fmt.Println("*", fname)
			r, err := cmap.Decode(r.Resolve)
			if err != nil {
				return err
			}
			_, err = io.Copy(os.Stdout, r)
			if err != nil {
				return err
			}
			fmt.Println("---------------------------")
		}
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
