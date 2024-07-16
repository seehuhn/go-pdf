package main

import (
	"fmt"

	"seehuhn.de/go/pdf/font/gofont"
)

func main() {
	for _, Fx := range gofont.All {
		F := Fx.New(nil)
		fmt.Println(F.FullName())
	}
}
