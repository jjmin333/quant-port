package marketdata

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

func TestDecodeNaverFinanceBody_EUCKR(t *testing.T) {
	want := "SOL 반도체전공정"
	encoded, err := io.ReadAll(transform.NewReader(strings.NewReader(want), korean.EUCKR.NewEncoder()))
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeNaverFinanceBody(encoded, "text/plain;charset=EUC-KR")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != want {
		t.Fatalf("got %q", string(out))
	}
	_ = bytes.Reader{}
}

func TestDecodeNaverFinanceBody_UTF8Passthrough(t *testing.T) {
	raw := []byte("hello")
	out, err := decodeNaverFinanceBody(raw, "application/json; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q", string(out))
	}
}
