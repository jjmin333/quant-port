package service

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"github.com/enact/quant-port/be/internal/repository"
)

// 실제 DB로 자산 태그 미리보기(읽기만) — ASSET_MENTION_TEST_DSN이 있을 때만.
func TestAssetMentionPreviewDB(t *testing.T) {
	dsn := os.Getenv("ASSET_MENTION_TEST_DSN")
	if dsn == "" {
		t.Skip("ASSET_MENTION_TEST_DSN 없음 — DB 시험은 건너뛴다")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := NewAssetMentionService(repository.NewAssetIntelRepo(db), repository.NewPublicDataRepo(db), repository.NewAssetWatchRepo(db))
	ctx := context.Background()
	for _, q := range []string{"헬리오", "송파", "정기예금", "국고채"} {
		got := svc.Search(ctx, q, "", 4)
		t.Logf("search %s → %+v", q, got)
		if len(got) == 0 {
			t.Errorf("search %s: empty", q)
		}
	}
	body := "@아파트[송파구 헬리오시티]:전세가율 · @지역[송파구]:4주 · #금리[국고채 3년] · @상품[없는은행 없는상품]"
	tokens, assets := svc.preview(ctx, parseMentions(body))
	for _, tk := range tokens {
		t.Logf("token %+v", tk)
	}
	for _, a := range assets {
		t.Logf("asset %s %s %s snap=%v extra=%v note=%q", a.Kind, a.Key, a.Name, a.Snap, a.Extra, a.Note)
	}
	if len(tokens) != 3 {
		t.Fatalf("want 3 tokens (없는 상품은 글자), got %d", len(tokens))
	}
}
