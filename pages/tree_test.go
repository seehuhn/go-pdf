package pages_test

import (
	"bytes"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

func TestBalance(t *testing.T) {
	// write a test file
	buf := &bytes.Buffer{}
	out, err := pdf.NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	tree := pages.InstallTree(out, &pages.InheritableAttributes{
		MediaBox: pages.A4,
	})
	for i := 0; i < 16*16; i++ { // maxDegree = 16 -> this should give depth 2
		page, err := graphics.AppendPage(tree)
		if err != nil {
			t.Fatal(err)
		}
		_, err = page.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	err = out.Close()
	if err != nil {
		t.Fatal(err)
	}
	testData := buf.Bytes()

	// read back the file and inspect the page tree
	readBuf := bytes.NewReader(testData)
	in, err := pdf.NewReader(readBuf, readBuf.Size(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(pages pdf.Object, depth int) error
	walk = func(obj pdf.Object, depth int) error {
		node, err := in.GetDict(obj)
		if err != nil {
			return err
		}
		switch node["Type"].(pdf.Name) {
		case "Pages":
			kids := node["Kids"].(pdf.Array)
			for _, kid := range kids {
				err = walk(kid, depth+1)
				if err != nil {
					return err
				}
			}
		case "Page":
			if depth > 2 {
				return fmt.Errorf("page at depth %d", depth)
			}
		}

		return nil
	}
	err = walk(in.Catalog.Pages, 0)
	if err != nil {
		t.Fatal(err)
	}
}
