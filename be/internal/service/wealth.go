package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
WealthValuer — 사용자 자산 하나를 평가해 asset_valuations 한 줄을 만든다 (2026-10-01).

평가는 자산 종류마다 다르다(valuation_method):

	comparable      부동산: 같은 건물·면적 실거래 → EstimateComparable(지수는 틈만 보정)
	region_unit     부동산: 같은 동·지역의 ㎡ 단가 × 면적 (단독·토지·상업처럼 지번이 가려진 종류, 단지 거래가 없을 때)
	fixed_income    예금·적금: 원금 + 경과이자(세후)
	bond            채권: 남은 현금흐름을 국고채 곡선 + 신용 스프레드로 할인(ECOS)
	market_price    코인(업비트 원화 시세) · 금(국제 현물 × 원/달러)
	user_estimated  자동차·미술품·기타 — 사용자 값(자동차는 선택적 연 감가율)

입력은 전부 DB(배치가 채운 표)와 1분 캐시(업비트) — 사용자가 늘어도 외부 호출이 늘지 않는다.
*/
type WealthValuer struct {
	wealth *repository.WealthRepo
	data   *repository.PublicDataRepo
	db     *sql.DB
	upbit  *marketdata.UpbitClient

	mu      sync.Mutex
	board   map[string]marketdata.CryptoTicker
	boardAt time.Time
}

func NewWealthValuer(db *sql.DB, upbit *marketdata.UpbitClient) *WealthValuer {
	return &WealthValuer{wealth: repository.NewWealthRepo(db), data: repository.NewPublicDataRepo(db), db: db, upbit: upbit}
}

// ReDetail — 부동산 detail JSON.
type ReDetail struct {
	PropertyType string  `json:"propertyType"` // apt · offi · rh · sh · land · nrg · indu · silv
	LawdCd       string  `json:"lawdCd"`
	RegionName   string  `json:"regionName,omitempty"`
	ComplexKey   string  `json:"complexKey,omitempty"`
	ComplexName  string  `json:"complexName,omitempty"`
	UmdNm        string  `json:"umdNm,omitempty"`
	Address      string  `json:"address,omitempty"`
	Area         float64 `json:"area"`
	Floor        *int    `json:"floor,omitempty"`
	// 세 준 집: 받은 보증금·월세(정보용 — 부채는 user_liabilities에 따로).
	LeaseDeposit *float64 `json:"leaseDeposit,omitempty"`
	MonthlyRent  *float64 `json:"monthlyRent,omitempty"`
	// 취득세·중개보수 등(수익률 계산용).
	AcquisitionCost *float64 `json:"acquisitionCost,omitempty"`
}

func parseReDetail(raw json.RawMessage) ReDetail {
	var d ReDetail
	_ = json.Unmarshal(raw, &d)
	return d
}

// FixedDetail — 예금·적금·채권 detail.
type FixedDetail struct {
	Principal   float64  `json:"principal"`             // 예금 원금 · 채권 액면 총액
	Monthly     float64  `json:"monthly,omitempty"`     // 적금 월 납입액
	Months      int      `json:"months,omitempty"`      // 적금 기간
	Rate        float64  `json:"rate"`                  // 연 % (예금·적금 금리, 채권 표면금리)
	StartDate   string   `json:"startDate"`             // YYYY-MM-DD
	Maturity    string   `json:"maturityDate"`          // YYYY-MM-DD
	Compound    bool     `json:"compound,omitempty"`    // 월복리
	TaxRate     *float64 `json:"taxRate,omitempty"`     // 기본 15.4
	Institution string   `json:"institution,omitempty"` // 은행·발행사
	CouponFreq  int      `json:"couponFreq,omitempty"`  // 채권 연 이자 횟수(0 = 할인채)
	Credit      string   `json:"credit,omitempty"`      // gov · AA · BBB
}

// MarketDetail — 코인·금.
type MarketDetail struct {
	Market string `json:"market,omitempty"` // KRW-BTC
	Unit   string `json:"unit,omitempty"`   // g (금)
}

// OtherDetail — 자동차 감가율.
type OtherDetail struct {
	DepreciationPct float64 `json:"depreciationPct,omitempty"` // 연 %
}

