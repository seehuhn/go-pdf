package pdf

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
)

func TestFlate(t *testing.T) {
	parmsss := []Dict{
		nil,
		{"Predictor": Integer(1)},
		{"Predictor": Integer(12), "Columns": Integer(5)},
	}
	for _, parms := range parmsss {
		ff := ffFromDict(parms)
		for _, in := range []string{"", "12345", "1234567890"} {
			buf := &bytes.Buffer{}
			w, err := ff.Encode(withoutClose{buf})
			if err != nil {
				t.Error(in, err)
				continue
			}
			_, err = w.Write([]byte(in))
			if err != nil {
				t.Error(in, err)
				continue
			}
			err = w.Close()
			if err != nil {
				t.Error(in, err)
				continue
			}

			fmt.Printf("%d %q\n", buf.Len(), buf.String())

			r, err := ff.Decode(buf)
			if err != nil {
				t.Error(in, err)
				continue
			}
			out, err := ioutil.ReadAll(r)
			if err != nil {
				t.Error(in, err)
				continue
			}

			if in != string(out) {
				t.Errorf("wrong results: %q vs %q", in, string(out))
			}
		}
	}
}

func TestPngUp(t *testing.T) {
	columns := 2

	for _, in := range []string{"", "11121314151617", "123456"} {
		buf := &bytes.Buffer{}
		w := &pngUpWriter{
			w:    buf,
			prev: make([]byte, columns),
			cur:  make([]byte, columns+1),
		}
		n, err := w.Write([]byte(in))
		if err != nil {
			t.Error("unexpected error:", err)
			continue
		}
		if n != len(in) {
			t.Errorf("wrong n: %d vs %d", n, len(in))
		}

		r := &pngUpReader{
			r:    buf,
			prev: make([]byte, columns+1),
			tmp:  make([]byte, columns+1),
		}
		res, err := ioutil.ReadAll(r)
		if err != nil {
			t.Error("unexpected error:", err)
			continue
		}

		if string(res) != in {
			t.Errorf("wrong result: %q vs %q", res, in)
		}
	}
}
