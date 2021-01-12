package pdf

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Info represents the information from a PDF /Info dictionary.
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

// Catalog represents the information from a PDF /Root dictionary.
type Catalog struct {
	_                 struct{} `pdf:"Type=Catalog"`
	Version           Name     `pdf:"optional"`
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

// FillStruct initialises a tagged struct using the data from a PDF
// dictionary.  s Must be a pointer to a struct.
func (r *Reader) FillStruct(s interface{}, d Dict, errPos int64) error {
	v := reflect.Indirect(reflect.ValueOf(s))
	vt := v.Type()
	n := vt.NumField()

	// To allow parsing malformed PDF files, we don't abort on error. Instead,
	// we fill all struct fields we can and then return the last error
	// encountered.
	var err error

	seen := map[string]bool{}
	extra := -1
fieldLoop:
	for i := 0; i < n; i++ {
		fVal := v.Field(i)
		fInfo := vt.Field(i)
		seen[fInfo.Name] = true

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

		if fInfo.PkgPath != "" {
			continue
		}

		dictVal, present := d[Name(fInfo.Name)]
		if !present {
			fVal.Set(reflect.Zero(fVal.Type()))
			if !optional {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("required Dict entry /%s not found", fInfo.Name),
				}
			}
			continue
		} else {
			switch fVal.Interface().(type) {
			case Object, *Reference:
				// pass
			default:
				dictVal, err = r.Get(dictVal)
				if err != nil {
					continue
				}
			}
		}

		if s, ok := dictVal.(String); allowstring && isName(fVal) && ok {
			dictVal = Name(s)
		}

		switch {
		case isTextString:
			s, ok := dictVal.(String)
			if !ok {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("/%s: expected pdf.String but got %T",
						fInfo.Name, dictVal),
				}
				continue
			}
			fVal.SetString(s.AsTextString())
		case isTime(fVal):
			s, ok := dictVal.(String)
			if !ok {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("/%s: expected pdf.String but got %T",
						fInfo.Name, dictVal),
				}
				continue
			}
			var t time.Time
			t, err = s.AsDateString()
			if err != nil {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("/%s: %s: %s",
						fInfo.Name, s.AsTextString(), err),
				}
				continue
			}
			fVal.Set(reflect.ValueOf(t))
		case fVal.Kind() == reflect.Bool:
			fVal.SetBool(dictVal == Bool(true))
		default:
			if reflect.TypeOf(dictVal).AssignableTo(fVal.Type()) {
				fVal.Set(reflect.ValueOf(dictVal))
			} else {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("/%s: expected %T but got %T",
						fInfo.Name, fVal.Interface(), dictVal),
				}
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

	return err
}

func makeDict(s interface{}) Dict {
	v := reflect.Indirect(reflect.ValueOf(s))
	if v.Kind() != reflect.Struct {
		return nil
	}
	vt := v.Type()
	n := vt.NumField()

	res := make(Dict)

fieldLoop:
	for i := 0; i < n; i++ {
		fVal := v.Field(i)
		fInfo := vt.Field(i)

		optional := false
		isTextString := false
		for _, t := range strings.Split(fInfo.Tag.Get("pdf"), ",") {
			switch t {
			case "optional":
				optional = true
			case "text string":
				isTextString = true
			case "extra":
				for key, val := range fVal.Interface().(map[string]string) {
					res[Name(key)] = TextString(val)
				}
				continue fieldLoop
			}
		}

		if fInfo.PkgPath != "" {
			continue
		}

		key := Name(fInfo.Name)
		switch {
		case optional && fVal.IsZero():
			continue
		case isTextString:
			res[key] = TextString(fVal.Interface().(string))
		case isTime(fVal):
			res[key] = DateString(fVal.Interface().(time.Time))
		case fVal.Kind() == reflect.Bool:
			res[key] = Bool(fVal.Bool())
		default:
			res[key] = fVal.Interface().(Object)
		}
	}

	return res
}

func isTime(obj reflect.Value) bool {
	_, ok := obj.Interface().(time.Time)
	return ok
}

func isName(obj reflect.Value) bool {
	_, ok := obj.Interface().(Name)
	return ok
}
