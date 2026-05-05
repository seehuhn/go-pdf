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
	"encoding/xml"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/xmp"
)

// ShowMetadata prints the metadata of a PDF file to stdout.
func ShowMetadata(r pdf.Getter) error {
	m := r.GetMeta()

	fmt.Println("PDF Version:", m.Version)
	if len(m.ID) > 0 {
		fmt.Println("ID:")
		for _, id := range m.ID {
			fmt.Printf("  %x\n", id)
		}
	}
	fmt.Println()

	if m.Info != nil {
		showInfo(m.Trailer["Info"], m.Info)
	}

	if m.Catalog.Metadata != nil {
		showXMP(m.Catalog.Metadata)
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
	if !info.CreationDate.IsZero() {
		fmt.Println("CreationDate:", info.CreationDate)
	}
	if !info.ModDate.IsZero() {
		fmt.Println("ModDate:", info.ModDate)
	}
	if trapped, ok := info.Trapped.Get(); ok {
		if trapped {
			fmt.Println("Trapped: True")
		} else {
			fmt.Println("Trapped: False")
		}
	}
	for name, value := range info.Custom {
		fmt.Printf("%s: %v\n", name, value)
	}
	fmt.Println()
}

func showXMP(md *pdf.MetadataStream) {
	title := "XMP Metadata stream"
	fmt.Println(title)
	fmt.Println(strings.Repeat("-", len(title)))
	fmt.Println()

	packet := md.Data

	dc := &xmp.DublinCore{}
	reportXMPDecodeErr(packet.Get(dc))
	showXMPStruct(packet, dc)

	basic := &xmp.Basic{}
	reportXMPDecodeErr(packet.Get(basic))
	showXMPStruct(packet, basic)

	pdfNS := &xmp.PDF{}
	reportXMPDecodeErr(packet.Get(pdfNS))
	showXMPStruct(packet, pdfNS)

	xmpMM := &xmp.MediaManagement{}
	reportXMPDecodeErr(packet.Get(xmpMM))
	showXMPStruct(packet, xmpMM)

	xmpRights := &xmp.RightsManagement{}
	reportXMPDecodeErr(packet.Get(xmpRights))
	showXMPStruct(packet, xmpRights)

	fmt.Println()
}

// reportXMPDecodeErr writes per-property decode failures from
// [xmp.Packet.Get] to stderr without aborting the dump.
func reportXMPDecodeErr(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
}

func showXMPStruct(p *xmp.Packet, v any) {
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
				showXMPValue(p, pfx+propertyName, val)
			}
		}
	}
}

func showXMPValue(p *xmp.Packet, label string, value xmp.Value) {
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
		if !value.Default.IsZero() {
			showXMPValue(p, "  [x-default]", value.Default)
		}
		for key, elem := range value.V {
			lab := fmt.Sprintf("  [%s]", key)
			showXMPValue(p, lab, elem)
		}
		ll := getXMPQualifiers(value.Q)
		for _, q := range ll {
			fmt.Println("  " + q)
		}
	default:
		raw, err := value.EncodeXMP(p)
		if err != nil {
			fmt.Println(label + " <invalid: " + err.Error() + ">")
			return
		}
		lines := getXMPRaw(label, raw)
		for _, line := range lines {
			fmt.Println(line)
		}
	}
}

func getXMPRaw(label string, value xmp.Raw) []string {
	var lines []string
	switch value := value.(type) {
	case xmp.Text:
		line := label + " " + value.V
		qq := getXMPQualifiers(value.Q)
		if len(qq) == 1 {
			lines = append(lines, line+" ["+qq[0]+"]")
		} else {
			lines = append(lines, line)
			for _, q := range qq {
				lines = append(lines, "  "+q)
			}
		}
	case xmp.URL:
		line := label + " " + value.V.String()
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
		keys := slices.SortedFunc(maps.Keys(value.Value), func(a, b xml.Name) int {
			if a.Space != b.Space {
				if a.Space < b.Space {
					return -1
				}
				return 1
			}
			if a.Local < b.Local {
				return -1
			}
			if a.Local > b.Local {
				return 1
			}
			return 0
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
		lines = append(lines, label)
		for i, elem := range value.Value {
			var label string
			switch value.Kind {
			case xmp.Ordered:
				label = fmt.Sprintf("%d.", i+1)
			case xmp.Unordered:
				label = "-"
			case xmp.Alternative:
				label = "*"
			}
			ll := getXMPRaw(label, elem)
			for _, l := range ll {
				lines = append(lines, "  "+l)
			}
		}
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
		var key string
		if q.Name.Space == xmp.NSXML {
			key = fmt.Sprintf("xml:%s", q.Name.Local)
		} else {
			key = fmt.Sprintf("%s %s:", q.Name.Space, q.Name.Local)
		}
		ll := getXMPRaw(key, q.Value)
		res = append(res, ll...)
	}
	return res
}
