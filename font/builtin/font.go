package builtin

import (
	"bufio"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
)

func Embed(w *pdf.Writer, name string, fname string, subset map[rune]bool) (*font.Font, error) {
	fd, err := afmData.Open("afm/" + fname + ".afm")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	needSubset := false
	if subset == nil {
		needSubset = true
		subset = make(map[rune]bool)
	}

	builtin := &font.Font{
		Name:      pdf.Name(name),
		CMap:      map[rune]font.GlyphIndex{},
		Ligatures: map[font.GlyphPair]font.GlyphIndex{},
	}

	var FontName string
	runeToName := make(map[rune]string)
	nameToGlyph := make(map[string]font.GlyphIndex)
	stdRuneToCode := make(map[rune]byte)

	type ligInfo struct {
		first, second, combined string
	}
	var ligatures []*ligInfo

	type kernInfo struct {
		first, second string
		val           int
	}
	var kerning []*kernInfo

	dingbats := fname == "ZapfDingbats"
	charMetrics := false
	kernPairs := false
	cIdx := font.GlyphIndex(0)

	// prepend an artificial entry for .notdef, so that CMap works
	builtin.GlyphExtent = append(builtin.GlyphExtent, font.Rect{})
	builtin.Width = append(builtin.Width, 250)
	cIdx++

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
			var name string
			var width int
			var code int
			var BBox font.Rect
			var ligTmp []*ligInfo

			keyVals := strings.Split(line, ";")
			for _, keyVal := range keyVals {
				ff := strings.Fields(keyVal)
				if len(ff) < 2 {
					continue
				}
				switch ff[0] {
				case "C":
					code, _ = strconv.Atoi(ff[1])
				case "WX":
					width, _ = strconv.Atoi(ff[1])
				case "N":
					name = ff[1]
				case "B":
					if len(ff) != 5 {
						panic("corrupted afm data for " + fname)
					}
					BBox.LLx, _ = strconv.Atoi(ff[1])
					BBox.LLy, _ = strconv.Atoi(ff[2])
					BBox.URx, _ = strconv.Atoi(ff[3])
					BBox.URy, _ = strconv.Atoi(ff[4])
				case "L":
					if len(ff) != 3 {
						panic("corrupted afm data for " + fname)
					}
					ligTmp = append(ligTmp, &ligInfo{
						second:   ff[1],
						combined: ff[2],
					})
				default:
					panic(ff[0] + " not implemented")
				}
			}

			rr := names.ToUnicode(name, dingbats)
			if len(rr) != 1 {
				panic("not implemented")
			}
			r := rr[0]
			if needSubset && code > 0 {
				subset[r] = true
			}
			if subset[r] {
				builtin.CMap[r] = cIdx
				nameToGlyph[name] = cIdx
				runeToName[r] = name
				if code > 0 {
					stdRuneToCode[r] = byte(code)
				}
				builtin.Width = append(builtin.Width, width)
				builtin.GlyphExtent = append(builtin.GlyphExtent, BBox)
				for _, lig := range ligTmp {
					lig.first = name
					ligatures = append(ligatures, lig)
				}
				cIdx++
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
			kern := &kernInfo{
				first:  fields[1],
				second: fields[2],
			}
			kern.val, _ = strconv.Atoi(fields[3])
			kerning = append(kerning, kern)
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
			FontName = fields[1]
		case "CapHeight":
			// x, err := strconv.ParseFloat(fields[1], 64)
			// if err != nil {
			// 	panic("corrupted afm data for " + fname)
			// }
			// builtin.CapHeight = x
		case "XHeight":
			// x, err := strconv.ParseFloat(fields[1], 64)
			// if err != nil {
			// 	panic("corrupted afm data for " + fname)
			// }
			// builtin.XHeight = x
		case "Ascender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fname)
			}
			builtin.Ascent = x
		case "Descender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fname)
			}
			builtin.Descent = x
		case "CharWidth":
			panic("not implemented")
		case "StartCharMetrics":
			charMetrics = true
		case "StartKernPairs":
			kernPairs = true
		case "StartTrackKern":
			panic("not implemented")
		}
	}
	if err := scanner.Err(); err != nil {
		panic("corrupted afm data for " + fname)
	}

	// TODO(voss): LineGap, ...

	// store the ligature information
	// TODO(voss): automatically extend the subset to include ligature targets?
	for _, lig := range ligatures {
		a, aOk := nameToGlyph[lig.first]
		b, bOk := nameToGlyph[lig.second]
		c, cOk := nameToGlyph[lig.combined]
		if aOk && bOk && cOk {
			builtin.Ligatures[font.GlyphPair{a, b}] = c
		}
	}

	// store the kerning information
	builtin.Kerning = make(map[font.GlyphPair]int)
	for _, kern := range kerning {
		a, aOk := nameToGlyph[kern.first]
		b, bOk := nameToGlyph[kern.second]
		if aOk && bOk && kern.val != 0 {
			builtin.Kerning[font.GlyphPair{a, b}] = kern.val
		}
	}

	// pick a BaseEncoding
	bestName, bestRuneToCode := chooseBaseEncoding(stdRuneToCode, subset)

	// fill in the gaps
	var todo []rune
	used := make(map[byte]bool)
	used[0] = true
	for r, ok := range subset {
		if !ok {
			continue
		}
		c, ok := bestRuneToCode[r]
		if ok {
			used[c] = true
		} else {
			todo = append(todo, r)
		}
	}
	sort.Slice(todo, func(i, j int) bool {
		return todo[i] < todo[j]
	})
	var unused []byte
	for i := 1; i < 256; i++ {
		if c := byte(i); !used[c] {
			unused = append(unused, c)
		}
	}
	if len(todo) > len(unused) {
		return nil, errors.New("subset too large")
	}
	type D struct {
		name string
		c    byte
	}
	var diff []D
	var missing []rune
	for i, r := range todo {
		c := unused[i]
		name, ok := runeToName[r]
		if !ok {
			missing = append(missing, r)
			continue
		}
		bestRuneToCode[r] = c
		diff = append(diff, D{name: name, c: c})
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("glyphs missing from font: %q", string(missing))
	}

	// Construct the /Encoding dict
	var Encoding pdf.Object
	if len(diff) == 0 {
		Encoding = bestName
	} else {
		Differences := pdf.Array{}
		next := byte(0)
		for _, d := range diff {
			if d.c != next {
				Differences = append(Differences, pdf.Integer(d.c))
			}
			Differences = append(Differences, pdf.Name(d.name))
			next = d.c + 1
		}
		Encoding = pdf.Dict{
			"Type":         pdf.Name("Encoding"),
			"BaseEncoding": bestName,
			"Differences":  Differences,
		}
	}

	// glyphToCode maps from character indices to bytes in a PDF string.
	// TODO(voss): use a slice instead of a map?
	glyphToCode := make(map[font.GlyphIndex]byte)
	for r, cIdx := range builtin.CMap {
		glyphToCode[cIdx] = bestRuneToCode[r]
	}

	builtin.Enc = func(ii ...font.GlyphIndex) []byte {
		res := make([]byte, len(ii))
		for i, idx := range ii {
			res[i] = glyphToCode[idx]
		}
		return res
	}

	Font := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(FontName),
		"Encoding": Encoding,
	}
	// TODO(voss): for w.Version == pdf.V1_0 we should set Font["Name"]
	builtin.Ref, err = w.Write(Font, nil)
	if err != nil {
		return nil, err
	}

	return builtin, nil
}

func chooseBaseEncoding(stdRuneToCode map[rune]byte, subset map[rune]bool) (pdf.Object, map[rune]byte) {
	var bestName pdf.Object
	bestCount := len(stdRuneToCode)
	bestRuneToCode := stdRuneToCode
	for name, enc := range stdEncs {
		count := 0
		runeToCode := make(map[rune]byte)
		for r, ok := range subset {
			if !ok {
				continue
			}
			if c, ok := enc.Encode(r); ok {
				count++
				runeToCode[r] = c
			}
		}
		if count > bestCount {
			bestName = name
			bestCount = count
			bestRuneToCode = runeToCode
		}
	}
	return bestName, bestRuneToCode
}

var stdEncs = map[pdf.Name]font.Encoding{
	"MacRomanEncoding": font.MacRomanEncoding,
	"WinAnsiEncoding":  font.WinAnsiEncoding,
	// TODO(voss): add MacExpertEncoding
}
