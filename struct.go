package pdf

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Info represents the information from a PDF /Info dictionary.
type Info struct {
	Title        string    `pdf:"text string,optional,V1.1"`
	Author       string    `pdf:"text string,optional"`
	Subject      string    `pdf:"text string,optional,V1.1"`
	Keywords     string    `pdf:"text string,optional,V1.1"`
	Creator      string    `pdf:"text string,optional"`
	Producer     string    `pdf:"text string,optional"`
	CreationDate time.Time `pdf:"optional"`
	ModDate      time.Time `pdf:"optional,V1.1"`
	Trapped      Name      `pdf:"optional,allowstring,V1.3"`
}

// DictToStruct initialises a tagged struct using the data from a PDF
// dictionary.  s Must be a pointer to a struct.
func (r *Reader) DictToStruct(s interface{}, d Dict, errPos int64) error {
	v := reflect.Indirect(reflect.ValueOf(s))
	vt := v.Type()
	n := vt.NumField()
	var err error
	for i := 0; i < n; i++ {
		f := v.Field(i)
		ft := vt.Field(i)

		optional := false
		isTextString := false
		allowstring := false
		tags := strings.Split(ft.Tag.Get("pdf"), ",")
		for _, t := range tags {
			switch t {
			case "optional":
				optional = true
			case "text string":
				isTextString = true
			case "allowstring":
				allowstring = true
			}
		}

		dictVal, present := d[Name(ft.Name)]
		if !present {
			f.Set(reflect.Zero(f.Type()))
			if !optional {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("required Dict entry /%s not found", ft.Name),
				}
			}
			continue
		} else {
			dictVal, err = r.Get(dictVal)
			if err != nil {
				continue
			}
		}

		if s, ok := dictVal.(String); allowstring && isName(f) && ok {
			dictVal = Name(s)
		}

		switch {
		case isTextString:
			s, ok := dictVal.(String)
			if !ok {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("/%s: expected pdf.String but got %T",
						ft.Name, dictVal),
				}
				continue
			}
			f.SetString(s.AsTextString())
		case isTime(f):
			s, ok := dictVal.(String)
			if !ok {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("/%s: expected pdf.String but got %T",
						ft.Name, dictVal),
				}
				continue
			}
			var t time.Time
			t, err = s.AsDateString()
			if err != nil {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("/%s: %s: %s",
						ft.Name, s.AsTextString(), err),
				}
				continue
			}
			f.Set(reflect.ValueOf(t))
		default:
			if reflect.TypeOf(dictVal).AssignableTo(f.Type()) {
				f.Set(reflect.ValueOf(dictVal))
			} else {
				err = &MalformedFileError{
					Pos: errPos,
					Err: fmt.Errorf("/%s: expected %T but got %T",
						ft.Name, f.Interface(), dictVal),
				}
			}
		}
	}
	return err
}

func isTime(obj reflect.Value) bool {
	_, ok := obj.Interface().(time.Time)
	return ok
}

func isName(obj reflect.Value) bool {
	_, ok := obj.Interface().(Name)
	return ok
}
