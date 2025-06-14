// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package traverse

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
)

type objectCtx struct {
	r   pdf.Getter
	obj pdf.Object
}

func (c *objectCtx) Next(key string) (Context, error) {
	var obj pdf.Object
	var err error

	switch x := c.obj.(type) {
	case pdf.Dict:
		forceKey := false
		if strings.HasPrefix(key, "/") {
			key = key[1:]
			forceKey = true
		}

		// Special case: @contents for Page objects
		if key == "@contents" {
			tp, err := pdf.GetName(c.r, x["Type"])
			if err == nil && tp == "Page" {
				_, hasParent := x["Parent"]
				_, hasContents := x["Contents"]
				if hasParent && hasContents {
					reader, err := pagetree.ContentStream(c.r, x)
					if err != nil {
						return nil, err
					}
					return &streamCtx{
						r:    reader,
						name: "page contents",
					}, nil
				}
			}
			return nil, &KeyError{Key: key, Ctx: "page dict"}
		}

		// Special case: @font for Font objects
		if key == "@font" {
			tp, err := pdf.GetName(c.r, x["Type"])
			if err == nil && tp == "Font" {
				_, hasSubtype := x["Subtype"]
				if hasSubtype {
					return newFontDictCtx(c.r, x)
				}
			}
			return nil, &KeyError{Key: key, Ctx: "font dict"}
		}

		// Special case: Pages dict with numeric keys for page access
		if !forceKey && intRegexp.MatchString(key) {
			tp, err := pdf.GetName(c.r, x["Type"])
			if err == nil && tp == "Pages" {
				_, hasKids := x["Kids"]
				_, hasCount := x["Count"]
				if hasKids && hasCount {
					if pageNo, err := strconv.ParseInt(key, 10, 0); err == nil {
						obj, _, err = pagetree.GetPage(c.r, int(pageNo)-1)
						if err != nil {
							return nil, err
						}
						// Page lookup succeeded, skip dict key lookup
					} else {
						// Fall through to regular dict key lookup
						var ok bool
						obj, ok = x[pdf.Name(key)]
						if !ok {
							return nil, &KeyError{Key: key, Ctx: "Dict"}
						}
					}
				} else {
					// Fall through to regular dict key lookup
					var ok bool
					obj, ok = x[pdf.Name(key)]
					if !ok {
						return nil, &KeyError{Key: key, Ctx: "Dict"}
					}
				}
			} else {
				// Fall through to regular dict key lookup
				var ok bool
				obj, ok = x[pdf.Name(key)]
				if !ok {
					return nil, &KeyError{Key: key, Ctx: "Dict"}
				}
			}
		} else {
			// Regular dict key lookup
			var ok bool
			obj, ok = x[pdf.Name(key)]
			if !ok {
				return nil, &KeyError{Key: key, Ctx: "Dict"}
			}
		}

	case pdf.Array:
		idx, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return nil, &KeyError{Key: key, Ctx: "Array"}
		}
		// negative indices count from the end, as in Python
		if idx < 0 && idx+int64(len(x)) >= 0 {
			idx += int64(len(x))
		}
		if idx < 0 || idx >= int64(len(x)) {
			return nil, &KeyError{Key: key, Ctx: "Array"}
		}
		obj = x[idx]

	case *pdf.Stream:
		switch key {
		case "@encoded":
			return &rawStreamCtx{r: x.R}, nil
		case "@raw":
			if c.r == nil {
				return nil, errors.New("reader is nil, cannot decode stream")
			}
			decoded, err := pdf.DecodeStream(c.r, x, 0)
			if err != nil {
				return nil, err
			}
			return &streamCtx{
				r:    decoded,
				name: "decoded stream",
			}, nil
		case "dict":
			obj = x.Dict
		default:
			key = strings.TrimPrefix(key, "/")
			var ok bool
			obj, ok = x.Dict[pdf.Name(key)]
			if !ok {
				return nil, &KeyError{Key: key, Ctx: "Stream dict"}
			}
		}

	default:
		return nil, &KeyError{Key: key, Ctx: fmt.Sprintf("%T", c.obj)}
	}

	obj, err = pdf.Resolve(c.r, obj)
	if err != nil {
		return nil, err
	}

	return &objectCtx{r: c.r, obj: obj}, nil
}

