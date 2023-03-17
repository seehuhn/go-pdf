package simple

import (
	"bytes"
	"testing"
)

func TestSinglePage(t *testing.T) {
	buf := &bytes.Buffer{}
	doc, err := WriteSinglePage(buf, 100, 100)
	if err != nil {
		t.Fatal(err)
	}

	doc.Circle(50, 50, 40)
	doc.Fill()

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkSinglePage(b *testing.B) {
	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()

		doc, err := WriteSinglePage(buf, 100, 100)
		if err != nil {
			b.Fatal(err)
		}

		doc.Circle(50, 50, 40)
		doc.Fill()

		err = doc.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}
