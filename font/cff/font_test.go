package cff

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-test/deep"
)

// func TestMany(t *testing.T) {
// 	names, err := filepath.Glob("../../demo/try-all-fonts/cff/*.cff")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	for _, name := range names {
// 		t.Run(filepath.Base(name), func(t *testing.T) {
// 			fd, err := os.Open(name)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			defer fd.Close()
// 			_, err = Read(fd)
// 			if err != nil {
// 				t.Fatal(err)
// 			}

// 			// ...
// 		})
// 	}
// }

func FuzzFont(f *testing.F) {
	names, err := filepath.Glob("../../demo/try-all-fonts/cff/*.cff")
	if err != nil {
		f.Fatal(err)
	}
	for _, name := range names {
		stat, err := os.Stat(name)
		if err != nil {
			f.Fatal(err)
		}
		if stat.Size() > 1500 {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}

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
