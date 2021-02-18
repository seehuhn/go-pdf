package boxes

import (
	"bufio"
	"fmt"
)

type context struct {
	xPos, yPos float64

	R, G, B float64
	W       *bufio.Writer
}

func (ctx *context) Error() error {
	return ctx.W.Flush()
}

func (ctx *context) Rectangle(x, y, w, h float64) {
	fmt.Fprintf(ctx.W, "%f %f %f %f re\n", x, y, w, h)
}

func (ctx *context) Stroke() {
	fmt.Fprint(ctx.W, "s\n")
}
