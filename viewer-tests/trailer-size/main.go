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

// Command trailer-size writes a PDF file which probes whether a viewer
// treats objects numbered beyond the trailer /Size entry as missing.
//
// The file is built by hand so that object numbers and /Size are under
// exact control.  Every object has an entry in the cross-reference table;
// only /Size claims that some of them do not exist.
//
// Object numbers:
//
//	 1  catalog, with /OCProperties << /OCGs [6 0 R] /D << /OFF 7 0 R >> >>
//	 2  page tree
//	 3  page, with /Annots 8 0 R
//	 4  font
//	 5  page content
//	 6  optional content group
//	 7  /OFF array [6 0 R]       <- numbered exactly /Size
//	 8  /Annots array [9 0 R]    <- numbered above /Size
//	 9  annotation
//	10  appearance stream of the annotation
//
// /Size is 7.  Both cut-off objects are the values of optional entries,
// and a null value for an optional entry is the same as an absent one, so
// a reader which drops either object violates nothing.
//
// Test A: the page content paints "yes" and then, inside the optional
// content group, "no" over it.  A reader which resolves the /OFF array
// switches the group off and reveals "yes".
//
// Test B: the page content paints "yes".  A reader which resolves the
// /Annots array draws the annotation, whose appearance paints "no" over it.
package main

import (
	"bytes"
	"fmt"
	"os"
)

const size = 7

// A4 in points; text positions are in points
const (
	pageW = 595
	pageH = 842

	marginX = 72
	answerX = 400
	answerW = 80
	answerH = 22

	rowAY = 610 // test A: object numbered exactly /Size
	rowBY = 500 // test B: object numbered above /Size
	noteY = 400 // closing note
)

// overlay paints the answer box with answer in it.  The origin is the
// lower left corner of the box; the border is inset so that it survives
// clipping to a bounding box of the same size.
func overlay(x, y int, answer string) string {
	return fmt.Sprintf("0.95 g %d %d %d %d re f 0.4 G 0.75 w %g %g %g %g re S 0 g BT /F1 12 Tf %d %d Td (%s) Tj ET\n",
		x, y, answerW, answerH,
		float64(x)+0.5, float64(y)+0.5, float64(answerW)-1, float64(answerH)-1,
		x+10, y+7, answer)
}

func pageContent() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "BT /F1 16 Tf %d %d Td (Does the reader obey the trailer /Size?) Tj ET\n", marginX, pageH-100)
	fmt.Fprintf(&buf, "0.6 G 0.5 w %d %d m %d %d l S\n", marginX, pageH-115, pageW-marginX, pageH-115)
	lines(&buf, marginX, pageH-140, []string{
		"The cross-reference table lists objects 1 to 10, but the trailer says /Size 7.",
		"Each test depends on an object whose number is at or beyond /Size.",
	})

	row(&buf, rowAY, "A", "Object numbered exactly /Size is read:", "yes", []string{
		"Object 7 is the /OFF array of the optional content configuration.",
		"Reading it switches off the group which hides the answer \"yes\".",
	})
	// the overlay for test A is optional content
	fmt.Fprintf(&buf, "/OC /P1 BDC\n%sEMC\n", overlay(answerX, rowAY, "no"))

	row(&buf, rowBY, "B", "Objects above /Size are ignored:", "yes", []string{
		"Object 8 is the page's /Annots array.  Reading it draws an",
		"annotation whose appearance paints \"no\" over the answer.",
	})

	lines(&buf, marginX, noteY, []string{
		"The specification defines /Size as one more than the highest object number,",
		"but only says that objects numbered greater than /Size shall be ignored.",
		"Test A probes this gap.  Its result is only meaningful if test B says \"yes\".",
	})

	return buf.String()
}

func row(buf *bytes.Buffer, y int, letter, label, answer string, notes []string) {
	fmt.Fprintf(buf, "BT /F1 12 Tf %d %d Td (%s) Tj ET\n", marginX, y+7, letter)
	fmt.Fprintf(buf, "BT /F1 12 Tf %d %d Td (%s) Tj ET\n", marginX+24, y+7, label)
	buf.WriteString(overlay(answerX, y, answer))
	lines(buf, marginX+24, y-16, notes)
}

func lines(buf *bytes.Buffer, x, y int, notes []string) {
	fmt.Fprintf(buf, "0.3 g BT /F1 10 Tf 13 TL %d %d Td\n", x, y)
	for _, line := range notes {
		fmt.Fprintf(buf, "(%s) '\n", line)
	}
	buf.WriteString("ET 0 g\n")
}

func stream(dict, body string) string {
	if dict != "" {
		dict += " "
	}
	return fmt.Sprintf("<< %s/Length %d >>\nstream\n%sendstream", dict, len(body), body)
}

func main() {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /OCProperties << /OCGs [6 0 R] /D << /OFF 7 0 R >> >> >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /Font << /F1 4 0 R >> /Properties << /P1 6 0 R >> >> /Contents 5 0 R /Annots 8 0 R >>", pageW, pageH),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		stream("", pageContent()),
		"<< /Type /OCG /Name (Test A overlay) >>",
		"[6 0 R]",
		"[9 0 R]",
		fmt.Sprintf("<< /Type /Annot /Subtype /Square /Rect [%d %d %d %d] /F 4 /AP << /N 10 0 R >> >>",
			answerX, rowBY, answerX+answerW, rowBY+answerH),
		stream(fmt.Sprintf("/Type /XObject /Subtype /Form /BBox [0 0 %d %d] /Resources << /Font << /F1 4 0 R >> >>",
			answerW, answerH), overlay(0, 0, "no")),
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}

	startXRef := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", size, startXRef)

	err := os.WriteFile("test.pdf", buf.Bytes(), 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
