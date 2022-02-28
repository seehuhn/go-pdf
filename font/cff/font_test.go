package cff

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
