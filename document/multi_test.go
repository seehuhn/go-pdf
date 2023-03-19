package document

import (
	"bytes"
	"testing"
)

func TestMultiPage(t *testing.T) {
	doc, err := CreateMultiPage("test.pdf", 100, 100)
	if err != nil {
		t.Fatal(err)
	}

	page := doc.AddPage()
	page.Circle(50, 50, 40)
	page.Fill()
	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	page = doc.AddPage()
	page.Rectangle(10, 10, 80, 80)
	page.Fill()
	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	page = doc.AddPage()
	page.MoveTo(10, 10)
	page.LineTo(90, 10)
	page.LineTo(50, 90)
	page.ClosePath()
	page.Fill()
	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkMultiPage(b *testing.B) {
	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()
		doc, err := WriteMultiPage(buf, 100, 100)
		if err != nil {
			b.Fatal(err)
		}

		page := doc.AddPage()
		page.Circle(50, 50, 40)
		page.Fill()
		err = page.Close()
		if err != nil {
			b.Fatal(err)
		}

		page = doc.AddPage()
		page.Rectangle(10, 10, 80, 80)
		page.Fill()
		err = page.Close()
		if err != nil {
			b.Fatal(err)
		}

		page = doc.AddPage()
		page.MoveTo(10, 10)
		page.LineTo(90, 10)
		page.LineTo(50, 90)
		page.ClosePath()
		page.Fill()
		err = page.Close()
		if err != nil {
			b.Fatal(err)
		}

		err = doc.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}
