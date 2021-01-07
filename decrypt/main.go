package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"

	"seehuhn.de/go/pdf"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	// fd, err := os.Open("../encrypted.pdf")
	fd, err := os.Open("../PDF32000_2008.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	fi, err := fd.Stat()
	if err != nil {
		log.Fatal(err)
	}

	r, err := pdf.NewReader(fd, fi.Size(), nil)
	if err != nil {
		log.Fatal(err)
	}

	enc, err := r.GetDict(r.Trailer["Encrypt"])
	if err != nil {
		log.Fatal(err)
	}

	for key, val := range enc {
		fmt.Println(string(key)+":", val)
	}

	oStr := enc["O"].(pdf.String)
	O := []byte(oStr)

	length := enc["Length"].(pdf.Integer)

	R := enc["R"].(pdf.Integer)
	if R < 3 {
		panic("not implemented")
	}

	userPwd := padPwd("")
	ownerPwd := findOwnerPwd(O, userPwd, int(length/8))
	if ownerPwd == nil {
		fmt.Println("not found")
	} else {
		fmt.Printf("found: %q\n", unpadPwd(ownerPwd))
	}
}

func findOwnerPwd(O, userPwd []byte, keyBytes int) []byte {
	distrib := make(chan []byte, 8)
	gather := make(chan []byte)
	for i := 0; i < runtime.NumCPU(); i++ {
		go worker(O, userPwd, keyBytes, distrib, gather)
	}

	fmt.Println("stage 0: empty passwd")
	distrib <- passwdPad // try the empty password first, just in case

	fmt.Println("stage 1: dictionary words")
	words, err := os.Open("/usr/share/dict/words")
	if err != nil {
		panic(err)
	}
	defer words.Close()
	scanner := bufio.NewScanner(words)
	for scanner.Scan() {
		line := []byte(scanner.Text())
		length := len(line)

		candidate := make([]byte, 32)
		copy(candidate, line)
		copy(candidate[length:], passwdPad)

		select {
		case found := <-gather:
			close(distrib)
			return found
		case distrib <- candidate:
			// pass
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}

	for length := 1; length < 32; length++ {
		fmt.Printf("stage %d: length %d\n", length+1, length)
		buf := bytes.Repeat([]byte{' '}, length)

	candLoop:
		for {
			candidate := make([]byte, 32)
			copy(candidate, buf)
			copy(candidate[length:], passwdPad)

			select {
			case found := <-gather:
				close(distrib)
				return found
			case distrib <- candidate:
				// pass
			}

			pos := 0
			for {
				buf[pos]++
				if buf[pos] <= 127 {
					break
				}
				buf[pos] = ' '
				pos++
				if pos >= length {
					break candLoop
				}
			}
		}
	}

	return nil
}

func worker(O, userPwd []byte, keyBytes int, in <-chan []byte, found chan<- []byte) {
	if keyBytes < 5 || keyBytes > 16 {
		panic("invalid /Length")
	}
	if len(O) != 32 {
		panic("invalid /O")
	}
	if len(userPwd) != 32 {
		panic("invalid user password")
	}

	h := md5.New()
	sum := make([]byte, 16)
	res := make([]byte, 32)
	tmpKey := make([]byte, keyBytes)
	c := &ArcFour{}

	for candidate := range in {
		h.Reset()
		h.Write(candidate)
		sum = h.Sum(sum[:0])
		for i := 0; i < 50; i++ {
			h.Reset()
			h.Write(sum)
			sum = h.Sum(sum[:0])
		}
		rc4key := sum[:keyBytes]

		copy(res, userPwd)
		c.Reset(rc4key)
		c.XORKeyStream(res, res)
		for i := byte(1); i <= 19; i++ {
			for j := range tmpKey {
				tmpKey[j] = rc4key[j] ^ i
			}
			c.Reset(tmpKey)
			c.XORKeyStream(res, res)
		}

		if bytes.Equal(res, O) {
			found <- candidate
		}
	}
}

func padPwd(passwd string) []byte {
	pw := make([]byte, 32)
	i := 0
	for _, r := range passwd {
		pw[i] = byte(r)
		i++

		if i >= 32 {
			break
		}
	}

	copy(pw[i:], passwdPad)

	return pw
}

func unpadPwd(padded []byte) string {
	var length int
	for ; length < 32; length++ {
		if bytes.HasPrefix(passwdPad, padded[length:]) {
			break
		}
	}
	pw := make([]rune, length)
	for i := 0; i < length; i++ {
		pw[i] = rune(padded[i])
	}
	return string(pw)
}

var passwdPad = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}
