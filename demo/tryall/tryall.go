package main

import (
	"bufio"
	"fmt"
	"io"
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

	root, err := r.Catalog()
	if err != nil {
		return err
	}
	catalog := &pdf.Catalog{}
	root.AsStruct(catalog, r.Get)
	pages, err := r.GetDict(catalog.Pages)
	if err != nil {
		return err
	}
	count, err := r.GetInt(pages["Count"])
	if err != nil {
		return err
	}

	_ = count
	// fmt.Println(count, fname)

	for {
		obj, _, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		_ = obj
		// dict, ok := obj.(pdf.Dict)
		// if !ok {
		// 	continue
		// }
		// if dict["Type"] == pdf.Name("Font") {
		// 	fmt.Println(dict["Subtype"])
		// }
	}

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
			sz := "?????????? "
			fi, e2 := os.Stat(fname)
			if e2 == nil {
				sz = fmt.Sprintf("%10d ", fi.Size())
			}
			fmt.Println(sz, fname+":", err)
			errors++
		}
	}
	fmt.Println(total, "files,", errors, "errors")
}
