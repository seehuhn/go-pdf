// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package mock

import "seehuhn.de/go/pdf"

// Getter is a [pdf.Getter] which returns no objects.
var Getter pdf.Getter = getter{}

type getter struct{}

func (r getter) GetMeta() *pdf.MetaInfo {
	m := &pdf.MetaInfo{
		Version: pdf.V2_0,
	}
	return m
}

func (r getter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	return nil, nil
}