func (c *objectCtx) Show() error {
	if c.obj == nil {
		fmt.Println("null")
		return nil
	}

	switch obj := c.obj.(type) {
	case *pdf.Stream:
		err := c.showDict(obj.Dict)
		if err != nil {
			return err
		}
		fmt.Println()

		stmData, err := pdf.DecodeStream(c.r, obj, 0)
		if err != nil {
			return err
		}
		buf := make([]byte, 128)
		n, err := stmData.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			fmt.Println("empty stream")
			return nil
		}
		if mostlyBinary(buf[:n]) {
			m, err := io.Copy(io.Discard, stmData)
			if err != nil {
				return err
			}
			fmt.Printf("... binary stream data (%d bytes) ...\n", int64(n)+m)
			return nil
		}
		fmt.Println("decoded stream contents:")
		fmt.Print(string(buf[:n]))
		_, err = io.Copy(os.Stdout, stmData)
		if err != nil {
			return err
		}

	case pdf.Dict:
		return c.showDict(obj)

	case pdf.Array:
		fmt.Println("[")
		for i, elem := range obj {
			msg, err := c.explainSingleLine(elem)
			if err != nil {
				return err
			}
			extra := ""
			if i%10 == 0 || i == len(obj)-1 {
				extra = fmt.Sprintf("  %% %d", i)
			}
			fmt.Println(msg + extra)
		}
		fmt.Println("]")

	default:
		err := pdf.Format(os.Stdout, pdf.OptPretty, obj)
		if err != nil {
			return err
		}
		fmt.Println()
	}
	return nil
}

func (c *objectCtx) showDict(dict pdf.Dict) error {
	keys := dictKeys(dict)
	fmt.Println("<<")
	for _, key := range keys {
		err := pdf.Format(os.Stdout, 0, key)
		if err != nil {
			return err
		}
		valString, err := c.explainSingleLine(dict[key])
		if err != nil {
			return err
		}
		fmt.Println(" " + valString)
	}
	fmt.Println(">>")
	return nil
}

func (c *objectCtx) Keys() []string {
	switch obj := c.obj.(type) {
	case pdf.Dict:
		var keys []string

		// Check if this is a Page dict that supports @contents
		if tp, err := pdf.GetName(c.r, obj["Type"]); err == nil && tp == "Page" {
			_, hasParent := obj["Parent"]
			_, hasContents := obj["Contents"]
			if hasParent && hasContents {
				keys = append(keys, "`@contents`")
			}
		}

		// Check if this is a Font dict that supports @font
		if tp, err := pdf.GetName(c.r, obj["Type"]); err == nil && tp == "Font" {
			_, hasSubtype := obj["Subtype"]
			if hasSubtype {
				keys = append(keys, "`@font`")
			}
		}

		// Check if this is a Pages dict that supports numeric navigation
		if tp, err := pdf.GetName(c.r, obj["Type"]); err == nil && tp == "Pages" {
			_, hasKids := obj["Kids"]
			_, hasCount := obj["Count"]
			if hasKids && hasCount {
				keys = append(keys, "page numbers")
			}
		}

		if len(obj) > 0 {
			keys = append(keys, "dict keys")
		}

		return keys

	case pdf.Array:
		n := len(obj)
		if n == 0 {
			return nil
		}
		s := fmt.Sprintf("array indices (%d to %d)", -n, n-1)
		return []string{s}

	case *pdf.Stream:
		ss := []string{"`@encoded`", "`@raw`", "`dict`"}
		if len(obj.Dict) > 0 {
			ss = append(ss, "stream dict keys")
		}
		return ss

	default:
		return nil
	}
}

// Helper functions ported from main.go

func dictKeys(obj pdf.Dict) []pdf.Name {
	keys := maps.Keys(obj)
	sort.Slice(keys, func(i, j int) bool {
		if order(keys[i]) != order(keys[j]) {
			return order(keys[i]) < order(keys[j])
		}
		return keys[i] < keys[j]
	})
	return keys
}

