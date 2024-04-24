// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package meta

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/xmp"
)

// ShowMetadata prints the metadata of a PDF file to stdout.
func ShowMetadata(r *pdf.Reader) error {
	m := r.GetMeta()

	fmt.Println("PDF Version:", m.Version)
	if len(m.ID) > 0 {
		fmt.Println("ID:")
		for _, id := range m.ID {
			fmt.Printf("  %x\n", id)
		}
	}
	fmt.Println()

	count := 0
	if m.Info != nil {
		showInfo(m.Trailer["Info"], m.Info)
		count++
	}

	if m.Catalog.Metadata != 0 {
		err := showXMP(r, m.Catalog.Metadata)
		if err != nil {
			return err
		}
		count++
	}

	return nil
}

func showInfo(obj pdf.Object, info *pdf.Info) {
	infoLoc := "embedded in the file trailer dictionary"
	if ref, ok := obj.(pdf.Reference); ok {
		infoLoc = ref.String()
	}

	title := fmt.Sprintf("Document Information Dictionary (%s)", infoLoc)
	fmt.Println(title)
	fmt.Println(strings.Repeat("-", len(title)))
	fmt.Println()

	if info.Title != "" {
		fmt.Println("Title:", info.Title)
	}
	if info.Author != "" {
		fmt.Println("Author:", info.Author)
	}
	if info.Subject != "" {
		fmt.Println("Subject:", info.Subject)
	}
	if info.Keywords != "" {
		fmt.Println("Keywords:", info.Keywords)
	}
	if info.Creator != "" {
		fmt.Println("Creator:", info.Creator)
	}
	if info.Producer != "" {
		fmt.Println("Producer:", info.Producer)
	}
	if info.CreationDate.IsZero() {
		fmt.Println("CreationDate:", info.CreationDate)
	}
	if !info.ModDate.IsZero() {
		fmt.Println("ModDate:", info.ModDate)
	}
	if info.Trapped != "" {
		fmt.Println("Trapped:", info.Trapped)
	}
	fmt.Println()
}

func showXMP(r *pdf.Reader, ref pdf.Reference) error {
	title := fmt.Sprintf("XMP Metadata stream (%s)", ref)
	fmt.Println(title)
	fmt.Println(strings.Repeat("-", len(title)))
	fmt.Println()

	stm, err := pdf.GetStream(r, ref)
	if err != nil {
		return err
	}

	body, err := pdf.DecodeStream(r, stm, 0)
	if err != nil {
		return err
	}

	packet, err := xmp.Read(body)
	if err != nil {
		return err
	}

	dc := &xmp.DublinCore{}
	packet.Get(dc)
	showXMPStruct(dc)

	basic := &xmp.XMP{}
	packet.Get(basic)
	showXMPStruct(basic)

	fmt.Println()

	names := maps.Keys(packet.Properties)
	sort.Slice(names, func(i, j int) bool {
		if names[i].Space != names[j].Space {
			return names[i].Space < names[j].Space
		}
		return names[i].Local < names[j].Local
	})
	for _, name := range names {
		label := fmt.Sprintf("%s %s:", name.Space, name.Local)
		raw := packet.Properties[name]
		lines := getXMPRaw(label, raw)
		for _, line := range lines {
			fmt.Println(line)
		}
	}

	return nil
}

func showXMPStruct(v any) {
	s := reflect.Indirect(reflect.ValueOf(v))
	if s.Kind() != reflect.Struct {
		panic("not a struct")
	}
	st := s.Type()

	prefixTagType := reflect.TypeFor[xmp.Prefix]()
	typeType := reflect.TypeFor[xmp.Value]()

	var pfx string
	for i := 0; i < st.NumField(); i++ {
		if s.Field(i).Type() == prefixTagType {
			pfx = st.Field(i).Tag.Get("xmp") + ":"
			break
		}
	}

	for i := 0; i < st.NumField(); i++ {
		fVal := s.Field(i)
		fInfo := st.Field(i)

		if fVal.CanInterface() && fVal.Type().Implements(typeType) {
			val := fVal.Interface().(xmp.Value)
			propertyName := fInfo.Tag.Get("xmp")
			if propertyName == "" {
				propertyName = fInfo.Name
			}
			if !val.IsZero() {
				showXMPValue(pfx+propertyName, val)
			}
		}
	}
}

