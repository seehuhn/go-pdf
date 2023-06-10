package main

import "fmt"

func main() {
	var lines []string
	lines = append(lines, "<00> <7f> 0\n")
	for r := rune(0x80); r <= 0x10_ffff; r += 64 {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}

		s1 := string([]rune{r})
		s2 := string([]rune{r + 63})
		lines = append(lines, fmt.Sprintf("<%02x> <%02x> %d\n",
			[]byte(s1), []byte(s2), r))
	}
	for len(lines) > 0 {
		k := len(lines)
		if k > 100 {
			k = 100
		}
		fmt.Printf("%d begincidrange\n", k)
		for _, line := range lines[:k] {
			fmt.Print(line)
		}
		fmt.Println("endcidrange")
		fmt.Println()
		lines = lines[k:]
	}
}
