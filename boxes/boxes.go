package boxes

type stuff interface {
	Draw(*context)
}

type frame struct {
	Width, Height, Depth float64
}

func (b *frame) Draw(ctx *context) {
	ctx.Rectangle(ctx.xPos, ctx.yPos-b.Depth, b.Width, b.Depth+b.Height)
	ctx.Stroke()
}
