package tounicode

import (
	"errors"
	"io"
	"regexp"
	"strconv"

	"seehuhn.de/go/pdf"
)

type CharCode uint32

// Info describes a mapping from character codes to Unicode characters sequences.
type Info struct {
	Map map[CharCode][]rune

	Name       pdf.Name
	Registry   pdf.String
	Ordering   pdf.String
	Supplement pdf.Integer
}

func Read(r io.Reader) (*Info, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// check that we have a valid CMap
	m := bodyRegexp.FindSubmatch(body)
	if len(m) != 2 {
		return nil, ErrInvalid
	}
	inner := m[1]

	m = typeRegexp.FindSubmatch(inner)
	if len(m) == 2 {
		if string(m[1]) != "2" {
			return nil, ErrInvalid
		}
	}

	// code space ranges
	mm := codespaceRegexp1.FindAllSubmatch(inner, -1)
	for _, m := range mm {
		qq := codespaceRegexp2.FindAll(m[1], -1)
		if len(qq)%2 != 0 {
			panic(qq)
		}
	}

	// read meta information
	res := &Info{}
	m = nameRegexp.FindSubmatch(inner)
	if len(m) == 2 {
		res.Name = pdf.Name(m[1]) // TODO(voss): parse the name
	}
	m = registryRegexp.FindSubmatch(inner)
	if len(m) == 2 {
		s, err := pdf.ParseString(m[1])
		if err == nil {
			res.Registry = s
		}
	}
	m = orderingRegexp.FindSubmatch(inner)
	if len(m) == 2 {
		s, err := pdf.ParseString(m[1])
		if err == nil {
			res.Ordering = s
		}
	}
	m = supplementRegexp.FindSubmatch(inner)
	if len(m) == 2 {
		x, err := strconv.Atoi(string(m[1]))
		if err == nil {
			res.Supplement = pdf.Integer(x)
		}
	}

	return res, nil
}

var (
	bodyRegexp = regexp.MustCompile(`(?is)\bbegincmap\b\s*(.+?)\s*\bendcmap\b`)
	typeRegexp = regexp.MustCompile(`(?is)/CMapType\b\s*(.+?)\s*\bdef\b`)

	nameRegexp       = regexp.MustCompile(`(?is)/CMapName\b\s*(/.+?)\s*\bdef\b`)
	registryRegexp   = regexp.MustCompile(`(?is)/CIDSystemInfo\s*<<.*?/Registry\s*(\(.+?\))`)
	orderingRegexp   = regexp.MustCompile(`(?is)/CIDSystemInfo\s*<<.*?/Ordering\s*(\(.+?\))`)
	supplementRegexp = regexp.MustCompile(`(?is)/CIDSystemInfo\s*<<.*?/Supplement\s*([0-9]+)`)

	codespaceRegexp1 = regexp.MustCompile(`(?is)\bbegincodespacerange\b\s*((?:<[0-9a-f]+>\s*)+)\bendcodespacerange\b`)
	codespaceRegexp2 = regexp.MustCompile(`(?is)<[0-9a-f]+>`)
)

var ErrInvalid = errors.New("invalid ToUnicode CMap")
