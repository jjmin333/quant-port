package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDartFSNoDataErr(t *testing.T) {
	if dartFSNoDataErr(nil) {
		t.Fatal("nil should not be no-data")
	}
	if dartFSNoDataErr(fmtError("dart: corp_code not found")) {
		t.Fatal("corp_code miss is not no-data")
	}
	if !dartFSNoDataErr(fmtError("dart: no data for year 2024")) {
		t.Fatal("013 mapping should be no-data")
	}
}

type fmtError string

func (e fmtError) Error() string { return string(e) }

func TestFetchFinancialAccountsCFSThenOFS(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs := r.URL.Query().Get("fs_div")
		calls = append(calls, fs)
		switch fs {
		case dartFSConsolidated:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "013", "message": "없음", "list": []any{},
			})
		case dartFSSeparate:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "000", "message": "정상",
				"list": []map[string]string{
					{"account_nm": "매출액", "thstrm_amount": "1000"},
					{"account_nm": "당기순이익", "thstrm_amount": "100"},
				},
			})
		default:
			t.Fatalf("unexpected fs_div %q", fs)
		}
	}))
	defer srv.Close()

	d := NewDartClient(DartConfig{APIKey: "test-key"}, nil)
	d.financialAPIBase = srv.URL
	d.httpClient = srv.Client()
	d.bySymbol = map[string]string{"403870": "01288827"}
	d.loadedAt = timeNowForTest()

	rows, err := d.fetchFinancialAccounts(context.Background(), "01288827", 2024, dartAnnualReportCode)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(calls) != 2 || calls[0] != dartFSConsolidated || calls[1] != dartFSSeparate {
		t.Fatalf("call order: %v", calls)
	}
	if pickDartTiered(rows, "revenue", dartAmountCurrent) == nil {
		t.Fatal("expected revenue from OFS")
	}
}

func TestFetchFinancialAccountsCFSOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fs_div") != dartFSConsolidated {
			t.Fatalf("should not call OFS when CFS succeeds")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "000", "message": "정상",
			"list": []map[string]string{{"account_nm": "매출액", "thstrm_amount": "500"}},
		})
	}))
	defer srv.Close()

	d := NewDartClient(DartConfig{APIKey: "k"}, nil)
	d.financialAPIBase = srv.URL
	d.httpClient = srv.Client()

	rows, err := d.fetchFinancialAccounts(context.Background(), "00126380", 2024, dartAnnualReportCode)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if pickDartTiered(rows, "revenue", dartAmountCurrent) == nil {
		t.Fatal("expected revenue")
	}
}

func TestFetchFinancialAccountsPropagatesNonMissingCFSFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status":"020","message":"rate limit"}`))
	}))
	defer srv.Close()

	d := NewDartClient(DartConfig{APIKey: "k"}, nil)
	d.financialAPIBase = srv.URL
	d.httpClient = srv.Client()

	_, err := d.fetchFinancialAccounts(context.Background(), "00126380", 2024, dartAnnualReportCode)
	if err == nil || strings.Contains(err.Error(), "no data for year") {
		t.Fatalf("expected transport/API error, got %v", err)
	}
}

func timeNowForTest() time.Time { return time.Now().Add(-time.Hour) }
