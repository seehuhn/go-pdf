package main

import (
	"fmt"
	"log"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/names"
	"seehuhn.de/go/pdf/pages"
)

const title = "The 14 Built-in PDF Fonts"

func main() {
	targetName := "Times-Roman" // "ZapfDingbats"
	targetAfm, err := builtin.ReadAfm(targetName)
	if err != nil {
		log.Fatal(err)
	}
	nGlyph := len(targetAfm.Code)
	fmt.Println(nGlyph)

	w, err := pdf.Create("builtin.pdf")
	if err != nil {
		log.Fatal(err)
	}
	pageFonts := pdf.Dict{}
	resources := pdf.Dict{
		"Font": pageFonts,
	}

	titleFont, err := builtin.Embed(w, "B", "Times-Bold")
	if err != nil {
		log.Fatal(err)
	}
	pageFonts[titleFont.Name] = titleFont.Ref
	labelFont, err := builtin.Embed(w, "F", "Times-Roman")
	if err != nil {
		log.Fatal(err)
	}
	pageFonts[labelFont.Name] = labelFont.Ref

	nFont := (nGlyph + 255) / 256
	tf := make([]*font.Font, nFont)
	for i := 0; i < nFont; i++ {
		name := fmt.Sprintf("T%d", i)
		targetFont, err := builtin.EmbedAfm(w, name, targetAfm)
		if err != nil {
			log.Fatal(err)
		}
		pageFonts[targetFont.Name] = targetFont.Ref
		tf[i] = targetFont
	}

	paper := pages.A4
	tree := pages.NewPageTree(w, &pages.DefaultAttributes{
		Resources: resources,
		MediaBox:  paper,
	})

	p := boxes.Parameters{
		BaseLineSkip: 12,
	}
	pageList := []boxes.Box{
		boxes.HBoxTo(paper.URx,
			boxes.Glue(0, 1, 1, 1, 1),
			boxes.Text(titleFont, 24, title),
			boxes.Glue(0, 1, 1, 1, 1),
		),
		boxes.Kern(72),
	}
	pageNo := 1
	flushPage := func() {
		if len(pageList) == 0 {
			return
		}

		pageList = append(pageList,
			boxes.Glue(0, 1, 1, 1, 1),
			boxes.HBoxTo(paper.URx,
				boxes.Glue(0, 1, 1, 1, 1),
				boxes.Text(labelFont, 10, fmt.Sprintf("- %d -", pageNo)),
				boxes.Glue(0, 1, 1, 1, 1),
			),
			boxes.Kern(36),
		)

		pageList = append([]boxes.Box{boxes.Kern(72)}, pageList...)
		pageBox := p.VBoxTo(paper.URy, pageList...)
		boxes.Ship(tree, pageBox)

		pageNo++
		pageList = nil
	}

	var columns []boxes.Box
	var col []boxes.Box
	done := false
	colNo := 1
	flushCol := func() {
		if col != nil {
			colBox := p.VTop(col...)
			if len(columns) > 0 {
				columns = append(columns, boxes.Kern(12))
			}
			columns = append(columns, colBox)
			colNo++
			col = nil

			if len(columns) >= 2*4-1 || done {
				columns = append([]boxes.Box{boxes.Kern(50)}, columns...)
				tmp := boxes.HBox(columns...)
				pageList = append(pageList, tmp)
				columns = nil
				flushPage()
			}
		}
	}
	for i := 0; i < nGlyph; i++ {
		numRows := 50
		if colNo > 4 {
			numRows = (nGlyph - 200 + 3) / 4
		}
		if len(col) >= numRows {
			flushCol()
		}
		iF := i / 256

		name := targetAfm.Name[i]
		rr := names.ToUnicode(name, targetAfm.IsDingbats)
		if len(rr) != 1 {
			name = name + " -"
		} else if gg := tf[iF].Layout(rr); len(gg) != 1 || gg[0].Gid != font.GlyphID(i+1) {
			name = name + " *"
		}

		line := boxes.HBoxTo(120,
			boxes.HBoxTo(16,
				boxes.Glue(0, 1, 1, 1, 1),
				boxes.Text(labelFont, 10, fmt.Sprintf("%d", i))),
			boxes.HBoxTo(24,
				boxes.Glue(0, 1, 1, 1, 1),
				boxes.Text(tf[iF], 10, string(rr)),
				boxes.Glue(0, 1, 1, 1, 1)),
			boxes.Text(labelFont, 10, name),
		)
		col = append(col, line)
	}
	done = true
	flushCol()
	flushPage()

	root, err := tree.Flush()
	if err != nil {
		log.Fatal(err)
	}

	w.SetInfo(&pdf.Info{
		Title:        "title",
		Producer:     "seehuhn.de/go/pdf/demo/builtin-fonts",
		CreationDate: time.Now(),
	})
	w.SetCatalog(&pdf.Catalog{
		Pages: root,
	})

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}
