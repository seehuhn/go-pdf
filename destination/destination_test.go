package destination

import (
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestXYZ(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &XYZ{
		Page: Target(pageRef),
		Left: 100,
		Top:  200,
		Zoom: 1.5,
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr, ok := obj.(pdf.Array)
	if !ok {
		t.Fatalf("expected Array, got %T", obj)
	}

	if len(arr) != 5 {
		t.Fatalf("expected 5 elements, got %d", len(arr))
	}

	if arr[0] != pageRef {
		t.Errorf("page: got %v, want %v", arr[0], pageRef)
	}

	if arr[1] != pdf.Name("XYZ") {
		t.Errorf("type: got %v, want XYZ", arr[1])
	}

	if arr[2] != pdf.Number(100) {
		t.Errorf("left: got %v, want 100", arr[2])
	}

	if arr[3] != pdf.Number(200) {
		t.Errorf("top: got %v, want 200", arr[3])
	}

	if arr[4] != pdf.Number(1.5) {
		t.Errorf("zoom: got %v, want 1.5", arr[4])
	}
}

func TestXYZWithUnset(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &XYZ{
		Page: Target(pageRef),
		Left: Unset,
		Top:  Unset,
		Zoom: Unset,
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)

	if arr[2] != nil {
		t.Errorf("left: got %v, want nil", arr[2])
	}

	if arr[3] != nil {
		t.Errorf("top: got %v, want nil", arr[3])
	}

	if arr[4] != nil {
		t.Errorf("zoom: got %v, want nil", arr[4])
	}
}

func TestXYZInvalidValues(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	tests := []struct {
		name string
		dest *XYZ
	}{
		{
			name: "infinite left",
			dest: &XYZ{Page: Target(pageRef), Left: math.Inf(1), Top: 0, Zoom: 1},
		},
		{
			name: "infinite top",
			dest: &XYZ{Page: Target(pageRef), Left: 0, Top: math.Inf(-1), Zoom: 1},
		},
		{
			name: "infinite zoom",
			dest: &XYZ{Page: Target(pageRef), Left: 0, Top: 0, Zoom: math.Inf(1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.dest.Encode(rm)
			if err == nil {
				t.Error("expected error for infinite value, got nil")
			}
		})
	}
}

func TestFit(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &Fit{
		Page: Target(pageRef),
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr, ok := obj.(pdf.Array)
	if !ok {
		t.Fatalf("expected Array, got %T", obj)
	}

	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}

	if arr[0] != pageRef {
		t.Errorf("page: got %v, want %v", arr[0], pageRef)
	}

	if arr[1] != pdf.Name("Fit") {
		t.Errorf("type: got %v, want Fit", arr[1])
	}
}

func TestFitH(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &FitH{
		Page: Target(pageRef),
		Top:  500,
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}

	if arr[1] != pdf.Name("FitH") {
		t.Errorf("type: got %v, want FitH", arr[1])
	}

	if arr[2] != pdf.Number(500) {
		t.Errorf("top: got %v, want 500", arr[2])
	}
}

func TestFitV(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &FitV{
		Page: Target(pageRef),
		Left: 100,
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}

	if arr[1] != pdf.Name("FitV") {
		t.Errorf("type: got %v, want FitV", arr[1])
	}

	if arr[2] != pdf.Number(100) {
		t.Errorf("left: got %v, want 100", arr[2])
	}
}
