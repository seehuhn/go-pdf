package cff

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/go-test/deep"
)

func FuzzFont(f *testing.F) {
	deep.FloatPrecision = 8

	f.Fuzz(func(t *testing.T, data []byte) {
		cff1, err := Read(bytes.NewReader(data))
		if err != nil {
			return
		}

		buf := &bytes.Buffer{}
		err = cff1.Encode(buf)
		if err != nil {
			fmt.Println(cff1)
			t.Fatal(err)
		}

		cff2, err := Read(bytes.NewReader(buf.Bytes()))
		if err != nil {
			return
		}

		for _, diff := range deep.Equal(cff1, cff2) {
			t.Error(diff)
		}
		// if !reflect.DeepEqual(cff1, cff2) {
		// 	fmt.Println(cff1)
		// 	fmt.Println(cff2)
		// 	t.Error("different")
		// }
	})
}
