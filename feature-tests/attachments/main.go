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
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/nametree"
	pdfpage "seehuhn.de/go/pdf/page"
)

var paper = document.A5

const (
	sourceName = "main.go"
	margin     = 50.0
	iconSize   = 24.0
)

func main() {
	fmt.Println("writing test.pdf ...")
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	source, modTime, err := readSource(sourceName)
	if err != nil {
		return err
	}

	opt := &pdf.WriterOptions{HumanReadable: true}
	doc, err := document.CreateMultiPage("test.pdf", paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	// three specs covering the three ways an attachment can appear in a PDF:
	//   mainSpec   — registered in the document catalogue only
	//   sharedSpec — registered in the catalogue AND on a page annotation
	//   onlySpec   — reachable only through a page annotation
	mainSpec := newSpec(sourceName, "text/plain",
		"Go source used to generate this PDF", source, modTime)
	sharedSpec := newTextSpec("shared.txt",
		"Reachable from both the EmbeddedFiles tree and a page annotation.\n",
		"Shared attachment: lives in the name tree and on the page")
	onlySpec := newTextSpec("annot-only.txt",
		"Only reachable through the page-level annotation below.\n",
		"Annotation-only attachment: no catalogue entry")

	// register the two catalogue entries before building the page so that
	// the annotation for shared.txt picks up the same embedded reference
	tree := map[pdf.Name]pdf.Object{}
	if err := registerInCatalogue(doc, tree, sourceName, mainSpec); err != nil {
		return err
	}
	if err := registerInCatalogue(doc, tree, "shared.txt", sharedSpec); err != nil {
		return err
	}

	page := doc.AddPage()
	fbStyle := fallback.NewStyle()

	iconRects := drawDescription(page)

	if err := addFileAttachment(page, fbStyle, sharedSpec,
		iconRects[0], "shared.txt"); err != nil {
		return err
	}
	if err := addFileAttachment(page, fbStyle, onlySpec,
		iconRects[1], "annot-only.txt"); err != nil {
		return err
	}

	if err := finaliseCatalogue(doc, tree); err != nil {
		return err
	}

	if err := page.Close(); err != nil {
		return err
	}
	return doc.Close()
}

// readSource reads the named file from the current working directory and
// returns its contents together with the file's modification time.
func readSource(name string) ([]byte, time.Time, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, time.Time{}, err
	}
	info, err := os.Stat(name)
	if err != nil {
		return nil, time.Time{}, err
	}
	return data, info.ModTime(), nil
}

// newSpec builds a file.Specification wrapping data as a single embedded
// file stream keyed under both F and UF.
func newSpec(name, mime, desc string, data []byte, modTime time.Time) *file.Specification {
	stream := &file.Stream{
		MimeType: mime,
		Size:     int64(len(data)),
		ModDate:  modTime,
		WriteData: func(w io.Writer) error {
			_, err := io.Copy(w, bytes.NewReader(data))
			return err
		},
	}
	return &file.Specification{
		FileName:        name,
		FileNameUnicode: name,
		Description:     desc,
		EmbeddedFiles:   map[string]*file.Stream{"F": stream, "UF": stream},
	}
}

// newTextSpec is shorthand for a small text/plain attachment.
func newTextSpec(name, content, desc string) *file.Specification {
	return newSpec(name, "text/plain", desc, []byte(content), time.Time{})
}

// registerInCatalogue embeds spec and records its indirect reference under
// key in tree.  finaliseCatalogue turns the collected entries into a name
// tree under the catalogue's Names.EmbeddedFiles key.
func registerInCatalogue(doc *document.MultiPage, tree map[pdf.Name]pdf.Object,
	key pdf.Name, spec *file.Specification) error {
	ref, err := doc.RM.Embed(spec)
	if err != nil {
		return err
	}
	tree[key] = ref
	return nil
}