func order(key pdf.Name) int {
	switch key {
	case "Type":
		return 0
	case "Subtype":
		return 1
	case "DescendantFonts":
		return 2
	case "BaseFont":
		return 3
	case "Encoding":
		return 4
	case "FontDescriptor":
		return 5
	case "FirstChar":
		return 10
	case "LastChar":
		return 11
	case "Widths":
		return 12
	default:
		return 999
	}
}

func mostlyBinary(buf []byte) bool {
	pos := 0
	n := len(buf)
	bad := 0
	for pos < n {
		r, size := utf8.DecodeRune(buf[pos:])
		if (r < 32 && r != '\t' && r != '\n' && r != '\r') || r == utf8.RuneError {
			bad++
		}
		pos += size
	}
	return bad > 16+n/10
}

func (c *objectCtx) explainSingleLine(obj pdf.Object) (string, error) {
	if obj == nil {
		return "null", nil
	}
	switch obj := obj.(type) {
	case *pdf.Stream:
		var parts []string
		tp, err := pdf.GetName(c.r, obj.Dict["Type"])
		if err == nil {
			parts = append(parts, string(tp)+" stream")
		} else {
			parts = append(parts, "stream")
		}
		length, err := pdf.GetInteger(c.r, obj.Dict["Length"])
		if err == nil {
			parts = append(parts, fmt.Sprintf("%d bytes", length))
		}
		ff, ok := obj.Dict["Filter"]
		if ok {
			if name, err := pdf.GetName(c.r, ff); err == nil {
				parts = append(parts, string(name))
			} else if arr, err := pdf.GetArray(c.r, ff); err == nil {
				for _, elem := range arr {
					if name, err := pdf.GetName(c.r, elem); err == nil {
						parts = append(parts, string(name))
					} else {
						parts = append(parts, "???")
					}
				}
			} else {
				parts = append(parts, "??!")
			}
		}
		return "<" + strings.Join(parts, ", ") + ">", nil
	case pdf.Dict:
		var parts []string
		if len(obj) <= 4 {
			keys := dictKeys(obj)
			for _, key := range keys {
				keyStr := string(key)
				if !strings.HasPrefix(keyStr, "/") {
					keyStr = "/" + keyStr
				}
				parts = append(parts, keyStr)
				valString, err := c.explainShort(obj[key])
				if err != nil {
					return "", err
				}
				parts = append(parts, valString)
			}
			return "<<" + strings.Join(parts, " ") + ">>", nil
		}
		tp, err := pdf.GetName(c.r, obj["Type"])
		if err == nil {
			parts = append(parts, string(tp)+" dict")
		} else {
			parts = append(parts, "dict")
		}
		if len(obj) != 1 {
			parts = append(parts, fmt.Sprintf("%d entries", len(obj)))
		} else {
			parts = append(parts, "1 entry")
		}
		return "<" + strings.Join(parts, ", ") + ">", nil
	case pdf.Array:
		if len(obj) <= 8 {
			var parts []string
			for _, elem := range obj {
				msg, err := c.explainShort(elem)
				if err != nil {
					return "", err
				}
				parts = append(parts, msg)
			}
			return "[" + strings.Join(parts, " ") + "]", nil
		}
		return fmt.Sprintf("<array, %d elements>", len(obj)), nil
	default:
		var buf bytes.Buffer
		err := pdf.Format(&buf, pdf.OptPretty, obj)
		return buf.String(), err
	}
}

func (c *objectCtx) explainShort(obj pdf.Object) (string, error) {
	if obj == nil {
		return "null", nil
	}
	switch obj := obj.(type) {
	case *pdf.Stream:
		return "stream", nil
	case pdf.Dict:
		return "<<...>>", nil
	case pdf.Array:
		return "[...]", nil
	default:
		var buf bytes.Buffer
		err := pdf.Format(&buf, 0, obj)
		return buf.String(), err
	}
}
