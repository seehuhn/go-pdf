package builder

import (
	"fmt"
	"testing"
)

func TestLexString(t *testing.T) {
	_, c := lex(`abc
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

func TestLexBackup(t *testing.T) {
	l := &lexer{
		input: "abc",
	}

	var out []rune
	for {
		r := l.next()
		l.backup()
		r2 := l.next()
		if r != r2 {
			t.Errorf("%q != %q", r, r2)
		}
		if r == 0 {
			break
		}
		out = append(out, r)
	}
	if string(out) != l.input {
		t.Errorf("%q != %q", out, l.input)
	}
}
