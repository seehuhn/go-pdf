package operator

import "seehuhn.de/go/pdf/graphics"

type State struct {
	Param         graphics.Parameters
	In            graphics.StateBits
	Out           graphics.StateBits
	CurrentObject graphics.ObjectType
}
