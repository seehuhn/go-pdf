// Read a CFF font and display a magnified version of each glyph
// in a PDF file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/pages"
)

func main() {
	flag.Parse()

	out, err := pdf.Create("out.pdf")
	if err != nil {
		log.Fatal(err)
	}

	F, err := builtin.Embed(out, "Courier", "F")
	if err != nil {
		log.Fatal(err)
	}

	tree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{
				F.InstName: F.Ref,
			},
		},
		MediaBox: &pdf.Rectangle{
			URx: 440,
			URy: 530,
		},
	})

	names := flag.Args()
	for _, fname := range names {
		cffData, err := loadCFFData(fname)
		if err != nil {
			log.Printf("%s: %v", fname, err)
			continue
		}

		cff, err := cff.Read(bytes.NewReader(cffData))
		if err != nil {
			log.Printf("%s: %v", fname, err)
			continue
		}

		for i := range cff.CharStrings {
			page, err := tree.AddPage(nil)
			if err != nil {
				log.Fatal(err)
			}

			err = illustrateGlyph(page, F, cff, i)
			if err != nil {
				log.Fatal(err)
			}

			err = page.Close()
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	pages, err := tree.Finish()
	if err != nil {
		log.Fatal(err)
	}
	out.SetCatalog(&pdf.Catalog{
		Pages: pages,
	})

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func loadCFFData(fname string) ([]byte, error) {
	if strings.HasSuffix(fname, ".otf") {
		f, err := sfnt.Open(fname, nil)
		if err != nil {
			return nil, err
		}
		return f.Header.ReadTableBytes(f.Fd, "CFF ")
	}
	return os.ReadFile(fname)
}

type context struct {
	page *pages.Page
	xx   []float64
	yy   []float64
	x, y float64
	ink  bool
}

func (ctx *context) RMoveTo(x, y float64) {
	if ctx.ink {
		ctx.page.Println("h")
	}
	ctx.x += x
	ctx.y += y
	ctx.page.Printf("%.2f %.2f m\n", ctx.x, ctx.y)
	ctx.xx = append(ctx.xx, ctx.x)
	ctx.yy = append(ctx.yy, ctx.y)
}

func (ctx *context) RLineTo(x, y float64) {
	ctx.ink = true
	ctx.x += x
	ctx.y += y
	ctx.page.Printf("%.2f %.2f l\n", ctx.x, ctx.y)
	ctx.xx = append(ctx.xx, ctx.x)
	ctx.yy = append(ctx.yy, ctx.y)
}

func (ctx *context) RCurveTo(dxa, dya, dxb, dyb, dxc, dyc float64) {
	ctx.ink = true
	xa := ctx.x + dxa
	ya := ctx.y + dya
	xb := xa + dxb
	yb := ya + dyb
	xc := xb + dxc
	yc := yb + dyc
	ctx.page.Printf("%.2f %.2f %.2f %.2f %.2f %.2f c\n",
		xa, ya, xb, yb, xc, yc)
	ctx.x = xc
	ctx.y = yc
	ctx.xx = append(ctx.xx, ctx.x)
	ctx.yy = append(ctx.yy, ctx.y)
}

func illustrateGlyph(page *pages.Page, F *font.Font, cff *cff.Font, i int) error {
	label := fmt.Sprintf("glyph %d: %s", i, cff.GlyphName[i])
	hss := boxes.Glue(0, 1, 1, 1, 1)
	nameBox := boxes.Text(F, 12, label)
	titleBox := boxes.HBoxTo(page.URx-page.LLx, hss, nameBox, hss)
	titleBox.Draw(page, page.LLx, 12)

	baseX := 40.0
	baseY := 120.0
	page.Println("q 0.2 1 0.2 RG 2 w")
	page.Printf("%.2f %.2f m %.2f %.2f l\n",
		page.LLx, baseY, page.URx, baseY)
	page.Printf("%.2f %.2f m %.2f %.2f l S Q\n",
		baseX, page.LLy, baseX, page.URy)

	page.Printf("0.4 0 0 0.4 %.2f %.2f cm\n", baseX, baseY)

	ctx := &context{
		page: page,
	}
	_, err := cff.DecodeCharString(cff.CharStrings[i], ctx)
	if ctx.ink {
		page.Println("h")
	}
	page.Println("S")
	if err != nil {
		return err
	}

	page.Println("q 0 0 0.8 rg")
	for i := range ctx.xx {
		x := ctx.xx[i]
		y := ctx.yy[i]
		label := boxes.Text(F, 16, fmt.Sprintf("%d", i))
		label.Draw(page, x, y)
	}
	page.Println("Q")

	return nil
}
