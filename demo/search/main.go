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
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf"
)

const fname = "/Users/voss/Library/Application Support/MailMate/Attachments/SherlockjacobDisabilityMemo.pdf"

func main() {
	alphabet := []byte("0123456789")
	nAlph := len(alphabet)
	c := make(chan string)
	go func(c chan<- string) {
		len := 9
		for {
			buf := make([]int, len)
			buf[0] = 2
			res := make([]byte, len)
		incLoop:
			for {
				for i, idx := range buf {
					res[i] = alphabet[idx]
				}
				c <- string(res)

				pos := len - 1
				for {
					buf[pos]++
					if buf[pos] < nAlph {
						break
					}
					buf[pos] = 0
					pos--
					if pos < 0 {
						break incLoop
					}
				}
			}
			len++
		}
	}(c)

	var last string
	readPwd := func([]byte, int) string {
		last = <-c
		fmt.Println(last)
		return last
	}

	fd, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	fi, err := fd.Stat()
	if err != nil {
		log.Fatal(err)
	}
	r, err := pdf.NewReader(fd, fi.Size(), readPwd)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	r.AuthenticateOwner()

	fmt.Println(last)
}
