package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GET /assets/peek — 공개 경로라 입력을 먼저 거른다(2026-10-09 요청 5).
func TestPeekAssetValidation(t *testing.T) {
	h := NewMentionHandler(nil) // 해석기 없음 → 색인에 없는 것과 같다
	for _, c := range []struct {
		query string
		want  int
	}{
		{"kind=bogus&key=1", http.StatusBadRequest},
		{"kind=region&key=", http.StatusBadRequest},
		{"kind=region&key=" + strings.Repeat("a", 81), http.StatusBadRequest},
		{"kind=region&key=11680", http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		h.PeekAsset(rec, httptest.NewRequest(http.MethodGet, "/assets/peek?"+c.query, nil))
		if rec.Code != c.want {
			t.Errorf("%s: got %d, want %d", c.query, rec.Code, c.want)
		}
	}
}