// finaliseCatalogue writes tree as a PDF name tree and installs it as the
// catalogue's EmbeddedFiles entry.  If tree is empty the catalogue is left
// untouched.
func finaliseCatalogue(doc *document.MultiPage, tree map[pdf.Name]pdf.Object) error {
	if len(tree) == 0 {
		return nil
	}
	treeRef, err := nametree.WriteMap(doc.Out, tree)
	if err != nil {
		return err
	}
	catalog := doc.Out.GetMeta().Catalog
	if catalog.Names == nil {
		catalog.Names = pdf.Dict{}
	}
	catalog.Names.(pdf.Dict)["EmbeddedFiles"] = treeRef
	return nil
}

// addFileAttachment creates a FileAttachment annotation at rect pointing
// at spec, generates a fallback appearance for it, and appends it to the
// page's Annots array.  If spec is already embedded in the document (via
// registerInCatalogue), the resource manager re-uses the same reference
// so that a conforming reader's Attachments panel lists the file once.
func addFileAttachment(page *document.Page, style *fallback.Style,
	spec *file.Specification, rect pdf.Rectangle, contents string) error {
	fa := &annotation.FileAttachment{
		Common: annotation.Common{
			Rect:     rect,
			Contents: contents,
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Markup: annotation.Markup{User: "Attachments test"},
		Icon:   annotation.FileAttachmentIconPushPin,
		FS:     spec,
	}
	if err := style.AddAppearance(fa); err != nil {
		return err
	}
	page.Page.Annots = append(page.Page.Annots, pdfpage.AnnotInfo{
		Annot: fa,
		Ref:   page.Out.Alloc(),
	})
	return nil
}

// drawDescription writes the explanatory text and reserves rectangles for
// the two FileAttachment icons.  Returns the rectangles in the same order
// as the corresponding icons appear on the page.
func drawDescription(page *document.Page) [2]pdf.Rectangle {
	title := font.Must(standard.HelveticaBold.New())
	body := font.Must(standard.Helvetica.New())

	y := paper.URy - margin - 20

	page.PushGraphicsState()
	page.TextBegin()
	page.TextSetFont(title, 16)
	page.TextSetMatrix(matrix.Translate(margin, y))
	page.TextShow("Attachments test")
	page.TextEnd()
	page.PopGraphicsState()

	lines := []string{
		"This PDF exercises three ways a file can be attached:",
		"",
		fmt.Sprintf("  • %q — document-level only (EmbeddedFiles tree),", sourceName),
		"    holds the Go source used to generate this PDF.",
		"  • \"shared.txt\" — in the EmbeddedFiles tree AND on the",
		"    page annotation marked below.",
		"  • \"annot-only.txt\" — reachable only via the second",
		"    page annotation below.",
		"",
		"Open the viewer's Attachments panel to inspect and extract.",
	}

	y -= 30
	page.PushGraphicsState()
	page.TextBegin()
	page.TextSetFont(body, 10.5)
	page.TextSetMatrix(matrix.Translate(margin, y))
	for i, line := range lines {
		if i > 0 {
			page.TextSecondLine(0, -14)
		}
		page.TextShow(line)
	}
	page.TextEnd()
	page.PopGraphicsState()

	// two icon rectangles plus their labels
	y -= float64(len(lines))*14 + 20
	iconX := margin + 10

	sharedRect := pdf.Rectangle{
		LLx: iconX, LLy: y - iconSize,
		URx: iconX + iconSize, URy: y,
	}
	drawIconLabel(page, body, sharedRect, "shared.txt (also in catalogue)")

	y -= iconSize + 10
	onlyRect := pdf.Rectangle{
		LLx: iconX, LLy: y - iconSize,
		URx: iconX + iconSize, URy: y,
	}
	drawIconLabel(page, body, onlyRect, "annot-only.txt")

	return [2]pdf.Rectangle{sharedRect, onlyRect}
}

func drawIconLabel(page *document.Page, f font.Layouter, r pdf.Rectangle, label string) {
	page.PushGraphicsState()
	page.TextBegin()
	page.TextSetFont(f, 10)
	page.TextSetMatrix(matrix.Translate(r.URx+10, r.LLy+(r.URy-r.LLy)/2-3))
	page.TextShow(label)
	page.TextEnd()
	page.PopGraphicsState()
}
