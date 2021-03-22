package truetype

import (
	"encoding/binary"
	"io"
)

func checksum(r io.Reader, isHead bool) (uint32, error) {
	var sum uint32

	buf := make([]byte, 256)
	used := 0
	i := 0
	for used < 4 {
		n, err := r.Read(buf[used:])
		used += n

		for err == io.EOF && used%4 != 0 {
			buf[used] = 0
			used++
		}

		pos := 0
		for pos+4 <= used {
			if i != 2 || !isHead {
				sum += binary.BigEndian.Uint32(buf[pos : pos+4])
			}
			pos += 4
			i++
		}
		copy(buf, buf[pos:])
		used -= pos

		if err == io.EOF {
			break
		} else if err != nil {
			return 0, err
		}
	}

	return sum, nil
}
