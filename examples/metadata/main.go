// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

import (
	"log"
	"time"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/xmp"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	doc, err := document.CreateSinglePage("test.pdf", document.A4r, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	font, err := type1.HelveticaBold.Embed(doc.Out, nil)
	if err != nil {
		return err
	}

	doc.TextSetFont(font, 50)
	doc.TextBegin()
	doc.TextFirstLine(50, 420)
	doc.TextShow("Hello, World!")
	doc.TextEnd()

	now := time.Now()

	dc := &xmp.DublinCore{}
	dc.Title.Set(language.MustParse("x-default"), "Test Document")
	dc.Title.Set(language.English, "Test Document")
	dc.Title.Set(language.German, "Testdatei")
	dc.Creator.Append(xmp.NewProperName("John Doe"))
	dc.Creator.Append(xmp.NewProperName("Jane Smith"))
	dc.Creator.Append(xmp.NewProperName("Michael Lee"))
	dc.Description.Set(language.MustParse("x-default"), "This is a test document.")
	dc.Description.Set(language.English, "This is a test document.")
	dc.Description.Set(language.German, "Dies ist eine Testdatei.")
	xmpInfo := &xmp.Basic{}
	xmpInfo.CreateDate = xmp.NewDate(now)
	xmpInfo.ModifyDate = xmp.NewDate(now)
	pdfInfo := &PDF{}
	pdfInfo.Keywords = xmp.NewText("test, XMP, metadata")
	pdfInfo.Producer = xmp.NewAgentName("seehuhn.de/go/pdf/examples/metadata")

	metadata := xmp.NewPacket()
	metadata.Set(dc, xmpInfo, pdfInfo)

	stmRef := doc.Out.Alloc()
	stmDict := pdf.Dict{
		"Type":    pdf.Name("Metadata"),
		"Subtype": pdf.Name("XML"),
	}
	stm, err := doc.Out.OpenStream(stmRef, stmDict)
	if err != nil {
		return err
	}
	xmlOpt := &xmp.PacketOptions{
		Pretty: true, // TODO(voss): tie to the pdf.HumanReadable flag
	}
	err = metadata.Write(stm, xmlOpt)
	if err != nil {
		return err
	}
	err = stm.Close()
	if err != nil {
		return err
	}

	doc.Out.GetMeta().Catalog.Metadata = stmRef

	return doc.Close()
}

// PDF is the XMP namespace for PDF metadata.
// See https://developer.adobe.com/xmp/docs/XMPNamespaces/pdf/
type PDF struct {
	_          xmp.Namespace `xmp:"http://ns.adobe.com/pdf/1.3/"`
	_          xmp.Prefix    `xmp:"pdf"`
	Keywords   xmp.Text
	PDFVersion xmp.Text
	Producer   xmp.AgentName
	Trapped    xmp.Text
}
