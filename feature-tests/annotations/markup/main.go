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
	"time"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics/color"
)

var paper = document.A4

func main() {
	fmt.Println("writing test.pdf ...")
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	doc, err := document.CreateMultiPage("test.pdf", paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	// page 1: reply threading
	if err := pageReplyThreading(doc); err != nil {
		return err
	}

	// page 2: grouping and nested replies
	if err := pageGroupingAndNesting(doc); err != nil {
		return err
	}

	// page 3: various markup types
	if err := pageMarkupTypes(doc); err != nil {
		return err
	}

	// page 4: edge cases
	if err := pageEdgeCases(doc); err != nil {
		return err
	}

	return doc.Close()
}

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 12, 0, 0, 0, time.UTC)
}

// pageReplyThreading creates a page with a simple reply chain.
func pageReplyThreading(doc *document.MultiPage) error {
	p := doc.AddPage()

	parent := &annotation.Text{
		Rect:         pdf.Rectangle{LLx: 72, LLy: 700, URx: 96, URy: 724},
		Contents:     pdf.Text("This paragraph needs revision.\rPlease check the references."),
		Color:        color.DeviceRGB{1, 0.8, 0},
		User:         pdf.Text("Alice"),
		CreationDate: date(2025, 3, 10),
		Subject:      pdf.Text("Note"),
		Icon:         annotation.TextIconComment,
	}

	// reserve the parent's reference so the replies can target it via IRT
	parentRef := p.RM.GetReference(parent)

	reply1 := &annotation.Text{
		Rect:         pdf.Rectangle{LLx: 72, LLy: 700, URx: 96, URy: 724},
		Contents:     pdf.Text("I've checked references 3 and 7. Reference 3 is outdated."),
		User:         pdf.Text("Bob"),
		CreationDate: date(2025, 3, 11),
		InReplyTo:    parentRef,
		Subject:      pdf.Text("Reply"),
		Icon:         annotation.TextIconComment,
	}

	reply2 := &annotation.Text{
		Rect:         pdf.Rectangle{LLx: 72, LLy: 700, URx: 96, URy: 724},
		Contents:     pdf.Text("Thanks Bob. I've updated reference 3 to the 2024 edition."),
		User:         pdf.Text("Alice"),
		CreationDate: date(2025, 3, 12),
		InReplyTo:    parentRef,
		Subject:      pdf.Text("Reply"),
		Icon:         annotation.TextIconComment,
	}

	p.Page.AddAnnots(parent, reply1, reply2)
	return p.Close()
}

// pageGroupingAndNesting creates a page with grouped annotations and nested replies.
func pageGroupingAndNesting(doc *document.MultiPage) error {
	p := doc.AddPage()

	// caret + strikeout group: a replacement edit
	caret := &annotation.Caret{
		Rect:         pdf.Rectangle{LLx: 200, LLy: 600, URx: 209, URy: 607},
		Contents:     pdf.Text("colour"),
		Color:        color.DeviceRGB{0, 0, 1},
		User:         pdf.Text("Editor"),
		CreationDate: date(2025, 1, 15),
		Subject:      pdf.Text("Replacement"),
	}

	// reserve the caret's reference so the group members can target it via IRT
	caretRef := p.RM.GetReference(caret)

	strikeout := &annotation.TextMarkup{
		Rect:         pdf.Rectangle{LLx: 150, LLy: 600, URx: 195, URy: 612},
		Color:        color.DeviceRGB{0, 0, 1},
		User:         pdf.Text("Editor"),
		CreationDate: date(2025, 1, 15),
		InReplyTo:    caretRef,
		RT:           "Group",
		Type:         annotation.TextMarkupTypeStrikeOut,
		QuadPoints: []vec.Vec2{
			{X: 150, Y: 612},
			{X: 195, Y: 612},
			{X: 150, Y: 600},
			{X: 195, Y: 600},
		},
	}

	// reply to the caret
	replyToCaret := &annotation.Text{
		Rect:         pdf.Rectangle{LLx: 200, LLy: 600, URx: 224, URy: 624},
		Contents:     pdf.Text("Should we use British or American spelling throughout?"),
		User:         pdf.Text("Author"),
		CreationDate: date(2025, 1, 16),
		InReplyTo:    caretRef,
		Subject:      pdf.Text("Question"),
		Icon:         annotation.TextIconHelp,
	}

	// reserve the reply's reference so the nested reply can target it via IRT
	replyToCaretRef := p.RM.GetReference(replyToCaret)

	// nested reply (reply to the reply)
	nestedReply := &annotation.Text{
		Rect:         pdf.Rectangle{LLx: 200, LLy: 600, URx: 224, URy: 624},
		Contents:     pdf.Text("British spelling for this document."),
		User:         pdf.Text("Editor"),
		CreationDate: date(2025, 1, 17),
		InReplyTo:    replyToCaretRef,
		Subject:      pdf.Text("Answer"),
		Icon:         annotation.TextIconComment,
	}

	// state annotation (review state on the caret)
	stateAnnot := &annotation.Text{
		Rect:         pdf.Rectangle{LLx: 200, LLy: 600, URx: 224, URy: 624},
		User:         pdf.Text("Author"),
		CreationDate: date(2025, 1, 18),
		InReplyTo:    caretRef,
		State:        annotation.TextStateAccepted,
	}

	p.Page.AddAnnots(caret, strikeout, replyToCaret, nestedReply, stateAnnot)
	return p.Close()
}

