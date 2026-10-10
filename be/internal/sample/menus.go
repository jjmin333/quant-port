package sample

import (
	"context"
	"math"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// The remaining four "내 투자" menus. Each set is chosen the same way as the
// stamps one: enough rows that the screen's own logic has something to do
// (a loss next to a gain, a pinned note next to a plain one, a target already
// reached next to one still far off) rather than a uniform block that makes
// every column look the same.
//
// The fixtures carry **no prices**. A seed says which symbol and how many days
// ago the row was created; every number on the screen is then read from real
// market data (see live.go). Hand-written prices drift the moment the market
// moves, and a demo whose 담은 뒤 수익률 is arithmetic on two invented numbers
// teaches a shape the real screen will never show.

// roundTick rounds a derived price to something a person would actually type.
// KRW names trade in whole won; USD names in cents.
func roundTick(v float64, currency string) float64 {
	if v <= 0 {
		return 0
	}
	if currency == "USD" {
		return math.Round(v*100) / 100
	}
	if v >= 50000 {
		return math.Round(v/1000) * 1000
	}
	if v >= 5000 {
		return math.Round(v/100) * 100
	}
	return math.Round(v/10) * 10
}

// ---------------------------------------------------------------- 관심 종목

// WatchlistEntry pairs the stored row with the derived numbers the list needs.
// The real endpoint returns only the row and the client fetches quotes; the
// sample carries both so a logged-out visitor sees a filled table without a
// second, also-gated request.
type WatchlistEntry struct {
	model.WatchlistItem
	Name       string  `json:"name"`
	Market     string  `json:"market,omitempty"`
	Currency   string  `json:"currency"`
	Price      float64 `json:"price"`
	ChangeRate float64 `json:"changeRate"`
	Score      *int    `json:"score,omitempty"`
	Grade      string  `json:"grade,omitempty"`
	Week52High float64 `json:"week52High,omitempty"`
	Week52Low  float64 `json:"week52Low,omitempty"`
	// Metrics carries 전일·전주·전월·52주 exactly as the signed-in list receives
	// them. Before this existed the client synthesised the block from the row's
	// changeRate alone, which is why 전주 and 전월 were "-" on every row: nothing
	// in the fixture had ever computed them.
	Metrics *model.WatchlistMetrics `json:"watchlistMetrics,omitempty"`
}

type watchlistSeed struct {
	symbol, name, market, currency string
	addedDaysAgo                   int
	score                          int
	grade                          string
}

// watchlistSeeds spreads the 담은 날짜 across a quarter so the 담은 뒤 column has
// a short hold next to a long one — with real prices the spread in returns comes
// from the market rather than from picked numbers.
var watchlistSeeds = []watchlistSeed{
	{"005930", "삼성전자", "KOSPI", "KRW", 96, 64, "C"},
	{"035720", "카카오", "KOSPI", "KRW", 61, 38, "D"},
	{"NVDA", "NVIDIA", "NASDAQ", "USD", 33, 82, "A"},
	{"AAPL", "애플", "NASDAQ", "USD", 18, 70, "B"},
	{"000660", "SK하이닉스", "KOSPI", "KRW", 12, 71, "B"},
	// 막 담은 것 — 담은 뒤 수익률이 거의 0인 행도 하나 있어야 자연스럽다.
	{"042660", "한화오션", "KOSPI", "KRW", 2, 55, "C"},
}

func Watchlist(ctx context.Context, now time.Time, q Quotes) []WatchlistEntry {
	symbols := make([]string, 0, len(watchlistSeeds))
	for _, s := range watchlistSeeds {
		symbols = append(symbols, s.symbol)
	}
	quotes := resolveQuotes(ctx, q, symbols)

	addedPairs := map[string]time.Time{}
	for _, s := range watchlistSeeds {
		at := ago(now, s.addedDaysAgo)
		addedPairs[closeKey(s.symbol, at)] = endOfDay(at)
	}
	closes := historicalCloses(ctx, q, addedPairs)

	out := make([]WatchlistEntry, 0, len(watchlistSeeds))
	for idx, s := range watchlistSeeds {
		added := ago(now, s.addedDaysAgo)
		quote := quotes[s.symbol]
		if quote == nil || quote.CurrentPrice <= 0 {
			continue
		}

		var metrics *model.WatchlistMetrics
		if q != nil {
			// Mutates quote.Change/ChangeRate to the session-correct baseline,
			// so read the rate *after* this call.
			metrics = q.WatchlistMetricsFor(ctx, quote, added)
		}

		entry := WatchlistEntry{
			WatchlistItem: model.WatchlistItem{
				ID:         int64(idx + 1),
				Symbol:     s.symbol,
				CreatedAt:  added.UTC().Format(time.RFC3339),
				AddedPrice: closes[closeKey(s.symbol, added)],
			},
			Name: s.name, Market: s.market, Currency: s.currency,
			Price: quote.CurrentPrice, ChangeRate: quote.ChangeRate,
			Score: i(s.score), Grade: s.grade,
			Metrics: metrics,
		}
		if metrics != nil {
			entry.Week52High = metrics.Week52High
			entry.Week52Low = metrics.Week52Low
		}
		out = append(out, entry)
	}
	return out
}

// ---------------------------------------------------------------- 내 포트폴리오

// HoldingEntry is a position plus the numbers the summary card needs.
type HoldingEntry struct {
	model.UserHolding
	Currency   string  `json:"currency"`
	Price      float64 `json:"price"`
	ChangeRate float64 `json:"changeRate"`
}

type holdingSeed struct {
	symbol, name, currency string
	qty                    float64
	boughtDaysAgo          int
}

// Buy dates are spread so the portfolio holds a long-standing position beside a
// recent one; whether each shows a gain or a loss is decided by the market, not
// here. Quantities differ enough that 비중 is not a flat six-way split.
var holdingSeeds = []holdingSeed{
	{"005930", "삼성전자", "KRW", 120, 96},
	{"000660", "SK하이닉스", "KRW", 18, 54},
	{"035720", "카카오", "KRW", 90, 61},
	{"NVDA", "NVIDIA", "USD", 22, 33},
	{"AAPL", "애플", "USD", 15, 18},
}

func Holdings(ctx context.Context, now time.Time, q Quotes) []HoldingEntry {
	symbols := make([]string, 0, len(holdingSeeds))
	for _, s := range holdingSeeds {
		symbols = append(symbols, s.symbol)
	}
	quotes := resolveQuotes(ctx, q, symbols)

	pairs := map[string]time.Time{}
	for _, s := range holdingSeeds {
		at := ago(now, s.boughtDaysAgo)
		pairs[closeKey(s.symbol, at)] = endOfDay(at)
	}
	closes := historicalCloses(ctx, q, pairs)

	out := make([]HoldingEntry, 0, len(holdingSeeds))
	for idx, s := range holdingSeeds {
		quote := quotes[s.symbol]
		if quote == nil || quote.CurrentPrice <= 0 {
			continue
		}
		bought := ago(now, s.boughtDaysAgo)
		// 평단은 산 날의 실제 종가다. 없으면 현재가로 떨어뜨린다 — 손익 0으로 보이는
		// 편이, 없는 평단 때문에 행 전체가 빈 칸이 되는 것보다 낫다.
		avg := closes[closeKey(s.symbol, bought)]
		if avg <= 0 {
			avg = quote.CurrentPrice
		}
		out = append(out, HoldingEntry{
			UserHolding: model.UserHolding{
				ID: int64(idx + 1), Symbol: s.symbol, Name: s.name,
				Quantity: s.qty, AvgPrice: &avg, CostCurrency: s.currency,
				Source: "manual", UpdatedAt: bought,
			},
			Currency: s.currency, Price: quote.CurrentPrice, ChangeRate: quote.ChangeRate,
		})
	}
	return out
}

// ---------------------------------------------------------------- 메모장

// JournalEntry carries the current price alongside the note so the
// 「작성 당시 → 현재」 strip has both ends. Both ends are real: 작성 당시 is the
// close on the day the note is dated, 현재 is the live quote.
type JournalEntry struct {
	model.JournalNote
	Price float64 `json:"price"`
}

type journalSeed struct {
	title, body, symbol, name string
	tags                      []string
	currency                  string
	pinned                    bool
	daysAgo                   int
}

// The bodies deliberately state a thesis and what would falsify it, never a
// price level that a live quote could contradict.
var journalSeeds = []journalSeed{
	{
		"삼성전자 — 왜 담았나", "HBM 공급 계약 확대가 실적에 반영되기 시작하는 시점이라고 봤다. 확인할 것: 다음 분기 반도체 부문 영업이익률, 외국인 순매수 지속 여부.\n\n손절 기준은 평단 -12%.",
		"005930", "삼성전자", []string{"매수근거"}, "KRW", true, 40,
	},
	{
		"이번 달 반성", "실적 발표 앞둔 종목을 이유 없이 들고 있었다. 발표 일정을 캘린더에 먼저 넣고 나서 담기로.",
		"", "", []string{"복기"}, "", true, 12,
	},
	{
		"카카오 — 손절 근거 정리", "광고 매출 둔화가 두 분기째다. 반등 근거로 삼았던 신규 서비스 지표가 아직 안 나온다. 다음 분기까지 확인하고 정리 판단.",
		"035720", "카카오", []string{"리스크", "복기"}, "KRW", false, 26,
	},
	{
		"NVDA 관찰", "데이터센터 매출 비중이 계속 오르는 중. 다만 밸류에이션 축이 5축 중 유일하게 30점대라 조정 시 변동성이 크다는 점을 기억할 것.",
		"NVDA", "NVIDIA", []string{"관찰"}, "USD", false, 9,
	},
	{
		"환율 메모", "달러 예수금 비중이 40%를 넘었다. 원/달러가 1,400원 아래로 내려오면 일부 환전 검토.",
		"", "", []string{"관찰"}, "", false, 4,
	},
}

func Journal(ctx context.Context, now time.Time, q Quotes) []JournalEntry {
	var symbols []string
	pairs := map[string]time.Time{}
	for _, s := range journalSeeds {
		if s.symbol == "" {
			continue
		}
		symbols = append(symbols, s.symbol)
		at := ago(now, s.daysAgo)
		pairs[closeKey(s.symbol, at)] = endOfDay(at)
	}
	quotes := resolveQuotes(ctx, q, symbols)
	closes := historicalCloses(ctx, q, pairs)

	out := make([]JournalEntry, 0, len(journalSeeds))
	for idx, s := range journalSeeds {
		at := ago(now, s.daysAgo)
		note := model.JournalNote{
			ID: int64(idx + 1), Title: s.title, Body: s.body,
			Symbol: s.symbol, SymbolName: s.name, Tags: s.tags, Pinned: s.pinned,
			CreatedAt: at.UTC().Format(time.RFC3339), UpdatedAt: at.UTC().Format(time.RFC3339),
		}
		entry := JournalEntry{JournalNote: note}
		if s.symbol != "" {
			if quote := quotes[s.symbol]; quote != nil && quote.CurrentPrice > 0 {
				entry.Price = quote.CurrentPrice
			}
			if written := closes[closeKey(s.symbol, at)]; written > 0 {
				price := written
				entry.PriceAtWrite = &price
				entry.PriceCurrency = s.currency
				entry.PriceDate = at.Format("2006-01-02")
			}
		}
		out = append(out, entry)
	}
	return out
}

// ---------------------------------------------------------------- 목표가

// TargetEntry carries the current price so the "남은 거리" column has both ends.
type TargetEntry struct {
	model.PriceTarget
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	ChangeRate float64 `json:"changeRate"`
}

type targetSeed struct {
	symbol, name, currency, note string
	// Levels are percentages off the 등록가 — the price on the day the target was
	// set — because that is how a person actually picks one ("여기서 15% 더").
	// Fixed prices could not survive a real quote: a 82,000원 target on a stock
	// trading at 270,000원 renders as 도달 and the screen loses every other state.
	targetPct, target2Pct, stopPct float64
	daysAgo                        int
}

// The seeds walk a natural range of intent, from a tight "여기서 5%만 더" to a
// long "두 배 가까이 본다". That spread is a property of how people set targets,
// not a guess about today's prices — but running it across five symbols with
// different real returns is what keeps 도달 / 임박 / 진행 중 / 손절 근접 all likely
// to appear. Which state each row lands in is left to the market; asserting one
// up front is what forced the old fixed prices to lie.
var targetSeeds = []targetSeed{
	{"005930", "삼성전자", "KRW", "HBM 실적 반영 시 재평가", 5, 0, -8, 41},
	{"000660", "SK하이닉스", "KRW", "1차 목표 도달 시 절반 정리 검토", 10, 25, -12, 34},
	{"NVDA", "NVIDIA", "USD", "실적 발표 이후 재설정", 15, 0, -15, 25},
	{"035720", "카카오", "KRW", "반등 시 손절가 근처에서 정리", 25, 0, -6, 19},
	{"AAPL", "애플", "USD", "", 40, 0, 0, 7},
}

func Targets(ctx context.Context, now time.Time, q Quotes) []TargetEntry {
	symbols := make([]string, 0, len(targetSeeds))
	pairs := map[string]time.Time{}
	for _, s := range targetSeeds {
		symbols = append(symbols, s.symbol)
		at := ago(now, s.daysAgo)
		pairs[closeKey(s.symbol, at)] = endOfDay(at)
	}
	quotes := resolveQuotes(ctx, q, symbols)
	closes := historicalCloses(ctx, q, pairs)

	out := make([]TargetEntry, 0, len(targetSeeds))
	for idx, s := range targetSeeds {
		quote := quotes[s.symbol]
		if quote == nil || quote.CurrentPrice <= 0 {
			continue
		}
		at := ago(now, s.daysAgo)
		base := closes[closeKey(s.symbol, at)]
		if base <= 0 {
			base = quote.CurrentPrice
		}

		target := model.PriceTarget{
			ID: int64(idx + 1), Symbol: s.symbol,
			TargetPrice: roundTick(base*(1+s.targetPct/100), s.currency),
			Currency:    s.currency, Note: s.note,
			BaseDate:  at.Format("2006-01-02"),
			CreatedAt: at.UTC().Format(time.RFC3339), UpdatedAt: at.UTC().Format(time.RFC3339),
		}
		target.BasePrice = optionalPrice(base)
		if s.target2Pct > 0 {
			target.TargetPrice2 = optionalPrice(roundTick(base*(1+s.target2Pct/100), s.currency))
		}
		if s.stopPct < 0 {
			target.StopPrice = optionalPrice(roundTick(base*(1+s.stopPct/100), s.currency))
		}

		// 고점·저점은 등록일 이후 실제 일봉에서 나온다 — 진행축이 "어디까지 갔다가
		// 왔는지"를 그리는 근거라, 지어내면 축만 그럴듯해지고 뜻이 사라진다.
		if high, low, ok := rangeSince(ctx, q, s.symbol, at, quote.CurrentPrice); ok {
			target.PeakPrice = optionalPrice(high)
			target.TroughPrice = optionalPrice(low)
			if high >= target.TargetPrice && target.TargetPrice > 0 {
				target.ReachedDate = reachedDateFor(ctx, q, s.symbol, at, target.TargetPrice)
			}
		}

		out = append(out, TargetEntry{
			PriceTarget: target, Name: s.name,
			Price: quote.CurrentPrice, ChangeRate: quote.ChangeRate,
		})
	}
	return out
}

// optionalPrice keeps a 0 seed out of the JSON — the fields are "not set",
// not "set to zero", and the screen distinguishes the two.
func optionalPrice(v float64) *float64 {
	if v <= 0 {
		return nil
	}
	return &v
}

// 픽스처 행 수 — 예시 응답이 「완전한가」를 판정하는 기준이다.
//
// 시세를 못 얻은 종목은 행이 통째로 빠지므로(위 Watchlist/Holdings의
// `if quote == nil || quote.CurrentPrice <= 0 { continue }`), 반환된 행 수가
// 이 값보다 적으면 시장 데이터가 덜 붙은 상태다. 캐시 계층이 그런 결과를
// 저장하지 않도록 이 값을 쓴다 — internal/handler/sample_cache.go 참조.
func WatchlistSeedCount() int { return len(watchlistSeeds) }
func HoldingSeedCount() int   { return len(holdingSeeds) }
