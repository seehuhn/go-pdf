package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"seehuhn.de/go/pdf/font/truetype"
)

const (
	// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap#platform-ids
	ttPlatformUnicode   = 0
	ttPlatformMacintosh = 1
	ttPlatformWindows   = 3

	// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap#unicode-platform-platform-id--0
	ttUnicode1_0          = 0 // deprecated
	ttUnicode1_1          = 1 // deprecated
	ttUnicodeISO10646     = 2 // deprecated
	ttUnicodeBMP          = 3
	ttUnicodeFull         = 4
	ttUnicodeUVS          = 5 // for use with subtable format 14
	ttUnicodeFallbackFull = 6 // for use with subtable format 13

	ttWindowsSymbol      = 0
	ttWindowsUnicodeBMP  = 1
	ttWindowsShiftJIS    = 2
	ttWindowsPRC         = 3
	ttWindowsBig5        = 4
	ttWindowsWansung     = 5
	ttWindowsJohab       = 6
	ttWindowsUnicodeFull = 10
)

type cmapTable struct {
	PlatformID     uint16
	EncodingID     uint16
	SubtableOffset uint32
}

func pickTable(tables []*cmapTable) *cmapTable {
	// try to find 32 bit unicode encoding
	for _, table := range tables {
		if table.PlatformID == ttPlatformWindows &&
			table.EncodingID == ttWindowsUnicodeFull ||
			table.PlatformID == ttPlatformUnicode &&
				table.EncodingID == ttUnicodeFull {
			return table
		}
	}

	// try to find a 16 bit unicode encoding
	for _, table := range tables {
		if table.PlatformID == ttPlatformWindows &&
			table.EncodingID == ttWindowsUnicodeBMP ||
			table.PlatformID == ttPlatformUnicode &&
				table.EncodingID == ttUnicodeBMP {
			return table
		}
	}

	return nil
}