func showXMPValue(label string, value xmp.Value) {
	switch value := value.(type) {
	case xmp.Date:
		line := label + " " + value.V.String()
		ll := getXMPQualifiers(value.Q)
		if len(ll) == 1 {
			fmt.Println(line + " [" + ll[0] + "]")
		} else {
			fmt.Println(line)
			for _, q := range ll {
				fmt.Println("  " + q)
			}
		}
	case xmp.Localized:
		fmt.Println(label)
		showXMPValue("  [x-default]", value.Default)
		for key, elem := range value.V {
			lab := fmt.Sprintf("  [%s]", key)
			showXMPValue(lab, elem)
		}
		ll := getXMPQualifiers(value.Q)
		for _, q := range ll {
			fmt.Println("  " + q)
		}
	case xmp.OrderedArray[xmp.ProperName]:
		fmt.Println(label)
		for i, elem := range value.V {
			showXMPValue(fmt.Sprintf("  [%d]", i+1), elem)
		}
		ll := getXMPQualifiers(value.Q)
		for _, q := range ll {
			fmt.Println("  " + q)
		}
	default:
		raw := value.GetXMP()
		lines := getXMPRaw(label, raw)
		for _, line := range lines {
			fmt.Println(line)
		}
	}
}

func getXMPRaw(label string, value xmp.Raw) []string {
	var lines []string
	switch value := value.(type) {
	case xmp.RawText:
		line := label + " " + value.Value
		qq := getXMPQualifiers(value.Q)
		if len(qq) == 1 {
			lines = append(lines, line+" ["+qq[0]+"]")
		} else {
			lines = append(lines, line)
			for _, q := range qq {
				lines = append(lines, "  "+q)
			}
		}
	case xmp.RawURI:
		line := label + " " + value.Value.String()
		qq := getXMPQualifiers(value.Q)
		if len(qq) == 1 {
			lines = append(lines, line+" ["+qq[0]+"]")
		} else {
			lines = append(lines, line)
			for _, q := range qq {
				lines = append(lines, "  "+q)
			}
		}
	case xmp.RawStruct:
		keys := maps.Keys(value.Value)
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].Space != keys[j].Space {
				return keys[i].Space < keys[j].Space
			}
			return keys[i].Local < keys[j].Local
		})
		for _, key := range keys {
			lab := fmt.Sprintf("%s %s:", key.Space, key.Local)
			ll := getXMPRaw(lab, value.Value[key])
			if len(ll) == 1 {
				lines = append(lines, lab+" "+ll[0])
			} else {
				lines = append(lines, lab)
				for _, l := range ll {
					lines = append(lines, "  "+l)
				}
			}
		}
		qq := getXMPQualifiers(value.Q)
		for _, q := range qq {
			lines = append(lines, "  "+q)
		}
	case xmp.RawArray:
		var start, end string
		switch value.Kind {
		case xmp.Ordered:
			start, end = "[", "]"
		case xmp.Unordered:
			start, end = "{", "}"
		case xmp.Alternative:
			start, end = "(", ")"
		}
		lines = append(lines, label+" "+start)
		for _, elem := range value.Value {
			ll := getXMPRaw("  -", elem)
			for _, l := range ll {
				lines = append(lines, "  "+l)
			}
		}
		lines = append(lines, "  "+end)
		qq := getXMPQualifiers(value.Q)
		for _, q := range qq {
			lines = append(lines, "  "+q)
		}
	}
	return lines
}

func getXMPQualifiers(Q []xmp.Qualifier) []string {
	var res []string
	for _, q := range Q {
		key := fmt.Sprintf("%s %s:", q.Name.Space, q.Name.Local)
		ll := getXMPRaw(key, q.Value)
		res = append(res, ll...)
	}
	return res
}
