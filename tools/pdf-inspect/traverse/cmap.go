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
	"bytes"
	"fmt"
	"regexp"

	"seehuhn.de/go/pdf/font/cmap"
)

// cmapCtx represents a CMap for traversal.
type cmapCtx struct {
	cmap *cmap.File
}

// newCMapCtx creates a new CMap context.
func newCMapCtx(cmapFile *cmap.File) *cmapCtx {
	return &cmapCtx{cmap: cmapFile}
}

// Show displays information about the CMap.
func (c *cmapCtx) Show() error {
	if c.cmap == nil {
		fmt.Println("CMap: (nil)")
		return nil
	}

	name := c.cmap.Name
	if c.cmap.IsPredefined() {
		name += " (predefined)"
	}
	fmt.Printf("CMap Name: %s\n", name)
	fmt.Printf("Writing Mode: %s\n", c.cmap.WMode)

	if c.cmap.ROS != nil {
		fmt.Printf("Registry-Ordering-Supplement: %s-%s-%d\n",
			c.cmap.ROS.Registry, c.cmap.ROS.Ordering, c.cmap.ROS.Supplement)
	}

	if c.cmap.Parent != nil {
		fmt.Printf("Parent CMap: %s\n", c.cmap.Parent.Name)
	}

	// Show code space ranges
	if len(c.cmap.CodeSpaceRange) > 0 {
		fmt.Printf("Code Space Ranges: %d\n", len(c.cmap.CodeSpaceRange))
		for i, csr := range c.cmap.CodeSpaceRange {
			if i >= 5 { // Limit output to first 5 ranges
				fmt.Printf("  ... and %d more\n", len(c.cmap.CodeSpaceRange)-5)
				break
			}
			fmt.Printf("  [%d] %d bytes: %X - %X\n", i, len(csr.Low), csr.Low, csr.High)
		}
	}

	// Show mapping counts
	fmt.Printf("CID Singles: %d\n", len(c.cmap.CIDSingles))
	fmt.Printf("CID Ranges: %d\n", len(c.cmap.CIDRanges))
	fmt.Printf("Notdef Singles: %d\n", len(c.cmap.NotdefSingles))
	fmt.Printf("Notdef Ranges: %d\n", len(c.cmap.NotdefRanges))

	return nil
}

// Next returns available steps for this context.
func (c *cmapCtx) Next() []Step {
	if c.cmap == nil {
		return nil
	}

	return []Step{{
		Match: regexp.MustCompile(`^@raw$`),
		Desc:  "`@raw`",
		Next: func(key string) (Context, error) {
			var buf bytes.Buffer
			err := c.cmap.WriteTo(&buf, true)
			if err != nil {
				return nil, err
			}
			return &rawStreamCtx{r: bytes.NewReader(buf.Bytes())}, nil
		},
	}}
}
