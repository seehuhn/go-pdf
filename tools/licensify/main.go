// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const header = `// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

`

func main() {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasPrefix(path, "./poppler-") {
			fmt.Println("skip " + path)
			return fs.SkipDir
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.HasPrefix(body, []byte(header)) {
			return nil
		}
		if !bytes.HasPrefix(body, []byte("package ")) {
			fmt.Println("ATTENTION " + path)
			return nil
		}

		fmt.Println("updating " + path)
		fd, err := os.Create(path)
		if err != nil {
			return err
		}
		_, err = fd.Write([]byte(header))
		if err != nil {
			return err
		}
		_, err = fd.Write(body)
		if err != nil {
			return err
		}
		err = fd.Close()
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