func tryFont(fname string) error {
	font, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer font.Close()

	info, err := truetype.ReadHeader(font)
	if err != nil {
		return err
	}

	cmapInfo, ok := info.Tables["cmap"]
	if !ok {
		return errors.New("no cmap table")
	}

	cmapFd := io.NewSectionReader(font, int64(cmapInfo.Offset), int64(cmapInfo.Length))

	cmapIndex := &struct {
		Version   uint16
		NumTables uint16
	}{}
	err = binary.Read(cmapFd, binary.BigEndian, cmapIndex)
	if err != nil {
		return err
	}
	if cmapIndex.Version != 0 {
		return errors.New("unsupported cmap version " +
			strconv.Itoa(int(cmapIndex.Version)))
	}

	var tables []*cmapTable
	for i := 0; i < int(cmapIndex.NumTables); i++ {
		encodingRecord := &cmapTable{}
		err = binary.Read(cmapFd, binary.BigEndian, encodingRecord)
		if err != nil {
			return err
		}
		PlatID := encodingRecord.PlatformID
		if PlatID != 0 && PlatID != 1 && PlatID != 3 {
			continue
		}
		tables = append(tables, encodingRecord)
	}
	if len(tables) == 0 {
		return errors.New("no cmap tables found")
	}

	table := pickTable(tables)
	if table == nil {
		return errors.New("unsupported font character encoding")
	}

	// The OpenType spec at
	// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap
	// documents the following cmap subtable formats:
	//     Format 0: Byte encoding table
	//     Format 2: High-byte mapping through table
	//     Format 4: Segment mapping to delta values
	//     Format 6: Trimmed table mapping
	//     Format 8: mixed 16-bit and 32-bit coverage
	//     Format 10: Trimmed array
	//     Format 12: Segmented coverage
	//     Format 13: Many-to-one range mappings
	//     Format 14: Unicode Variation Sequences
	// For the *.ttf and *.otf files on my system, I have found the
	// following frequencies for these formats:
	//
	//     count | format
	//     ------+-----------
	//     21320 | Format 4
	//      5747 | Format 6
	//      3519 | Format 0
	//      3225 | Format 12
	//       143 | Format 2
	//       107 | Format 14
	//         4 | Format 13

	_, err = cmapFd.Seek(int64(table.SubtableOffset), io.SeekStart)
	if err != nil {
		return err
	}
	var format uint16
	err = binary.Read(cmapFd, binary.BigEndian, &format)
	if err != nil {
		return err
	}

	switch format {
	case 4:
		type cmapFormat4 struct {
			Length        uint16
			Language      uint16
			SegCountX2    uint16
			SearchRange   uint16
			EntrySelector uint16
			RangeShift    uint16
		}
		data := &cmapFormat4{}
		err = binary.Read(cmapFd, binary.BigEndian, data)
		if err != nil {
			return err
		}
		if data.SegCountX2%2 != 0 {
			return errors.New("corrupted cmap subtable")
		}
		segCount := int(data.SegCountX2 / 2)
		tt := make([]uint16, 4*segCount+1)
		err = binary.Read(cmapFd, binary.BigEndian, tt)
		if err != nil {
			return err
		}
		startCode := tt[segCount+1 : 2*segCount+1]
		endCode := tt[:segCount]
		idDelta := tt[2*segCount+1 : 3*segCount+1]
		idRangeOffset := tt[3*segCount+1 : 4*segCount+1]
		glyphIdBase, err := cmapFd.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}

		for k := 0; k < segCount; k++ {
			if idRangeOffset[k] == 0 {
				delta := idDelta[k]
				for r := rune(startCode[k]); r <= rune(endCode[k]); r++ {
					c := uint32(uint16(r) + delta)
					if c == 0 {
						continue
					}
					fmt.Printf("%10q %5d -> %d\n", string([]rune{r}), r, c)
				}
			} else {
				d := int(idRangeOffset[k])/2 - (segCount - k)
				if d < 0 {
					return errors.New("corrupt cmap table")
				}
				tmp := make([]uint16, int(endCode[k]-startCode[k])+1)
				_, err = cmapFd.Seek(glyphIdBase+2*int64(d), io.SeekStart)
				if err != nil {
					return err
				}
				err = binary.Read(cmapFd, binary.BigEndian, tmp)
				if err != nil {
					return err
				}
				for r := rune(startCode[k]); r <= rune(endCode[k]); r++ {
					c := uint32(tmp[int(r)-int(startCode[k])])
					fmt.Printf("%10q %5d -> %d\n", string([]rune{r}), r, c)
				}
			}
		}
		panic("fish")

	case 12:
		type cmapFormat12 struct {
			_         uint16 // reserved
			Length    uint32
			Language  uint32
			NumGroups uint32
		}
		data := &cmapFormat12{}
		err = binary.Read(cmapFd, binary.BigEndian, data)
		if err != nil {
			return err
		}

		type segment struct {
			StartCharCode uint32 //	First character code in this group
			EndCharCode   uint32 //	Last character code in this group
			StartGlyphID  uint32 //	Glyph index corresponding to the starting character code
		}
		for i := 0; i < int(data.NumGroups); i++ {
			seg := &segment{}
			err = binary.Read(cmapFd, binary.BigEndian, seg)
			if err != nil {
				return err
			}
			if seg.EndCharCode < seg.StartCharCode || seg.EndCharCode > 0x10FFFF {
				return errors.New("invalid character code in font")
			}

			c := seg.StartGlyphID
			for r := rune(seg.StartCharCode); r <= rune(seg.EndCharCode); r++ {
				fmt.Printf("%10q %5d -> %d\n", string([]rune{r}), r, c)
				c++
			}
		}
		panic("fish")

	default:
		fmt.Println(format)
		// try another table
	}

	return nil
}

func main() {
	fd, err := os.Open("all-fonts")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		fname := scanner.Text()
		err = tryFont(fname)
		if err != nil {
			fmt.Println(fname+":", err)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("main loop failed:", err)
	}
}
