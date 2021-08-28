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
	"fmt"
	"io"
	"log"
	"os"
	"unicode"

	"seehuhn.de/go/pdf/font/sfnt"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dump-tt-tables font.ttf [name]")
		os.Exit(1)
	}

	tt, err := sfnt.Open(args[0])
	if err != nil {
		log.Fatal(err)
	}
	if len(args) == 1 {
		fmt.Println(" name |     offset |     length |   checksum")
		fmt.Println("------+------------+------------+-----------")
		records := tt.Header.Records
		for i := range records {
			name := records[i].Tag.String()
			extra := ""
			body, err := tt.Header.ReadTableBytes(tt.Fd, name)
			if err != nil {
				log.Fatal(err)
			}
			if name == "head" {
				body[8] = 0
				body[9] = 0
				body[10] = 0
				body[11] = 0
			}
			computedSum := sfnt.Checksum(body)
			if computedSum != records[i].CheckSum {
				extra = fmt.Sprintf(" != 0x%08x", computedSum)
			}

			fmt.Printf(" %4s |%11d |%11d | 0x%08x%s\n",
				name, records[i].Offset, records[i].Length,
				records[i].CheckSum, extra)
		}
		return
	}

	name := args[1]
	table := tt.Header.Find(name)
	if table == nil {
		log.Fatalf("table %q not found", name)
	}
	tableFd := io.NewSectionReader(tt.Fd, int64(table.Offset), int64(table.Length))
	var buf [16]byte
	pos := 0

	fmt.Printf("table %q (%d bytes)\n\n", name, table.Length)
	for {
		n, err := io.ReadFull(tableFd, buf[:])
		if n > 0 {
			hex := fmt.Sprintf("% 02x", buf[:n])
			if len(hex) > 3*8 {
				hex = hex[:3*8] + " " + hex[3*8:]
			}

			var rr []rune
			for _, c := range buf[:n] {
				if len(rr) == 8 {
					rr = append(rr, ' ')
				}
				r := rune(c)
				if unicode.IsGraphic(r) {
					rr = append(rr, r)
				} else {
					rr = append(rr, '.')
				}
			}
			ascii := string(rr)

			fmt.Printf("%08x  %-49s  %s\n", pos, hex, ascii)
			pos += n
		}

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
	}
}
