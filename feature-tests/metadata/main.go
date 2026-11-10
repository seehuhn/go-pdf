// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"log"
	"time"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/xmp"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	now := time.Now()

	dc := &xmp.DublinCore{}
	dc.Title.Default = xmp.NewText("Test Document")
	dc.Title.Set(language.English, "Test Document")
	dc.Title.Set(language.German, "Testdatei")
	dc.Creator.Append(xmp.NewProperName("John Doe"))
	dc.Creator.Append(xmp.NewProperName("Jane Smith"))
	dc.Creator.Append(xmp.NewProperName("Michael Lee"))
	dc.Description.Default = xmp.NewText("This is a test document.")
	dc.Description.Set(language.English, "This is a test document.")
	dc.Description.Set(language.German, "Dies ist eine Testdatei.")
	xmpInfo := &xmp.Basic{}
	xmpInfo.CreateDate = xmp.NewDate(now, xmp.PrecisionSecond)
	xmpInfo.ModifyDate = xmp.NewDate(now, xmp.PrecisionSecond)
	pdfInfo := &xmp.PDF{}
	pdfInfo.Keywords = xmp.NewText("test, XMP, metadata")
	pdfInfo.Producer = xmp.NewAgentName("seehuhn.de/go/pdf/examples/metadata")

	packet := xmp.NewPacket()
	if err := packet.Set(dc, xmpInfo, pdfInfo); err != nil {
		return err
	}

	opt := &pdf.WriterOptions{
		DocumentMetadata: &pdf.MetadataStream{Data: packet},
	}
	doc, err := document.CreateSinglePage("test.pdf", document.A4r, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	font := font.Must(standard.HelveticaBold.New())

	doc.TextSetFont(font, 50)
	doc.TextBegin()
	doc.TextFirstLine(50, 420)
	doc.TextShow("Hello, World!")
	doc.TextEnd()

	return doc.Close()
}
