package pdf

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// AsStruct initialises a tagged struct using the data from a PDF dictionary.
// The argument s must be a pointer to a struct, or the function will panic.
// The Reader r is used to resolve references to indirect objects, where
// needed.
//
// TODO(voss): remove?
func (d Dict) AsStruct(s interface{}, r *Reader) error {
	v := reflect.Indirect(reflect.ValueOf(s))
	vt := v.Type()

	// To allow parsing malformed PDF files, we don't abort on error. Instead,
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
		dictVal := d[Name(fInfo.Name)]
		if fInfo.Type != objectType && fInfo.Type != refType {
			// follow references to indirect objects where needed
			obj, err := r.Get(dictVal)
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
		case fInfo.Type == timeType:
			s, ok := dictVal.(String)
			if ok {
				t, err := s.AsDate()
				if firstErr == nil && err != nil {
					firstErr = fmt.Errorf("/%s: %s: %s",
						fInfo.Name, s.AsTextString(), err)
					continue
				}
				fVal.Set(reflect.ValueOf(t))
			} else if firstErr == nil {
				firstErr = fmt.Errorf("/%s: expected pdf.String but got %T",
					fInfo.Name, dictVal)
			}
		case fInfo.Type.Kind() == reflect.Bool:
			fVal.SetBool(dictVal == Bool(true))
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
		for keyName, valObj := range d {
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

	return firstErr
}

// Struct creates a PDF Dict object, encoding the fields of a Go struct.
func Struct(s interface{}) Dict {
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

		if !fVal.CanSet() {
			// Ignore fields which AsStruct() cannot set
			continue
		}

		key := Name(fInfo.Name)
		switch {
		case optional && fVal.IsZero():
			continue
		case isTextString:
			res[key] = TextString(fVal.Interface().(string))
		case fInfo.Type == timeType:
			res[key] = Date(fVal.Interface().(time.Time))
		case fVal.Kind() == reflect.Bool:
			res[key] = Bool(fVal.Bool())
		default:
			res[key] = fVal.Interface().(Object)
		}
	}

	return res
}

var (
	objectType = reflect.TypeOf((*Object)(nil)).Elem()
	refType    = reflect.TypeOf(&Reference{})
	nameType   = reflect.TypeOf(Name(""))
	timeType   = reflect.TypeOf(time.Time{})
)
