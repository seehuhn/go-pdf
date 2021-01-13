package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf"
)

func getNames() <-chan string {
	fd, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	c := make(chan string)
	go func(c chan<- string) {
		scanner := bufio.NewScanner(fd)
		for scanner.Scan() {
			c <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			log.Println("cannot read more file names:", err)
		}

		fd.Close()
		close(c)
	}(c)
	return c
}

func doOneFile(fname string) error {
	r, err := pdf.Open(fname)
	if err != nil {
		return err
	}
	defer r.Close()

	pages, err := r.GetDict(r.Catalog.Pages)
	if err != nil {
		return err
	}

	count, err := r.GetInt(pages["Count"])
	if err != nil {
		return err
	}
	// fmt.Println(count, fname)
	_ = count

	return nil
}

func main() {
	total := 0
	errors := 0
	c := getNames()
	for fname := range c {
		total++
		err := doOneFile(fname)
		if err != nil {
			fmt.Println(fname+":", err)
			errors++
		}
	}
	fmt.Println(total, "files,", errors, "errors")
}
