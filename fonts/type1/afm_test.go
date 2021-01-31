package type1

import (
	"fmt"
	"testing"
)

func TestAfm(t *testing.T) {
	name := "Times-Roman"
	font := afm.lookup(name)
	fmt.Println(font.FullName)
	for _, c := range font.Chars {
		uni := DecodeGlyphName(c.Name, name == "ZapfDingbats")
		fmt.Println(".", c.Code, c.Name, c.Width, string(uni), c.Kern)
	}
	t.Error("fish")
}
