package marketdata

import (
	"bytes"
	"io"
	"strings"

	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

// decodeNaverFinanceBody converts Naver finance.naver.com API payloads to UTF-8 JSON bytes.
// Those endpoints declare Content-Type: text/plain;charset=EUC-KR.
func decodeNaverFinanceBody(raw []byte, contentType string) ([]byte, error) {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "charset=utf-8") || strings.Contains(ct, "charset=utf8") {
		return raw, nil
	}
	reader := transform.NewReader(bytes.NewReader(raw), korean.EUCKR.NewDecoder())
	out, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return out, nil
}
