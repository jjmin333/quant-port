package sample

import (
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// The remaining four "내 투자" menus. Each set is chosen the same way as the
// stamps one: enough rows that the screen's own logic has something to do
// (a loss next to a gain, a pinned note next to a plain one, a target already
// reached next to one still far off) rather than a uniform block that makes
// every column look the same.

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
}

func Watchlist(now time.Time) []WatchlistEntry {
	seeds := []struct {
		symbol, name, market, currency string
		addedDaysAgo                   int
		addedPrice, price, changeRate  float64
		score                          int
		grade                          string
		high, low                      float64
	}{
		// 담은 뒤 크게 오른 것 — "담은 뒤 수익률" 열이 왜 있는지 보여준다.
		{"005930", "삼성전자", "KOSPI", "KRW", 96, 61800, 74200, 1.24, 64, "C", 88800, 52900},
		// 담은 뒤 떨어진 것 — 같은 열의 반대쪽.
		{"035720", "카카오", "KOSPI", "KRW", 61, 45200, 38100, -1.81, 38, "D", 62000, 36500},
		// 점수는 높은데 밸류에이션이 부담인 미국 종목.
		{"NVDA", "NVIDIA", "NASDAQ", "USD", 33, 152.40, 171.40, -0.92, 82, "A", 212.00, 96.00},
		{"AAPL", "애플", "NASDAQ", "USD", 18, 219.80, 231.10, 0.44, 70, "B", 268.00, 169.00},
		// 52주 최고가 근처 — 레인지 바가 오른쪽 끝에 붙는다.
		{"000660", "SK하이닉스", "KOSPI", "KRW", 12, 201500, 214500, 2.06, 71, "B", 245000, 132000},
		// 막 담은 것 — 담은 뒤 수익률이 거의 0인 행도 하나 있어야 자연스럽다.
		{"042660", "한화오션", "KOSPI", "KRW", 2, 63800, 64100, 0.47, 55, "C", 71200, 38900},
	}

	out := make([]WatchlistEntry, 0, len(seeds))
	for idx, s := range seeds {
		added := ago(now, s.addedDaysAgo)
		out = append(out, WatchlistEntry{
			WatchlistItem: model.WatchlistItem{
				ID:         int64(idx + 1),
				Symbol:     s.symbol,
				CreatedAt:  added.UTC().Format(time.RFC3339),
				AddedPrice: s.addedPrice,
			},
			Name: s.name, Market: s.market, Currency: s.currency,
			Price: s.price, ChangeRate: s.changeRate,
			Score: i(s.score), Grade: s.grade,
			Week52High: s.high, Week52Low: s.low,
		})
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

func Holdings(now time.Time) []HoldingEntry {
	seeds := []struct {
		symbol, name, currency  string
		qty, avg, price, change float64
	}{
		// 크게 번 것과 잃은 것을 같이 둔다 — 손익 색이 한쪽으로만 쏠리면
		// 화면이 무엇을 구분해 보여주는지 알 수 없다.
		{"005930", "삼성전자", "KRW", 120, 61800, 74200, 1.24},
		{"000660", "SK하이닉스", "KRW", 18, 178200, 214500, 2.06},
		{"035720", "카카오", "KRW", 90, 45200, 38100, -1.81},
		{"NVDA", "NVIDIA", "USD", 22, 152.40, 171.40, -0.92},
		{"AAPL", "애플", "USD", 15, 219.80, 231.10, 0.44},
	}
	out := make([]HoldingEntry, 0, len(seeds))
	for idx, s := range seeds {
		avg := s.avg
		out = append(out, HoldingEntry{
			UserHolding: model.UserHolding{
				ID: int64(idx + 1), Symbol: s.symbol, Name: s.name,
				Quantity: s.qty, AvgPrice: &avg, CostCurrency: s.currency,
				Source: "manual", UpdatedAt: ago(now, 1),
			},
			Currency: s.currency, Price: s.price, ChangeRate: s.change,
		})
	}
	return out
}

// ---------------------------------------------------------------- 메모장

// JournalEntry carries the current price alongside the note so the
// 「작성 당시 → 현재」 strip has both ends.
//
// The screen deliberately does not overlay live quotes on sample rows: these
// notes were authored to read a certain way ("샀던 근거가 맞았다"), and a real
// quote would flip that sentence from one visit to the next.
type JournalEntry struct {
	model.JournalNote
	Price float64 `json:"price"`
}

func Journal(now time.Time) []JournalEntry {
	seeds := []struct {
		title, body, symbol, name string
		tags                      []string
		priceAtWrite, price       float64
		currency                  string
		pinned                    bool
		daysAgo                   int
	}{
		{
			"삼성전자 — 왜 담았나", "HBM 공급 계약 확대가 실적에 반영되기 시작하는 시점이라고 봤다. 확인할 것: 다음 분기 반도체 부문 영업이익률, 외국인 순매수 지속 여부.\n\n손절 기준은 평단 -12%.",
			"005930", "삼성전자", []string{"매수근거"}, 68400, 74200, "KRW", true, 40,
		},
		{
			"이번 달 반성", "실적 발표 앞둔 종목을 이유 없이 들고 있었다. 발표 일정을 캘린더에 먼저 넣고 나서 담기로.",
			"", "", []string{"복기"}, 0, 0, "", true, 12,
		},
		{
			"카카오 — 손절 근거 정리", "광고 매출 둔화가 두 분기째다. 반등 근거로 삼았던 신규 서비스 지표가 아직 안 나온다. 다음 분기까지 확인하고 정리 판단.",
			"035720", "카카오", []string{"리스크", "복기"}, 41200, 38100, "KRW", false, 26,
		},
		{
			"NVDA 관찰", "데이터센터 매출 비중이 계속 오르는 중. 다만 밸류에이션 축이 5축 중 유일하게 30점대라 조정 시 변동성이 크다는 점을 기억할 것.",
			"NVDA", "NVIDIA", []string{"관찰"}, 158.20, 171.40, "USD", false, 9,
		},
		{
			"환율 메모", "달러 예수금 비중이 40%를 넘었다. 원/달러가 1,400원 아래로 내려오면 일부 환전 검토.",
			"", "", []string{"관찰"}, 0, 0, "", false, 4,
		},
	}
	out := make([]JournalEntry, 0, len(seeds))
	for idx, s := range seeds {
		at := ago(now, s.daysAgo)
		note := model.JournalNote{
			ID: int64(idx + 1), Title: s.title, Body: s.body,
			Symbol: s.symbol, SymbolName: s.name, Tags: s.tags, Pinned: s.pinned,
			CreatedAt: at.UTC().Format(time.RFC3339), UpdatedAt: at.UTC().Format(time.RFC3339),
		}
		if s.priceAtWrite > 0 {
			price := s.priceAtWrite
			note.PriceAtWrite = &price
			note.PriceCurrency = s.currency
			note.PriceDate = at.Format("2006-01-02")
		}
		out = append(out, JournalEntry{JournalNote: note, Price: s.price})
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

// Targets fixtures cover every state the screen can render: reached, close to
// the target, close to the stop, and a plain single-level target with no stop.
// A demo that only shows the happy row teaches nothing about the other four.
func Targets(now time.Time) []TargetEntry {
	seeds := []struct {
		symbol, name, currency, note string
		target, target2, stop        float64
		base, price, change          float64
		reachedDaysAgo               int
		peak, trough                 float64
		daysAgo                      int
	}{
		// 이미 목표를 넘긴 것 — "도달" 상태가 화면에 한 번은 나와야 한다.
		{"000660", "SK하이닉스", "KRW", "1차 목표 도달 — 절반 정리 검토",
			210000, 240000, 178000, 186500, 214500, 2.06, 6, 218000, 181200, 34},
		{"005930", "삼성전자", "KRW", "HBM 실적 반영 시 재평가",
			82000, 0, 68000, 68400, 78600, 1.24, 0, 81200, 67900, 41},
		{"NVDA", "NVIDIA", "USD", "실적 발표 이후 재설정",
			210, 0, 150, 158.20, 171.40, -0.92, 0, 184.60, 152.10, 25},
		{"035720", "카카오", "KRW", "반등 시 손절가 근처에서 정리",
			42000, 0, 37000, 41200, 38100, -1.81, 0, 42600, 37350, 19},
		{"AAPL", "애플", "USD", "", 250, 0, 0, 228.40, 231.10, 0.44, 0, 236.80, 224.30, 7},
	}
	out := make([]TargetEntry, 0, len(seeds))
	for idx, s := range seeds {
		at := ago(now, s.daysAgo)
		target := model.PriceTarget{
			ID: int64(idx + 1), Symbol: s.symbol, TargetPrice: s.target,
			Currency: s.currency, Note: s.note,
			BaseDate:  at.Format("2006-01-02"),
			CreatedAt: at.UTC().Format(time.RFC3339), UpdatedAt: at.UTC().Format(time.RFC3339),
		}
		target.BasePrice = optionalPrice(s.base)
		target.TargetPrice2 = optionalPrice(s.target2)
		target.StopPrice = optionalPrice(s.stop)
		target.PeakPrice = optionalPrice(s.peak)
		target.TroughPrice = optionalPrice(s.trough)
		if s.reachedDaysAgo > 0 {
			target.ReachedDate = ago(now, s.reachedDaysAgo).Format("2006-01-02")
		}
		out = append(out, TargetEntry{
			PriceTarget: target, Name: s.name, Price: s.price, ChangeRate: s.change,
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
