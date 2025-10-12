package action

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestActionListEncode_Empty(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	var al ActionList
	obj, err := al.Encode(rm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj != nil {
		t.Errorf("expected nil for empty ActionList, got %v", obj)
	}
}

func TestGoToAction(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	// create a simple XYZ destination
	dest := &destination.XYZ{
		Page: pdf.Reference(5),
		Left: 100,
		Top:  200,
		Zoom: 1.5,
	}

	action := &GoTo{
		Dest: dest,
	}

	// encode
	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	// verify S field
	if dict["S"] != pdf.Name("GoTo") {
		t.Errorf("S = %v, want GoTo", dict["S"])
	}

	// decode
	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	goToAction, ok := decoded.(*GoTo)
	if !ok {
		t.Fatalf("expected *GoTo, got %T", decoded)
	}

	if goToAction.ActionType() != TypeGoTo {
		t.Errorf("ActionType = %v, want %v", goToAction.ActionType(), TypeGoTo)
	}
}

func TestURIAction(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	action := &URI{
		URI:   "https://example.com",
		IsMap: true,
	}

	// encode
	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["S"] != pdf.Name("URI") {
		t.Errorf("S = %v, want URI", dict["S"])
	}
	if string(dict["URI"].(pdf.String)) != "https://example.com" {
		t.Errorf("URI = %v, want https://example.com", dict["URI"])
	}

	// decode
	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	uriAction, ok := decoded.(*URI)
	if !ok {
		t.Fatalf("expected *URI, got %T", decoded)
	}

	if uriAction.URI != "https://example.com" {
		t.Errorf("URI = %v, want https://example.com", uriAction.URI)
	}
	if !uriAction.IsMap {
		t.Errorf("IsMap = false, want true")
	}
}

func TestNamedAction(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	action := &Named{
		N: "NextPage",
	}

	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["N"] != pdf.Name("NextPage") {
		t.Errorf("N = %v, want NextPage", dict["N"])
	}

	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	namedAction := decoded.(*Named)
	if namedAction.N != "NextPage" {
		t.Errorf("N = %v, want NextPage", namedAction.N)
	}
}

func TestGoToRAction(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	action := &GoToR{
		F: &file.Specification{FileName: "other.pdf"},
		D: pdf.Array{pdf.Integer(0), pdf.Name("Fit")},
	}

	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["S"] != pdf.Name("GoToR") {
		t.Errorf("S = %v, want GoToR", dict["S"])
	}

	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	goToRAction := decoded.(*GoToR)
	if goToRAction.F == nil {
		t.Error("F is nil")
	}
}

func TestActionListChaining(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	// create a chain: GoTo -> URI -> Named
	action1 := &GoTo{
		Dest: &destination.Fit{Page: pdf.Reference(1)},
	}
	action2 := &URI{
		URI: "https://example.com",
	}
	action3 := &Named{
		N: "NextPage",
	}

	// chain them
	action1.Next = ActionList{action2}
	action2.Next = ActionList{action3}

	// encode
	obj, err := action1.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	// decode
	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, obj)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// verify chain
	goTo := decoded.(*GoTo)
	if len(goTo.Next) != 1 {
		t.Fatalf("expected 1 next action, got %d", len(goTo.Next))
	}

	uri := goTo.Next[0].(*URI)
	if uri.URI != "https://example.com" {
		t.Errorf("URI = %v, want https://example.com", uri.URI)
	}

	if len(uri.Next) != 1 {
		t.Fatalf("expected 1 next action in chain, got %d", len(uri.Next))
	}

	named := uri.Next[0].(*Named)
	if named.N != "NextPage" {
		t.Errorf("N = %v, want NextPage", named.N)
	}
}

func TestActionListMultipleActions(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	// create multiple parallel actions
	action := &GoTo{
		Dest: &destination.Fit{Page: pdf.Reference(1)},
		Next: ActionList{
			&URI{URI: "https://example.com"},
			&Named{N: "NextPage"},
		},
	}

	// encode
	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict := obj.(pdf.Dict)
	nextArr, ok := dict["Next"].(pdf.Array)
	if !ok {
		t.Fatalf("expected Next to be array, got %T", dict["Next"])
	}

	if len(nextArr) != 2 {
		t.Errorf("expected 2 actions in Next array, got %d", len(nextArr))
	}

	// decode and verify
	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, obj)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	goTo := decoded.(*GoTo)
	if len(goTo.Next) != 2 {
		t.Fatalf("expected 2 next actions, got %d", len(goTo.Next))
	}
}

