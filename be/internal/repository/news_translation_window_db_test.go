package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

/*
번역 대기 질의의 **창**이 실제로 창 노릇을 하는지 DB에서 확인한다.

# 왜 DB가 필요한가

이 규칙은 Go 코드가 아니라 `published_at >= DATE_SUB(NOW(), INTERVAL ? DAY)`
한 줄이다. 파라미터를 상수에서 인자로 옮긴 변경(2026-09-15)이 노린 것이 정확히
그 줄이라, 목 객체로는 아무것도 시험하지 못한다.

시험이 지키는 것은 하나다: **창 밖의 행은 창을 넓혀야만 보인다.** 2026-09-15에
7일 창 밖에 580건이 잠들어 있었고, 그것들은 창이 있는 한 영원히 번역되지 않는다.

# 왜 기본적으로 건너뛰는가

`NEWS_TRANSLATE_TEST_DSN`이 없으면 skip 한다 — 나머지 repository 단위 시험은
DB 없이 돌아야 한다(ops_incidents_db_test.go와 같은 규칙).

	NEWS_TRANSLATE_TEST_DSN='quantport:quantport@tcp(127.0.0.1:3306)/quantport?parseTime=true' \
	  go test ./internal/repository/ -run TestListMissingTranslationWindow -v

# 남기지 않는다

id에 시험 전용 접두사를 붙이고 끝에서 지운다. 운영 표에 시험 행이 남으면 그
표를 보는 사람이 제일 먼저 속고, 여기서는 그 행이 **화면의 뉴스 목록에** 뜬다.
*/
const newsTranslateTestPrefix = "__test__translate_window"

func TestListMissingTranslationWindow(t *testing.T) {
	dsn := os.Getenv("NEWS_TRANSLATE_TEST_DSN")
	if dsn == "" {
		t.Skip("NEWS_TRANSLATE_TEST_DSN 없음 — DB 시험은 건너뛴다")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	repo := NewNewsRepo(db)

	// 하나는 창 안(어제), 하나는 창 밖(30일 전). 둘 다 번역이 비어 있다.
	rows := []struct {
		id  string
		age time.Duration
	}{
		{newsTranslateTestPrefix + "_recent", 24 * time.Hour},
		{newsTranslateTestPrefix + "_old", 30 * 24 * time.Hour},
	}
	cleanup := func() {
		for _, r := range rows {
			if _, err := db.ExecContext(ctx, `DELETE FROM news_articles WHERE id = ?`, r.id); err != nil {
				t.Errorf("시험 행 정리 실패 (%s): %v", r.id, err)
			}
		}
	}
	cleanup()
	/*
		**`t.Cleanup`이 아니라 `defer`다.** t.Cleanup은 시험 함수가 돌아온 **뒤에**
		돌고, 위의 `defer db.Close()`는 돌아오는 **그 순간에** 돈다 — 그래서
		t.Cleanup으로 두면 닫힌 풀에 DELETE를 쏘게 된다. 실제로 그렇게 두었다가
		운영 표에 시험 행 둘을 남겼다(2026-09-15). defer는 LIFO라 여기 둔 것이
		Close보다 먼저 돈다.

		오류를 삼키지 않는 것도 같은 이유다 — 조용히 실패하면 다음 사람도 같은
		자리에서 같은 것을 남긴다.
	*/
	defer cleanup()

	for _, r := range rows {
		_, err := db.ExecContext(ctx, `
			INSERT INTO news_articles
				(id, market, url_hash, url, headline, source_id, published_at, translate_attempts)
			VALUES (?, 'us', ?, ?, 'Nasdaq rises on chip earnings', 'rss', ?, 0)
		`, r.id, r.id, "https://example.test/"+r.id, time.Now().Add(-r.age))
		if err != nil {
			t.Fatalf("insert %s: %v", r.id, err)
		}
	}

	found := func(windowDays int) map[string]bool {
		t.Helper()
		// 운영 행이 섞이므로 넉넉히 가져와 id로 고른다.
		recs, err := repo.ListMissingTranslationWindow(ctx, "us", 5000, windowDays)
		if err != nil {
			t.Fatalf("list(window=%d): %v", windowDays, err)
		}
		got := map[string]bool{}
		for _, rec := range recs {
			got[rec.ID] = true
		}
		return got
	}

	inWindow := found(7)
	if !inWindow[rows[0].id] {
		t.Fatal("창 안(어제) 기사가 7일 창에서 빠졌다")
	}
	if inWindow[rows[1].id] {
		t.Fatal("창 밖(30일 전) 기사가 7일 창에 들어왔다 — 창이 창 노릇을 못 한다")
	}

	wide := found(3650)
	if !wide[rows[1].id] {
		t.Fatal("창을 넓혔는데도 30일 전 기사가 안 나온다 — 한 번에 비우기가 불가능해진다")
	}
}
