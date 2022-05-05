package builder

import (
	"testing"

	"seehuhn.de/go/pdf/font/debug"
)

func TestParser(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()
	lookups, err := parse(fontInfo, `
	GSUB_1: A->B, M->N
	GSUB_1: A->B, B->C, C->D, M->N, N->O
	GSUB_1: A->X, B->X, C->X, M->X, N->X
	GSUB_2: A -> "AA", B -> "AA", C -> "ABAAC"
	GSUB_3: A -> "BCD"
	GSUB_4: -marks A A -> A
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = lookups
}
