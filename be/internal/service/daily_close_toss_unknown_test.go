package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

/*
종가는 크리티컬 데이터다(AGENT_STATE 개발 원칙). 그래서 「대상에서 뺀다」는 판단은
조건 셋을 **모두** 만족해야 하고, 그 셋이 여기서 잠긴다 (2026-09-11 사용자 요청 7).

배경 실측: 해외 구멍 메우기가 회차마다 3,250여 종목을 실패하고 다음 회차에 같은
종목을 또 시도했다(하루 두 번 = 6,500여 Toss 호출). 우리 카탈로그의 NORMAL 종목이
Toss가 취급하는 집합의 상위집합이라서다(워런트·우선주·라이츠).
*/

func TestMarkFetchOutcome_noSyncStateChangesNothing(t *testing.T) {
	// 배선이 없으면 예전 동작 그대로 — 아무것도 건너뛰지 않는다.
	s := &DailyCloseService{}
	notFound := &toss.Error{StatusCode: http.StatusNotFound, Code: "stock-not-found"}
	if s.markFetchOutcome(nil, "NTRBW", 0, notFound) {
		t.Fatal("sync 배선 없이 종목을 건너뛰었다")
	}
}

func TestTossUnknownFilter_emptyWithoutSyncState(t *testing.T) {
	// 필터가 비어야 쿼리가 예전과 같은 대상을 고른다.
	s := &DailyCloseService{}
	if got := s.tossUnknownFilter(); got != "" {
		t.Fatalf("배선 없으면 필터가 비어야 한다: %q", got)
	}
}

func TestTossUnknownFilter_onlyExcludesNotFoundWithinCooldown(t *testing.T) {
	// 필터의 조건이 셋이어야 한다: kind · status=not_found · 30일 안.
	// (SQL 문자열을 읽는 시험이다 — DB 없이 조건이 빠지는 회귀를 잡는다.)
	// 빈 리포로 충분하다 — tossUnknownFilter 는 SQL 문자열만 만들고 DB를 쓰지 않는다.
	s := &DailyCloseService{sync: &repository.SyncStateRepo{}}
	sql := s.tossUnknownFilter()
	for _, want := range []string{
		model.SyncKindDailyCloseToss,
		model.SyncNotFound,
		"INTERVAL 30 DAY",
		"symbol_sync_state",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("필터에 %q 조건이 없다:\n%s", want, sql)
		}
	}
	// error 상태를 빼면 안 된다 — 그쪽은 다시 물어야 나아진다.
	if strings.Contains(sql, "'"+model.SyncError+"'") {
		t.Fatalf("error 상태까지 제외하고 있다:\n%s", sql)
	}
}

func TestIsStockNotFound_gateForSkipping(t *testing.T) {
	// 건너뛰기의 1번 조건: 오류 코드가 stock-not-found 일 때만.
	// 429·타임아웃·5xx 로는 절대 건너뛰지 않는다.
	skippable := &toss.Error{StatusCode: http.StatusNotFound, Code: "stock-not-found"}
	if !toss.IsStockNotFound(skippable) {
		t.Fatal("stock-not-found 를 못 알아본다")
	}
	for _, err := range []error{
		&toss.Error{StatusCode: http.StatusTooManyRequests, Code: "rate-limit-exceeded"},
		&toss.Error{StatusCode: http.StatusServiceUnavailable, Code: "unavailable"},
		&toss.Error{StatusCode: http.StatusBadRequest, Code: "invalid-request"},
	} {
		if toss.IsStockNotFound(err) {
			t.Fatalf("일시 실패로 종목을 뺄 수 있게 되어 있다: %v", err)
		}
	}
}
