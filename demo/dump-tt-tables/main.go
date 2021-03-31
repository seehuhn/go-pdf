package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"unicode"

	"seehuhn.de/go/pdf/font/truetype"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dump-tt-tables font.ttf [name]")
		os.Exit(1)
	}

	tt, err := truetype.Open(args[0])
	if err != nil {
		log.Fatal(err)
	}
	if len(args) == 1 {
		fmt.Println(" name |     offset |     length")
		fmt.Println("------+------------+------------")
		records := tt.Header.Records
		for i := range records {
			fmt.Printf(" %4s |%11d |%11d\n",
				records[i].Tag, records[i].Offset, records[i].Length)
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