func kstToday(now time.Time) time.Time {
	k := kstNow(now)
	return time.Date(k.Year(), k.Month(), k.Day(), 0, 0, 0, 0, time.UTC)
}

// Valuate computes today's valuation and an explanation payload for the screen.
func (v *WealthValuer) Valuate(ctx context.Context, a repository.UserAsset, now time.Time) (repository.AssetValuation, any, error) {
	today := kstToday(now)
	out := repository.AssetValuation{AssetID: a.ID, AsOf: today.Format("2006-01-02")}
	userInput := func(source string) (repository.AssetValuation, any, error) {
		if a.UserValue == nil {
			return out, nil, fmt.Errorf("값이 없습니다")
		}
		out.Value, out.Kind, out.Source = *a.UserValue, "user_input", source
		return out, nil, nil
	}
	switch a.AssetType {
	case "real_estate":
		if a.ValuationMethod == "user_estimated" {
			return userInput("직접 입력")
		}
		d := parseReDetail(a.Detail)
		est, ok, err := v.estimateRealEstate(ctx, d, today)
		if err != nil {
			return out, nil, err
		}
		if !ok {
			if a.UserValue != nil {
				return userInput("직접 입력 (비교 거래 없음)")
			}
			return out, nil, fmt.Errorf("비교할 실거래가 아직 없습니다")
		}
		lo, hi := est.Low, est.High
		out.Value, out.Low, out.High, out.Kind, out.Confidence = est.Value, &lo, &hi, "estimate", est.Confidence
		out.Source = "국토부 실거래가"
		if est.IndexAdjusted {
			out.Source += " · 한국부동산원 주간지수"
		}
		inputs, _ := json.Marshal(est)
		out.Inputs = inputs
		return out, est, nil

	case "deposit", "saving":
		var d FixedDetail
		_ = json.Unmarshal(a.Detail, &d)
		tax := 15.4
		if d.TaxRate != nil {
			tax = *d.TaxRate
		}
		start, _ := time.Parse("2006-01-02", d.StartDate)
		if start.IsZero() {
			return userInput("직접 입력")
		}
		var value, interest float64
		if a.AssetType == "saving" {
			value, _, interest = SavingValue(d.Monthly, d.Rate, start, d.Months, today, tax)
		} else {
			mat, _ := time.Parse("2006-01-02", d.Maturity)
			value, interest = DepositValue(d.Principal, d.Rate, start, mat, today, d.Compound, tax)
		}
		out.Value, out.Kind, out.Source = value, "accrual", fmt.Sprintf("원금 + 경과이자(세후 %.1f%%)", tax)
		ex := map[string]any{"interest": interest, "taxRate": tax}
		out.Inputs, _ = json.Marshal(ex)
		return out, ex, nil

	case "bond":
		var d FixedDetail
		_ = json.Unmarshal(a.Detail, &d)
		mat, err := time.Parse("2006-01-02", d.Maturity)
		if err != nil || d.Principal <= 0 {
			return userInput("직접 입력")
		}
		y, spreadSrc, ok := v.bondYield(ctx, d.Credit, mat.Sub(today).Hours()/24/365)
		if !ok {
			return userInput("직접 입력 (금리 자료 없음)")
		}
		out.Value = BondValue(d.Principal, d.Rate, d.CouponFreq, mat, today, y)
		out.Kind, out.Confidence, out.Source = "estimate", "medium", "한국은행 ECOS "+spreadSrc
		ex := map[string]any{"discountYield": math.Round(y*1000) / 1000, "curve": spreadSrc}
		out.Inputs, _ = json.Marshal(ex)
		return out, ex, nil

	case "crypto":
		var d MarketDetail
		_ = json.Unmarshal(a.Detail, &d)
		t, ok := v.cryptoTicker(ctx, d.Market)
		if !ok || a.Quantity == nil {
			return userInput("직접 입력 (시세 없음)")
		}
		out.Value, out.Kind, out.Source = math.Round(*a.Quantity*t.Price), "market", "업비트 원화 시세"
		ex := map[string]any{"price": t.Price, "changeRate": t.ChangeRate, "koreanName": t.KoreanName}
		out.Inputs, _ = json.Marshal(ex)
		return out, ex, nil

	case "gold":
		if a.Quantity == nil {
			return userInput("직접 입력")
		}
		xau, okX := v.latestIndexClose(ctx, "gold")
		fx, okF := v.latestIndexClose(ctx, "usd-krw")
		if !okX || !okF {
			return userInput("직접 입력 (시세 없음)")
		}
		out.Value, out.Kind, out.Source = GoldValue(*a.Quantity, xau, fx), "market", "국제 금 현물 × 원/달러"
		ex := map[string]any{"xauUsd": xau, "usdKrw": fx, "perGram": math.Round(xau / 31.1034768 * fx)}
		out.Inputs, _ = json.Marshal(ex)
		return out, ex, nil
	}

	// 자동차·미술품·기타.
	if a.UserValue == nil {
		return out, nil, fmt.Errorf("값이 없습니다")
	}
	var d OtherDetail
	_ = json.Unmarshal(a.Detail, &d)
	value := *a.UserValue
	source := "직접 입력"
	if a.AssetType == "vehicle" && d.DepreciationPct > 0 {
		// 입력한 날(마지막 수정)부터 연 감가율만큼 깎는다.
		years := today.Sub(a.UpdatedAt).Hours() / 24 / 365
		if years > 0 {
			value = math.Round(value * math.Pow(1-d.DepreciationPct/100, years))
			source = fmt.Sprintf("직접 입력 · 연 %.0f%% 감가", d.DepreciationPct)
		}
	}
	out.Value, out.Kind, out.Source = value, "user_input", source
	return out, nil, nil
}

