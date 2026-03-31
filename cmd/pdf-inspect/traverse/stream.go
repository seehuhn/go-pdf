// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package traverse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

type streamCtx struct {
	r    io.Reader
	name string
}

func (c *streamCtx) Next() []Step {
	return []Step{}
}

func (c *streamCtx) Show() error {
	// Read the first 512 bytes of the stream to determine if the contents are
	// binary or text.
	buf := make([]byte, 512)
	n, err := io.ReadFull(c.r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
	}
	buf = buf[:n]

	if len(buf) == 0 {
		fmt.Printf("empty stream (%s)\n", c.name)
		return nil
	}

	if mostlyBinary(buf) {
		remaining, err := io.Copy(io.Discard, c.r)
		if err != nil {
			return err
		}
		totalBytes := int64(n) + remaining
		fmt.Printf("... binary stream data (%d bytes) ...\n", totalBytes)
		return nil
	}

	fmt.Printf("decoded stream contents (%s):\n", c.name)
	body := io.MultiReader(bytes.NewReader(buf), c.r)
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	return scanner.Err()
}
