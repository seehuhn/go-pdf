package cmap

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func FuzzCmapHeader(f *testing.F) {
	f.Add([]byte{
		0, 0,
		0, 2,
		0, 0, 0, 4, 0, 0, 0, 20,
		0, 3, 0, 10, 0, 0, 0, 20,
		0, 6, 0, 10, 0, 0, 0, 0,
	})
	buf := bytes.Buffer{}
	ss := Subtables{
		{
			PlatformID: 3,
			EncodingID: 10,
			Data:       []byte{0, 1, 0, 8, 1, 2, 3, 4, 101, 102, 103, 104},
		},
		{
			PlatformID: 0,
			EncodingID: 4,
			Data:       []byte{0, 1, 0, 8, 5, 6, 7, 8, 101, 102, 103, 104},
		},
	}
	ss.Write(&buf)
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		ss, err := LocateSubtables(data)
		if err != nil {
			return
		}
		buf := bytes.Buffer{}
		ss.Write(&buf)
		if len(buf.Bytes()) > len(data) {
			t.Errorf("too long")
		}
		ss2, err := LocateSubtables(buf.Bytes())
		if err != nil {
			for i := 0; i < len(ss); i++ {
				fmt.Printf("%d %d % x\n", ss[i].PlatformID, ss[i].EncodingID, ss[i].Data)
			}
			fmt.Printf("% x\n", buf.Bytes())
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ss, ss2) {
			t.Errorf("ss != ss2")
		}
	})
}
