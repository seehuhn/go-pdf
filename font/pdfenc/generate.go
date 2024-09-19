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

// The file "data" was extracted from table D.2 in the PDF spec.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf"
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

	err = writeLatin(data, "latin-gen.go")

	err = writeTable(data, "standard-gen.go", "standardEncoding", 0)
	if err != nil {
		return err
	}

	err = writeTable(data, "macroman-gen.go", "macRomanEncoding", 1)
	if err != nil {
		return err
	}

	err = writeTable(data, "winansi-gen.go", "winAnsiEncoding", 2)
	if err != nil {
		return err
	}

	err = writeTable(data, "pdfdoc-gen.go", "pdfDocEncoding", 3)
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

	err = writeTable(data, "macexpert-gen.go", "macExpertEncoding", 0)
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

func writeLatin(data map[pdf.Name]record, fname string) error {
	w, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer w.Close()

	glyphNames := maps.Keys(data)
	sort.Slice(glyphNames, func(i, j int) bool {
		return glyphNames[i] < glyphNames[j]
	})

	_, err = w.WriteString(header)
	if err != nil {
		return err
	}

	_, err = w.WriteString("var standardLatinHas = map[string]bool{\n")
	if err != nil {
		return err
	}
	for _, name := range glyphNames {
		quoted := fmt.Sprintf("%q:", name)
		_, err = fmt.Fprintf(w, "\t%-18strue,\n", quoted)
		if err != nil {
			return err
		}
	}
	_, err = w.WriteString("}\n\n")
	if err != nil {
		return err
	}

	return nil
}

func writeTable(data map[pdf.Name]record, fname string, encName string, col int) error {
	buf := &bytes.Buffer{}

	_, err := buf.WriteString(header)
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
	case "winAnsiEncoding":
		// Footnote 5 after table D.2: The hyphen (U+002D) character is also
		// encoded as 255 (octal) in WinAnsiEncoding.
		encoding[0o255] = "hyphen"
		val[0o255] = "-"
		// Footnote 6 after table D.2: The space (U+0020) character is also
		// encoded [...] as 240 (octal) in WinAnsiEncoding.
		encoding[0o240] = "space"
		val[0o240] = " "
	case "macRomanEncoding":
		// Footnote 6 after table D.2: The space (U+0020) character is also
		// encoded as 312 (octal) in MacRomanEncoding [...].
		encoding[0o312] = "space"
		val[0o312] = " "
	}

	fmt.Fprintf(buf, "var %s = [256]string{\n", encName)
	var names []string
	for i := 0; i < 256; i++ {
		name := encoding[i]
		if name == "" {
			name = ".notdef"
		}
		var valString string
		if name != ".notdef" {
			valString = fmt.Sprintf(" %q", val[i])
			names = append(names, string(name))
		}
		fmt.Fprintf(buf, "%q, // %-3d 0x%02x \\%03o%s\n",
			name, i, i, i, valString)
	}
	fmt.Fprintln(buf, "}")
	fmt.Fprintln(buf)

	fmt.Fprintf(buf, "var %sHas = map[string]bool{\n", encName)
	sort.Strings(names)
	var prev string
	for _, name := range names {
		if name == prev {
			continue
		}
		prev = name
		fmt.Fprintf(buf, "%q: true,\n", name)
	}
	fmt.Fprintln(buf, "}")

	body, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Println(buf.String())
		return err
	}
	err = os.WriteFile(fname, body, 0644)
	if err != nil {
		return err
	}

	return nil
}

var header = `// Code generated - DO NOT EDIT.

package pdfenc

`
