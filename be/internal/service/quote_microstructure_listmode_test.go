package service

import (
	"context"
	"testing"
)

/*
2026-09-11 실측 사고의 회귀선.

마감 후 국내 뉴스 화면 **한 번 진입**이 `/quotes/batch` 한 요청으로 Toss
`rate-limit exceeded`를 90건(orderbook 50 · trades 40) 만들었다. 그때 폴러의 터미널
집합은 0이었고, 호출 스택은 이 경로를 가리켰다:

	BatchQuoteSummariesResult → EnsureQuoteDetail → ensureQuoteDetailCached
	  → loadQuoteDetailPersisted → finalizeQuoteDetail → ensurePriceMicrostructure

목록은 체결·호가를 그리지 않으므로 그 2호출은 전부 버려지는 호출이었다.
*/

func TestEnsurePriceMicrostructure_doesNothingInListMode(t *testing.T) {
	// Toss 클라이언트가 nil인 서비스로도 판정 순서를 볼 수 있다: 목록 모드에서
	// 곧바로 돌아오면 shouldLivePollSymbol 이나 캐시 조회로 내려가지 않는다.
	// nil 의존성을 건드리면 패닉하므로, 패닉하지 않는 것이 곧 「아무것도 안 했다」다.
	s := &QuotesService{}
	ctx := withListMode(context.Background())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("목록 모드에서 아래 경로로 내려갔다 (panic: %v)", r)
		}
	}()
	s.ensurePriceMicrostructure(ctx, "005930")
}

func TestListMode_isNotSetByDefault(t *testing.T) {
	// 가드가 의미를 가지려면 상세 화면 경로에서는 꺼져 있어야 한다.
	if listMode(context.Background()) {
		t.Fatal("기본 컨텍스트가 목록 모드다 — 상세 화면도 체결·호가를 못 받는다")
	}
	if !listMode(withListMode(context.Background())) {
		t.Fatal("withListMode 가 켜지지 않는다")
	}
}
