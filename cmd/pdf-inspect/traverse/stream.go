// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
