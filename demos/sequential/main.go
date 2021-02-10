package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"seehuhn.de/go/pdf"
)

func main() {
	fname := os.Args[1]
	r, err := pdf.Open(fname)
	if err != nil {
		log.Fatal(err)
	}

	for {
		obj, ref, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		fmt.Println(ref, obj)
	}
}
