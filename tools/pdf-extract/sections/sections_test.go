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

package sections

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/outline"
)

func TestPages(t *testing.T) {
	// create a test PDF with outline
	buf := &bytes.Buffer{}
	doc, err := document.WriteMultiPage(buf, &pdf.Rectangle{URx: 612, URy: 792}, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}

	// pre-allocate page references
	page0Ref := doc.Out.Alloc()
	page1Ref := doc.Out.Alloc()
	page2Ref := doc.Out.Alloc()
	page3Ref := doc.Out.Alloc()

	// create pages
	page0 := doc.AddPage()
	page0.Ref = page0Ref
	err = page0.Close()
	if err != nil {
		t.Fatal(err)
	}

	page1 := doc.AddPage()
	page1.Ref = page1Ref
	err = page1.Close()
	if err != nil {
		t.Fatal(err)
	}

	page2 := doc.AddPage()
	page2.Ref = page2Ref
	err = page2.Close()
	if err != nil {
		t.Fatal(err)
	}

	page3 := doc.AddPage()
	page3.Ref = page3Ref
	err = page3.Close()
	if err != nil {
		t.Fatal(err)
	}

	// create outline with test sections
	tree := &outline.Outline{
		Items: []*outline.Item{
			{
				Title:       "Introduction",
				Destination: &destination.XYZ{Page: page0Ref, Left: 0, Top: 700, Zoom: 0},
			},
			{
				Title:       "Chapter 1: Getting Started",
				Destination: &destination.XYZ{Page: page1Ref, Left: 0, Top: 750, Zoom: 0},
			},
			{
				Title:       "Chapter 2: Advanced Topics",
				Destination: &destination.XYZ{Page: page2Ref, Left: 0, Top: 600, Zoom: 0},
			},
			{
				Title:       "Conclusion",
				Destination: &destination.XYZ{Page: page3Ref, Left: 0, Top: 500, Zoom: 0},
			},
		},
	}

	err = tree.Write(doc.RM)
	if err != nil {
		t.Fatal(err)
	}

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}

	// open the PDF for reading
	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	// test successful match
	result, err := Pages(r, "Chapter 1")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if result.FirstPage != 1 {
		t.Errorf("expected FirstPage=1, got %d", result.FirstPage)
	}
	if result.LastPage != 2 {
		t.Errorf("expected LastPage=2, got %d", result.LastPage)
	}
	if result.YMax != 750 {
		t.Errorf("expected YMax=750, got %f", result.YMax)
	}
	if result.YMin != 600 {
		t.Errorf("expected YMin=600 (where Chapter 2 starts), got %f", result.YMin)
	}

	// test no matches
	_, err = Pages(r, "Nonexistent Section")
	if err == nil {
		t.Error("expected error for nonexistent section, got nil")
	}

	// test multiple matches
	_, err = Pages(r, "Chapter")
	if err == nil {
		t.Error("expected error for multiple matches, got nil")
	}

	// test regex patterns
	result, err = Pages(r, "Chapter 2:.*")
	if err != nil {
		t.Fatalf("expected success with regex, got error: %v", err)
	}
	if result.FirstPage != 2 {
		t.Errorf("expected FirstPage=2, got %d", result.FirstPage)
	}
}

func TestPagesWithoutOutline(t *testing.T) {
	// create a PDF without outline
	buf := &bytes.Buffer{}
	doc, err := document.WriteMultiPage(buf, &pdf.Rectangle{URx: 612, URy: 792}, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}

	page := doc.AddPage()
	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	_, err = Pages(r, "anything")
	if err == nil {
		t.Error("expected error for PDF without outline, got nil")
	}
}

func TestPagesSpanningMultiplePages(t *testing.T) {
	// create a test PDF with a section spanning multiple pages
	buf := &bytes.Buffer{}
	doc, err := document.WriteMultiPage(buf, &pdf.Rectangle{URx: 612, URy: 792}, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}

	// pre-allocate page references
	var pageRefs []pdf.Reference
	for range 6 {
		pageRefs = append(pageRefs, doc.Out.Alloc())
	}

	// create pages
	for i := range 6 {
		page := doc.AddPage()
		page.Ref = pageRefs[i]
		err = page.Close()
		if err != nil {
			t.Fatal(err)
		}
	}

	// create outline where "Long Chapter" spans from page 1 to page 4
	tree := &outline.Outline{
		Items: []*outline.Item{
			{
				Title:       "Preface",
				Destination: &destination.XYZ{Page: pageRefs[0], Left: 0, Top: 700, Zoom: 0},
			},
			{
				Title:       "Long Chapter",
				Destination: &destination.XYZ{Page: pageRefs[1], Left: 0, Top: 600, Zoom: 0},
			},
			{
				Title:       "Epilogue",
				Destination: &destination.XYZ{Page: pageRefs[5], Left: 0, Top: 400, Zoom: 0},
			},
		},
	}

	err = tree.Write(doc.RM)
	if err != nil {
		t.Fatal(err)
	}

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	result, err := Pages(r, "Long Chapter")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if result.FirstPage != 1 {
		t.Errorf("expected FirstPage=1, got %d", result.FirstPage)
	}
	if result.LastPage != 5 {
		t.Errorf("expected LastPage=5, got %d", result.LastPage)
	}
	if result.YMax != 600 {
		t.Errorf("expected YMax=600, got %f", result.YMax)
	}
	if result.YMin != 400 {
		t.Errorf("expected YMin=400 (where Epilogue starts), got %f", result.YMin)
	}
}
