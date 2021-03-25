package builtin

import (
	"bufio"
	"strconv"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
)

func Embed(w *pdf.Writer, fname string, subset map[rune]bool) (*font.NewFont, error) {
	fd, err := afmData.Open("afm/" + fname + ".afm")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	info := &font.Info{
		CMap: map[rune]font.GlyphIndex{},
	}

	dingbats := fname == "ZapfDingbats"
	charMetrics := false
	kernPairs := false
	cIdx := font.GlyphIndex(0)
	// TODO(voss): prepend an artificial entry for .notdef, so that CMap works
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
			var BBox font.NewRect

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
					rr := names.ToUnicode(name, dingbats)
					if len(rr) != 1 {
						panic("not implemented")
					}
					info.CMap[rr[0]] = cIdx
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
			info.GlyphExtent = append(info.GlyphExtent, BBox)
			info.Width = append(info.Width, width)
			cIdx++
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
			// kern := &kernInfo{
			// 	first:  fields[1],
			// 	second: fields[2],
			// }
			// kern.val, _ = strconv.ParseFloat(fields[3], 64)
			// kerning = append(kerning, kern)
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
			info.FontName = fields[1]
		case "CapHeight":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fname)
			}
			info.CapHeight = x
		case "XHeight":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fname)
			}
			info.XHeight = x
		case "Ascender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fname)
			}
			info.Ascent = x
		case "Descender":
			x, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				panic("corrupted afm data for " + fname)
			}
			info.Descent = x
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

	enc := font.CustomEncoding(subset)
	encoding := font.Describe(enc)

	cmap := map[rune]font.GlyphIndex{}
	for r, ok := range subset {
		if !ok {
			continue
		}
		idx, ok := enc.Encode(r)
		if ok {
			cmap[r] = font.GlyphIndex(idx)
		}
	}

	Font := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(info.FontName),
		"Encoding": encoding,
	}
	// TODO(voss): for w.Version == pdf.V1_0 we should set Font["Name"]

	FontRef, err := w.Write(Font, nil)
	if err != nil {
		return nil, err
	}

	font := &font.NewFont{
		Ref:  FontRef,
		CMap: cmap,
		Enc: func(idx font.GlyphIndex) []byte {
			return []byte{byte(idx)} // TODO(voss): ...
		},
		Ligatures:   map[font.NewGlyphPair]font.GlyphIndex{}, // TODO(voss): ...
		Kerning:     map[font.NewGlyphPair]int{},             // TODO(voss): ...
		GlyphExtent: info.GlyphExtent,
		Width:       info.Width,
		Ascent:      info.Ascent,
		Descent:     info.Descent,
		LineGap:     info.LineGap,
	}

	return font, nil
}
