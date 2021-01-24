package pdf

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	cases := []struct {
		in  Object
		out string
	}{
		{nil, "null"},
		{String("a"), "(a)"},
		{String("a (test version)"), "(a (test version))"},
		{String("a (test version"), "(a \\(test version)"},
		{String(""), "()"},
		{String("\000"), "<00>"},
		{Array{Integer(1), nil, Integer(3)}, "[1 null 3]"},
	}
	for _, test := range cases {
		out := format(test.in)
		if out != test.out {
			t.Errorf("string wrongly formatted, expected %q but got %q",
				test.out, out)
		}
	}
}

func TestTextString(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"\000\011\n\f\r",
		"ein Bär",
		"o țesătură",
		"中文",
		"日本語",
	}
	for _, test := range cases {
		enc := TextString(test)
		out := enc.AsTextString()
		if out != test {
			t.Errorf("wrong text: %q != %q", out, test)
		}
	}
}

func TestDateString(t *testing.T) {
	PST := time.FixedZone("PST", -8*60*60)
	cases := []time.Time{
		time.Date(1998, 12, 23, 19, 52, 0, 0, PST),
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2020, 12, 24, 16, 30, 12, 0, time.FixedZone("", 90*60)),
	}
	for _, test := range cases {
		enc := Date(test)
		out, err := enc.AsDate()
		if err != nil {
			t.Error(err)
		} else if !test.Equal(out) {
			fmt.Println(test, string(enc), out)
			t.Errorf("wrong time: %s != %s", out, test)
		}
	}
}

func TestStream(t *testing.T) {
	dataIn := "\nbinary stream data\000123\n   "
	rIn := strings.NewReader(dataIn)
	stream := &Stream{
		Dict: map[Name]Object{
			"Length": Integer(len(dataIn)),
		},
		R: rIn,
	}
	rOut, err := stream.Decode()
	if err != nil {
		t.Fatal(err)
	}
	dataOut, err := ioutil.ReadAll(rOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(dataOut) != dataIn {
		t.Errorf("wrong result:\n  %q\n  %q", dataIn, dataOut)
	}
}
