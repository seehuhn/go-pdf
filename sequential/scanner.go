package sequential

import (
	"io"
	"regexp"
)

type scanner struct {
	file    io.Reader
	filePos int64 // file position corresponding to the start of buf
	buf     []byte
	bufPos  int // current position within buf
	bufEnd  int // end of valid data within buf
	overlap int // maximum length of a regular expression match
}

func newScanner(file io.Reader, bufSize int, overlap int) *scanner {
	return &scanner{
		file:    file,
		buf:     make([]byte, bufSize),
		overlap: overlap,
	}
}

func (s *scanner) refill() error {
	// move the remaining data to the beginning of the buffer
	s.filePos += int64(s.bufPos)
	copy(s.buf, s.buf[s.bufPos:s.bufEnd])
	s.bufEnd -= s.bufPos
	s.bufPos = 0

	// try to read more data
	n, err := s.file.Read(s.buf[s.bufEnd:])
	s.bufEnd += n
	if n > 0 && err == io.EOF {
		err = nil
	}
	return err
}

// find returns the next non-overlapping occurrence of the regular expression pat
// in the file. It returns the position of the match, and the submatches as
// returned by regexp.FindStringSubmatch.
func (s *scanner) find(pat *regexp.Regexp) (int64, []string, error) {
	for {
		// search for a match in the current buffer
		m := pat.FindSubmatchIndex(s.buf[s.bufPos:s.bufEnd])
		if m != nil {
			matchPos := s.filePos + int64(s.bufPos+m[0])

			// found a match
			res := make([]string, len(m)/2)
			for i := range res {
				a, b := m[2*i], m[2*i+1]
				if a >= 0 && b > a {
					res[i] = string(s.buf[s.bufPos+a : s.bufPos+b])
				}
			}

			s.bufPos += m[1]
			return matchPos, res, nil
		}

		// There are no more matches in the current buffer, so we read more data.
		// We need to be prepared for a partial match at the end of the buffer.
		nextPos := s.bufEnd - s.overlap
		if nextPos > s.bufPos {
			s.bufPos = nextPos
		}
		err := s.refill()
		if err != nil {
			return 0, nil, err
		}
	}
}
