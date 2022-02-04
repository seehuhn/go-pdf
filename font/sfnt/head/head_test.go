package head

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestHeadLength(t *testing.T) {
	info := &Info{}
	data, _ := info.Encode()
	if len(data) != headLength {
		t.Errorf("expected %d, got %d", headLength, len(data))
	}
}

func FuzzHead(f *testing.F) {
	info := &Info{}
	data, _ := info.Encode()
	f.Add(data)

	f.Fuzz(func(t *testing.T, d1 []byte) {
		i1, err := Read(bytes.NewReader(d1))
		if err != nil {
			return
		}

		d2, err := i1.Encode()
		if err != nil {
			t.Fatal(err)
		}

		i2, err := Read(bytes.NewReader(d2))
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(i1, i2) {
			fmt.Println(i1)
			fmt.Println(i2)
			t.Fatal("not equal")
		}
	})
}
