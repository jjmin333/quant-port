package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// DART 점검(800) 중 corpCode 받기 실패 — 2026-10-09·10 insider·financials가 종목마다 다시 받으려 했다.
func TestEnsureCorpCodes_maintenance(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"status":"800","message":"시스템 점검으로 인한 서비스가 중지 중입니다."}`))
	}))
	defer srv.Close()

	d := NewDartClient(DartConfig{APIKey: "k"}, nil)
	d.httpClient = srv.Client()
	d.corpCodeURL = srv.URL

	// 목록이 없으면 실패하되, 같은 실패를 종목마다 다시 받지 않는다.
	for range 3 {
		if err := d.ensureCorpCodes(context.Background()); err == nil {
			t.Fatal("목록 없이 점검이면 오류여야 한다")
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("corpCode 호출 %d회 — 실패 뒤엔 1회여야 한다", hits.Load())
	}

	// 옛 목록이 있으면 그것을 계속 쓴다.
	d.corpCodeErr = nil
	d.bySymbol = map[string]string{"005930": "00126380"}
	d.loadedAt = time.Now().Add(-25 * time.Hour)
	if err := d.ensureCorpCodes(context.Background()); err != nil {
		t.Fatalf("옛 목록이 있으면 오류가 아니어야 한다: %v", err)
	}
	if d.lookupCorpCode("005930") != "00126380" {
		t.Fatal("옛 목록이 사라졌다")
	}
	if err := d.ensureCorpCodes(context.Background()); err != nil || hits.Load() != 2 {
		t.Fatalf("재시도 간격 안에서는 다시 받지 않아야 한다 (호출 %d회, err %v)", hits.Load(), err)
	}
}
