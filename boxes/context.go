package boxes

import (
	"fmt"
	"io"
)

type context struct {
	xPos, yPos float64

	R, G, B float64
	W       io.Writer
}

func (ctx *context) Rectangle(x, y, w, h float64) error {
	_, err := fmt.Fprintf(ctx.W, "%f %f %f %f re\n", x, y, w, h)
	return err
}

func (ctx *context) Stroke() error {
	_, err := fmt.Fprint(ctx.W, "s\n")
	return err
}
