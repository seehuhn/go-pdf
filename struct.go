// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pdf

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/text/language"
)

// AsDict creates a PDF Dict object, encoding the fields of a Go struct.
//
// Field tags control encoding:
//   - "optional": skip the field when its value is the Go zero value.
//   - "extra": the field is a map[string]string whose entries are emitted
//     as additional dict entries.
//   - "-": skip the field entirely; manual handling owns the corresponding
//     dict key.
//   - A "Key=Value" tag on a struct{} dummy field emits the literal pair.
//
// If the argument is nil, the result is nil.
func AsDict(s any) Dict {
	if s == nil {
		return nil
	}

	v := reflect.Indirect(reflect.ValueOf(s))
	if v.Kind() != reflect.Struct {
		return nil
	}
	vt := v.Type()

	res := make(Dict)
fieldLoop:
	for i := 0; i < vt.NumField(); i++ {
		fVal := v.Field(i)
		fInfo := vt.Field(i)

		optional := false
		for t := range strings.SplitSeq(fInfo.Tag.Get("pdf"), ",") {
			switch t {
			case "":
				// pass
			case "-":
				continue fieldLoop
			case "optional":
				optional = true
			case "extra":
				for key, val := range fVal.Interface().(map[string]string) {
					res[Name(key)] = TextString(val)
				}
				continue fieldLoop
			default:
				assign := strings.SplitN(t, "=", 2)
				if len(assign) != 2 {
					continue
				}
				res[Name(assign[0])] = Name(assign[1])
			}
		}

		key := Name(fInfo.Name)
		switch {
		case optional && fVal.IsZero():
			continue
		case fInfo.Type == textStringType:
			res[key] = fVal.Interface().(TextString)
		case fInfo.Type == dateType:
			res[key] = fVal.Interface().(Date)
		case fInfo.Type == languageType:
			tag := fVal.Interface().(language.Tag)
			if !tag.IsRoot() {
				res[key] = TextString(tag.String())
			}
		case fInfo.Type == versionType:
			version := fVal.Interface().(Version)
			versionString, err := version.ToString()
			if err == nil { // ignore invalid and unknown versions
				res[key] = Name(versionString)
			}
		case fVal.Kind() == reflect.Bool:
			res[key] = Boolean(fVal.Bool())
		case fInfo.Type == referenceType:
			ref := fVal.Interface().(Reference)
			if ref != 0 {
				res[key] = ref
			}
		default:
			if fVal.CanInterface() {
				val := fVal.Interface()
				if obj, ok := val.(Object); ok {
					res[key] = obj
				} else {
					panic(fmt.Sprintf("unsupported field type %T", val))
				}
			}
		}
	}

	return res
}

var (
	referenceType = reflect.TypeFor[Reference]()

	textStringType = reflect.TypeFor[TextString]()
	dateType       = reflect.TypeFor[Date]()
	languageType   = reflect.TypeFor[language.Tag]()
	versionType    = reflect.TypeFor[Version]()
)
