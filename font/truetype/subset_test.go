package truetype

import (
	"testing"
)

func TestGetSubsetTag(t *testing.T) {
	tag := getSubsetTag(nil, 0)
	if tag != "AAAAAA" {
		t.Error("wrong tag " + tag)
	}
}
