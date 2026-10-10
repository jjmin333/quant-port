package service

import (
	"context"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

const referenceDataPollTick = time.Minute

// RunReferenceDataPoller refreshes shares outstanding and fundamentals on admin-configured intervals.
func RunReferenceDataPoller(
	ctx context.Context,
	settings *MarketDataSettingsStore,
	market *MarketService,
	fundamentals *FundamentalsService,
) {
	if settings == nil {
		return
	}
	lastShares := time.Now()
	/*
		지표 로테이션의 시계는 **DB의 마지막 시도 시각**에서 시작한다(2026-09-28).
		예전엔 time.Now()라 「기동 +24시간」에야 돌았는데, api를 거의 매일 다시 띄우는
		동안 09-22 이후 한 번도 못 돌았다. 기록이 없으면 zero → 첫 틱에 돈다.
	*/
	lastFundamentals := fundamentals.LastRotationAt(ctx)

	ticker := time.NewTicker(referenceDataPollTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			cfg := settings.Get(ctx)
			sharesInterval := time.Duration(cfg.SharesPollHours) * time.Hour
			if lastShares.IsZero() || now.Sub(lastShares) >= sharesInterval {
				if market != nil {
					if n, err := market.RefreshAllSharesOutstanding(ctx); err != nil {
						log.Printf("reference data poller: shares: %v", err)
					} else if n > 0 {
						log.Printf("reference data poller: refreshed shares for %d symbols", n)
					}
				}
				lastShares = now
			}

			fundInterval := time.Duration(cfg.FundamentalsPollHours) * time.Hour
			if lastFundamentals.IsZero() || now.Sub(lastFundamentals) >= fundInterval {
				if fundamentals != nil {
					refreshFundamentalsRotation(ctx, fundamentals)
				}
				lastFundamentals = now
			}
		}
	}
}

/*
한 주기에 도는 종목 수 — 시장별 상한.

국내 400은 저녁 DART 잡의 몫(9,000 예약)을 남긴 뒤 하루 여유(18,000 − 9,000 = 9,000)를
종목당 실측 7호출로 나눈 1,285보다 **한참 낮게** 잡은 값이다. 굳이 낮춘 이유는 이 갱신이
급할 이유가 하나도 없기 때문이다 — 지표는 분기에 한 번 바뀌고, 국내 2,800종목이면
7일에 한 바퀴다. 로테이션은 관심·보유를 앞에 세우므로 **사람이 실제로 보는 종목**이
그 안에서 먼저 돈다.

해외 600은 EDGAR이라 일일 한도가 없다(분당 제한만). 같은 시각에 도는 `financials_overseas`
(등급제 600)와 같은 크기로 맞춘다.
*/
const (
	fundamentalsRotationLimitDomestic = 400
	fundamentalsRotationLimitOverseas = 600
)

/*
두 시장을 차례로 돈다 — **국내를 먼저**.

국내가 예산(DART)에 걸리는 쪽이므로 남은 헤드룸을 먼저 본다. 해외를 먼저 돌리면
EDGAR 호출에 드는 10분 남짓 동안 저녁 DART 잡이 시작될 수 있고, 그때 국내 슬라이스가
예약분만 남은 한도를 마주한다 — 결과는 같지만 로그가 「예산 없음」으로만 남아
원인을 읽기 어려워진다.
*/
func refreshFundamentalsRotation(ctx context.Context, fundamentals *FundamentalsService) {
	for _, run := range []struct {
		market model.QuoteAssetType
		limit  int
	}{
		{model.AssetDomestic, fundamentalsRotationLimitDomestic},
		{model.AssetUS, fundamentalsRotationLimitOverseas},
	} {
		res, err := fundamentals.RefreshRotation(ctx, run.market, run.limit)
		if err != nil {
			log.Printf("reference data poller: fundamentals %s: %v", run.market, err)
			continue
		}
		if res.Attempted == 0 && res.Stopped == "" {
			continue
		}
		log.Printf(
			"reference data poller: fundamentals %s — 시도 %d · 갱신 %d · 실패 %d%s",
			run.market, res.Attempted, res.Updated, res.Failed,
			stoppedSuffix(res.Stopped),
		)
		if ctx.Err() != nil {
			return
		}
	}
}

func stoppedSuffix(stopped string) string {
	if stopped == "" {
		return ""
	}
	return " (" + stopped + ")"
}
