// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package head

import "time"

func encodeTime(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix() - zeroTime
}

func decodeTime(t int64) time.Time {
	if t == 0 {
		return time.Time{}
	}
	return time.Unix(zeroTime+t, 0)
}

const zeroTime int64 = -2082844800 // start of January 1904 in GMT/UTC time zone
