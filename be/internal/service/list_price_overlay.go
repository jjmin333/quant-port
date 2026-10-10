package service

import (
	"context"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
)

/*
목록 시세 오버레이 — **규칙 한 벌, 적용 여러 곳** (2026-09-23 QA P0-01).

# 무엇이 문제였나

AI 추천 랭킹(`/stocks/scores/top`)은 장중에 확정 종가를 지금 값으로 덮고
`priceAsOf`를 `live`로 바꿨다. 스크리너(`POST /stocks/screen`)는 그러지 않았다.
둘 다 같은 `stock_daily_closes`를 같은 LATERAL로 읽으므로 **장이 닫혀 있는 동안에는
두 화면의 숫자가 같고**, 열리는 순간 갈린다 — 같은 종목이 한 화면에서 +1.70%,
다른 화면에서 +0.49%가 된다. 2026-09-23 실측(02:05 KST, 장마감)에서 두 응답이
소수점까지 같았던 것이 오히려 이 결함을 숨기고 있었다.

# 왜 함수를 나눴나

고칠 방법은 「스크리너도 같은 일을 하게 한다」인데, 그 일을 **두 번 적으면 다시
갈린다.** 특히 갈리기 쉬운 것이 기준가 고르기(`prevCloseHint`)다 — 저장된 종가가
이미 오늘 것인지를 **그 종목 시장의 표준시로** 재야 하고, 그 한 줄이 틀리면 미국
애프터마켓에서만 한 세션씩 밀린다. 눈으로는 안 갈리는 종류의 차이다.

그래서 여기 있는 것은 **판정**뿐이다(`ResolveListPrices`). 판정을 행에 적어 넣는
일은 호출자가 한다 — 두 행 타입의 필드 이름이 다르고(`ClosePrice` vs `Close`),
그 차이를 없애려고 응답 계약을 바꾸면 화면 쪽이 통째로 따라와야 한다. 나뉘는
자리를 「이름」에 두고 「규칙」은 하나로 두는 편이 안전하다.
*/

// ListPriceInput is one row's price columns as the database gave them.
type ListPriceInput struct {
	Symbol    string
	AssetType string
	/** 저장된 마지막 종가. */
	Close *float64
	/** 그 종가 행의 직전 종가. */
	PrevClose *float64
	/** 그 종가의 거래일 (YYYY-MM-DD). */
	PriceAsOf string
}

// ListPriceDecision is what the screen should show for that row.
type ListPriceDecision struct {
	/*
		BasePrice — 등락률의 기준가. **실시간 시세를 못 구해도 채운다.**

		화면이 SSE로 받은 가격에서 등락률을 다시 낼 때 쓰는 값이라, 이 함수가 시세를
		구했는지와 무관하다. 0이면 모른다는 뜻이고, 그때 화면은 등락률을 0.00%로
		때우지 않고 **그리지 않는다.**
	*/
	BasePrice float64
	/** 실시간 값으로 덮어야 하나. 거짓이면 호출자는 행을 손대지 않는다. */
	Live       bool
	Price      float64
	ChangeRate float64
	Currency   string
}

/*
listPrevCloseHint — 저장된 종가에서 「직전 세션 종가」를 고른다.

행이 들고 있는 종가는 **점수일 이하 중 가장 최근** 것이다. 그게 그 종목 시장의 오늘
세션 것이라면(미국 애프터마켓 중에는 그렇다 — 정규장이 끝나고 적재가 이미 돌았다)
직전 종가는 그 행의 `PrevClose`이고, 아직 오늘 것이 아니라면 그 행의 종가 자체가
직전 종가다.

「오늘」은 그 종목 시장의 표준시로 읽는다 — KST 자정을 넘긴 미국 애프터마켓을 국내
날짜로 재면 한 세션씩 밀린다.
*/
func listPrevCloseHint(in ListPriceInput) float64 {
	if in.Close == nil || *in.Close <= 0 {
		return 0
	}
	schedule := calendar.ScheduleKRX
	if in.AssetType == string(model.AssetUS) || in.AssetType == string(model.AssetETF) {
		schedule = calendar.ScheduleUS
	}
	today := time.Now().In(calendar.LocationFor(schedule)).Format("2006-01-02")
	if in.PriceAsOf >= today && in.PrevClose != nil && *in.PrevClose > 0 {
		return *in.PrevClose
	}
	return *in.Close
}

/*
ResolveListPrices — 행 묶음에 대한 판정을 입력과 **같은 순서로** 돌려준다.

시세를 못 구한 행은 `Live: false`로 나오고, 그 행의 `BasePrice`는 그래도 채워져
있다. 호출자는 「절반은 실시간, 절반은 빈칸」이 아니라 「그 종목만 조용히 종가」가
되도록 손대지 않기만 하면 된다.
*/
func (s *ScoreService) ResolveListPrices(ctx context.Context, in []ListPriceInput) []ListPriceDecision {
	out := make([]ListPriceDecision, len(in))
	if s == nil || len(in) == 0 {
		return out
	}
	for i := range in {
		out[i].BasePrice = listPrevCloseHint(in[i])
	}
	if s.liveQuotes == nil {
		return out
	}
	reqs := make([]LiveChangeRequest, len(in))
	for i := range in {
		reqs[i] = LiveChangeRequest{
			Symbol:        in[i].Symbol,
			AssetType:     in[i].AssetType,
			PrevCloseHint: out[i].BasePrice,
		}
	}
	live := s.liveQuotes.LiveChanges(ctx, reqs)
	if len(live) == 0 {
		return out
	}
	for i := range in {
		lc, ok := live[strings.ToUpper(in[i].Symbol)]
		if !ok || lc.Price <= 0 {
			continue
		}
		out[i].Live = true
		out[i].Price = lc.Price
		out[i].ChangeRate = lc.ChangeRate
		out[i].Currency = lc.Currency
	}
	return out
}
