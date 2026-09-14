package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

/*
토스 유니버스 적재 — 하루 7호출로 「무엇을 취급하는가」를 확정한다.

# 이 잡이 메우는 자리

우리 카탈로그(status=NORMAL)는 토스가 취급하는 집합의 **상위집합**이다. 그 차이를
예전에는 종목마다 404를 한 번씩 맞아 가며 배웠다 — 실측으로 경고 스윕 호출의
19%(17,918 중 3,388)와 해외 종가 구멍 메우기의 회차당 3,250건이 그 404였다.

`/stocks/all`은 그 집합을 마켓 단위로 한 번에 준다(스펙: 페이지네이션 없음 ·
마켓당 최대 수천 건 · gzip 30KB · 하루 1회 권장). 마켓이 일곱이니 **하루 7호출**이고,
그 버킷(`STOCK_ALL`)은 우리가 **아무것도 쓰지 않던** 버킷이다 — 경고 스윕이 쓰는
`STOCK`(관측 5/s)과 다투지 않는다.

# 한국어 이름을 같이 줍는다 (2026-09-12 사용자 요청 2)

응답의 `name`이 **한국어 종목명**이고 주식 종류까지 들어 있다(GOOGL=알파벳 A ·
GOOG=알파벳 C · BRK.A=버크셔 해서웨이 A). 뉴스 검색·태깅의 별칭이 그 값이다.
별칭을 손으로 적을 필요가 없었던 이유가 이것이다 — **상류가 이미 주고 있었다.**

이름 갱신은 유니버스 판정과 **같은 응답**에서 나오므로 호출이 추가되지 않는다.
*/
type TossUniverseService struct {
	toss     *toss.Client
	universe *repository.TossUniverseRepo
}

func NewTossUniverseService(
	tossClient *toss.Client,
	universe *repository.TossUniverseRepo,
) *TossUniverseService {
	return &TossUniverseService{toss: tossClient, universe: universe}
}

/*
tossUniverseMarketPause — 마켓 사이 간격.

`STOCK_ALL` 버킷의 공표 한도가 없어 스스로 낮게 잡는다. 응답이 큰 호출(마켓당 수천
건)이라 연달아 던질 이유가 없고, 하루 일곱 번이라 간격의 비용이 0이다.
*/
const tossUniverseMarketPause = 2 * time.Second

// TossUniverseResult is one run's outcome for the batch ledger.
type TossUniverseResult struct {
	Markets   int   `json:"markets"`
	Symbols   int   `json:"symbols"`
	Tradable  int64 `json:"tradable"`
	Hidden    int64 `json:"hidden"`
	Renamed   int   `json:"renamed"`
	Unknown   int   `json:"unknown"`
	Incomplete bool `json:"incomplete"`
}

/*
Run — 일곱 마켓을 돌아 유니버스를 확정한다.

**하나라도 실패하면 반영하지 않는다.** 일곱의 합집합이 곧 전체이므로, 빠진 마켓이
있는 합집합으로 「나머지를 숨김」을 돌리면 멀쩡한 종목이 사라진다. 그 판단은
`ApplyUniverse`가 `complete` 인자로 강제한다 — 여기서 잊어도 거기서 막힌다.
*/
func (s *TossUniverseService) Run(ctx context.Context) (map[string]any, string, error) {
	if s == nil || s.toss == nil || !s.toss.Enabled() {
		return nil, "", fmt.Errorf("toss 자격증명이 없어 유니버스를 받을 수 없습니다")
	}
	if s.universe == nil {
		return nil, "", fmt.Errorf("유니버스 저장소가 배선되지 않았습니다")
	}

	markets := toss.AllTossMarkets()
	seen := make(map[string]struct{}, 20000)
	symbols := make([]string, 0, 20000)
	names := make(map[string]string, 20000)
	failed := make([]string, 0, len(markets))

	for i, market := range markets {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-time.After(tossUniverseMarketPause):
			}
		}
		listed, err := s.toss.GetListedStocks(ctx, market)
		if err != nil {
			log.Printf("toss universe %s: %v", market, err)
			failed = append(failed, string(market))
			continue
		}
		for _, item := range listed {
			sym := strings.ToUpper(strings.TrimSpace(item.Symbol))
			if sym == "" {
				continue
			}
			if _, dup := seen[sym]; !dup {
				seen[sym] = struct{}{}
				symbols = append(symbols, sym)
			}
			if name := strings.TrimSpace(item.Name); name != "" {
				names[sym] = name
			}
		}
	}

	out := TossUniverseResult{
		Markets:    len(markets) - len(failed),
		Symbols:    len(symbols),
		Incomplete: len(failed) > 0,
	}
	if len(failed) > 0 {
		return map[string]any{"result": out},
			fmt.Sprintf("마켓 %d/%d 실패 — 반영하지 않음 (%s)",
				len(failed), len(markets), strings.Join(failed, ",")),
			fmt.Errorf("toss universe: %s 마켓을 받지 못했다", strings.Join(failed, ","))
	}

	tradable, hidden, err := s.universe.ApplyUniverse(ctx, symbols, true)
	if err != nil {
		return map[string]any{"result": out}, "", err
	}
	out.Tradable, out.Hidden = tradable, hidden

	// 한국어 이름은 같은 응답에서 나온 것이라 추가 호출이 없다.
	out.Renamed = s.syncLocalNames(ctx, names)

	if _, _, unknown, cErr := s.universe.UniverseCoverage(ctx); cErr == nil {
		out.Unknown = unknown
	}

	msg := fmt.Sprintf(
		"유니버스 %d종목 (마켓 %d) — 거래가능 %d · 숨김 %d · 이름 %d건 갱신",
		out.Symbols, out.Markets, out.Tradable, out.Hidden, out.Renamed,
	)
	if out.Unknown > 0 {
		msg += fmt.Sprintf(" · 미확인 %d", out.Unknown)
	}
	return map[string]any{"result": out}, msg, nil
}

/*
syncLocalNames — 한국어 종목명이 바뀐 것만 갱신한다.

**바뀐 것만** 쓰는 이유는 이 표가 13,000행이고 매일 전부 UPDATE 할 이유가 없기
때문이다(운영 DB가 1 ECPU · 스토리지 29 MiB/s다). 이름이 바뀌는 일은 드물고,
바뀔 때가 바로 별칭이 틀려 있던 때다.
*/
func (s *TossUniverseService) syncLocalNames(ctx context.Context, names map[string]string) int {
	if s.universe == nil || len(names) == 0 {
		return 0
	}
	changed := 0
	for sym, name := range names {
		if ctx.Err() != nil {
			return changed
		}
		n, err := s.universe.UpdateLocalNameIfChanged(ctx, sym, name)
		if err != nil {
			log.Printf("toss universe: rename %s: %v", sym, err)
			continue
		}
		changed += n
	}
	return changed
}
