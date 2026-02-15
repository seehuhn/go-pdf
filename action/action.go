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
func Decode(x *pdf.Extractor, obj pdf.Object) (pdf.Action, error) {
	dict, err := x.GetDictTyped(obj, "Action")
	if err != nil {
		return nil, err
	}

	actionType, err := x.GetName(dict["S"])
	if err != nil {
		return nil, err
	}

	switch actionType {
	case TypeGoTo:
		return decodeGoTo(x, dict)
	case TypeGoToR:
		return decodeGoToR(x, dict)
	case TypeGoToE:
		return decodeGoToE(x, dict)
	case TypeGoToDp:
		return decodeGoToDp(x, dict)
	case TypeLaunch:
		return decodeLaunch(x, dict)
	case TypeThread:
		return decodeThread(x, dict)
	case TypeURI:
		return decodeURI(x, dict)
	case TypeSound:
		return decodeSound(x, dict)
	case TypeMovie:
		return decodeMovie(x, dict)
	case TypeHide:
		return decodeHide(x, dict)
	case TypeNamed:
		return decodeNamed(x, dict)
	case TypeSubmitForm:
		return decodeSubmitForm(x, dict)
	case TypeResetForm:
		return decodeResetForm(x, dict)
	case TypeImportData:
		return decodeImportData(x, dict)
	case TypeSetOCGState:
		return decodeSetOCGState(x, dict)
	case TypeRendition:
		return decodeRendition(x, dict)
	case TypeTrans:
		return decodeTrans(x, dict)
	case TypeGoTo3DView:
		return decodeGoTo3DView(x, dict)
	case TypeJavaScript:
		return decodeJavaScript(x, dict)
	case TypeRichMediaExecute:
		return decodeRichMediaExecute(x, dict)
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
func DecodeActionList(x *pdf.Extractor, obj pdf.Object) (ActionList, error) {
	if obj == nil {
		return nil, nil
	}

	// try single action dictionary first
	dict, err := x.GetDict(obj)
	if err == nil && dict != nil {
		action, err := Decode(x, dict)
		if err != nil {
			return nil, err
		}
		return ActionList{action}, nil
	}

	// array of actions
	arr, err := x.GetArray(obj)
	if err != nil {
		return nil, err
	}

	result := make(ActionList, 0, len(arr))
	for _, item := range arr {
		action, err := Decode(x, item)
		if err != nil {
			return nil, err
		}
		result = append(result, action)
	}
	return result, nil
}
