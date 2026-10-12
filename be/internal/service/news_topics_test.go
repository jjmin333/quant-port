package service

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestClassifyNewsTopic(t *testing.T) {
	cases := []struct{ title, summary, want string }{
		{"송파 잠실 아파트값 6주 연속 상승", "", "real_estate"},
		{"한은 기준금리 동결…연내 인하 가능성", "", "rates"},
		{"주담대 금리 인하 … 은행권 대출 경쟁", "", "loan"},   // 좁은 주제(대출)가 금리를 이긴다
		{"저축은행 예금 금리 4%대 특판 등장", "", "deposit"}, // 예금 > 금리
		{"삼성전자 3분기 영업익 10.7조", "부동산 시장은 한편", ""}, // 요약만으로는 분류하지 않는다
		{"엘리스그룹 IPO 청약 증거금 10조 돌파", "", ""},       // 공모주 청약은 주식
		{"美증시, 유가·국채금리 상승에도 버티는 5가지 이유", "", ""},
		{"코스피 2,600 회복", "", ""},
	}
	for _, c := range cases {
		if got := classifyNewsTopic(c.title, c.summary); got != c.want {
			t.Errorf("%q → %q, want %q", c.title, got, c.want)
		}
	}
}

// topicFor — 종목이 붙거나 해외면 주식이다.
func TestTopicForKeepsStockArticles(t *testing.T) {
	st := &NewsStore{}
	ctx := context.Background()
	if topic, _ := st.topicFor(ctx, "domestic", "현대건설, 재건축 수주", "", []string{"000720"}); topic != "stock" {
		t.Fatalf("종목 기사 → %s", topic)
	}
	if topic, _ := st.topicFor(ctx, "us", "Mortgage rates rise, 주담대", "", nil); topic != "stock" {
		t.Fatalf("해외 → %s", topic)
	}
	if topic, _ := st.topicFor(ctx, "domestic", "전세대출 금리 오른다", "", nil); topic != "loan" {
		t.Fatalf("국내 대출 → %s", topic)
	}
}

// 실제 DB 색인으로 제목 칩 — ASSET_MENTION_TEST_DSN이 있을 때만(읽기만).
func TestTagHeadlineDB(t *testing.T) {
	dsn := os.Getenv("ASSET_MENTION_TEST_DSN")
	if dsn == "" {
		t.Skip("ASSET_MENTION_TEST_DSN 없음")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := NewAssetMentionService(repository.NewAssetIntelRepo(db), repository.NewPublicDataRepo(db), repository.NewAssetWatchRepo(db))
	tags := svc.TagHeadline(context.Background(), "송파구·강남구 아파트값 상승, 기준금리 동결 영향 — database CD")
	t.Logf("%+v", tags)
	kinds := map[string]bool{}
	for _, tg := range tags {
		kinds[tg.Kind+":"+tg.Key] = true
	}
	if !kinds["rate:base"] {
		t.Fatalf("기준금리 칩 없음: %+v", tags)
	}
	if kinds["rate:cd91"] {
		t.Fatal("영문 CD가 걸렸다")
	}
	if len(tags) < 3 || tags[0].Kind != "region" {
		t.Fatalf("지역 둘 + 금리, 제목 순서여야: %+v", tags)
	}
	// 시군구 이름 없이 법정동·단지만 쓴 제목(2026-10-09) — 전국에 하나뿐인 이름이면 칩이 붙는다.
	for _, h := range []string{"잠실동 아파트값 6주 연속 상승", "헬리오시티 전용 84㎡ 신고가", "범어역우방유쉘 매매 늘어"} {
		got := svc.TagHeadline(context.Background(), h)
		t.Logf("%s → %+v", h, got)
		if len(got) == 0 {
			t.Errorf("%s: 칩 없음", h)
		}
	}
	// 일반어 제목에 단지가 걸리면 안 된다.
	if got := svc.TagHeadline(context.Background(), "기준금리 동결 영향 제한적…시장 관망"); len(got) != 1 {
		t.Errorf("일반 제목에 칩 %+v", got)
	}
}

// 시·도 칩(2026-10-10) — 부동산 제목에만, 「경기」 단독은 경기(景氣)와 겹쳐 받지 않는다.
func TestSidoHeadlineDB(t *testing.T) {
	dsn := os.Getenv("ASSET_MENTION_TEST_DSN")
	if dsn == "" {
		t.Skip("ASSET_MENTION_TEST_DSN 없음")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := NewAssetMentionService(repository.NewAssetIntelRepo(db), repository.NewPublicDataRepo(db), repository.NewAssetWatchRepo(db))
	ctx := context.Background()
	has := func(h, kind, key string) bool {
		for _, tg := range svc.TagHeadline(ctx, h) {
			if tg.Kind == kind && tg.Key == key {
				return true
			}
		}
		return false
	}
	if !has("서울 아파트값 상승세 한풀 꺾였다", "sido", "11") {
		t.Error("서울 부동산 제목에 서울 칩 없음")
	}
	if !has("전국 집값 6주 연속 올라", "sido", "00") {
		t.Error("전국 칩 없음")
	}
	if !has("경기도 아파트 거래 늘어", "sido", "41") {
		t.Error("경기도 칩 없음")
	}
	if has("경기 침체 우려에 아파트 매수 심리 위축", "sido", "41") {
		t.Error("경기(景氣)에 경기도 칩")
	}
	if has("서울 은행들 예금 금리 인하", "sido", "11") {
		t.Error("부동산이 아닌 제목에 시·도 칩")
	}
	a, ok := svc.Peek(ctx, "sido", "11")
	if !ok || a.Snap["sido.w4"] == 0 && a.Score == nil {
		t.Fatalf("서울 미리보기 값 없음: %+v", a)
	}
	t.Logf("%+v", a)
}

func TestNewsTopicFinanceFilter(t *testing.T) {
	if !IsNewsTopicFilter("finance") || !IsNewsTopicFilter("rates") || IsNewsTopicFilter("bogus") {
		t.Fatal("IsNewsTopicFilter")
	}
	// 관리자 재분류(IsNewsTopic)는 묶음 값을 기사 topic으로 받으면 안 된다.
	if IsNewsTopic("finance") {
		t.Fatal("finance is a filter, not an article topic")
	}
	st := &NewsStore{}
	items := []model.NewsArticle{
		{ID: "a", Market: "domestic", Topic: "rates"},
		{ID: "b", Market: "domestic", Topic: "deposit"},
		{ID: "c", Market: "domestic", Topic: "loan"},
		{ID: "d", Market: "domestic", Topic: "real_estate"},
	}
	got := st.filterArticles(context.Background(), items, model.NewsListFilter{Market: "domestic", Topic: "finance"})
	if len(got) != 3 {
		t.Fatalf("finance filter: got %d, want 3", len(got))
	}
}
