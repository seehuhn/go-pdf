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
	"time"

	"golang.org/x/text/language"
)

// AsDict creates a PDF Dict object, encoding the fields of a Go struct.
// This is the converse of [DecodeDict].
func AsDict(s interface{}) Dict {
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
		isTextString := false
		for _, t := range strings.Split(fInfo.Tag.Get("pdf"), ",") {
			switch t {
			case "":
				// pass
			case "optional":
				optional = true
			case "text string":
				isTextString = true
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
		case isTextString:
			res[key] = TextString(fVal.Interface().(string))
		case fInfo.Type == versionType:
			version := fVal.Interface().(Version)
			versionString, err := version.ToString()
			if err == nil { // ignore invalid and unknown versions
				res[key] = Name(versionString)
			}
		case fInfo.Type == timeType:
			res[key] = Date(fVal.Interface().(time.Time))
		case fInfo.Type == languageType:
			tag := fVal.Interface().(language.Tag)
			if !tag.IsRoot() {
				res[key] = TextString(tag.String())
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
				res[key] = fVal.Interface().(Object)
			}
		}
	}

	return res
}

// DecodeDict initialises a struct using the data from a PDF dictionary.
// The argument dst must be a pointer to a struct, or the function will panic.
//
// Go struct tags can be used to control the decoding process.  The following
// tags are supported:
//
//   - "optional": the field is optional and may be omitted from the PDF
//     dictionary.  Omitted fields default to the Go zero value for the
//     field type.
//   - "text string": the field is a string which should be encoded as a PDF
//     text string.
//   - "allowstring": the field is a Name, but the PDF dictionary may contain
//     a String instead.  If a String is found, it will be converted to a Name.
//   - "extra": the field is a map[string]string which contains all
//     entries in the PDF dictionary which are not otherwise decoded.
//
// This function is the converse of [AsDict].
func DecodeDict(r Getter, dst interface{}, src Dict) error {
	v := reflect.Indirect(reflect.ValueOf(dst))
	vt := v.Type()

	// To allow parsing malformed PDF files, we don't abort on error.  Instead,
	// we fill all struct fields we can and then return the first error
	// encountered.
	var firstErr error

	seen := map[string]bool{}
	extra := -1
fieldLoop:
	for i := 0; i < vt.NumField(); i++ {
		fVal := v.Field(i)
		if !fVal.CanSet() {
			continue
		}
		fInfo := vt.Field(i)
		seen[fInfo.Name] = true
		fVal.Set(reflect.Zero(fInfo.Type)) // zero all fields

		// read the struct tags
		optional := false
		isTextString := false
		allowstring := false
		for _, t := range strings.Split(fInfo.Tag.Get("pdf"), ",") {
			switch t {
			case "optional":
				optional = true
			case "text string":
				isTextString = true
			case "allowstring":
				allowstring = true
			case "extra":
				extra = i
				continue fieldLoop
			}
		}

		// get and fix up the value from the Dict
		dictVal := src[Name(fInfo.Name)]
		if fInfo.Type != objectType && fInfo.Type != refType {
			// follow references to indirect objects where needed
			obj, err := Resolve(r, dictVal)
			if err != nil {
				firstErr = err
				continue
			}
			dictVal = obj
		}
		if dictVal == nil {
			if !optional && firstErr == nil {
				firstErr = fmt.Errorf("required Dict entry /%s not found",
					fInfo.Name)
			}
			continue
		}
		if allowstring && fInfo.Type == nameType {
			if s, ok := dictVal.(String); ok {
				dictVal = Name(s)
			}
		}

		// finally, assign the value to the field
		switch {
		case isTextString:
			s, ok := dictVal.(String)
			if ok {
				fVal.SetString(s.AsTextString())
			} else if firstErr == nil {
				firstErr = fmt.Errorf("/%s: expected pdf.String but got %T",
					fInfo.Name, dictVal)
			}
		case fInfo.Type == versionType:
			var vString string
			switch x := dictVal.(type) {
			case Name:
				vString = string(x)
			case String:
				vString = x.AsTextString()
			case Real:
				vString = fmt.Sprintf("%.1f", x)
			default:
				if firstErr == nil {
					firstErr = fmt.Errorf("/%s: expected pdf.Name but got %T",
						fInfo.Name, dictVal)
				}
			}
			version, err := ParseVersion(vString)
			if err == nil {
				fVal.Set(reflect.ValueOf(version))
			} else if firstErr == nil {
				firstErr = fmt.Errorf("/%s: %s: %s", fInfo.Name, vString, err)
			}
		case fInfo.Type == timeType:
			s, ok := dictVal.(String)
			if ok {
				t, err := s.AsDate()
				if err == nil {
					fVal.Set(reflect.ValueOf(t))
				} else if firstErr == nil {
					firstErr = fmt.Errorf("/%s: %s: %s",
						fInfo.Name, s.AsTextString(), err)
				}
			} else if firstErr == nil {
				firstErr = fmt.Errorf("/%s: expected pdf.String but got %T",
					fInfo.Name, dictVal)
			}
		case fInfo.Type == languageType:
			s, ok := dictVal.(String)
			if ok {
				tagString := s.AsTextString()
				tag, err := language.Parse(tagString)
				if err == nil {
					fVal.Set(reflect.ValueOf(tag))
				} else if tagString != "" && firstErr == nil {
					firstErr = fmt.Errorf("/%s: %s: %s",
						fInfo.Name, tagString, err)
				}
			} else if firstErr == nil {
				firstErr = fmt.Errorf("/%s: expected pdf.String but got %T",
					fInfo.Name, dictVal)
			}
		case fInfo.Type.Kind() == reflect.Bool:
			fVal.SetBool(dictVal == Boolean(true))
		case reflect.TypeOf(dictVal).AssignableTo(fInfo.Type):
			fVal.Set(reflect.ValueOf(dictVal))
		default:
			if firstErr == nil {
				firstErr = fmt.Errorf("/%s: expected %T but got %T",
					fInfo.Name, fVal.Interface(), dictVal)
			}
		}
	}

	if extra >= 0 {
		extraDict := make(map[string]string)
		for keyName, valObj := range src {
			key := string(keyName)
			if seen[key] {
				continue
			}
			if val, ok := valObj.(String); ok && len(val) > 0 {
				extraDict[key] = val.AsTextString()
			} else if val, ok := valObj.(Name); ok && len(val) > 0 {
				extraDict[key] = string(val)
			}
		}
		v.Field(extra).Set(reflect.ValueOf(extraDict))
	}

	if firstErr != nil {
		return &MalformedFileError{Err: firstErr}
	}
	return nil
}

var (
	objectType    = reflect.TypeOf((*Object)(nil)).Elem()
	refType       = reflect.TypeOf(Reference(0))
	nameType      = reflect.TypeOf(Name(""))
	versionType   = reflect.TypeOf(V1_7)
	timeType      = reflect.TypeOf(time.Time{})
	languageType  = reflect.TypeOf(language.Tag{})
	referenceType = reflect.TypeOf(Reference(0))
)