func TestActionRoundTrip(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	tests := []struct {
		name   string
		action Action
	}{
		{
			name: "JavaScript",
			action: &JavaScript{
				JS: pdf.String("app.alert('Hello');"),
			},
		},
		{
			name: "Hide",
			action: &Hide{
				T: pdf.String("MyAnnotation"),
				H: true,
			},
		},
		{
			name: "SubmitForm",
			action: &SubmitForm{
				F:      pdf.String("http://example.com/submit"),
				Fields: pdf.Array{pdf.String("field1"), pdf.String("field2")},
				Flags:  1,
			},
		},
		{
			name: "ResetForm",
			action: &ResetForm{
				Fields: pdf.Array{pdf.String("field1")},
				Flags:  0,
			},
		},
		{
			name: "ImportData",
			action: &ImportData{
				F: &file.Specification{FileName: "data.fdf"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// encode
			obj, err := tt.action.Encode(rm)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

			// decode
			x := pdf.NewExtractor(w)
			decoded, err := Decode(x, obj)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}

			// verify type
			if decoded.ActionType() != tt.action.ActionType() {
				t.Errorf("action type mismatch: got %v, want %v",
					decoded.ActionType(), tt.action.ActionType())
			}
		})
	}
}

func TestNewWindowMode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)
	x := pdf.NewExtractor(w)

	tests := []struct {
		name   string
		action Action
		mode   NewWindowMode
	}{
		{
			name: "GoToR default",
			action: &GoToR{
				F:         &file.Specification{FileName: "other.pdf"},
				D:         pdf.Array{pdf.Integer(0), pdf.Name("Fit")},
				NewWindow: NewWindowDefault,
			},
			mode: NewWindowDefault,
		},
		{
			name: "GoToR replace",
			action: &GoToR{
				F:         &file.Specification{FileName: "other.pdf"},
				D:         pdf.Array{pdf.Integer(0), pdf.Name("Fit")},
				NewWindow: NewWindowReplace,
			},
			mode: NewWindowReplace,
		},
		{
			name: "GoToR new",
			action: &GoToR{
				F:         &file.Specification{FileName: "other.pdf"},
				D:         pdf.Array{pdf.Integer(0), pdf.Name("Fit")},
				NewWindow: NewWindowNew,
			},
			mode: NewWindowNew,
		},
		{
			name: "GoToE default",
			action: &GoToE{
				F:         &file.Specification{FileName: "embedded.pdf"},
				D:         pdf.Array{pdf.Integer(0), pdf.Name("Fit")},
				NewWindow: NewWindowDefault,
			},
			mode: NewWindowDefault,
		},
		{
			name: "GoToE new",
			action: &GoToE{
				F:         &file.Specification{FileName: "embedded.pdf"},
				D:         pdf.Array{pdf.Integer(0), pdf.Name("Fit")},
				NewWindow: NewWindowNew,
			},
			mode: NewWindowNew,
		},
		{
			name: "Launch default",
			action: &Launch{
				F:         &file.Specification{FileName: "app.exe"},
				NewWindow: NewWindowDefault,
			},
			mode: NewWindowDefault,
		},
		{
			name: "Launch replace",
			action: &Launch{
				F:         &file.Specification{FileName: "app.exe"},
				NewWindow: NewWindowReplace,
			},
			mode: NewWindowReplace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := tt.action.Encode(rm)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

			dict := obj.(pdf.Dict)

			// verify encoding
			if tt.mode == NewWindowDefault {
				if dict["NewWindow"] != nil {
					t.Errorf("expected NewWindow to be omitted for default, got %v", dict["NewWindow"])
				}
			} else {
				expected := pdf.Boolean(tt.mode == NewWindowNew)
				if dict["NewWindow"] != expected {
					t.Errorf("NewWindow = %v, want %v", dict["NewWindow"], expected)
				}
			}

			// decode and verify
			decoded, err := Decode(x, obj)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}

			var decodedMode NewWindowMode
			switch a := decoded.(type) {
			case *GoToR:
				decodedMode = a.NewWindow
			case *GoToE:
				decodedMode = a.NewWindow
			case *Launch:
				decodedMode = a.NewWindow
			default:
				t.Fatalf("unexpected action type: %T", decoded)
			}

			if decodedMode != tt.mode {
				t.Errorf("decoded NewWindow = %v, want %v", decodedMode, tt.mode)
			}
		})
	}
}