// estimateRealEstate — 같은 건물(단지) 비교 → 없으면 같은 동 ㎡ 단가.
func (v *WealthValuer) estimateRealEstate(ctx context.Context, d ReDetail, today time.Time) (ReEstimate, bool, error) {
	if d.LawdCd == "" || d.Area <= 0 {
		return ReEstimate{}, false, nil
	}
	since := today.AddDate(-1, 0, -7)
	if d.ComplexKey != "" {
		trades, err := v.data.ComplexTrades(ctx, d.ComplexKey, "sale", since)
		if err != nil {
			return ReEstimate{}, false, err
		}
		if est, ok := EstimateComparable(trades, d.Area, today, v.regionIndex(ctx, d.LawdCd)); ok {
			v.attachJeonse(ctx, &est, d, today)
			return est, true, nil
		}
	}
	kind := d.PropertyType
	if kind == "" {
		kind = "apt"
	}
	trades, err := v.data.AreaTrades(ctx, d.LawdCd, kind, d.UmdNm, since)
	if err != nil {
		return ReEstimate{}, false, err
	}
	est, ok := EstimateRegionUnit(trades, d.Area, today)
	if ok {
		v.attachJeonse(ctx, &est, d, today)
	}
	return est, ok, nil
}

// jeonseInfo — 전세 시세(화면용).
type jeonseInfo struct {
	Median float64 `json:"median"`
	N      int     `json:"n"`
}

func (v *WealthValuer) attachJeonse(ctx context.Context, est *ReEstimate, d ReDetail, today time.Time) {
	if d.ComplexKey == "" {
		return
	}
	rents, err := v.data.ComplexTrades(ctx, d.ComplexKey, "rent", today.AddDate(-1, 0, -7))
	if err != nil {
		return
	}
	// 같은 단지 전월세는 aptSeq로 같은 열쇠다.
	if m, n := JeonseEstimate(rents, d.Area, today); n > 0 {
		est.Jeonse = &jeonseInfo{Median: m, N: n}
	}
}

// RegionIndexFor is regionIndex for the API (단지 화면의 추정가).
func (v *WealthValuer) RegionIndexFor(ctx context.Context, lawdCd string) IndexAt {
	if lawdCd == "" {
		return nil
	}
	return v.regionIndex(ctx, lawdCd)
}

