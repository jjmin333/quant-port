package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
해외 일반 뉴스 카테고리 로테이션 (2026-09-11 사용자 요청 7).

배경: 네 갈래를 한 회차에 차례로 부르면 티커 주기(2분)와 최소 간격(2분)이 같아
**첫 갈래만 성공하고 나머지 셋은 언제나 실패**했다 — merger·forex·crypto 가 한 번도
수집되지 않았고 로그에는 같은 줄이 회차마다 셋씩 찍혔다.
*/

func TestFinnhubCategoryRotation_coversEveryCategory(t *testing.T) {
	st := &NewsStore{}
	seen := map[string]int{}
	// 한 바퀴 두 번 돌린다 — 순서가 돌고 빠짐이 없어야 한다.
	for i := 0; i < len(finnhubUSCategories)*2; i++ {
		st.mu.Lock()
		category := finnhubUSCategories[st.finnhubCategoryCursor%len(finnhubUSCategories)]
		st.finnhubCategoryCursor = (st.finnhubCategoryCursor + 1) % len(finnhubUSCategories)
		st.mu.Unlock()
		seen[category]++
	}
	if len(seen) != len(finnhubUSCategories) {
		t.Fatalf("갈래 %d개 중 %d개만 돌았다: %v", len(finnhubUSCategories), len(seen), seen)
	}
	for _, category := range finnhubUSCategories {
		if seen[category] != 2 {
			t.Fatalf("%s 가 %d번 — 두 바퀴면 각 2번이어야 한다 (%v)", category, seen[category], seen)
		}
	}
}

func TestFinnhubCategories_areNotEmpty(t *testing.T) {
	// 목록이 비면 로테이션이 0으로 나눈다.
	if len(finnhubUSCategories) == 0 {
		t.Fatal("카테고리 목록이 비었다")
	}
	if finnhubUSCategories[0] != "general" {
		t.Fatalf("재기동 직후 가장 넓은 갈래가 먼저 와야 한다: %v", finnhubUSCategories)
	}
}

/*
종목별 뉴스는 **버킷이 달라야** 한 회차가 끝난다 (2026-09-13 사용자 요청 3).

시장 전체 뉴스와 같은 버킷(최소 간격 2분)에 있으면 32종목 중 첫 종목만 성공하고
나머지 31개가 전부 "min interval not elapsed"로 떨어진다. 실제로 그렇게 돌고
있었고, 큐 한 바퀴가 8분이 아니라 7시간이었다.
*/
func TestFinnhubCompanyNews_hasItsOwnBucket(t *testing.T) {
	if apibudget.ProviderFinnhubNewsCompany == apibudget.ProviderFinnhubNews {
		t.Fatal("종목별 뉴스가 시장 전체 뉴스와 같은 버킷이다 — 첫 종목 뒤 전부 막힌다")
	}

	budget := apibudget.NewManager(nil)
	pace := budget.MinInterval(apibudget.ProviderFinnhubNewsCompany)
	if pace <= 0 {
		t.Fatal("종목별 뉴스 버킷에 간격이 없다 — 상대 상한을 지킬 근거가 사라진다")
	}

	// 한 회차가 자기 주기 안에 끝나야 한다. 끝나지 못하면 큐가 밀리고,
	// 밀린 만큼 관심 종목의 뉴스가 늦게 들어온다.
	span := time.Duration(newsFinnhubCompanySymbolsPerTick-1) * pace
	if span >= newsUSCompanyPollInterval {
		t.Fatalf("한 회차가 %s 걸리는데 주기는 %s다 — 회차가 서로 밟는다", span, newsUSCompanyPollInterval)
	}
}

/*
종목별 회차는 **공용 select 루프를 붙잡으면 안 된다** (2026-09-13).

그 회차는 종목마다 2초씩 쉬므로 한 번에 약 1분을 쓰는데, 뉴스 소스 전부가 select
한 줄을 나눠 쓴다. 붙잡고 있으면 그 사이 naver·rss 틱이 **버려진다**
(`staggeredTicker`는 큐가 아니다). 그래서 고루틴으로 내고 겹침만 막는다.
*/
func TestUSCompanySweep_guardAllowsOneAtATime(t *testing.T) {
	st := &NewsStore{}

	if !st.usCompanySweeping.CompareAndSwap(false, true) {
		t.Fatal("첫 회차가 시작하지 못했다")
	}
	// 앞 회차가 아직 도는 동안 온 틱은 거른다 — 기다려서 쌓으면 한도를 두 배로 쓴다.
	if st.usCompanySweeping.CompareAndSwap(false, true) {
		t.Fatal("회차가 겹쳤다 — 상대가 느려지면 한도를 두 배로 쓴다")
	}
	st.usCompanySweeping.Store(false)
	if !st.usCompanySweeping.CompareAndSwap(false, true) {
		t.Fatal("끝난 뒤 다음 회차가 막혔다 — 한 번 늦으면 영영 안 돈다")
	}
}

/*
한 회차가 주기를 넘기지 않는다 — 넘기면 위 가드가 매번 걸러 실효 주기가 배가 된다.
*/
func TestUSCompanySweep_fitsWithinItsInterval(t *testing.T) {
	budget := apibudget.NewManager(nil)
	pace := budget.MinInterval(apibudget.ProviderFinnhubNewsCompany)
	span := time.Duration(newsFinnhubCompanySymbolsPerTick-1) * pace
	if span >= newsUSCompanyPollInterval {
		t.Fatalf("회차 %s ≥ 주기 %s", span, newsUSCompanyPollInterval)
	}
}
