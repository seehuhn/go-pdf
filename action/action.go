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

package action

import (
	"seehuhn.de/go/pdf"
)

const (
	TypeGoTo             pdf.Name = "GoTo"
	TypeGoToR            pdf.Name = "GoToR"
	TypeGoToE            pdf.Name = "GoToE"
	TypeGoToDp           pdf.Name = "GoToDp"
	TypeLaunch           pdf.Name = "Launch"
	TypeThread           pdf.Name = "Thread"
	TypeURI              pdf.Name = "URI"
	TypeSound            pdf.Name = "Sound"
	TypeMovie            pdf.Name = "Movie"
	TypeHide             pdf.Name = "Hide"
	TypeNamed            pdf.Name = "Named"
	TypeSubmitForm       pdf.Name = "SubmitForm"
	TypeResetForm        pdf.Name = "ResetForm"
	TypeImportData       pdf.Name = "ImportData"
	TypeSetOCGState      pdf.Name = "SetOCGState"
	TypeRendition        pdf.Name = "Rendition"
	TypeTrans            pdf.Name = "Trans"
	TypeGoTo3DView       pdf.Name = "GoTo3DView"
	TypeJavaScript       pdf.Name = "JavaScript"
	TypeRichMediaExecute pdf.Name = "RichMediaExecute"
)

// NewWindowMode specifies how a target document should be displayed.
type NewWindowMode uint8

const (
	// NewWindowDefault indicates the viewer should use its preference.
	NewWindowDefault NewWindowMode = 0
	// NewWindowReplace indicates the target should replace the current window.
	NewWindowReplace NewWindowMode = 1
	// NewWindowNew indicates the target should open in a new window.
	NewWindowNew NewWindowMode = 2
)

// Decode reads an action from a PDF object.
//
// Always invoke this via [pdf.Decode] so that indirect references are
// resolved and cycle detection covers self- and back-references.
func Decode(c pdf.Cursor, obj pdf.Object, _ bool) (pdf.Action, error) {
	dict, err := c.DictTyped(obj, "Action")
	if err != nil {
		return nil, err
	}

	actionType, err := c.Name(dict["S"])
	if err != nil {
		return nil, err
	}

	switch actionType {
	case TypeGoTo:
		return decodeGoTo(c, dict)
	case TypeGoToR:
		return decodeGoToR(c, dict)
	case TypeGoToE:
		return decodeGoToE(c, dict)
	case TypeGoToDp:
		return decodeGoToDp(c, dict)
	case TypeLaunch:
		return decodeLaunch(c, dict)
	case TypeThread:
		return decodeThread(c, dict)
	case TypeURI:
		return decodeURI(c, dict)
	case TypeSound:
		return decodeSound(c, dict)
	case TypeMovie:
		return decodeMovie(c, dict)
	case TypeHide:
		return decodeHide(c, dict)
	case TypeNamed:
		return decodeNamed(c, dict)
	case TypeSubmitForm:
		return decodeSubmitForm(c, dict)
	case TypeResetForm:
		return decodeResetForm(c, dict)
	case TypeImportData:
		return decodeImportData(c, dict)
	case TypeSetOCGState:
		return decodeSetOCGState(c, dict)
	case TypeRendition:
		return decodeRendition(c, dict)
	case TypeTrans:
		return decodeTrans(c, dict)
	case TypeGoTo3DView:
		return decodeGoTo3DView(c, dict)
	case TypeJavaScript:
		return decodeJavaScript(c, dict)
	case TypeRichMediaExecute:
		return decodeRichMediaExecute(c, dict)
	default:
		return nil, pdf.Error("unknown action type: " + string(actionType))
	}
}

// PDF 2.0 sections: 12.6.1 12.6.2

// ActionList represents a sequence of actions to be performed.
type ActionList []pdf.Action

// Encode encodes the action list for the Next entry.
// Returns nil for empty, single dict for one action, array for multiple.
func (al ActionList) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if len(al) == 0 {
		return nil, nil
	}
	if len(al) == 1 {
		return al[0].Encode(rm)
	}
	arr := make(pdf.Array, len(al))
	for i, action := range al {
		dict, err := action.Encode(rm)
		if err != nil {
			return nil, err
		}
		arr[i] = dict
	}
	return arr, nil
}

// DecodeActionList reads an action list from a PDF object.
// Handles both single dictionary and array formats.
//
// Always invoke this via [pdf.Decode] so that indirect references are
// resolved and cycle detection covers self- and back-references.
func DecodeActionList(c pdf.Cursor, obj pdf.Object, _ bool) (ActionList, error) {
	if obj == nil {
		return nil, nil
	}

	// try single action dictionary first
	dict, err := c.Dict(obj)
	if err == nil && dict != nil {
		// The dict has already been resolved; any indirect ref it came
		// from was added to path by the outer Decode, so calling
		// Decode directly preserves cycle detection.
		action, err := Decode(c, dict, false)
		if err != nil {
			return nil, err
		}
		return ActionList{action}, nil
	}

	// array of actions
	arr, err := c.Array(obj)
	if err != nil {
		return nil, err
	}

	result := make(ActionList, 0, len(arr))
	for _, item := range arr {
		action, err := pdf.Decode(c, item, Decode)
		if err != nil {
			return nil, err
		}
		result = append(result, action)
	}
	return result, nil
}
