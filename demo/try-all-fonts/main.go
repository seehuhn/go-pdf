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
	"log"
	"os"

	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/sfntcff"
)

func tryFont(fname string) error {
	r, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer r.Close()

	// header, err := table.ReadHeader(r)
	// if err != nil {
	// 	return err
	// }

	// for _, tableName := range []string{"GPOS", "GSUB"} {
	// 	rec, ok := header.Toc[tableName]
	// 	if !ok {
	// 		continue
	// 	}
	// 	r := io.NewSectionReader(r, int64(rec.Offset), int64(rec.Length))

	// 	_, err = gtab.Read(tableName, r)
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	info, err := sfntcff.Read(r)
	if err != nil {
		return err
	}

	if info.Gsub == nil || len(info.Gsub.LookupList) == 0 {
		return nil
	}
	lookups := info.Gsub.LookupList
	for _, lookup := range lookups {
		for _, subtable := range lookup.Subtables {
			switch l := subtable.(type) {
			case *gtab.SeqContext1:
				for _, rules := range l.Rules {
					for _, rule := range rules {
						printActions(rule.Actions, lookups)
					}
				}
			case *gtab.SeqContext2:
				for _, rules := range l.Rules {
					for _, rule := range rules {
						printActions(rule.Actions, lookups)
					}
				}
			case *gtab.SeqContext3:
				printActions(l.Actions, lookups)
			case *gtab.ChainedSeqContext1:
				for _, rules := range l.Rules {
					for _, rule := range rules {
						printActions(rule.Actions, lookups)
					}
				}
			case *gtab.ChainedSeqContext2:
				for _, rules := range l.Rules {
					for _, rule := range rules {
						printActions(rule.Actions, lookups)
					}
				}
			case *gtab.ChainedSeqContext3:
				printActions(l.Actions, lookups)
			}
		}
	}

	return nil
}

func printActions(actions gtab.Nested, lookups gtab.LookupList) {
	for _, a := range actions {
		fmt.Println("GSUB nested", lookups[a.LookupListIndex].Meta.LookupType)
	}
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
