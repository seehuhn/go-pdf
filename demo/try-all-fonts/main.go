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

	"seehuhn.de/go/pdf/sfnt/cff"
	"seehuhn.de/go/pdf/sfnt/head"
	"seehuhn.de/go/pdf/sfnt/header"
	"seehuhn.de/go/pdf/sfnt/os2"
)

func wrap[T any, R io.Reader](f func(r R) (T, error)) func(r io.Reader) error {
	return func(r io.Reader) error {
		_, err := f(r.(R))
		return err
	}
}

type tableReader struct {
	name   string
	reader func(r io.Reader) error
}

var tableReaders = []tableReader{
	{"OS/2", wrap(os2.Read)},
	{"head", wrap(head.Read)},
	{"CFF ", wrap(cff.Read)},
}

func tryFont(fname string) error {
	r, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer r.Close()

	tables, err := header.Read(r)
	if err != nil {
		return err
	}

	for _, rec := range tableReaders {
		fd, err := tables.TableReader(r, rec.name)
		if header.IsMissing(err) {
			continue
		} else if err != nil {
			return err
		}

		err = rec.reader(fd)
		if err != nil {
			panic(err)
		}
	}

	// _, err := sfnt.ReadFile(fname)

	return nil
}

func main() {
	fd, err := os.Open("all-fonts")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		fname := scanner.Text()
		err = tryFont(fname)
		if err != nil {
			fmt.Fprintln(os.Stderr, fname+":", err)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("main loop failed:", err)
	}
}
