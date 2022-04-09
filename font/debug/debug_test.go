package debug

import (
	"testing"
)

func TestDebugFont(t *testing.T) {
	info, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = info
}