// regionIndex — 그 시군구의 주간 매매지수(없으면 nil — 보정 없이 평가).
func (v *WealthValuer) regionIndex(ctx context.Context, lawdCd string) IndexAt {
	regions, err := v.data.Regions(ctx)
	if err != nil {
		return nil
	}
	cls := 0
	for _, g := range regions {
		if g.LawdCd == lawdCd && g.RoneClsID != nil {
			cls = *g.RoneClsID
		}
	}
	if cls == 0 {
		return nil
	}
	pts, err := v.data.IndexSeries(ctx, roneWeeklySale, cls, 0, time.Now().AddDate(-2, 0, 0))
	if err != nil || len(pts) == 0 {
		return nil
	}
	return func(t time.Time) (float64, bool) {
		d := t.Format("2006-01-02")
		val, ok := 0.0, false
		for _, p := range pts {
			if p.Date > d {
				break
			}
			val, ok = p.Value, true
		}
		if !ok {
			return pts[0].Value, true
		}
		return val, true
	}
}

// bondYield — 국고채 곡선 보간 + 신용 스프레드(회사채 3년 − 국고채 3년).
func (v *WealthValuer) bondYield(ctx context.Context, credit string, years float64) (float64, string, bool) {
	latest, err := v.data.LatestRates(ctx)
	if err != nil {
		return 0, "", false
	}
	curve := map[float64]float64{}
	byKey := map[string]float64{}
	for _, d := range RateSeriesDefs {
		if p, ok := latest[d.ID()]; ok && p[0].Date != "" {
			byKey[d.Key] = p[0].Value
			if d.Years > 0 {
				curve[d.Years] = p[0].Value
			}
		}
	}
	y, ok := CurveYield(curve, math.Max(years, 0.25))
	if !ok {
		return 0, "", false
	}
	src := "국고채 곡선"
	switch credit {
	case "AA":
		if aa, ok := byKey["corpAA"]; ok {
			y += aa - byKey["ktb3"]
			src += " + AA- 스프레드"
		}
	case "BBB":
		if b, ok := byKey["corpBBB"]; ok {
			y += b - byKey["ktb3"]
			src += " + BBB- 스프레드"
		}
	}
	return y, src, true
}

func (v *WealthValuer) latestIndexClose(ctx context.Context, id string) (float64, bool) {
	var x float64
	err := v.db.QueryRowContext(ctx, `SELECT close_value FROM index_daily_closes WHERE index_id = ? ORDER BY trade_date DESC LIMIT 1`, id).Scan(&x)
	return x, err == nil && x > 0
}

// CryptoBoard — 업비트 원화 마켓 전체(1분 캐시). 실패하면 마지막 값을 쓴다.
func (v *WealthValuer) CryptoBoard(ctx context.Context) map[string]marketdata.CryptoTicker {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.board != nil && time.Since(v.boardAt) < time.Minute {
		return v.board
	}
	if v.upbit == nil {
		return v.board
	}
	list, err := v.upbit.KRWBoard(ctx)
	if err != nil || len(list) == 0 {
		return v.board
	}
	m := make(map[string]marketdata.CryptoTicker, len(list))
	for _, t := range list {
		m[t.Market] = t
	}
	v.board, v.boardAt = m, time.Now()
	return m
}

func (v *WealthValuer) cryptoTicker(ctx context.Context, market string) (marketdata.CryptoTicker, bool) {
	t, ok := v.CryptoBoard(ctx)[strings.ToUpper(strings.TrimSpace(market))]
	return t, ok
}

// RevalueRealEstate re-runs today's valuation for every real-estate asset (실거래 적재 뒤).
func (v *WealthValuer) RevalueRealEstate(ctx context.Context) (int, error) {
	return v.revalue(ctx, []string{"real_estate"})
}

// RevalueAll writes today's valuation for every asset (매일 한 줄 — 순자산 추이의 재료).
func (v *WealthValuer) RevalueAll(ctx context.Context) (int, error) {
	return v.revalue(ctx, []string{"real_estate", "deposit", "saving", "bond", "crypto", "gold", "vehicle", "art", "other"})
}

func (v *WealthValuer) revalue(ctx context.Context, types []string) (int, error) {
	n := 0
	var firstErr error
	for _, t := range types {
		assets, err := v.wealth.AssetsOfType(ctx, t)
		if err != nil {
			return n, err
		}
		for _, a := range assets {
			val, _, err := v.Valuate(ctx, a, time.Now())
			if err != nil {
				continue // 값이 없는 자산은 건너뛴다(입력 대기)
			}
			if err := v.wealth.UpsertValuation(ctx, val); err != nil && firstErr == nil {
				firstErr = err
				continue
			}
			n++
		}
	}
	return n, firstErr
}
