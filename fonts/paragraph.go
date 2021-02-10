package fonts

// state represents the current graphics state.
type state struct {
	BaseLineSkip float64
}

type frag []struct {
	Characters []byte
	Kern       float64
}

// example: "We {\it will} rock you!"
//     -> [Text("We "), push(italic), Text("will"), pop(), Text(" rock you")]
//     -> PDF: (W)

// Glyphs need the following information:
// - font and size
// - colour
// - super-/subscript information
// - maybe underlining etc?
