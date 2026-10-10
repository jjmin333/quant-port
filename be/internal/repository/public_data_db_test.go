package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// 실제 DB로 단지 찾기 — CHANGE_TEST_DSN이 있을 때만(읽기만). 2026-10-09 제보 「범어역우방유쉘이 안 찾아진다」의 변형들.
func TestSearchComplexDirectoryDB(t *testing.T) {
	dsn := os.Getenv("CHANGE_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_TEST_DSN 없음")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewPublicDataRepo(db)
	for _, q := range []string{"범어역우방유쉘", "범어역 우방유쉘", "범어역 우방 유쉘", "범어동 35-1", "수성구 범어역우방유쉘"} {
		hits, err := repo.SearchComplexDirectory(context.Background(), "", q, 5)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, h := range hits {
			if h.Key == "27260-1324" {
				found = true
			}
		}
		t.Logf("%q → %d건 %+v", q, len(hits), hits[:min(1, len(hits))])
		if !found {
			t.Errorf("%q: 범어역우방유쉘(27260-1324)을 못 찾음", q)
		}
	}
}
