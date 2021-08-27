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

package sfnt

import (
	"encoding/binary"
)

type check struct {
	sum  uint32
	buf  [4]byte
	used int
}

func (s *check) Write(p []byte) (int, error) {
	n := 0
	buf := s.buf
	for len(p) > 0 {
		k := copy(buf[s.used:], p)
		p = p[k:]
		n += k
		s.used += k

		if s.used == 4 {
			s.sum += binary.BigEndian.Uint32(buf[:])
			s.used = 0
		}
	}
	return n, nil
}

func (s *check) Sum() uint32 {
	if s.used != 0 {
		s.Write([]byte{0, 0, 0}[:4-s.used])
	}
	return s.sum
}

func (s *check) Reset() {
	s.sum = 0
	s.used = 0
}

func checksum(data []byte) uint32 {
	cc := &check{}
	_, _ = cc.Write(data)
	return cc.Sum()
}
