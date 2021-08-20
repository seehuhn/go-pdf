package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"

	"seehuhn.de/go/pdf/font/names"
)

func main() {
	fd, err := os.Open("tmp")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	var from [256]rune
	for i := range from {
		from[i] = unicode.ReplacementChar
	}

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		line := scanner.Text()
		ff := strings.Fields(line)
		cc, err := strconv.ParseInt(ff[0], 8, 32)
		if err != nil {
			log.Fatal(err)
		}
		code := int(cc)
		name := ff[1]

		rr := names.ToUnicode(name, false)
		if len(rr) != 1 || code < 0 || code > 256 {
			log.Fatal(name)
		}
		from[code] = rr[0]
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 256; i++ {
		r := from[i]
		rString := fmt.Sprintf("0x%04x", r)
		if r == unicode.ReplacementChar {
			rString = "noRune"
		}
		if i%8 == 0 {
			fmt.Print("\n\t")
		}
		fmt.Print(rString + ", ")
	}
	fmt.Println()
	for i := 0; i < 256; i++ {
		r := from[i]
		if r == unicode.ReplacementChar {
			continue
		}
		if r != rune(i) {
			fmt.Printf("\t0x%04x: %d,\n", r, i)
		}
	}
}
