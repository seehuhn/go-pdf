// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package pieceinfo

import (
	"errors"
	"sync"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pdfcopy"
)

// PieceInfo represents a page-piece dictionary containing private data from
// PDF processors. This maps application names to the corresponding data.
type PieceInfo struct {
	Entries map[pdf.Name]Data

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// Data represents an entry in a page-piece dictionary data dictionary. It
// provides access to the modification time and embeds the private data.
//
// Concrete implementations of Data can be registered using the [Register]
// function.  For entries without a registered handler, an opaque internal
// implementation is used.
type Data interface {
	LastModified() time.Time

	// Embed generates the value for the /Private entry in the data dictionary.
	//
	// This implements the [pdf.Embedder] interface.
	pdf.Embedder
}

// Embed converts the PieceInfo to a PDF dictionary for embedding.
//
// This implements the [pdf.Embedder] interface.
func (p *PieceInfo) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if p == nil || len(p.Entries) == 0 {
		return nil, nil
	}

	result := pdf.Dict{}
	for name, data := range p.Entries {
		dataDict := pdf.Dict{
			"LastModified": pdf.Date(data.LastModified()),
		}
		privateVal, err := rm.Embed(data)
		if err != nil {
			return nil, err
		}
		if privateVal != nil {
			dataDict["Private"] = privateVal
		}

		result[name] = dataDict
	}

	if p.SingleUse {
		return result, nil
	}
	ref := rm.Alloc()
	err := rm.Out().Put(ref, result)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// Extract reads a page-piece dictionary from a PDF object.
// Returns nil if obj is nil.
func Extract(r pdf.Getter, obj pdf.Object) (*PieceInfo, error) {
	dict, err := pdf.GetDict(r, obj)
	if dict == nil {
		return nil, err
	}

	info := &PieceInfo{
		Entries: make(map[pdf.Name]Data),
	}

	// Set SingleUse based on whether obj is a direct dictionary or indirect reference.
	_, isReference := obj.(pdf.Reference)
	info.SingleUse = !isReference

	for key, val := range dict {
		dataDict, _ := pdf.GetDict(r, val)
		if dataDict == nil {
			continue // skip malformed entries
		}

		lastModified, err := pdf.GetDate(r, dataDict["LastModified"])
		if err != nil {
			continue // skip entries without valid LastModified
		}

		privateObj := dataDict["Private"]
		registryMu.RLock()
		handler, exists := registry[key]
		registryMu.RUnlock()

		var data Data
		if exists {
			data, err = handler(r, privateObj)
			if err != nil {
				if errors.Is(err, ErrDiscard) {
					continue // skip this entry
				}
				return nil, err
			}
		} else {
			data = &unknown{
				lastModified: time.Time(lastModified),
				sourceReader: r,
				Private:      privateObj,
			}
		}

		info.Entries[key] = data
	}

	return info, nil
}

// unknown represents the value of one entry in a page-piece dictionary.
// The class does not interpret the data in any way.
// On embedding, it just copies the data from the source PDF to the target PDF.
// This is the default handler, used when no registered handler matches the name of the entry.
type unknown struct {
	lastModified time.Time
	sourceReader pdf.Getter

	// Private holds the /Private entry of the data dictionary in the source PDF.
	Private pdf.Object
}

func (u *unknown) LastModified() time.Time {
	return u.lastModified
}

func (u *unknown) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	if u.Private == nil {
		return nil, nil
	}

	// copy the private object using pdfcopy
	copier := pdfcopy.NewCopier(rm.Out(), u.sourceReader)
	copied, err := copier.Copy(u.Private.AsPDF(rm.Out().GetOptions()))
	if err != nil {
		return nil, err
	}
	return copied, nil
}

// Register installs a handler for page-piece dictionary entries with the given
// key. The function handler is called with the contents of the /Private entry
// in the data dictionary.
//
// If the handler returns ErrDiscard, the entry is ignored and not included in
// the PieceInfo. Any other error returned by the handler will cause Extract to
// abort and return the error.
func Register(name pdf.Name, handler func(r pdf.Getter, private pdf.Object) (Data, error)) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = handler
}

var (
	registryMu sync.RWMutex
	registry   = make(map[pdf.Name]func(r pdf.Getter, private pdf.Object) (Data, error))
)

// ErrDiscard is a special error value which can be returned by a handler
// to indicate that the entry should be discarded and not included in the
// PieceInfo structure.
var ErrDiscard = errors.New("discard")
