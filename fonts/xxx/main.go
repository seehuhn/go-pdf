package main

// https://github.com/UglyToad/PdfPig/tree/master/src/UglyToad.PdfPig.Fonts/Encodings

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"seehuhn.de/go/pdf/fonts/type1"
)

func main() {
	fd, err := os.Open("x")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	from := make(map[byte]rune)
	to := make(map[rune]byte)

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		line := scanner.Text()
		ff := strings.Fields(line)
		if len(ff) != 2 {
			continue
		}

		code, err := strconv.ParseUint(ff[0], 8, 8)
		if err != nil {
			log.Fatal(err)
		}

		name := ff[1][1 : len(ff[1])-1]
		value := type1.DecodeGlyphName(name, false)
		if len(value) != 1 {
			panic("fish")
		}

		from[byte(code)] = value[0]
		to[value[0]] = byte(code)
		// fmt.Printf("%2s %20s %03o\n", string(value), name, code)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("var fromEncoding = [256]rune{")
	var row []string
	for i := 0; i < 256; i++ {
		c := byte(i)
		r, ok := from[c]
		if ok {
			row = append(row, fmt.Sprintf(" 0x%04x,", r))
		} else {
			row = append(row, " noRune,")
		}
		if len(row) >= 8 {
			fmt.Println("   " + strings.Join(row, ""))
			row = row[:0]
		}
	}
	fmt.Println("}")
	fmt.Println()

	var keys []rune
	for key, val := range to {
		if rune(val) != key {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return to[keys[i]] < to[keys[j]]
	})
	fmt.Println("var toEncoding = map[rune]byte{")
	for _, key := range keys {
		fmt.Printf("    0x%04x: %d,\n", key, to[key])
	}
	fmt.Println("}")
}
