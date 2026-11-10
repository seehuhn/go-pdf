// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package traverse

import "fmt"

type KeyError struct {
	Key string
	Ctx string
}

func (e *KeyError) Error() string {
	return fmt.Sprintf("key %q not found in %s", e.Key, e.Ctx)
}
