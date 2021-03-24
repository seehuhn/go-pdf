package truetype

import (
	"fmt"
	"log"
	"os"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/pages"
)

func TestCMap(t *testing.T) {
	tt, err := Open("FreeSerif.ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer tt.Close()

	cmap := make(map[rune]int)
	for c, rr := range tt.CMap {
		for _, r := range rr {
			cmap[r] = c
		}
	}

	fd, err := os.Create("test.cmap")
	if err != nil {
		t.Fatal(err)
	}
	defer tt.Close()

	err = font.WriteCMap(fd, "MyTestCMap", tt.Info.FontName, cmap)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEmbedFont(t *testing.T) {
	w, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F1, err := w.Write(pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name("Times-Roman"),
		"Encoding": pdf.Name("MacRomanEncoding"),
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	tt, err := Open("FreeSerif.ttf")
	if err != nil {
		t.Fatal(err)
	}
	CIDFont, err := tt.EmbedAsType0(w)
	if err != nil {
		t.Fatal(err)
	}
	err = tt.Close()
	if err != nil {
		t.Fatal(err)
	}

	F2, err := w.Write(pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(tt.Info.FontName), // TODO(voss): make sure this is consistent
		"Encoding":        pdf.Name("Identity-H"),
		"DescendantFonts": pdf.Array{CIDFont},
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	contents, err := pages.SinglePage(w, &pages.Attributes{
		Resources: map[pdf.Name]pdf.Object{
			"Font": pdf.Dict{
				"F1": F1,
				"F2": F2,
			},
		},
		MediaBox: pages.A4,
	})
	if err != nil {
		t.Fatal(err)
	}

	for col := 0; col < 6; col++ {
		contents.Println("BT")
		contents.Printf("%d %f Td\n", 50+90*col, contents.URy-50-10)
		for c := 50 * col; c < 50*(col+1); c++ {
			if c >= 256 {
				break
			}
			contents.Println("/F1 12 Tf")
			s := pdf.String(fmt.Sprintf("%3d: ", c))
			out := pdf.String(append([]byte(s), byte(c), ' '))
			out.PDF(contents)
			contents.Print(" Tj ")
			contents.Println("/F2 12 Tf")
			out = pdf.String([]byte{0, byte(c)})
			out.PDF(contents)
			contents.Println(" Tj")
			contents.Println("0 -15 TD")
		}
		contents.Println("ET")
	}

	err = contents.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
