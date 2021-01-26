package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"

	"seehuhn.de/go/pdf"
)

// Info represents the information from a PDF /Info dictionary.
type Info struct {
	Title        string    `pdf:"text string,optional"`
	Author       string    `pdf:"text string,optional"`
	Subject      string    `pdf:"text string,optional"`
	Keywords     string    `pdf:"text string,optional"`
	Creator      string    `pdf:"text string,optional"`
	Producer     string    `pdf:"text string,optional"`
	CreationDate time.Time `pdf:"optional"`
	ModDate      time.Time `pdf:"optional"`
	Trapped      pdf.Name  `pdf:"optional,allowstring"`

	Custom map[string]string `pdf:"extra"`
}

// Catalog represents the information from a PDF /Root dictionary.
type Catalog struct {
	_                 struct{}   `pdf:"Type=Catalog"`
	Version           pdf.Name   `pdf:"optional,allowstring"`
	Extensions        pdf.Object `pdf:"optional"`
	Pages             *pdf.Reference
	PageLabels        pdf.Object     `pdf:"optional"`
	Names             pdf.Object     `pdf:"optional"`
	Dests             pdf.Object     `pdf:"optional"`
	ViewerPreferences pdf.Object     `pdf:"optional"`
	PageLayout        pdf.Name       `pdf:"optional"`
	PageMode          pdf.Name       `pdf:"optional"`
	Outlines          *pdf.Reference `pdf:"optional"`
	Threads           *pdf.Reference `pdf:"optional"`
	OpenAction        pdf.Object     `pdf:"optional"`
	AA                pdf.Object     `pdf:"optional"`
	URI               pdf.Object     `pdf:"optional"`
	AcroForm          pdf.Object     `pdf:"optional"`
	MetaData          *pdf.Reference `pdf:"optional"`
	StructTreeRoot    pdf.Object     `pdf:"optional"`
	MarkInfo          pdf.Object     `pdf:"optional"`
	Lang              string         `pdf:"text string,optional"`
	SpiderInfo        pdf.Object     `pdf:"optional"`
	OutputIntents     pdf.Object     `pdf:"optional"`
	PieceInfo         pdf.Object     `pdf:"optional"`
	OCProperties      pdf.Object     `pdf:"optional"`
	Perms             pdf.Object     `pdf:"optional"`
	Legal             pdf.Object     `pdf:"optional"`
	Requirements      pdf.Object     `pdf:"optional"`
	Collection        pdf.Object     `pdf:"optional"`
	NeedsRendering    bool           `pdf:"optional"`
}

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
	catalog := &Catalog{}
	root.AsStruct(catalog, r.Get)
	pages, err := r.GetDict(catalog.Pages)
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
