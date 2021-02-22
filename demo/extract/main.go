package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"seehuhn.de/go/pdf"
)

func main() {
	passwd := flag.String("p", "", "PDF password")
	decode := flag.Bool("d", false, "decode streams")
	flag.Parse()

	var tryPasswd func() string
	if *passwd != "" {
		tryPasswd = func() string {
			var res string
			if passwd != nil {
				res = *passwd
				passwd = nil
			}
			return res
		}
	}

	args := flag.Args()

	fd, err := os.Open(args[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fi, err := fd.Stat()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	r, err := pdf.NewReader(fd, fi.Size(), tryPasswd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer r.Close()

	var obj pdf.Object
	switch {
	case len(args) < 2 || args[1] == "catalog":
		obj, err = r.Catalog()
	case args[1] == "info":
		obj, err = r.Info()
	default:
		var number int
		number, err = strconv.Atoi(args[1])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var generation uint16
		if len(args) > 2 {
			tmp, err := strconv.ParseUint(args[2], 10, 16)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			generation = uint16(tmp)
		}

		ref := &pdf.Reference{
			Number:     number,
			Generation: uint16(generation),
		}
		obj, err = r.Get(ref)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if stm, ok := obj.(*pdf.Stream); ok && *decode {
		err = stm.Dict.PDF(os.Stdout)
		fmt.Println()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("decoded stream")
		r, err := stm.Decode()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		_, err = io.Copy(os.Stdout, r)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("\nendstream")
		return
	}

	if obj == nil {
		_, err = fmt.Println("null")
	} else {
		err = obj.PDF(os.Stdout)
	}
	fmt.Println()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
