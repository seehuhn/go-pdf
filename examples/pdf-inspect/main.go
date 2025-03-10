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
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/exp/maps"
	"golang.org/x/term"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/examples/pdf-inspect/meta"
	"seehuhn.de/go/pdf/pagetree"
)

var (
	debug        = flag.Bool("d", false, "debug mode")
	passwdArg    = flag.String("p", "", "PDF password")
	showMetadata = flag.Bool("show-metadata", false, "show PDF metadata")
)

func main() {
	flag.Parse()
	flag.CommandLine.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s [options] <file.pdf> <path>...\n",
			filepath.Base(os.Args[0]))
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "The given path describes an object in the PDF file,")
		fmt.Fprintln(flag.CommandLine.Output(), "starting from the document catalog.")
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "Options:")
		flag.PrintDefaults()
	}
	args := flag.Args()

	if len(args) == 0 {
		flag.CommandLine.Usage()
		os.Exit(1)
	}

	err := printObject(args...)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func printObject(args ...string) error {
	tryPasswd := func(_ []byte, try int) string {
		if *passwdArg != "" && try == 0 {
			return *passwdArg
		}
		fmt.Print("password: ")
		passwd, _ := term.ReadPassword(syscall.Stdin)
		fmt.Println("***")
		return string(passwd)
	}

	fd, err := os.Open(args[0])
	if err != nil {
		return err
	}
	opt := &pdf.ReaderOptions{
		ReadPassword:  tryPasswd,
		ErrorHandling: pdf.ErrorHandlingReport,
	}
	r, err := pdf.NewReader(fd, opt)
	if err != nil {
		return err
	}
	for _, err := range r.Errors {
		fmt.Fprintln(os.Stderr, err)
	}
	defer r.Close()

	if *showMetadata {
		err = meta.ShowMetadata(r)
		return err
	}

	e := &explainer{
		r:   r,
		buf: &bytes.Buffer{},
	}

	action := "@show"

	path := flag.Args()[1:]
	err = e.abs("catalog")
	if err != nil {
		return err
	}
	for i, key := range path {
		if i == len(path)-1 && strings.HasPrefix(key, "@") {
			action = key
			break
		}
		if i == 0 {
			err = e.abs(key)
			if err == nil {
				continue
			}
		}
		err = e.rel(key)
		if err != nil {
			return err
		}
	}

	switch action {
	case "@show":
		fmt.Println(strings.Join(e.loc, ".") + ":")
		err = e.show(e.obj)
		if err != nil {
			return err
		}

	case "@raw":
		stm, ok := e.obj.(*pdf.Stream)
		if !ok {
			return fmt.Errorf("expected a PDF stream but got %T", e.obj)
		}
		_, err = io.Copy(os.Stdout, stm.R)
		return err

	case "@stream":
		stm, ok := e.obj.(*pdf.Stream)
		if !ok {
			return fmt.Errorf("expected a PDF stream but got %T", e.obj)
		}
		stmData, err := pdf.DecodeStream(r, stm, 0)
		if err != nil {
			return err
		}
		_, err = io.Copy(os.Stdout, stmData)
		return err

	case "@contents":
		pageDict, err := pdf.GetDictTyped(e.r, e.obj, "Page")
		if err != nil {
			return err
		}
		contents, err := pdf.Resolve(r, pageDict["Contents"])
		if err != nil {
			return err
		}
		switch contents := contents.(type) {
		case *pdf.Stream:
			stmData, err := pdf.DecodeStream(r, contents, 0)
			if err != nil {
				return err
			}
			// TODO(voss): sanitise line endings
			_, err = io.Copy(os.Stdout, stmData)
			if err != nil {
				return err
			}
		case pdf.Array:
			for _, elem := range contents {
				stm, err := pdf.GetStream(r, elem)
				if err != nil {
					return err
				}
				stmData, err := pdf.DecodeStream(r, stm, 0)
				if err != nil {
					return err
				}
				// TODO(voss): sanitise line endings
				_, err = io.Copy(os.Stdout, stmData)
				if err != nil {
					return err
				}
				fmt.Println()
			}
		default:
			return fmt.Errorf("unexpected type %T for page contents", contents)
		}
		return nil
	}

	return nil
}

type explainer struct {
	r   *pdf.Reader
	buf *bytes.Buffer

	obj pdf.Object
	loc []string
}

