package color

// The following types implement the ColorSpace interface:
var (
	_ Space = SpaceDeviceGray{}
	_ Space = SpaceDeviceRGB{}
	_ Space = SpaceDeviceCMYK{}
	_ Space = (*SpaceCalGray)(nil)
	_ Space = (*SpaceCalRGB)(nil)
	_ Space = (*SpaceLab)(nil)
	// TODO(voss): ICCBased
	_ Space = spacePatternColored{}
	_ Space = spacePatternUncolored{}
	_ Space = (*SpaceIndexed)(nil)
	// TODO(voss): Separation colour spaces
	// TODO(voss): DeviceN colour spaces
)

// The following types implement the Color interface.
var (
	_ Color = colorDeviceGray(0)
	_ Color = colorDeviceRGB{0, 0, 0}
	_ Color = colorDeviceCMYK{0, 0, 0, 1}
	_ Color = colorCalGray{}
	_ Color = colorCalRGB{}
	_ Color = colorLab{}
	// TODO(voss): ICCBased
	_ Color = PatternColored{}
	_ Color = colorPatternUncolored{}
	_ Color = colorIndexed{}
	// TODO(voss): Separation colour spaces
	// TODO(voss): DeviceN colour spaces
)
