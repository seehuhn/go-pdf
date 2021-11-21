package cff

func bias(nSubrs int) int {
	if nSubrs < 1240 {
		return 107
	} else if nSubrs < 33900 {
		return 1131
	} else {
		return 32768
	}
}

func (cff *Font) getSubr(biased int) []byte {
	idx := biased + bias(len(cff.subrs))
	return cff.subrs[idx]
}

func (cff *Font) getGSubr(biased int) []byte {
	idx := biased + bias(len(cff.gsubrs))
	return cff.gsubrs[idx]
}
