package truetype

import (
	"testing"
)

func TestFormatString(t *testing.T) {
	s, err := formatPDFString("abc", []byte{'d', 'e'}, 'f', 2)
	if err != nil {
		t.Error(err)
	} else if s != "(abcdef2)" {
		t.Errorf("wrong result %q", s)
	}

	s, err = formatPDFString("x", 1.2)
	if err == nil {
		t.Error("missing error")
	} else if s != "" {
		t.Error("wrong string with error")
	}
}

func TestFormatName(t *testing.T) {
	name, err := formatPDFName("abc")
	if err != nil {
		t.Error(err)
	} else if name != "/abc" {
		t.Errorf("wrong result %q", name)
	}
}
