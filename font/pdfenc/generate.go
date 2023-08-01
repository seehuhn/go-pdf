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

//go:build ignore
// +build ignore

// The file "data" was extracted from table D.2 in the PDF spec.

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/type1/names"
)

func main() {
	err := run("data")
	if err != nil {
		log.Fatal(err)
	}
	err = run2("data2")
	if err != nil {
		log.Fatal(err)
	}
}

type record struct {
	val  string
	code [4]int
}

func run(fname string) error {
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()

	data := make(map[pdf.Name]record)

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		line := scanner.Text()
		ff := strings.Fields(line)
		if len(ff) == 5 && ff[0] == "space" {
			ff = append([]string{" "}, ff...)
		}
		if len(ff) != 6 {
			return fmt.Errorf("invalid line: %q", line)
		}

		name := pdf.Name(ff[1])
		data[name] = record{
			val:  ff[0],
			code: [4]int{oct(ff[2]), oct(ff[3]), oct(ff[4]), oct(ff[5])},
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	err = sanityCheck(data)
	if err != nil {
		return err
	}

	err = writeTable(data, "standard.go", "StandardEncoding", 0)
	if err != nil {
		return err
	}

	err = writeTable(data, "macroman.go", "MacRomanEncoding", 1)
	if err != nil {
		return err
	}

	err = writeTable(data, "winansi.go", "WinAnsiEncoding", 2)
	if err != nil {
		return err
	}

	err = writeTable(data, "pdfdoc.go", "PDFDocEncoding", 3)
	if err != nil {
		return err
	}

	return nil
}

func run2(fname string) error {
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()

	data := make(map[pdf.Name]record)

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		line := scanner.Text()
		ff := strings.Fields(line)
		if len(ff) == 2 && ff[0] == "space" {
			ff = append([]string{" "}, ff...)
		}
		if len(ff) != 3 {
			return fmt.Errorf("invalid line: %q", line)
		}

		name := pdf.Name(ff[1])
		data[name] = record{
			val:  ff[0],
			code: [4]int{oct(ff[2]), 0, 0, 0},
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// err = sanityCheck(data)
	// if err != nil {
	// 	return err
	// }

	err = writeTable(data, "macexpert.go", "MacExpertEncoding", 0)
	if err != nil {
		return err
	}

	return nil
}

func oct(s string) int {
	if s == "—" {
		return -1
	}
	x, err := strconv.ParseUint(s, 8, 8)
	if err != nil {
		panic(err)
	}
	return int(x)
}

func sanityCheck(data map[pdf.Name]record) error {
	for name, r := range data {
		rr := names.ToUnicode(string(name), false)
		if string(rr) != r.val {
			return fmt.Errorf("%q: %q != %q %s", name, r.val, string(rr), string(rr))
		}
	}
	return nil
}

func writeTable(data map[pdf.Name]record, fname string, encName string, col int) error {
	w, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = w.WriteString(header)
	if err != nil {
		return err
	}

	encoding := make([]pdf.Name, 256)
	val := make([]string, 256)
	for name, r := range data {
		code := r.code[col]
		if code < 0 {
			continue
		}
		if encoding[code] != "" {
			return fmt.Errorf("%s: duplicate code %d", encName, code)
		}
		encoding[code] = name
		val[code] = r.val
	}

	switch encName {
	case "WinAnsiEncoding":
		// Footnote 5 after table D.2: The hyphen (U+002D) character is also
		// encoded as 255 (octal) in WinAnsiEncoding.
		encoding[0o255] = "hyphen"
		val[0o255] = "-"
		// Footnote 6 after table D.2: The space (U+0020) character is also
		// encoded [...] as 240 (octal) in WinAnsiEncoding.
		encoding[0o240] = "space"
		val[0o240] = " "
	case "MacRomanEncoding":
		// Footnote 6 after table D.2: The space (U+0020) character is also
		// encoded as 312 (octal) in MacRomanEncoding [...].
		encoding[0o312] = "space"
		val[0o312] = " "
	}

	_, err = w.WriteString(comment[col])
	if err != nil {
		return err
	}
	wd := 18
	if encName == "MacExpertEncoding" {
		wd = 23
	}
	fmt.Fprintf(w, "var %s = [256]string{\n", encName)
	for i := 0; i < 256; i++ {
		name := encoding[i]
		if name == "" {
			name = ".notdef"
		}
		nameString := fmt.Sprintf("%q,", name)
		var valString string
		if name != ".notdef" {
			valString = fmt.Sprintf(" %q", val[i])
		}
		fmt.Fprintf(w, "\t%-*s// %-3d 0x%02x \\%03o%s\n",
			wd, nameString, i, i, i, valString)
	}
	fmt.Fprintln(w, "}")
	return nil
}

var header = `// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

// Code generated .* DO NOT EDIT\.$

package pdfenc

`

var comment = [4]string{
	``,
	`// MacRomanEncoding is the PDF version of the MacOS standard encoding for Latin
// text in Western writing systems.
//
// See Appendix D.2 of PDF 32000-1:2008.
`,
	``,
	``,
}
