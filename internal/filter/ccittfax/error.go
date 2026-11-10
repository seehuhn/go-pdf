// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package ccittfax

import "errors"

var (
	errTooManyRows    = errors.New("ccittfax: too many rows")
	errInvalidRunCode = errors.New("ccittfax: invalid run-length code")
	errInvalid2DCode  = errors.New("ccittfax: invalid 2D mode code")
	errPrematureEOL   = errors.New("ccittfax: premature end of line")
	errNoProgress     = errors.New("ccittfax: no forward progress in 2D decode")
	errExtension      = errors.New("ccittfax: unsupported extension code")
)
