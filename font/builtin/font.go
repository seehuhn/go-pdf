package builtin

import (
	"bufio"
	"strconv"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
)

func Embed(w *pdf.Writer, fname string, subset map[rune]bool) (*font.Font, error) {
	fd, err := afmData.Open("afm/" + fname + ".afm")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	// enc maps between unicode runes and bytes in PDF strings.
	enc := font.CustomEncoding(subset)
	// glyphToByte maps from character indices to bytes in a PDF string.
	glyphToByte := make(map[font.GlyphIndex]byte)

	builtin := &font.Font{
		CMap: map[rune]font.GlyphIndex{},
	}

	var FontName string
	byName := make(map[string]font.GlyphIndex)
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
			var BBox font.Rect

			keyVals := strings.Split(line, ";")
			for _, keyVal := range keyVals {
				ff := strings.Fields(keyVal)
				if len(ff) < 2 {
					continue
				}
				switch ff[0] {
				case "C":
					// TODO(voss): is this needed?
					// glyphIdx, _ = strconv.Atoi(ff[1])
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
					// if len(ff) != 3 {
					// 	panic("corrupted afm data for " + fontName)
					// }
					// ligTmp = append(ligTmp, &ligInfo{
					// 	second:   ff[1],
					// 	combined: ff[2],
					// })
				default:
					panic(ff[0] + " not implemented")
				}
			}

			rr := names.ToUnicode(name, dingbats)
			if len(rr) != 1 {
				panic("not implemented")
			}
			r := rr[0]
			if subset[r] {
				builtin.CMap[r] = cIdx
				c, ok := enc.Encode(r)
				if ok {
					glyphToByte[cIdx] = c
				}
				byName[name] = cIdx
				builtin.GlyphExtent = append(builtin.GlyphExtent, BBox)
				builtin.Width = append(builtin.Width, width)
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

	builtin.Enc = func(ii ...font.GlyphIndex) []byte {
		res := make([]byte, len(ii))
		for i, idx := range ii {
			res[i] = glyphToByte[idx]
		}
		return res
	}

	// TODO(voss): builtin.Ligatures, LineGap, ...

	builtin.Kerning = make(map[font.GlyphPair]int)
	for _, kern := range kerning {
		a, aOk := byName[kern.first]
		b, bOk := byName[kern.second]
		if !aOk || !bOk || kern.val == 0 {
			continue
		}
		builtin.Kerning[font.GlyphPair{a, b}] = kern.val
	}

	Font := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(FontName),
		"Encoding": font.Describe(enc),
	}
	// TODO(voss): for w.Version == pdf.V1_0 we should set Font["Name"]
	builtin.Ref, err = w.Write(Font, nil)
	if err != nil {
		return nil, err
	}

	return builtin, nil
}
