package cmap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"seehuhn.de/go/dijkstra"
	"seehuhn.de/go/pdf/font"
)

func TestFormat4Samples(t *testing.T) {
	// TODO(voss): remove
	names, err := filepath.Glob("../../../demo/try-all-fonts/cmap/04-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 2 {
		t.Fatal("not enough samples")
	}
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		_, err = decodeFormat4(data)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestF4MakeSegment(t *testing.T) {
	m := map[uint16]font.GlyphID{
		1:     1,
		2:     2,
		3:     3,
		4:     4,
		5:     5,
		100:   100,
		101:   102,
		102:   104,
		103:   106,
		104:   108,
		200:   200,
		201:   202,
		202:   204,
		203:   206,
		204:   208,
		205:   210,
		206:   211,
		207:   212,
		208:   213,
		209:   214,
		1000:  2000,
		65532: 23,
		65533: 22,
	}

	g := makeSegments(m)
	ss, err := dijkstra.ShortestPath[uint32, *segment, int](g, 0, 0x10000)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range ss {
		fmt.Println(i, s)
	}
	// TODO(voss): do some checks on the output
}

func FuzzFormat4(f *testing.F) {
	f.Add([]byte{
		0x00, 0x04, 0x00, 0x18, 0x00, 0x00, 0x00, 0x02,
		0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff,
		0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0x00, 0x00,
	})

	f.Add([]byte{
		0x00, 0x04, 0x00, 0x20, 0x00, 0x00, 0x00, 0x04,
		0x00, 0x04, 0x00, 0x01, 0x00, 0x00, 0xe3, 0x3f,
		0xff, 0xff, 0x00, 0x00, 0xe1, 0x00, 0xff, 0xff,
		0x1f, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00,
	})

	f.Add([]byte{
		0x00, 0x04, 0x00, 0x38, 0x00, 0x00, 0x00, 0x0a,
		0x00, 0x08, 0x00, 0x02, 0x00, 0x02, 0x00, 0x00,
		0x00, 0x0d, 0x00, 0x20, 0x00, 0xa0, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x20,
		0x00, 0xa0, 0xff, 0xff, 0x00, 0x01, 0xff, 0xf5,
		0xff, 0xe3, 0xff, 0x95, 0x00, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})

	f.Fuzz(func(t *testing.T, data []byte) {
		c1, err := decodeFormat4(data)
		if err != nil {
			return
		}

		data2 := c1.Encode(0)
		// if len(data2) > len(data) {
		// 	t.Error("too long")
		// }

		c2, err := decodeFormat4(data2)
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(c1, c2) {
			// for i := uint32(0); i < 65536; i++ {
			// 	if c1[i] != c2[i] {
			// 		fmt.Printf("%5d | %5d | %5d \n", i, c1[i], c2[i])
			// 	}
			// }
			t.Error("not equal")
		}
	})
}

var _ Subtable = Format4(nil)
