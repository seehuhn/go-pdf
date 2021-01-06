package pdf

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestWalk(t *testing.T) {
	fd, err := os.Open("test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	fi, err := fd.Stat()
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(fd, fi.Size(), nil)
	if err != nil {
		t.Fatal(err)
	}

	seen := make(map[Reference]bool)
	err = r.Walk(r.Trailer, seen, func(obj Object) error {
		fmt.Println(format(obj))
		return nil
	})
	// t.Error("fish")
}

func TestReader6c3fdd9c(t *testing.T) {
	// found by go-fuzz - check that the code doesn't panic
	buf := strings.NewReader("%PDF-1.0\n0 0obj<startxref8")
	NewReader(buf, buf.Size(), nil)
}

func TestReader4d613ef2(t *testing.T) {
	// found by go-fuzz - check that the code doesn't panic
	buf := strings.NewReader("%PDF-1.0\n0 0obj<</Le" +
		"ngth -40>>stream\nsta" +
		"rtxref8\n")
	NewReader(buf, buf.Size(), nil)
}

func TestReader215874cf(t *testing.T) {
	// found by go-fuzz - check that the code doesn't hang
	buf := strings.NewReader("%PDF-1.4\n%\xb5\xed\xae\xfb\n3 0 o" +
		"bj\n<< /Length 3 0 R\n" +
		"   /Filter /FlateDec" +
		"ode\n>>\nstream\nx\x9c+\xe42P" +
		"\x00\xc1 w\x05\xfdD\x03\x85\xf4b.#\x85r\xa0\x98\x17\x10g" +
		"qE\xc7*\x18\xe8\x19(\xa4p\x19\x1a(\xf8*\x00\tK\x03\x85" +
		"\\\x10\x01d\xe6(\x04s\x05r\x01\x00\xacp\f\xeb\nend" +
		"stream\nendobj\n4 0 ob" +
		"j\n   63\nendobj\n2 0 o" +
		"bj\n<<\n   /ExtGState " +
		"<<\n      /a0 << /CA " +
		"1 /ca 1 >>\n   >>\n>>\n" +
		"endobj\n5 0 obj\n<< /T" +
		"ype /Page\n   /Parent" +
		" 1 0 R\n   /MediaBox " +
		"[ 0 0 100 100 ]\n   /" +
		"Contents 3 0 R\n   /G" +
		"roup <<\n      /Type " +
		"/Group\n      /S /Tra" +
		"nsparency\n      /CS " +
		"/DeviceRGB\n   >>\n   " +
		"/Resources 2 0 R\n>>\n" +
		"endobj\n1 0 obj\n<< /T" +
		"ype /Pages\n   /Kids " +
		"[ 5 0 R ]\n   /Count " +
		"1\n>>\nendobj\n6 0 obj\n" +
		"<< /Creator (cairo 1" +
		".8.10 (http://cairog" +
		"raphics.org))\n   /Pr" +
		"oducer (cairo 1.8.10" +
		" (http://cairographi" +
		"cs.org))\n>>\nendobj\n7" +
		" 0 obj\n<< /Type /Cat" +
		"alog\n   /Pages 1 0 R" +
		"\n>>\nendobj\nxref0 8\n0" +
		"000000000 65535 f \n0" +
		"000000447 00000 n \n0" +
		"000000175 00000 n \n0" +
		"000000015 00000 n \n7" +
		"000000154 00000 n \n0" +
		"000000247 00000 n \n0" +
		"000000512 00000 n \n0" +
		"000000639 00000 n \nt" +
		"railer<</ 8/Root 7 0" +
		"R/ 6 0R>>startxref69" +
		"1\n")
	r, err := NewReader(buf, buf.Size(), nil)
	if err != nil {
		return
	}

	seen := make(map[Reference]bool)
	r.Walk(r.Trailer, seen, func(o Object) error {
		if stream, ok := o.(*Stream); ok {
			_, err := io.Copy(ioutil.Discard, stream.R)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func TestReader65af881f(t *testing.T) {
	// found by go-fuzz - check that the code doesn't panic
	buf := strings.NewReader("%PDF-1.0\n0 0obj<</ 0 0%startxref8")
	NewReader(buf, buf.Size(), nil)
}

func TestReaderb74cc49a0(t *testing.T) {
	// found by go-fuzz - check that the code doesn't panic
	buf := strings.NewReader("%PDF-1.0\n0 0obj(startxref8")
	NewReader(buf, buf.Size(), nil)
}
