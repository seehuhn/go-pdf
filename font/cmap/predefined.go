package cmap

import (
	"embed"

	"seehuhn.de/go/pdf"
)

// go:embed predefined/*.gz
var predefined embed.FS

func Load(name pdf.Name) ([]byte, error) {
	data, err := predefined.ReadFile("predefined/" + string(name) + ".gz")
	if err != nil {
		return nil, err
	}
	return data, nil
}
