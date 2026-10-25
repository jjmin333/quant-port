package marketdata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 키는 헤더로만 — URL에 남으면 타임아웃 오류 메시지가 키를 로그에 찍는다.
func TestWithFinnhubTokenSendsHeaderNotQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Finnhub-Token") != "k1secret" || r.URL.Query().Has("token") {
			t.Errorf("header=%q query=%q", r.Header.Get("X-Finnhub-Token"), r.URL.RawQuery)
		}
		time.Sleep(50 * time.Millisecond)
	}))
	defer srv.Close()

	hc := withFinnhubToken(&http.Client{Timeout: 10 * time.Millisecond}, "k1secret")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/quote?symbol=AAPL", nil)
	_, err := hc.Do(req)
	if err == nil {
		t.Fatal("want timeout")
	}
	if strings.Contains(err.Error(), "k1secret") {
		t.Fatalf("key leaked into error: %v", err)
	}
}
