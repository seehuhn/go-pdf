package main

import (
	"fmt"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/fonts"
	"seehuhn.de/go/pdf/fonts/type1"
	"seehuhn.de/go/pdf/pages"
)

// Font selection for all text on the page.
const (
	FontName     = "Times-Italic"
	FontEncoding = "MacRomanEncoding"
	FontSize     = 48
)

var encTable = map[string]fonts.Encoding{
	"StandardEncoding": fonts.StandardEncoding,
	"MacRomanEncoding": fonts.MacRomanEncoding,
	"WinAnsiEncoding":  fonts.WinAnsiEncoding,
}

// Rect takes the coordinates of two diagonally opposite points
// and returns a PDF rectangle.
func Rect(llx, lly, urx, ury int) pdf.Array {
	return pdf.Array{pdf.Integer(llx), pdf.Integer(lly),
		pdf.Integer(urx), pdf.Integer(ury)}
}

// WritePage emits a single page to the PDF file and returns the page dict.
func WritePage(out *pdf.Writer, width, height float64) (pdf.Dict, error) {
	stream, contentNode, err := out.OpenStream(nil, nil, nil)
	if err != nil {
		return nil, err
	}

	enc := encTable[FontEncoding]
	F1 := type1.Lookup(FontName, enc, FontSize)

	margin := 50.0
	baseLineSkip := 1.2 * FontSize

	_, err = stream.Write([]byte("q\n1 .5 .5 RG\n"))
	if err != nil {
		return nil, err
	}
	yPos := height - margin - F1.Ascender
	for y := yPos; y > margin; y -= baseLineSkip {
		_, err = stream.Write([]byte(fmt.Sprintf("%.1f %.1f m %.1f %.1f l\n",
			margin, y, width-margin, y)))
		if err != nil {
			return nil, err
		}
	}
	_, err = stream.Write([]byte("s\nQ\n"))
	if err != nil {
		return nil, err
	}

	text := "Waterflask & fish bucket"
	var codes []byte
	var last byte
	for _, r := range text {
		c, ok := F1.Encoding.Encode(r)
		if !ok {
			panic("character " + string([]rune{r}) + " not in font")
		}
		if len(codes) > 0 {
			pair := fonts.GlyphPair{last, c}
			lig, ok := F1.Ligatures[pair]
			if ok {
				codes = codes[:len(codes)-1]
				c = lig
			}
		}
		codes = append(codes, c)
		last = c
	}

	_, err = stream.Write([]byte("q\n.2 1 .2 RG\n"))
	if err != nil {
		return nil, err
	}
	var formatted pdf.Array
	pos := 0
	xPos := margin
	for i, c := range codes {
		bbox := F1.BBox[c]
		if bbox.IsPrint() {
			_, err = stream.Write([]byte(fmt.Sprintf("%.2f %.2f %.2f %.2f re\n",
				xPos+bbox.LLx, yPos+bbox.LLy, bbox.URx-bbox.LLx, bbox.URy-bbox.LLy)))
			if err != nil {
				return nil, err
			}
		}
		xPos += F1.Width[c]

		if i == len(codes)-1 {
			formatted = append(formatted, pdf.String(codes[pos:]))
			break
		}

		kern, ok := F1.Kerning[fonts.GlyphPair{c, codes[i+1]}]
		if !ok {
			continue
		}
		xPos += kern * float64(FontSize) / 1000
		var kObj pdf.Object
		if kern == float64(int64(kern)) {
			kObj = pdf.Integer(-kern)
		} else {
			kObj = pdf.Real(-kern)
		}
		formatted = append(formatted,
			pdf.String(codes[pos:(i+1)]), kObj)
		pos = i + 1
	}
	_, err = stream.Write([]byte("s\nQ\n"))
	if err != nil {
		return nil, err
	}

	_, err = stream.Write([]byte(fmt.Sprintf("BT\n/F1 %d Tf\n%.1f %.1f Td\n",
		FontSize, margin, yPos)))
	if err != nil {
		return nil, err
	}
	err = formatted.PDF(stream)
	if err != nil {
		return nil, err
	}
	_, err = stream.Write([]byte(" TJ\nET"))
	if err != nil {
		return nil, err
	}

	err = stream.Close()
	if err != nil {
		return nil, err
	}
	return pdf.Dict{
		"Type":     pdf.Name("Page"),
		"Contents": contentNode,
	}, nil
}

func main() {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	font, err := out.Write(pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(FontName),
		"Encoding": pdf.Name(FontEncoding),
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	const width = 8 * 72
	const height = 6 * 72

	pageTree := pages.NewPageTree(out)
	page, err := WritePage(out, width, height)
	if err != nil {
		log.Fatal(err)
	}
	err = pageTree.Ship(page, nil)
	if err != nil {
		log.Fatal(err)
	}

	pages, pagesRef, err := pageTree.Flush()
	if err != nil {
		log.Fatal(err)
	}
	pages["CropBox"] = Rect(0, 0, width, height)
	pages["Resources"] = pdf.Dict{
		"Font": pdf.Dict{"F1": font},
	}
	_, err = out.Write(pages, pagesRef)
	if err != nil {
		log.Fatal(err)
	}

	info, err := out.Write(pdf.Dict{
		"Title":  pdf.TextString("PDF Test Document"),
		"Author": pdf.TextString("Jochen Voß"),
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	catalog, err := out.Write(pdf.Dict{
		"Type":  pdf.Name("Catalog"),
		"Pages": pagesRef,
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = out.Close(catalog, info)
	if err != nil {
		log.Fatal(err)
	}
}
