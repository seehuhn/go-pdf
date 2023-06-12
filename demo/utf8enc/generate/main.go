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

package main

import "fmt"

func main() {
	var lines []string
	lines = append(lines, "<00> <7f> 0\n")
	for r := rune(0x80); r <= 0x10_ffff; r += 64 {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}

		s1 := string([]rune{r})
		s2 := string([]rune{r + 63})
		lines = append(lines, fmt.Sprintf("<%02x> <%02x> %d\n",
			[]byte(s1), []byte(s2), r))
	}
	for len(lines) > 0 {
		k := len(lines)
		if k > 100 {
			k = 100
		}
		fmt.Printf("%d begincidrange\n", k)
		for _, line := range lines[:k] {
			fmt.Print(line)
		}
		fmt.Println("endcidrange")
		fmt.Println()
		lines = lines[k:]
	}
}
