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
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"seehuhn.de/go/pdf"
)

func main() {
	passwd := flag.String("p", "", "PDF password")
	decode := flag.Bool("d", false, "decode streams")
	flag.Parse()

	var tryPasswd func([]byte, int) string
	if *passwd != "" {
		tryPasswd = func(_ []byte, try int) string {
			if passwd != nil && try == 0 {
				return *passwd
			}
			return ""
		}
	}

	args := flag.Args()

	fd, err := os.Open(args[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fi, err := fd.Stat()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	r, err := pdf.NewReader(fd, fi.Size(), tryPasswd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer r.Close()

	var obj pdf.Object
	switch {
	case len(args) < 2 || args[1] == "catalog":
		var cat *pdf.Catalog
		cat, err = r.GetCatalog()
		obj = pdf.AsDict(cat)
	case args[1] == "info":
		var info *pdf.Info
		info, err = r.GetInfo()
		obj = pdf.AsDict(info)
	default:
		var number int
		number, err = strconv.Atoi(args[1])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var generation uint16
		if len(args) > 2 {
			tmp, err := strconv.ParseUint(args[2], 10, 16)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			generation = uint16(tmp)
		}

		ref := &pdf.Reference{
			Number:     number,
			Generation: generation,
		}
		obj, err = r.Resolve(ref)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if stm, ok := obj.(*pdf.Stream); ok && *decode {
		err = stm.Dict.PDF(os.Stdout)
		fmt.Println()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("decoded stream")
		r, err := stm.Decode(r.Resolve)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		_, err = io.Copy(os.Stdout, r)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("\nendstream")
		return
	}

	if obj == nil {
		_, err = fmt.Println("null")
	} else {
		err = obj.PDF(os.Stdout)
	}
	fmt.Println()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
