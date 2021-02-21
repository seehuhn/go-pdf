package boxes

import "fmt"

type stuff interface {
	Extent()
}

type frame struct {
	Width, Height, Depth float64
}

func (b *frame) Draw(ctx *context) {
	ctx.Rectangle(ctx.xPos, ctx.yPos-b.Depth, b.Width, b.Depth+b.Height)
	ctx.Stroke()
}

type vBox struct {
	Width  float64
	Height float64
	Depth  float64

	Contents []stuff
}

func (b *vBox) Draw(ctx *context) {
	fmt.Println("hello")
}
