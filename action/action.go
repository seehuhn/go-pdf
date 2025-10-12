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

// PDF 2.0 sections: 12.6.1 12.6.2

package action

import (
	"seehuhn.de/go/pdf"
)

// Action represents a PDF action that can be performed when triggered.
// Actions can navigate within or between documents, launch applications,
// play media, manipulate form fields, and more.
type Action interface {
	ActionType() Type
	pdf.Encoder
}

// Type identifies the type of action.
type Type pdf.Name

const (
	TypeGoTo             Type = "GoTo"
	TypeGoToR            Type = "GoToR"
	TypeGoToE            Type = "GoToE"
	TypeGoToDp           Type = "GoToDp"
	TypeLaunch           Type = "Launch"
	TypeThread           Type = "Thread"
	TypeURI              Type = "URI"
	TypeSound            Type = "Sound"
	TypeMovie            Type = "Movie"
	TypeHide             Type = "Hide"
	TypeNamed            Type = "Named"
	TypeSubmitForm       Type = "SubmitForm"
	TypeResetForm        Type = "ResetForm"
	TypeImportData       Type = "ImportData"
	TypeSetOCGState      Type = "SetOCGState"
	TypeRendition        Type = "Rendition"
	TypeTrans            Type = "Trans"
	TypeGoTo3DView       Type = "GoTo3DView"
	TypeJavaScript       Type = "JavaScript"
	TypeRichMediaExecute Type = "RichMediaExecute"
)

// Decode reads an action from a PDF object.
func Decode(x *pdf.Extractor, obj pdf.Object) (Action, error) {
	dict, err := pdf.GetDict(x.R, obj)
	if err != nil {
		return nil, err
	}

	actionType, err := pdf.GetName(x.R, dict["S"])
	if err != nil {
		return nil, err
	}

	switch Type(actionType) {
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

// ActionList represents a sequence of actions to be performed.
type ActionList []Action

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
	dict, err := pdf.GetDict(x.R, obj)
	if err == nil && dict != nil {
		action, err := Decode(x, dict)
		if err != nil {
			return nil, err
		}
		return ActionList{action}, nil
	}

	// array of actions
	arr, err := pdf.GetArray(x.R, obj)
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

// stub decode functions to be implemented in subsequent tasks

func decodeGoToE(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("GoToE action not yet implemented")
}

func decodeGoToDp(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("GoToDp action not yet implemented")
}

func decodeLaunch(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("Launch action not yet implemented")
}

func decodeThread(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("Thread action not yet implemented")
}

func decodeSound(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("Sound action not yet implemented")
}

func decodeMovie(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("Movie action not yet implemented")
}

func decodeHide(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("Hide action not yet implemented")
}

func decodeSubmitForm(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("SubmitForm action not yet implemented")
}

func decodeResetForm(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("ResetForm action not yet implemented")
}

func decodeImportData(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("ImportData action not yet implemented")
}

func decodeSetOCGState(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("SetOCGState action not yet implemented")
}

func decodeRendition(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("Rendition action not yet implemented")
}

func decodeTrans(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("Trans action not yet implemented")
}

func decodeGoTo3DView(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("GoTo3DView action not yet implemented")
}

func decodeJavaScript(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("JavaScript action not yet implemented")
}

func decodeRichMediaExecute(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	return nil, pdf.Error("RichMediaExecute action not yet implemented")
}
