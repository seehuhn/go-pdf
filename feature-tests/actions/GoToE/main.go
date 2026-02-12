// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/nametree"
)

var paper = document.A5

const (
	boxWidth  = 100.0
	boxTitleH = 20.0
	boxPageH  = 20.0
	boxTotalH = boxTitleH + 3*boxPageH

	numPages = 3
)

func main() {
	fmt.Println("Generating GoToE test PDF...")
	if err := generateTest(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created test.pdf")
}

func generateTest() error {
	nav := newNavInfo()

	child11Data, err := generateChildPDF("child1.1", nil, nav)
	if err != nil {
		return fmt.Errorf("generating child1.1: %w", err)
	}

	child1Data, err := generateChildPDF("child1", child11Data, nav)
	if err != nil {
		return fmt.Errorf("generating child1: %w", err)
	}

	child2Data, err := generateChildPDF("child2", nil, nav)
	if err != nil {
		return fmt.Errorf("generating child2: %w", err)
	}

	return generateMainPDF(child1Data, child2Data, nav)
}

type navInfo struct {
	tree             map[string]*docInfo
	child2AnnotIndex int
}

type docInfo struct {
	name        string
	parent      string
	embedMethod string
	embedName   string
}

func newNavInfo() *navInfo {
	return &navInfo{
		tree: map[string]*docInfo{
			"main":     {name: "main"},
			"child1":   {name: "child1", parent: "main", embedMethod: "named", embedName: "child1"},
			"child1.1": {name: "child1.1", parent: "child1", embedMethod: "named", embedName: "child1.1"},
			"child2":   {name: "child2", parent: "main", embedMethod: "annotation"},
		},
		child2AnnotIndex: 0,
	}
}

func (nav *navInfo) buildTarget(from, to string) action.Target {
	if from == to {
		return nil
	}

	fromPath := nav.getPathToRoot(from)
	toPath := nav.getPathToRoot(to)

	commonIdx := 0
	for commonIdx < len(fromPath) && commonIdx < len(toPath) && fromPath[commonIdx] == toPath[commonIdx] {
		commonIdx++
	}

	var target action.Target

	for i := len(toPath) - 1; i >= commonIdx; i-- {
		info := nav.tree[toPath[i]]
		switch info.embedMethod {
		case "named":
			target = &action.TargetNamedChild{
				Name: pdf.String(info.embedName),
				Next: target,
			}
		case "annotation":
			target = &action.TargetAnnotationChild{
				Page:       pdf.Integer(0),
				Annotation: pdf.Integer(nav.child2AnnotIndex),
				Next:       target,
			}
		}
	}

	stepsUp := len(fromPath) - commonIdx
	for range stepsUp {
		target = &action.TargetParent{Next: target}
	}

	return target
}

func (nav *navInfo) getPathToRoot(docName string) []string {
	var path []string
	for docName != "" {
		path = append([]string{docName}, path...)
		docName = nav.tree[docName].parent
	}
	return path
}

func generateChildPDF(docName string, embeddedChild []byte, nav *navInfo) ([]byte, error) {
	buf := &bytes.Buffer{}
	doc, err := document.WriteMultiPage(buf, paper, pdf.V1_7, nil)
	if err != nil {
		return nil, err
	}

	if embeddedChild != nil {
		for name, info := range nav.tree {
			if info.parent == docName {
				if err := embedFile(doc, name, embeddedChild); err != nil {
					return nil, err
				}
				break
			}
		}
	}

	pageRefs := allocatePageRefs(doc, numPages)
	if err := generatePages(doc, docName, pageRefs, nav); err != nil {
		return nil, err
	}

	if err := doc.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func generateMainPDF(child1Data, child2Data []byte, nav *navInfo) error {
	doc, err := document.CreateMultiPage("test.pdf", paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	if err := embedFile(doc, "child1", child1Data); err != nil {
		return err
	}

	pageRefs := allocatePageRefs(doc, numPages)

	for pageNum := range numPages {
		page := doc.AddPage()
		page.Ref = pageRefs[pageNum]

		if pageNum == 0 {
			addFileAttachment(page, "child2.pdf", child2Data)
		}

		if err := drawTree(page, "main", pageNum, pageRefs, nav); err != nil {
			return err
		}

		if err := page.Close(); err != nil {
			return err
		}
	}

	return doc.Close()
}

func createFileStream(data []byte) *file.Stream {
	return &file.Stream{
		MimeType: "application/pdf",
		ModDate:  time.Now(),
		Size:     int64(len(data)),
		WriteData: func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		},
	}
}

func embedFile(doc *document.MultiPage, name string, data []byte) error {
	spec := &file.Specification{
		FileName:        name + ".pdf",
		FileNameUnicode: name + ".pdf",
		Description:     name + " Document",
		EmbeddedFiles:   map[string]*file.Stream{"F": createFileStream(data)},
	}

	ref, err := doc.RM.Embed(spec)
	if err != nil {
		return err
	}

	treeRef, err := nametree.WriteMap(doc.Out, map[pdf.Name]pdf.Object{
		pdf.Name(name): ref,
	})
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

func addFileAttachment(page *document.Page, name string, data []byte) {
	spec := &file.Specification{
		FileName:        name,
		FileNameUnicode: name,
		EmbeddedFiles:   map[string]*file.Stream{"F": createFileStream(data)},
	}

	annot := &annotation.FileAttachment{
		Common: annotation.Common{
			Rect:     pdf.Rectangle{LLx: 10, LLy: paper.URy - 30, URx: 20, URy: paper.URy - 20},
			Contents: name,
		},
		FS: spec,
	}

	page.Page.Annots = append(page.Page.Annots, annot)
}

func allocatePageRefs(doc *document.MultiPage, n int) []pdf.Reference {
	refs := make([]pdf.Reference, n)
	for i := range refs {
		refs[i] = doc.Out.Alloc()
	}
	return refs
}

func generatePages(doc *document.MultiPage, docName string, pageRefs []pdf.Reference, nav *navInfo) error {
	for pageNum := range pageRefs {
		page := doc.AddPage()
		page.Ref = pageRefs[pageNum]

		if err := drawTree(page, docName, pageNum, pageRefs, nav); err != nil {
			return err
		}

		if err := page.Close(); err != nil {
			return err
		}
	}
	return nil
}

type position struct {
	x, y float64
}

var boxPositions = map[string]position{
	"main":     {x: 160, y: 515},
	"child1":   {x: 70, y: 380},
	"child2":   {x: 250, y: 380},
	"child1.1": {x: 70, y: 245},
}

func drawTree(page *document.Page, currentDoc string, currentPage int, pageRefs []pdf.Reference, nav *navInfo) error {
	drawConnections(page)

	font := standard.Helvetica.New()
	for docName, pos := range boxPositions {
		err := drawDocBox(page, font, docName, pos.x, pos.y, currentDoc, currentPage, pageRefs, nav)
		if err != nil {
			return err
		}
	}

	return nil
}

func drawConnections(page *document.Page) {
	page.PushGraphicsState()
	defer page.PopGraphicsState()

	page.SetLineWidth(1.0)
	page.SetStrokeColor(color.DeviceGray(0))

	connectBoxes := func(parent, child string) {
		p1, p2 := boxPositions[parent], boxPositions[child]
		page.MoveTo(p1.x+boxWidth/2, p1.y-boxTotalH)
		page.LineTo(p2.x+boxWidth/2, p2.y)
	}

	connectBoxes("main", "child1")
	connectBoxes("main", "child2")
	connectBoxes("child1", "child1.1")

	page.Stroke()
}

func drawDocBox(page *document.Page, font font.Layouter, docName string, x, y float64, currentDoc string, currentPage int, pageRefs []pdf.Reference, nav *navInfo) error {
	titleY := y - boxTitleH
	drawFilledBox(page, x, titleY, boxWidth, boxTitleH, color.DeviceGray(1))
	drawTextAt(page, font, x+3, titleY+boxTitleH/2-3, 10, docName)

	for i := range numPages {
		pageY := y - boxTitleH - float64(i+1)*boxPageH
		isCurrentPage := docName == currentDoc && i == currentPage

		fillColor := color.DeviceGray(1)
		if isCurrentPage {
			fillColor = color.DeviceGray(0.8)
		}

		drawFilledBox(page, x, pageY, boxWidth, boxPageH, fillColor)
		drawTextAt(page, font, x+3, pageY+boxPageH/2-2, 8, fmt.Sprintf("page %d", i+1))

		if !isCurrentPage {
			rect := pdf.Rectangle{LLx: x, LLy: pageY, URx: x + boxWidth, URy: pageY + boxPageH}
			err := addPageLink(page, currentDoc, docName, i, rect, pageRefs, nav)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func drawFilledBox(page *document.Page, x, y, width, height float64, fill color.Color) {
	page.PushGraphicsState()
	defer page.PopGraphicsState()

	page.SetFillColor(fill)
	page.SetStrokeColor(color.DeviceGray(0))
	page.SetLineWidth(1.0)
	page.Rectangle(x, y, width, height)
	page.FillAndStroke()
}

func drawTextAt(page *document.Page, font font.Layouter, x, y, size float64, text string) {
	page.PushGraphicsState()
	defer page.PopGraphicsState()

	page.TextBegin()
	page.TextSetFont(font, size)
	page.SetFillColor(color.DeviceGray(0))
	page.TextFirstLine(x, y)
	page.TextShow(text)
	page.TextEnd()
}

func addPageLink(page *document.Page, fromDoc, toDoc string, toPage int, rect pdf.Rectangle, pageRefs []pdf.Reference, nav *navInfo) error {
	link := &annotation.Link{Common: annotation.Common{Rect: rect}}

	isInternalLink := fromDoc == toDoc && pageRefs != nil
	if isInternalLink {
		link.Destination = &destination.Fit{Page: destination.Target(pageRefs[toPage])}
	} else {
		target := nav.buildTarget(fromDoc, toDoc)
		link.Action = &action.GoToE{
			T:         target,
			D:         &destination.Fit{Page: destination.Target(pdf.Integer(toPage))},
			NewWindow: action.NewWindowReplace,
		}
	}

	page.Page.Annots = append(page.Page.Annots, link)

	return nil
}
