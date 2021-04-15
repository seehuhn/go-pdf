package sfnt

import "testing"

func TestRead(t *testing.T) {
	tt, err := Open("../truetype/ttf/SourceSerif4-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}

	_, err = tt.ReadGsubLigInfo("DEU ", "latn")
	if err != nil {
		t.Fatal(err)
	}

	t.Error("fish")
}
