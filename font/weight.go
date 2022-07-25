// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"fmt"
	"strconv"
)

// Weight indicates the visual weight (degree of blackness or thickness of
// strokes) of the characters in the font.  Values from 1 to 1000 are valid.
type Weight uint16

// WeightFromString returns the weight class for the given string.
// This is the converse of Weight.String().
func WeightFromString(s string) Weight {
	switch s {
	case "Thin":
		return WeightThin
	case "Extra Light":
		return WeightExtraLight
	case "Light":
		return WeightLight
	case "Normal":
		return WeightNormal
	case "Medium":
		return WeightMedium
	case "Semi Bold":
		return WeightSemiBold
	case "Bold":
		return WeightBold
	case "Extra Bold":
		return WeightExtraBold
	case "Black":
		return WeightBlack
	default:
		x, _ := strconv.Atoi(s)
		if x < 0 || x > 1000 {
			x = 0
		}
		return Weight(x)
	}
}

func (w Weight) String() string {
	switch w {
	case WeightThin:
		return "Thin"
	case WeightExtraLight:
		return "Extra Light"
	case WeightLight:
		return "Light"
	case WeightNormal:
		return "Normal"
	case WeightMedium:
		return "Medium"
	case WeightSemiBold:
		return "Semi Bold"
	case WeightBold:
		return "Bold"
	case WeightExtraBold:
		return "Extra Bold"
	case WeightBlack:
		return "Black"
	default:
		return fmt.Sprintf("%d", w)
	}
}

// SimpleString converts the Weight to a string, prefering the nearest
// textual description over the precise numeric value where needed.
func (w Weight) SimpleString() string {
	w = (w + 50) / 100 * 100
	return w.String()
}

// Pre-defined weight classes.
const (
	WeightThin       Weight = 100
	WeightExtraLight Weight = 200
	WeightLight      Weight = 300
	WeightNormal     Weight = 400
	WeightMedium     Weight = 500
	WeightSemiBold   Weight = 600
	WeightBold       Weight = 700
	WeightExtraBold  Weight = 800
	WeightBlack      Weight = 900
)

// Width indicates the aspect ratio (width to height ratio) as specified by a
// font designer for the glyphs in a font.
type Width uint16

// WidthFromString returns the width class for the given string.
// This is the converse of Width.String().
func WidthFromString(s string) Width {
	switch s {
	case "Ultra Condensed":
		return WidthUltraCondensed
	case "Extra Condensed":
		return WidthExtraCondensed
	case "Condensed":
		return WidthCondensed
	case "Semi Condensed":
		return WidthSemiCondensed
	case "Normal":
		return WidthNormal
	case "Semi Expanded":
		return WidthSemiExpanded
	case "Expanded":
		return WidthExpanded
	case "Extra Expanded":
		return WidthExtraExpanded
	case "Ultra Expanded":
		return WidthUltraExpanded
	default:
		return 0
	}
}

func (w Width) String() string {
	switch w {
	case WidthUltraCondensed:
		return "Ultra Condensed"
	case WidthExtraCondensed:
		return "Extra Condensed"
	case WidthCondensed:
		return "Condensed"
	case WidthSemiCondensed:
		return "Semi Condensed"
	case WidthNormal:
		return "Normal"
	case WidthSemiExpanded:
		return "Semi Expanded"
	case WidthExpanded:
		return "Expanded"
	case WidthExtraExpanded:
		return "Extra Expanded"
	case WidthUltraExpanded:
		return "Ultra Expanded"
	default:
		return fmt.Sprintf("Width(%d)", w)
	}
}

// Valid width values.
const (
	WidthUltraCondensed Width = 1 // 50% of WidthNormal
	WidthExtraCondensed Width = 2 // 62.5% of WidthNormal
	WidthCondensed      Width = 3 // 75% of WidthNormal
	WidthSemiCondensed  Width = 4 // 87.5% of WidthNormal
	WidthNormal         Width = 5
	WidthSemiExpanded   Width = 6 // 112.5% of WidthNormal
	WidthExpanded       Width = 7 // 125% of WidthNormal
	WidthExtraExpanded  Width = 8 // 150% of WidthNormal
	WidthUltraExpanded  Width = 9 // 200% of WidthNormal
)
