package tounicode

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"unicode/utf16"

	"seehuhn.de/go/pdf"
)

func Read(r io.Reader) (*Info, error) {
	outer, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// check that we have a valid CMap
	m := bodyRegexp.FindSubmatch(outer)
	if len(m) != 2 {
		return nil, ErrInvalid
	}
	body := m[1]

	m = typeRegexp.FindSubmatch(body)
	if len(m) == 2 {
		if string(m[1]) != "2" {
			return nil, ErrInvalid
		}
	}

	res := &Info{}

	// read the code space ranges
	mm := codespaceRegexp.FindAllSubmatch(body, -1)
	for _, m := range mm {
		inner := m[1]
		for {
			var first, last CharCode

			inner = skipComments(inner)
			if len(inner) == 0 {
				break
			}

			inner, first, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			inner = skipComments(inner)

			inner, last, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			res.CodeSpace = append(res.CodeSpace, CodeSpaceRange{
				First: first,
				Last:  last,
			})
		}
	}

	// read the bfchar mappings
	mm = bfcharRegexp.FindAllSubmatch(body, -1)
	for _, m := range mm {
		inner := m[1]
		for {
			var code CharCode
			var rr []rune

			inner = skipComments(inner)
			if len(inner) == 0 {
				break
			}

			inner, code, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			inner = skipComments(inner)

			inner, rr, err = parseString(inner)
			if err != nil {
				return nil, err
			}

			res.Singles = append(res.Singles, Single{
				Code: code,
				Text: string(rr),
			})
		}
	}

	// read the bfrange mappings
	mm = bfrangeRegexp.FindAllSubmatch(body, -1)
	for _, m := range mm {
		inner := m[1]
		for {
			var first, last CharCode
			var rr []rune

			inner = skipComments(inner)
			if len(inner) == 0 {
				break
			}

			inner, first, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			inner = skipComments(inner)

			inner, last, err = parseCharCode(inner)
			if err != nil {
				return nil, err
			}

			inner = skipComments(inner)

			nextRange := Range{
				First: first,
				Last:  last,
			}

			m = arrayRegexp.FindSubmatch(inner)
			if m != nil {
				inner = inner[len(m[0]):]
				array := m[1]
				for {
					array = skipComments(array)
					if len(array) == 0 {
						break
					}

					array, rr, err = parseString(array)
					if err != nil {
						return nil, err
					}

					nextRange.Text = append(nextRange.Text, string(rr))
				}
				continue
			} else {
				inner, rr, err = parseString(inner)
				if err != nil {
					return nil, err
				}
				nextRange.Text = []string{string(rr)}
			}

			res.Ranges = append(res.Ranges, nextRange)
		}
	}

	// read meta information
	m = nameRegexp.FindSubmatch(body)
	if len(m) == 2 {
		res.Name = pdf.Name(m[1]) // TODO(voss): parse the name
	}
	m = registryRegexp.FindSubmatch(body)
	if len(m) == 2 {
		s, err := pdf.ParseString(m[1])
		if err == nil {
			res.Registry = s
		}
	}
	m = orderingRegexp.FindSubmatch(body)
	if len(m) == 2 {
		s, err := pdf.ParseString(m[1])
		if err == nil {
			res.Ordering = s
		}
	}
	m = supplementRegexp.FindSubmatch(body)
	if len(m) == 2 {
		x, err := strconv.Atoi(string(m[1]))
		if err == nil {
			res.Supplement = pdf.Integer(x)
		}
	}

	return res, nil
}

func skipComments(buf []byte) []byte {
	for {
		m := commentRegexp.FindSubmatch(buf)
		if m == nil {
			return buf
		}
		buf = buf[len(m[0]):]
	}
}

func parseCharCode(buf []byte) ([]byte, CharCode, error) {
	m := charCodeRegexp.FindSubmatch(buf)
	if m == nil {
		return nil, 0, ErrInvalid
	}

	x, err := strconv.ParseUint(string(m[1]), 16, 32)
	if err != nil {
		return nil, 0, ErrInvalid
	}

	return buf[len(m[0]):], CharCode(x), nil
}

func parseString(buf []byte) ([]byte, []rune, error) {
	m := stringRegexp.FindSubmatch(buf)
	if m == nil {
		return nil, nil, ErrInvalid
	}

	q := bytes.ReplaceAll(m[1], []byte{' '}, []byte{})
	if len(q)%4 != 0 {
		return nil, nil, ErrInvalid
	}

	var s []uint16
	for len(q) > 0 {
		x, err := strconv.ParseUint(string(q[:4]), 16, 16)
		if err != nil {
			fmt.Printf("C %q\n", buf)
			return nil, nil, ErrInvalid
		}
		s = append(s, uint16(x))
		q = q[4:]
	}

	return buf[len(m[0]):], utf16.Decode(s), nil
}

var (
	bodyRegexp = regexp.MustCompile(`(?is)\bbegincmap\b\s*(.+?)\s*\bendcmap\b`)
	typeRegexp = regexp.MustCompile(`(?is)/CMapType\b\s*(.+?)\s*\bdef\b`)

	codespaceRegexp = regexp.MustCompile(`(?is)\bbegincodespacerange\b\s*(.*?)\bendcodespacerange\b`)
	bfcharRegexp    = regexp.MustCompile(`(?is)\bbeginbfchar\b\s*(.*?)\bendbfchar\b`)
	bfrangeRegexp   = regexp.MustCompile(`(?is)\bbeginbfrange\b\s*(.*?)\bendbfrange\b`)

	commentRegexp  = regexp.MustCompile(`^%.*?(?:\n|\r)\s*`)
	charCodeRegexp = regexp.MustCompile(`^<([0-9a-fA-F]*)>\s*`)
	stringRegexp   = regexp.MustCompile(`^<([0-9a-fA-F ]*)>\s*`)
	arrayRegexp    = regexp.MustCompile(`^\[(.*?)\]\s*`)

	nameRegexp       = regexp.MustCompile(`(?is)/CMapName\b\s*(/.+?)\s*\bdef\b`)
	registryRegexp   = regexp.MustCompile(`(?is)/CIDSystemInfo\s*<<.*?/Registry\s*(\(.+?\))`)
	orderingRegexp   = regexp.MustCompile(`(?is)/CIDSystemInfo\s*<<.*?/Ordering\s*(\(.+?\))`)
	supplementRegexp = regexp.MustCompile(`(?is)/CIDSystemInfo\s*<<.*?/Supplement\s*([0-9]+)`)
)

var ErrInvalid = errors.New("invalid ToUnicode CMap")