func (e *explainer) abs(key string) error {
	var obj pdf.Object
	switch {
	case key == "" || key == "catalog":
		obj = pdf.AsDict(e.r.GetMeta().Catalog)
		key = "catalog"
	case key == "info":
		obj = pdf.AsDict(e.r.GetMeta().Info)
	case key == "trailer":
		obj = e.r.GetMeta().Trailer
	case objNumberRegexp.MatchString(key):
		m := objNumberRegexp.FindStringSubmatch(key)
		number, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			return err
		}
		var generation uint16
		if m[2] != "" {
			tmp, err := strconv.ParseUint(m[2], 10, 16)
			if err != nil {
				return err
			}
			generation = uint16(tmp)
		}
		ref := pdf.NewReference(uint32(number), generation)
		obj, err = pdf.Resolve(e.r, ref)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	e.obj = obj
	e.loc = []string{key}

	if *debug {
		msg, err := e.explainSingleLine(obj)
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", strings.Join(e.loc, "."), msg)
	}

	return nil
}

func (e *explainer) rel(key string) error {
	var err error
	obj := e.obj
	loc := strings.Join(e.loc, ".")

	switch x := obj.(type) {
	case pdf.Dict:
		forceKey := false
		if strings.HasPrefix(key, "/") {
			key = key[1:]
			forceKey = true
		}

		if loc == "catalog.Pages" && intRegexp.MatchString(key) && !forceKey {
			pageNo, err := strconv.ParseInt(key, 10, 0)
			if err != nil {
				return err
			}
			_, obj, err = pagetree.GetPage(e.r, int(pageNo)-1)
			if err != nil {
				return err
			}
		} else {
			var ok bool
			obj, ok = x[pdf.Name(key)]
			if !ok {
				return fmt.Errorf("%s: key %q not found", loc, key)
			}
		}
	case pdf.Array:
		idx, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return err
		}
		if idx < 0 && idx+int64(len(x)) >= 0 {
			idx += int64(len(x))
		} else if idx < 0 || idx >= int64(len(x)) {
			return fmt.Errorf("%s: index %d out of range 0...%d", loc, idx, len(x)-1)
		}
		obj = x[idx]
	case *pdf.Stream:
		if key == "dict" {
			obj = x.Dict
		} else {
			key = strings.TrimPrefix(key, "/")

			var ok bool
			obj, ok = x.Dict[pdf.Name(key)]
			if !ok {
				return fmt.Errorf("%s: key %q not found", loc, key)
			}
		}
	}

	obj, err = pdf.Resolve(e.r, obj)
	if err != nil {
		return err
	}
	e.obj = obj
	e.loc = append(e.loc, key)

	if *debug {
		msg, err := e.explainSingleLine(obj)
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", strings.Join(e.loc, "."), msg)
	}

	return nil
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
		err := pdf.Format(e.buf, 0, obj)
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
		tp, err := pdf.GetName(e.r, obj.Dict["Type"])
		if err == nil {
			parts = append(parts, string(tp)+" stream")
		} else {
			parts = append(parts, "stream")
		}
		length, err := pdf.GetInteger(e.r, obj.Dict["Length"])
		if err == nil {
			parts = append(parts, fmt.Sprintf("%d bytes", length))
		}
		ff, ok := obj.Dict["Filter"]
		if ok {
			if name, err := pdf.GetName(e.r, ff); err == nil {
				parts = append(parts, string(name))
			} else if arr, err := pdf.GetArray(e.r, ff); err == nil {
				for _, elem := range arr {
					if name, err := pdf.GetName(e.r, elem); err == nil {
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
				err := pdf.Format(e.buf, 0, key)
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
		tp, err := pdf.GetName(e.r, obj["Type"])
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
		err := pdf.Format(e.buf, pdf.OptPretty, obj)
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
		err := e.show(obj.Dict)
		if err != nil {
			return err
		}
		fmt.Println()

		stmData, err := pdf.DecodeStream(e.r, obj, 0)
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
		// TODO(voss): fix line endings (see type1-deobfuscate/main.go)
		_, err = io.Copy(os.Stdout, stmData)
		if err != nil {
			return err
		}
	case pdf.Dict:
		keys := dictKeys(obj)
		fmt.Println("<<")
		for _, key := range keys {
			err := pdf.Format(os.Stdout, 0, key)
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
		err := pdf.Format(os.Stdout, pdf.OptPretty, obj)
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

var (
	intRegexp       = regexp.MustCompile(`^(\d+)$`)
	objNumberRegexp = regexp.MustCompile(`^(\d+)(?:\.(\d+))?$`)
)
