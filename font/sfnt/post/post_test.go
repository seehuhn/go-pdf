package post

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func FuzzOS2(f *testing.F) {
	f.Fuzz(func(t *testing.T, in []byte) {
		i1, err := Read(bytes.NewReader(in))
		if err != nil {
			return
		}

		buf := i1.Encode()
		i2, err := Read(bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(i1, i2) {
			fmt.Printf("%#v\n", i1)
			fmt.Printf("%#v\n", i2)
			t.Fatal("not equal")
		}
	})
}
