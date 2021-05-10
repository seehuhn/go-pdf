package parser

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt"
)

func TestGpos(t *testing.T) {
	tt, err := sfnt.Open("../../truetype/ttf/SourceSerif4-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer tt.Close()

	pars := New(tt)
	info, err := pars.ReadGposInfo("latn", "ENG ")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(info)
}
