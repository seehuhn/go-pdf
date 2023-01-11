// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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
	"log"
	"os"
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/header"
)

func tryFont(fname string) error {
	fmt.Println()
	fmt.Println("#", fname)

	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()
	header, err := header.Read(fd)
	if err != nil {
		return err
	}

	cmapData, err := header.ReadTableBytes(fd, "cmap")
	if err != nil {
		return err
	}
	fmt.Printf("cmap table: %d bytes\n", len(cmapData))
	subtables, err := cmap.Decode(cmapData)
	if err != nil {
		return err
	}
	fmt.Printf("%d subtables\n", len(subtables))

	keys := maps.Keys(subtables)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].PlatformID != keys[j].PlatformID {
			return keys[i].PlatformID < keys[j].PlatformID
		}
		if keys[i].EncodingID != keys[j].EncodingID {
			return keys[i].EncodingID < keys[j].EncodingID
		}
		return keys[i].Language < keys[j].Language
	})
	for i, key := range keys {
		subtableData := subtables[key]
		format := uint16(subtableData[0])<<8 | uint16(subtableData[1])
		fmt.Println()
		fmt.Printf("## subtable %d: PlatformID=%d, EncodingID=%d, language=%d\n",
			i+1, key.PlatformID, key.EncodingID, key.Language)
		fmt.Printf("format: %d\n", format)
		fmt.Printf("length: %d bytes\n", len(subtables[key]))

		subtable, err := subtables.Get(key)
		if err != nil {
			fmt.Println("cannot parse subtable:", err)
			continue
		}

		_ = subtable
	}

	return nil
}

func main() {
	fonts := os.Args[1:]
	fmt.Printf("analyzing %d fonts\n", len(fonts))
	for _, fname := range fonts {
		err := tryFont(fname)
		if err != nil {
			log.Fatal(err)
		}
	}
}
