package builder

import (
	"testing"

	"seehuhn.de/go/pdf/font/debug"
)

func TestParser(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()
	parse(fontInfo, `A marker "DE" 12`)
	t.Error("fish")
}
