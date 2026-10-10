package service

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

/*
순위에 올랐는데 카탈로그에 없는 종목을 등록한다 (2026-09-26 외부 방문자 점검).

# 무엇이 보였나

실시간 차트 국내 거래대금 5위가 이름 없이 「0010S0」로 떴다(9/23 신규 상장
와이즈플래닛컴퍼니, +288%). 이름은 `attachRankingNames`가 `stocks` 표에서 붙이는데,
카탈로그에 새 종목을 넣는 길은 **누군가 그 종목 화면을 열 때**(`MarketService.Stocks`의
토스 조회 → 저장)뿐이다. 매일 01:00 `toss_universe`는 있는 종목의 거래 가능·이름만
고치고 새 종목은 넣지 않는다(유니버스 중 약 900종목이 카탈로그 밖 — ETN 등 일부러
안 받는 것이 섞여 있어 통째로 넣지 않는다). 그래서 상장 첫날 가장 눈에 띄는 종목이
사흘 넘게 코드로 남았다.

# 여기서 하는 일

순위 폴러가 받은 목록 중 카탈로그에 없는 것만 `Stocks`로 한 번 조회해 저장한다 —
화면을 연 사람이 있었을 때와 **같은 경로**다. 순위에 오를 만큼 거래되는 종목이라
검색에 보여도 되는 종목이다. 요청 경로가 아니라 폴러(배경)에서 하므로 방문자가
토스 호출을 기다리지 않는다.

토스가 모른다고 답하면 같은 종목을 6시간 동안 다시 묻지 않는다 — 폴러는 5분마다 돈다.
*/
const rankingCatalogRetry = 6 * time.Hour

var rankingCatalogTried = struct {
	sync.Mutex
	at map[string]time.Time
}{at: map[string]time.Time{}}

func (s *MarketService) registerUnknownRankingSymbols(ctx context.Context, items []toss.RankingItem) {
	if s == nil || s.stocks == nil || len(items) == 0 {
		return
	}
	symbols := make([]string, 0, len(items))
	for _, it := range items {
		if sym := strings.ToUpper(strings.TrimSpace(it.Symbol)); sym != "" {
			symbols = append(symbols, sym)
		}
	}
	if len(symbols) == 0 {
		return
	}
	rows, err := s.stocks.ListBySymbols(ctx, symbols)
	if err != nil {
		return
	}
	have := make(map[string]bool, len(rows))
	for _, st := range rows {
		have[strings.ToUpper(strings.TrimSpace(st.Symbol))] = true
	}
	missing := pickRankingSymbolsToRegister(symbols, have, time.Now())
	if len(missing) == 0 {
		return
	}
	if _, err := s.Stocks(ctx, missing); err != nil {
		log.Printf("rankings: 카탈로그 밖 종목 %d개 등록 실패: %v", len(missing), err)
		return
	}
	log.Printf("rankings: 카탈로그 밖 종목 %d개를 등록했다 (%s)", len(missing), strings.Join(missing, ","))
}

// pickRankingSymbolsToRegister는 카탈로그에 없고 최근에 묻지 않은 종목을 고르며, 물은 시각을 적는다.
func pickRankingSymbolsToRegister(symbols []string, have map[string]bool, now time.Time) []string {
	rankingCatalogTried.Lock()
	defer rankingCatalogTried.Unlock()
	var out []string
	seen := make(map[string]bool, len(symbols))
	for _, sym := range symbols {
		if have[sym] || seen[sym] {
			continue
		}
		seen[sym] = true
		if at, ok := rankingCatalogTried.at[sym]; ok && now.Sub(at) < rankingCatalogRetry {
			continue
		}
		rankingCatalogTried.at[sym] = now
		out = append(out, sym)
	}
	return out
}
