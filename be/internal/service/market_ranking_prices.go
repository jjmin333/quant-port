package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/toss"
)

/*
랭킹 목록의 **가격·등락률만** 다시 준다 — 순위는 건드리지 않는다.

# 왜 /rankings를 다시 부르지 않나

세 가지 이유다.

**① 순위는 5초마다 바뀌지 않는다.** 순위 스냅샷은 폴러가 갱신한다(실시간 기준
기본 5분). 그 사이에 `/rankings`를 5초마다 불러 봐야 **같은 순서**를 다시 받는다 —
목록 전체를 다시 만드는 값을 치르고 얻는 것이 없다.

**② 줄이 튀면 안 된다.** 화면이 응답을 통째로 갈아 끼우면 스크롤 위치와 행 자리가
흔들린다. 가격만 돌려주면 화면은 **있는 행에 숫자만 덮어쓴다.**

**③ 마감 후에는 공짜다.** `LiveChanges`는 세션이 닫혀 있으면 아무것도 돌려주지
않는다(그 함수 헤더 참조). 장이 닫힌 동안 5초 폴링은 심볼 목록만 훑고 끝난다 —
하루의 대부분이 그 상태다.

# 비용

`LiveChanges`가 `withListMode`로 도므로 종목별 업스트림 호출이 없고, 배치 가격
조회 한 번(`fetchPriceMap`, MGET 1회) + 종목마다 캐시 조회다. 웹소켓이 채우고 있는
종목은 그 캐시에 이미 최신 체결가가 들어와 있어(`applyStreamPrice`가 같은 키에
쓴다) **REST도 DB도 타지 않는다.**
*/

// RankingPriceRow는 한 종목의 지금 가격과 등락률이다.
type RankingPriceRow struct {
	Symbol     string  `json:"symbol"`
	LastPrice  float64 `json:"lastPrice"`
	ChangeRate float64 `json:"changeRate"`
	Currency   string  `json:"currency,omitempty"`
}

/*
RankingPrices — 주어진 랭킹 조건의 종목들에 대해 지금 가격·등락률만 돌려준다.

심볼 목록을 **요청에서 받지 않고 서버가 스냅샷에서 뽑는다.** 화면이 심볼 100개를
쿼리스트링에 실어 보내면 URL이 길어지고, 무엇보다 **화면이 보고 있는 목록과
서버가 아는 목록이 어긋날 수 있다** — 조건만 받으면 그 어긋남이 생기지 않는다.

장이 닫혀 있으면 빈 배열이 나간다. 화면은 그때 `/rankings`가 준 확정 종가를
그대로 들고 있으면 된다.
*/
func (s *MarketService) RankingPrices(ctx context.Context, p toss.GetRankingsParams) ([]RankingPriceRow, error) {
	if s.quotes == nil || !isSupportedRankingPollParams(p) {
		return []RankingPriceRow{}, nil
	}

	storeParams := p
	storeParams.Count = rankingsPollFetchCount
	stored, ok := s.loadRankingsFromStore(ctx, storeParams)
	if !ok {
		return []RankingPriceRow{}, nil
	}

	result := stored.toAPIResult(rankingsAPIResultCount(p.Count))
	if len(result.Rankings) == 0 {
		return []RankingPriceRow{}, nil
	}
	s.attachRankingNames(ctx, result)

	reqs := make([]LiveChangeRequest, 0, len(result.Rankings))
	for i := range result.Rankings {
		row := &result.Rankings[i]
		reqs = append(reqs, LiveChangeRequest{
			Symbol:    row.Symbol,
			AssetType: string(row.AssetType),
			// 스냅샷의 기준가를 힌트로 넘긴다 — 캐시된 기준가가 없을 때
			// 업스트림으로 내려가지 않게 하는 값이다(LiveChangeRequest 주석).
			PrevCloseHint: parseDecimal(row.Price.BasePrice),
		})
	}

	live := s.quotes.LiveChanges(ctx, reqs)
	out := make([]RankingPriceRow, 0, len(live))
	for i := range result.Rankings {
		sym := result.Rankings[i].Symbol
		// LiveChanges는 대문자 심볼로 키를 잡는다(그쪽 중복 제거 루프).
		change, ok := live[strings.ToUpper(strings.TrimSpace(sym))]
		if !ok || change.Price <= 0 {
			continue
		}
		out = append(out, RankingPriceRow{
			Symbol:     sym,
			LastPrice:  change.Price,
			ChangeRate: change.ChangeRate,
			Currency:   change.Currency,
		})
	}
	return out, nil
}