// pageMarkupTypes creates a page with various markup annotation types.
func pageMarkupTypes(doc *document.MultiPage) error {
	p := doc.AddPage()

	y := 750.0

	// highlight
	hl := &annotation.TextMarkup{
		Rect:         pdf.Rectangle{LLx: 72, LLy: y - 12, URx: 300, URy: y},
		Contents:     pdf.Text("This definition is key to the argument."),
		Color:        color.DeviceRGB{1, 1, 0},
		User:         pdf.Text("Reviewer"),
		CreationDate: date(2025, 2, 1),
		Subject:      pdf.Text("Highlight"),
		Type:         annotation.TextMarkupTypeHighlight,
		QuadPoints: []vec.Vec2{
			{X: 72, Y: y}, {X: 300, Y: y},
			{X: 72, Y: y - 12}, {X: 300, Y: y - 12},
		},
	}
	p.Page.AddAnnots(hl)
	y -= 50

	// underline
	ul := &annotation.TextMarkup{
		Rect:         pdf.Rectangle{LLx: 72, LLy: y - 12, URx: 250, URy: y},
		Contents:     pdf.Text("Verify this claim with the source data."),
		Color:        color.DeviceRGB{0, 0.5, 0},
		User:         pdf.Text("Reviewer"),
		CreationDate: date(2025, 2, 1),
		Subject:      pdf.Text("Underline"),
		Type:         annotation.TextMarkupTypeUnderline,
		QuadPoints: []vec.Vec2{
			{X: 72, Y: y}, {X: 250, Y: y},
			{X: 72, Y: y - 12}, {X: 250, Y: y - 12},
		},
	}
	p.Page.AddAnnots(ul)
	y -= 50

	// squiggly
	sq := &annotation.TextMarkup{
		Rect:         pdf.Rectangle{LLx: 72, LLy: y - 12, URx: 220, URy: y},
		Contents:     pdf.Text("Possible grammatical error here."),
		Color:        color.DeviceRGB{1, 0, 0},
		User:         pdf.Text("Proofreader"),
		CreationDate: date(2025, 2, 2),
		Subject:      pdf.Text("Grammar"),
		Type:         annotation.TextMarkupTypeSquiggly,
		QuadPoints: []vec.Vec2{
			{X: 72, Y: y}, {X: 220, Y: y},
			{X: 72, Y: y - 12}, {X: 220, Y: y - 12},
		},
	}
	p.Page.AddAnnots(sq)
	y -= 50

	// freetext
	ft := &annotation.FreeText{
		Rect:              pdf.Rectangle{LLx: 72, LLy: y - 40, URx: 350, URy: y},
		Contents:          pdf.Text("TODO: Add a figure showing the data flow between components."),
		User:              pdf.Text("Author"),
		CreationDate:      date(2025, 2, 3),
		Subject:           pdf.Text("FreeText"),
		DefaultAppearance: "/Helvetica 10 Tf 0 0 0 rg",
	}
	p.Page.AddAnnots(ft)
	y -= 70

	// stamp
	st := &annotation.Stamp{
		Rect:         pdf.Rectangle{LLx: 72, LLy: y - 50, URx: 200, URy: y},
		Contents:     pdf.Text("Approved by the review board on 2025-02-05."),
		Color:        color.DeviceRGB{0, 0.6, 0},
		User:         pdf.Text("Chair"),
		CreationDate: date(2025, 2, 5),
		Subject:      pdf.Text("Approval"),
		Icon:         "Approved",
	}
	p.Page.AddAnnots(st)
	y -= 70

	// line
	ln := &annotation.Line{
		Rect:         pdf.Rectangle{LLx: 72, LLy: y - 30, URx: 300, URy: y},
		Contents:     pdf.Text("This measurement seems incorrect."),
		Color:        color.DeviceRGB{1, 0, 0},
		User:         pdf.Text("Reviewer"),
		CreationDate: date(2025, 2, 4),
		Subject:      pdf.Text("Line"),
		Coords:       [4]float64{72, y - 15, 300, y - 15},
	}
	p.Page.AddAnnots(ln)
	y -= 60

	// square
	sqr := &annotation.Square{
		Rect:         pdf.Rectangle{LLx: 72, LLy: y - 60, URx: 200, URy: y},
		Contents:     pdf.Text("This diagram needs updating."),
		Color:        color.DeviceRGB{0, 0, 1},
		User:         pdf.Text("Reviewer"),
		CreationDate: date(2025, 2, 4),
		Subject:      pdf.Text("Box"),
	}
	p.Page.AddAnnots(sqr)
	y -= 80

	// ink
	ink := &annotation.Ink{
		Rect:         pdf.Rectangle{LLx: 72, LLy: y - 40, URx: 200, URy: y},
		Contents:     pdf.Text("See my handwritten note."),
		Color:        color.DeviceRGB{0.5, 0, 0.5},
		User:         pdf.Text("Reviewer"),
		CreationDate: date(2025, 2, 6),
		Subject:      pdf.Text("Ink"),
		InkList: [][]vec.Vec2{
			{{X: 80, Y: y - 10}, {X: 100, Y: y - 30}, {X: 120, Y: y - 10}, {X: 140, Y: y - 30}},
		},
	}
	p.Page.AddAnnots(ink)

	return p.Close()
}

