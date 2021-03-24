package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf/font/truetype"
)

var minCharSet = map[rune]bool{
	'a': true,
	'z': true,
	'A': true,
	'Z': true,
	'ﬀ': true,
	'ﬁ': true,
	'ﬂ': true,
	'ț': true,
	'â': true,
}

func tryFont(fname string) error {
	tt, err := truetype.Open(fname)
	if err != nil {
		return err
	}
	defer tt.Close()

	// fmt.Printf("%08x %5t %5t %5t\n", tt.Offsets.ScalerType,
	// 	tt.Tables["glyf"] != nil, tt.Tables["CFF "] != nil, tt.Tables["CFF2"] != nil)

	return nil
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
			fmt.Println(fname+":", err)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("main loop failed:", err)
	}
}
