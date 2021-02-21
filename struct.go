package pdf

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

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
			if fVal.CanInterface() {
				res[key] = fVal.Interface().(Object)
			}
		}
	}

	return res
}

// AsStruct initialises a tagged struct using the data from a PDF dictionary.
// The argument s must be a pointer to a struct, or the function will panic.
// The function get(), if non-nil, is used to resolve references to indirect
// objects, where needed; the Reader.Get() method can be used for this
// argument.
func (d Dict) AsStruct(s interface{}, get func(Object) (Object, error)) error {
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
		if fInfo.Type != objectType && fInfo.Type != refType && get != nil {
			// follow references to indirect objects where needed
			obj, err := get(dictVal)
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

// Catalog represents a PDF Document Catalog.
//
// This can be used together with the `Struct()` function to construct
// the argument for `Writer.SetDocument()`.
//
// The only required field in this structure is `Pages`, which specifies
// the root of the page tree.
//
// The Document Catalog is documented in section 7.7.2 of PDF 32000-1:2008.
type Catalog struct {
	_                 struct{} `pdf:"Type=Catalog"`
	Version           Name     `pdf:"optional,allowstring"`
	Extensions        Object   `pdf:"optional"`
	Pages             *Reference
	PageLabels        Object     `pdf:"optional"`
	Names             Object     `pdf:"optional"`
	Dests             Object     `pdf:"optional"`
	ViewerPreferences Object     `pdf:"optional"`
	PageLayout        Name       `pdf:"optional"`
	PageMode          Name       `pdf:"optional"`
	Outlines          *Reference `pdf:"optional"`
	Threads           *Reference `pdf:"optional"`
	OpenAction        Object     `pdf:"optional"`
	AA                Object     `pdf:"optional"`
	URI               Object     `pdf:"optional"`
	AcroForm          Object     `pdf:"optional"`
	MetaData          *Reference `pdf:"optional"`
	StructTreeRoot    Object     `pdf:"optional"`
	MarkInfo          Object     `pdf:"optional"`
	Lang              string     `pdf:"text string,optional"`
	SpiderInfo        Object     `pdf:"optional"`
	OutputIntents     Object     `pdf:"optional"`
	PieceInfo         Object     `pdf:"optional"`
	OCProperties      Object     `pdf:"optional"`
	Perms             Object     `pdf:"optional"`
	Legal             Object     `pdf:"optional"`
	Requirements      Object     `pdf:"optional"`
	Collection        Object     `pdf:"optional"`
	NeedsRendering    bool       `pdf:"optional"`
}

// Info represents a PDF Document Information Dictionary.
//
// This can be used together with the `Struct() function to construct
// the argument for `Writer.SetInfo()`.
//
// All fields in this structure are optional.
//
// The Document Information Dictionary is documented in section
// 14.3.3 of PDF 32000-1:2008.
type Info struct {
	Title        string    `pdf:"text string,optional"`
	Author       string    `pdf:"text string,optional"`
	Subject      string    `pdf:"text string,optional"`
	Keywords     string    `pdf:"text string,optional"`
	Creator      string    `pdf:"text string,optional"`
	Producer     string    `pdf:"text string,optional"`
	CreationDate time.Time `pdf:"optional"`
	ModDate      time.Time `pdf:"optional"`
	Trapped      Name      `pdf:"optional,allowstring"`

	Custom map[string]string `pdf:"extra"`
}

var (
	objectType = reflect.TypeOf((*Object)(nil)).Elem()
	refType    = reflect.TypeOf(&Reference{})
	nameType   = reflect.TypeOf(Name(""))
	timeType   = reflect.TypeOf(time.Time{})
)
