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

package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
)

const (
	margin      = 72.0
	titleY      = 800.0
	cellTopY    = 620.0
	cellBottomY = 500.0
	cellWidth   = 220.0
	cellGap     = 11.0
	cellPadX    = 18.0
	cellPadY    = 38.0 // text origin sits this far above cellBottomY
	crosshair   = 4.0
	tickLength  = 60.0
	footerY     = 460.0
	wrapWidth   = 451.0 // page width − 2*margin

	controlText = "Hello!"
)

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}
	return page.Close()
}
