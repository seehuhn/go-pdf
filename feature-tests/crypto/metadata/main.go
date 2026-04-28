// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// This program creates an encrypted PDF whose XMP metadata stream is
// stored in plaintext and padded to a fixed length, so that the metadata
// can be edited in place without re-encrypting or rewriting the host file.
//
// Two pieces work together:
//
//   - WriterOptions.UnencryptedMetadata=true tells the standard security
//     handler to set /EncryptMetadata false in the encrypt dictionary,
//     exempting the document-level metadata stream from document encryption.
//   - metadata.Stream.PadToLength=N pads the XMP packet to N bytes (no
//     compression) and emits the writable trailer <?xpacket end="w"?>.
//
// External tools can locate the XMP packet by scanning for its marker
// string and rewrite the bytes in place up to the padded length.
package main

import (
	"log"
	"time"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/xmp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	opt := &pdf.WriterOptions{
		UserPassword:        "user",
		OwnerPassword:       "owner",
		UnencryptedMetadata: true,
	}
	doc, err := document.CreateSinglePage("test.pdf", document.A4r, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	doc.TextSetFont(font.Must(standard.HelveticaBold.New()), 50)
	doc.TextBegin()
	doc.TextFirstLine(50, 420)
	doc.TextShow("Editable metadata demo")
	doc.TextEnd()

	dc := &xmp.DublinCore{}
	dc.Title.Set(language.English, "Editable Metadata Demo")
	dc.Creator.Append(xmp.NewProperName("Quire"))
	xmpInfo := &xmp.Basic{
		CreateDate: xmp.NewDate(time.Now()),
		ModifyDate: xmp.NewDate(time.Now()),
	}
	pdfInfo := &xmp.PDF{
		Keywords: xmp.NewText("encrypted, padded, editable, XMP"),
		Producer: xmp.NewAgentName("seehuhn.de/go/pdf/feature-tests/crypto/metadata"),
	}

	packet := xmp.NewPacket()
	if err := packet.Set(dc, xmpInfo, pdfInfo); err != nil {
		return err
	}

	ref, err := doc.RM.Embed(&metadata.Stream{
		Data:        packet,
		PadToLength: 2048,
	})
	if err != nil {
		return err
	}
	doc.Out.GetMeta().Catalog.Metadata = ref.(pdf.Reference)

	return doc.Close()
}
