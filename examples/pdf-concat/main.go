// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

// Concat concatenates PDF files.
//
// This is a simplistic implementation which copies the page contents
// and the document outlines, but ignores all other document structure
// and document-level meta information.
package main

import (
	"flag"
	"fmt"
	"os"

	"seehuhn.de/go/pdf"
)

func main() {
	out := flag.String("o", "out.pdf", "output file name")
	force := flag.Bool("f", false, "overwrite output file if it exists")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "error: no input files given")
		flag.Usage()
		os.Exit(1)
	}

	if !*force {
		if _, err := os.Stat(*out); !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: output file %q already exists\n", *out)
			os.Exit(1)
		}
	}

	err := concatFiles(*out, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func concatFiles(out string, in []string) error {
	v := pdf.V1_0
	for _, fname := range in {
		ver, err := getVersion(fname)
		if err != nil {
			return err
		}
		if ver > v {
			v = ver
		}
	}

	c, err := NewConcat(out, v)
	if err != nil {
		return err
	}

	for _, fname := range in {
		err = c.Append(fname)
		if err != nil {
			return err
		}
	}

	err = c.Close()
	if err != nil {
		return err
	}
	return nil
}

func getVersion(fname string) (pdf.Version, error) {
	r, err := pdf.Open(fname, nil)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	return r.GetMeta().Version, nil
}
