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

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/exp/maps"
	"golang.org/x/term"
	"seehuhn.de/go/pdf"
)

func main() {
	passwdArg := flag.String("p", "", "PDF password")
	flag.Parse()

	tryPasswd := func(_ []byte, try int) string {
		if *passwdArg != "" && try == 0 {
			return *passwdArg
		}
		fmt.Print("password: ")
		passwd, err := term.ReadPassword(syscall.Stdin)
		fmt.Println("***")
		check(err)
		return string(passwd)
	}

	args := flag.Args()

	fd, err := os.Open(args[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	opt := &pdf.ReaderOptions{
		ReadPassword: tryPasswd,
	}
	r, err := pdf.NewReader(fd, opt)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer r.Close()

	e := &explainer{
		r:   r,
		buf: &bytes.Buffer{},
	}
	obj, err := e.locate(flag.Args()[1:]...)
	check(err)
	err = e.show(obj)
	check(err)
}

type explainer struct {
	r   *pdf.Reader
	buf *bytes.Buffer
}

func (e *explainer) locate(desc ...string) (pdf.Object, error) {
	var obj pdf.Object = pdf.AsDict(e.r.Catalog)

	debug := false

	if debug {
		msg, err := e.explainSingleLine(obj)
		if err != nil {
			return nil, err
		}
		fmt.Println(".", msg)
	}

	for _, key := range desc {
		keyInt, err := strconv.ParseInt(key, 10, 64)
		isInt := err == nil

		switch {
		case key == "":
			return nil, errors.New("empty selector")
		case key == "@info":
			x, err := e.r.GetInfo()
			if err != nil {
				return nil, err
			}
			obj = pdf.AsDict(x)
		case key[0] == '@':
			ff := strings.Split(key[1:], ".")
			if len(ff) > 2 {
				return nil, errors.New("invalid selector " + key)
			}
			var number uint64
			number, err := strconv.ParseUint(ff[0], 10, 32)
			if err != nil {
				return nil, err
			}

			var generation uint16
			if len(ff) > 1 {
				tmp, err := strconv.ParseUint(ff[1], 10, 16)
				if err != nil {
					return nil, err
				}
				generation = uint16(tmp)
			}

			ref := pdf.NewReference(uint32(number), generation)
			obj, err = e.r.Resolve(ref)
			if err != nil {
				return nil, err
			}
		default:
			switch x := obj.(type) {
			case pdf.Dict:
				val, ok := x[pdf.Name(key)]
				if !ok {
					return nil, fmt.Errorf("key %q not present in dict", key)
				}
				obj, err = e.r.Resolve(val)
				if err != nil {
					return nil, err
				}
			case pdf.Array:
				if !isInt {
					return nil, fmt.Errorf("key %q not valid for type Array", key)
				}
				idx := keyInt
				if idx < 0 {
					idx += int64(len(x))
				}
				if idx < 0 || idx >= int64(len(x)) {
					return nil, fmt.Errorf("index %d out of range 0...%d", keyInt, len(x)-1)
				}
				obj, err = e.r.Resolve(x[idx])
				if err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("key %q not valid for type %T", key, obj)
			}
		}
		if debug {
			msg, err := e.explainSingleLine(obj)
			if err != nil {
				return nil, err
			}
			fmt.Println(key, msg)
		}
	}
	return obj, nil
}

func (e *explainer) explainShort(obj pdf.Object) (string, error) {
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
		e.buf.Reset()
		err := obj.PDF(e.buf)
		if err != nil {
			return "", err
		}
		return e.buf.String(), nil
	}
}

func (e *explainer) explainSingleLine(obj pdf.Object) (string, error) {
	if obj == nil {
		return "null", nil
	}
	switch obj := obj.(type) {
	case *pdf.Stream:
		var parts []string
		tp, err := e.r.GetName(obj.Dict["Type"])
		if err == nil {
			parts = append(parts, string(tp)+" stream")
		} else {
			parts = append(parts, "stream")
		}
		length, err := e.r.GetInt(obj.Dict["Length"])
		if err == nil {
			parts = append(parts, fmt.Sprintf("%d bytes", length))
		}
		ff, ok := obj.Dict["Filter"]
		if ok {
			if name, err := e.r.GetName(ff); err == nil {
				parts = append(parts, string(name))
			} else if arr, err := e.r.GetArray(ff); err == nil {
				for _, elem := range arr {
					if name, err := e.r.GetName(elem); err == nil {
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
				e.buf.Reset()
				err := key.PDF(e.buf)
				if err != nil {
					return "", err
				}
				parts = append(parts, e.buf.String())
				valString, err := e.explainShort(obj[key])
				if err != nil {
					return "", err
				}
				parts = append(parts, valString)
			}
			return "<<" + strings.Join(parts, " ") + ">>", nil
		}
		tp, err := e.r.GetName(obj["Type"])
		if err == nil {
			parts = append(parts, string(tp)+" dict")
		} else {
			parts = append(parts, "dict")
		}
		parts = append(parts, fmt.Sprintf("%d entries", len(obj)))
		return "<" + strings.Join(parts, ", ") + ">", nil
	case pdf.Array:
		if len(obj) <= 8 {
			var parts []string
			for _, elem := range obj {
				msg, err := e.explainShort(elem)
				if err != nil {
					return "", err
				}
				parts = append(parts, msg)
			}
			return "[" + strings.Join(parts, " ") + "]", nil
		}
		return fmt.Sprintf("<array, %d elements>", len(obj)), nil
	default:
		e.buf.Reset()
		err := obj.PDF(e.buf)
		if err != nil {
			return "", err
		}
		return e.buf.String(), nil
	}
}

func (e *explainer) show(obj pdf.Object) error {
	if obj == nil {
		fmt.Println("null")
		return nil
	}

	switch obj := obj.(type) {
	case *pdf.Stream:
		e.show(obj.Dict)
		fmt.Println()

		stmData, err := e.r.DecodeStream(obj, 0)
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
		fmt.Println()
	case pdf.Dict:
		keys := dictKeys(obj)
		fmt.Println("<<")
		for _, key := range keys {
			err := key.PDF(os.Stdout)
			if err != nil {
				return err
			}
			valString, err := e.explainSingleLine(obj[key])
			if err != nil {
				return err
			}
			fmt.Println(" " + valString)
		}
		fmt.Println(">>")
	case pdf.Array:
		fmt.Println("[")
		for i, elem := range obj {
			msg, err := e.explainSingleLine(elem)
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
		err := obj.PDF(os.Stdout)
		if err != nil {
			return err
		}
		fmt.Println()
	}
	return nil
}

func dictKeys(obj pdf.Dict) []pdf.Name {
	keys := maps.Keys(obj)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "Type" && keys[j] != "Type" {
			return true
		}
		if keys[j] == "Type" {
			return false
		}
		return keys[i] < keys[j]
	})
	return keys
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// MostlyBinary returns true if the contents of buf should not be
// printed to the screen without quoting.
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
