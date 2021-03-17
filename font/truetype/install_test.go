package truetype

import (
	"testing"

	"seehuhn.de/go/pdf"
)

func TestInstallFont(t *testing.T) {
	w, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	_, err = Install(w, "cmr10.ttf")
	if err != nil {
		t.Fatal(err)
	}

	w.SetCatalog(pdf.Struct(pdf.Catalog{}))

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
