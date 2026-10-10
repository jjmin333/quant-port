package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

// 실제 DB로 금융상품 AI 입력 — ASSET_MENTION_TEST_DSN이 있을 때만(읽기만, LLM 0).
func TestFinMarketInputsDB(t *testing.T) {
	dsn := os.Getenv("ASSET_MENTION_TEST_DSN")
	if dsn == "" {
		t.Skip("ASSET_MENTION_TEST_DSN 없음")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &AssetAIService{fin: repository.NewFinProductRepo(db), now: time.Now}
	in := s.finMarketInputs(context.Background(), 2.5)
	for _, k := range []string{"fin_savings", "fin_loans"} {
		b, _ := json.Marshal(in[k])
		t.Logf("%s %s", k, b)
		if len(b) < 50 {
			t.Fatalf("%s 입력이 비었다", k)
		}
		if est := len(b) / 3; est > 600 {
			t.Fatalf("%s 입력이 길다(약 %d토큰)", k, est)
		}
	}
}
