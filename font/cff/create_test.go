package cff

import (
	"fmt"
	"os"
	"testing"

	"seehuhn.de/go/pdf/font/type1"
)

func TestCreate(t *testing.T) {
	meta := &type1.FontDict{
		FontName: "Test",
	}
	b := NewBuilder(meta, 500, 500)

	g, err := b.AddGlyph(".notdef")
	if err != nil {
		t.Fatal(err)
	}
	g.SetWidth(500)
	g.MoveTo(50, 50)
	g.LineTo(450, 50)
	g.LineTo(450, 950)
	g.LineTo(50, 950)
	g.MoveTo(100, 900)
	g.LineTo(400, 900)
	g.LineTo(400, 100)
	g.LineTo(100, 100)
	g.Close()

	// an arrow pointing right
	g, err = b.AddGlyph("A")
	if err != nil {
		t.Fatal(err)
	}
	g.SetWidth(1000)
	g.MoveTo(0, 450)
	g.LineTo(700, 450)
	g.LineTo(700, 350)
	g.LineTo(1000, 500)
	g.LineTo(700, 650)
	g.LineTo(700, 550)
	g.LineTo(0, 550)
	g.Close()

	cff, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}

	fd, err := os.Create("test.cff")
	if err != nil {
		t.Fatal(err)
	}
	err = cff.Encode(fd)
	if err != nil {
		t.Fatal(err)
	}
	err = fd.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEncode(t *testing.T) {
	in, err := os.Open("Atkinson-Hyperlegible-BoldItalic-102.cff")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	cff, err := Read(in)
	if err != nil {
		t.Fatal(err)
	}

	gid := 4

	ctx := &encoder{}
	orig, err := cff.doDecode(ctx, cff.charStrings[gid])
	if err != nil {
		t.Fatal(err)
	}

	defWidth, _ := cff.privateDict.getInt(opDefaultWidthX, 0)
	nomWidth, _ := cff.privateDict.getInt(opNominalWidthX, 0)
	code := ctx.encode(int16(defWidth), int16(nomWidth))
	fmt.Printf("=> % x\n", code)
	fmt.Printf(".. % x\n", orig)
}
