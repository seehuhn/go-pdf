package pdf_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/optional"
)

var infoTestCases = []struct {
	name    string
	version pdf.Version
	info    *pdf.Info
}{
	{
		name:    "empty",
		version: pdf.V1_7,
		info:    &pdf.Info{},
	},
	{
		name:    "title_only",
		version: pdf.V1_7,
		info: &pdf.Info{
			Title: "Test Title",
		},
	},
	{
		name:    "all_text_fields",
		version: pdf.V1_7,
		info: &pdf.Info{
			Title:    "Test Title",
			Author:   "Test Author",
			Subject:  "Test Subject",
			Keywords: "test, keywords",
			Creator:  "Test Creator",
			Producer: "Test Producer",
		},
	},
	{
		name:    "dates",
		version: pdf.V1_7,
		info: &pdf.Info{
			Title:        "Test",
			CreationDate: pdf.Date(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)),
			ModDate:      pdf.Date(time.Date(2024, 6, 20, 14, 30, 0, 0, time.UTC)),
		},
	},
	{
		name:    "trapped_true",
		version: pdf.V1_7,
		info: &pdf.Info{
			Title:   "Test",
			Trapped: optional.NewBool(true),
		},
	},
	{
		name:    "trapped_false",
		version: pdf.V1_7,
		info: &pdf.Info{
			Title:   "Test",
			Trapped: optional.NewBool(false),
		},
	},
	{
		name:    "trapped_unknown",
		version: pdf.V1_7,
		info: &pdf.Info{
			Title: "Test",
			// Trapped unset = unknown
		},
	},
	{
		name:    "custom_fields",
		version: pdf.V1_7,
		info: &pdf.Info{
			Custom: map[string]string{
				"grumpy": "bärbeißig",
				"funny":  "\000\001\002 \\<>'\")(",
			},
		},
	},
	{
		name:    "full",
		version: pdf.V2_0,
		info: &pdf.Info{
			Title:        "Test Title",
			Author:       "Test Author",
			Subject:      "Test Subject",
			Keywords:     "test, keywords",
			Creator:      "Test Creator",
			Producer:     "Test Producer",
			CreationDate: pdf.Date(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)),
			ModDate:      pdf.Date(time.Date(2024, 6, 20, 14, 30, 0, 0, time.UTC)),
			Trapped:      optional.NewBool(true),
			Custom: map[string]string{
				"CustomField": "CustomValue",
			},
		},
	},
}

func TestInfoRoundTrip(t *testing.T) {
	for _, tc := range infoTestCases {
		t.Run(tc.name, func(t *testing.T) {
			infoRoundTripTest(t, tc.version, tc.info)
		})
	}
}

func infoRoundTripTest(t *testing.T, version pdf.Version, original *pdf.Info) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	embedded, err := rm.Embed(original)
	if err != nil {
		t.Fatal(err)
	}

	// empty Info returns nil
	if embedded == nil {
		if original.Title != "" || original.Author != "" || original.Custom != nil {
			t.Fatal("unexpected nil for non-empty Info")
		}
		return
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	extracted, err := pdf.ExtractInfo(x, embedded)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(original, extracted); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestExtractInfoNil(t *testing.T) {
	x := pdf.NewExtractor(nil)
	info, err := pdf.ExtractInfo(x, nil)
	if err != nil {
		t.Fatal(err)
	}
	if info != nil {
		t.Error("expected nil Info for nil object")
	}
}

func FuzzInfoRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range infoTestCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)

		embedded, err := rm.Embed(tc.info)
		if err != nil {
			continue
		}
		if embedded == nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = embedded

		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing object")
		}

		x := pdf.NewExtractor(r)
		info, err := pdf.ExtractInfo(x, obj)
		if err != nil {
			t.Skip("malformed info")
		}
		if info == nil {
			t.Skip("nil info")
		}

		infoRoundTripTest(t, pdf.GetVersion(r), info)
	})
}
