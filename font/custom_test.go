package font

import (
	"bytes"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestCustom(t *testing.T) {
	chars := "abcABCăĂâÂîÎșȘțȚ .,;:'"
	enc := CustomEncoding([]rune(chars))
	desc := Describe(enc)

	buf := &bytes.Buffer{}
	desc.PDF(buf)
	fmt.Println(buf.String())
	// TODO(voss): finish this
}

func TestDescribe(t *testing.T) {
	for name, enc := range stdEncs {
		desc := Describe(enc)
		if dName, ok := desc.(pdf.Name); !ok || string(dName) != name {
			t.Errorf("failed to describe " + name)
		}
	}
}
