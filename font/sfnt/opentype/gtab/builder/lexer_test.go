package builder

import (
	"fmt"
	"testing"
)

func TestLexString(t *testing.T) {
	_, c := lex("test", `abc
	def
	ghi`)

	var items []item
	for i := range c {
		items = append(items, i)
	}

	for i, item := range items {
		fmt.Println(i, item)
	}
	t.Error("fish")
}