// pageEdgeCases creates a page with edge cases for the tool.
func pageEdgeCases(doc *document.MultiPage) error {
	p := doc.AddPage()

	// annotation without a date (should sort first)
	nodate := &annotation.Text{
		Rect:     pdf.Rectangle{LLx: 72, LLy: 700, URx: 96, URy: 724},
		Contents: pdf.Text("This annotation has no creation date."),
		User:     pdf.Text("Legacy"),
		Subject:  pdf.Text("Undated"),
		Icon:     annotation.TextIconNote,
	}

	// reserve the undated annotation's reference so the reply can target it
	nodateRef := p.RM.GetReference(nodate)

	// annotation with long multi-paragraph text
	longText := &annotation.Text{
		Rect: pdf.Rectangle{LLx: 72, LLy: 650, URx: 96, URy: 674},
		Contents: pdf.Text("First paragraph of a long annotation. " +
			"This contains enough text to require word wrapping " +
			"when displayed by the pdf-annotations tool. " +
			"The tool should break lines at 78 characters.\r" +
			"Second paragraph after a carriage return. " +
			"This tests the PDF spec requirement that CR (0x0D) " +
			"separates paragraphs in the Contents string.\r" +
			"Third and final paragraph."),
		User:         pdf.Text("Verbose"),
		CreationDate: date(2025, 4, 1),
		Subject:      pdf.Text("Long Note"),
		Icon:         annotation.TextIconNote,
	}

	// annotation with empty contents but with subject
	empty := &annotation.Text{
		Rect:         pdf.Rectangle{LLx: 72, LLy: 600, URx: 96, URy: 624},
		User:         pdf.Text("Marker"),
		CreationDate: date(2025, 4, 2),
		Subject:      pdf.Text("Empty Note"),
		Icon:         annotation.TextIconKey,
	}

	// reply to the undated annotation
	reply := &annotation.Text{
		Rect:         pdf.Rectangle{LLx: 72, LLy: 700, URx: 96, URy: 724},
		Contents:     pdf.Text("Adding a date stamp now."),
		User:         pdf.Text("Admin"),
		CreationDate: date(2025, 5, 1),
		InReplyTo:    nodateRef,
		Icon:         annotation.TextIconComment,
	}

	p.Page.AddAnnots(nodate, longText, empty, reply)
	return p.Close()
}
