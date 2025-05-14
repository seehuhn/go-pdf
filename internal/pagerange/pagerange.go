// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package pagerange

import (
	"fmt"
	"strconv"
	"strings"
)

// PageRange represents a 1-based range of pages in a PDF document.
type PageRange struct {
	Start int
	End   int
}

func (pr *PageRange) String() string {
	if pr.Start == pr.End {
		return fmt.Sprintf("%d", pr.Start)
	}
	return fmt.Sprintf("%d-%d", pr.Start, pr.End)
}

// Set parses a string of the form "from-to" or "from" and sets the PageRange.
// This implements the [flag.Value] interface.
func (pr *PageRange) Set(value string) error {
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return fmt.Errorf("invalid page range format")
	}

	from, err := strconv.Atoi(parts[0])
	if err != nil || from < 1 {
		return fmt.Errorf("invalid 'from' page number")
	}

	to := from
	if len(parts) == 2 {
		to, err = strconv.Atoi(parts[1])
		if err != nil || from < 1 || to < from {
			return fmt.Errorf("invalid 'to' page number")
		}
	}

	pr.Start = from
	pr.End = to
	return nil
}
