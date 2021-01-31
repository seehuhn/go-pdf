package type1

import (
	"bufio"
	"embed"
	"strconv"
	"strings"
	"sync"
)

//go:embed afm/*.afm
var afmData embed.FS

type box struct {
	llx, lly, urx, ury float64
}

// All units are in 1/1000 of the scale of the font being formatted.
// Multiplying with the scale factor gives values in 1000*bp.

type font struct {
	FontName  string
	FullName  string
	CapHeight float64
	XHeight   float64
	Ascender  float64
	Descender float64
	Chars     []*character
}

type character struct {
	Code  int
	Width float64
	Name  string
	BB    box
	Lig   map[string]string
	Kern  map[string]float64
}

type afmMap struct {
	sync.Mutex

	data map[string]*font
}

func (m *afmMap) lookup(fontName string) *font {
	m.Lock()
	defer m.Unlock()

	f, ok := m.data[fontName]
	if ok {
		return f
	}
	fd, err := afmData.Open("afm/" + fontName + ".afm")
	if err != nil {
		return nil
	}

	f = &font{}
	byName := make(map[string]*character)
	charMetrics := false
	kernPairs := false
	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "EndCharMetrics") {
			charMetrics = false
			continue
		}
		if charMetrics {
			c := &character{}
			keyVals := strings.Split(line, ";")
			for _, keyVal := range keyVals {
				ff := strings.Fields(keyVal)
				if len(ff) < 2 {
					continue
				}
				switch ff[0] {
				case "C":
					c.Code, _ = strconv.Atoi(ff[1])
				case "WX":
					c.Width, _ = strconv.ParseFloat(ff[1], 64)
				case "N":
					c.Name = ff[1]
				case "B":
					if len(ff) != 5 {
						panic("corrupted afm data for " + fontName)
					}
					c.BB.llx, _ = strconv.ParseFloat(ff[1], 64)
					c.BB.lly, _ = strconv.ParseFloat(ff[2], 64)
					c.BB.urx, _ = strconv.ParseFloat(ff[3], 64)
					c.BB.ury, _ = strconv.ParseFloat(ff[4], 64)
				case "L":
					if len(ff) != 3 {
						panic("corrupted afm data for " + fontName)
					}
					if c.Lig == nil {
						c.Lig = make(map[string]string)
					}
					c.Lig[ff[1]] = ff[2]
				default:
					panic(ff[0] + " not implemented")
				}
			}
			f.Chars = append(f.Chars, c)
			if c.Name != "" {
				byName[c.Name] = c
			}
			continue
		}

		fields := strings.Fields(line)

		if fields[0] == "EndKernPairs" {
			kernPairs = false
			continue
		}
		if kernPairs {
			if len(fields) != 4 || fields[0] != "KPX" {
				panic("unsupported KernPair " + line)
			}
			c := byName[fields[1]]
			if c.Kern == nil {
				c.Kern = make(map[string]float64)
			}
			c.Kern[fields[2]], _ = strconv.ParseFloat(fields[3], 64)
			continue
		}

		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MetricsSets":
			if fields[1] != "0" {
				panic("unsupported writing direction")
			}
		case "FontName":
			f.FontName = fields[1]
		case "FullName":
			f.FullName = fields[1]
		case "CapHeight":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fontName)
			}
			f.CapHeight = x
		case "XHeight":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fontName)
			}
			f.XHeight = x
		case "Ascender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fontName)
			}
			f.Ascender = x
		case "Descender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fontName)
			}
			f.Descender = x
		case "CharWidth":
			panic("not implemented")
		case "StartCharMetrics":
			charMetrics = true
		case "StartKernPairs":
			kernPairs = true
		}
	}
	if err := scanner.Err(); err != nil {
		panic("corrupted afm data for " + fontName)
	}
	return f
}

var afm = &afmMap{
	data: make(map[string]*font),
}
