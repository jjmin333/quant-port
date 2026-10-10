package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

// IndexBackfillService loads index_daily_closes with dated history.
//
// Every source below carries its own dates. That matters: the index poller only
// ever writes "today", so without dated history there is no way to reconstruct
// July from a sparkline — a sparkline is an array of closes with no calendar
// attached, and guessing the dates would silently misalign every row.
type IndexBackfillService struct {
	db      *sql.DB
	indices *repository.IndexDailyCloseRepo
	toss    *toss.Client
	fred    *marketdata.FREDClient
	upbit   *marketdata.UpbitClient
}

func NewIndexBackfillService(
	db *sql.DB,
	indices *repository.IndexDailyCloseRepo,
	tossClient *toss.Client,
	cfg *config.Config,
) *IndexBackfillService {
	httpClient := &http.Client{Timeout: 20 * time.Second}
	return &IndexBackfillService{
		db:      db,
		indices: indices,
		toss:    tossClient,
		fred:    marketdata.NewFREDClient(cfg.MarketData.FRED.APIKey, httpClient, nil),
		// 업비트 시세(Quotation) API는 키가 필요 없다 — 일봉도 마찬가지다.
		upbit: marketdata.NewUpbitClient("", "", httpClient, nil),
	}
}

// IndexBackfillResult reports one run.
type IndexBackfillResult struct {
	Rows    int
	Indices int
	Report  []string
}

// etfProxyIndices are tracked as indices but are really ETFs we already store
// daily closes for, so their history is a local read rather than an API call.
var etfProxyIndices = map[string]string{
	"nq100": "QQQ",
	"sox":   "SOXX",
}

/*
fredHistoryOverrides — **실시간 소스와 이력 소스가 다른** 지수.

원/달러의 실시간 값은 토스 지표에서 온다. 그런데 토스 지표 캔들은 `USD/KRW` 심볼에
400을 돌려준다(`backfill-index-closes` 실측). 그래서 이 지수만 이력이 일별 폴러로
하루에 한 점씩 쌓이는 중이었다 — 실측 08-31 기준 **15행**, 나머지 열 종은 145행.
차트를 열면 2주치가 전부라 "1주 전"이 사실상 그래프의 절반이었다.

FRED `DEXKOUS`(원/달러, 영업일)를 이력으로 쓴다. 이 계열은 공표가 며칠 늦지만 그건
**과거를 채우는 데는 상관이 없다** — 오늘 값은 여전히 토스 실시간이 들고 있고, 백필은
지나간 날짜만 메운다.
*/
var fredHistoryOverrides = map[string]string{
	"usd-krw": "DEXKOUS",
}

// upbitHistoryMarkets — 업비트 일봉으로 이력을 채우는 지수.
//
// 비트코인도 원/달러와 같은 처지였다(22행). 실시간은 업비트 티커를 쓰면서 이력은
// 아무도 안 읽어 왔는데, 같은 API의 일봉 엔드포인트가 날짜까지 들고 온다.
var upbitHistoryMarkets = map[string]string{
	"btc": "KRW-BTC",
}

// Run backfills every index it has a dated source for.
func (s *IndexBackfillService) Run(ctx context.Context, since time.Time) (IndexBackfillResult, error) {
	var res IndexBackfillResult
	if s == nil || s.db == nil || s.indices == nil {
		return res, fmt.Errorf("index backfill not configured")
	}

	for _, entry := range marketIndexCatalog {
		var (
			points []datedClose
			err    error
			source string
		)

		switch {
		case etfProxyIndices[entry.id] != "":
			source = "etf_proxy"
			points, err = s.fromStockCloses(ctx, etfProxyIndices[entry.id], since)
		case upbitHistoryMarkets[entry.id] != "":
			source = "upbit"
			points, err = s.fromUpbit(ctx, upbitHistoryMarkets[entry.id], since)
		case fredHistoryOverrides[entry.id] != "":
			// 실시간과 이력의 소스가 다른 지수 — 위 `fredHistoryOverrides` 참고.
			source = "fred"
			points, err = s.fromFRED(ctx, fredHistoryOverrides[entry.id], since)
		case entry.source == indexSourceToss:
			source = "toss"
			points, err = s.fromToss(ctx, entry.symbol, since)
		case entry.source == indexSourceFRED:
			source = "fred"
			points, err = s.fromFRED(ctx, entry.symbol, since)
		case entry.source == indexSourceEcos:
			source = "ecos"
			points, err = s.fromMacroRates(ctx, entry.symbol, since)
		default:
			res.Report = append(res.Report,
				fmt.Sprintf("%-10s 건너뜀 — %s 는 날짜 있는 이력 소스가 없습니다(이후 일별 배치로 누적)", entry.id, entry.source))
			continue
		}

		if err != nil {
			res.Report = append(res.Report, fmt.Sprintf("%-10s 실패 (%s): %v", entry.id, source, err))
			continue
		}
		if len(points) == 0 {
			res.Report = append(res.Report, fmt.Sprintf("%-10s 데이터 없음 (%s)", entry.id, source))
			continue
		}

		n, err := s.store(ctx, entry.id, string(entry.currency), source, points)
		if err != nil {
			res.Report = append(res.Report, fmt.Sprintf("%-10s 저장 실패: %v", entry.id, err))
			continue
		}
		res.Rows += n
		res.Indices++
		res.Report = append(res.Report,
			fmt.Sprintf("%-10s %3d행 (%s, %s ~ %s)", entry.id, n, source,
				points[0].date.Format("2006-01-02"), points[len(points)-1].date.Format("2006-01-02")))
	}

	/*
		마지막으로 **폴러가 하루 밀려 적어 둔 유령 행**을 치운다.

		FRED 계열은 관측치를 하루 늦게 공표해서, 세션이 끝날 때 값을 받아 적는 폴러가
		전일 종가를 오늘 자리에 남겼다 — 같은 값이 이틀 연속으로 서고 등락률이 0%가
		된다(실측 08-31: 나스닥·나스닥100·S&P 500·다우·VIX). 쓰는 쪽은
		`indexLiveDate`가 막았고, 여기서는 이미 쌓인 것을 지운다.

		백필 뒤에 두는 이유는 순서가 중요해서다: 날짜 있는 소스로 진짜 행을 먼저 채워
		놓아야, 유령 행을 지운 자리가 비지 않는다.
	*/
	if removed, err := s.indices.DeletePollerDuplicateCloses(ctx); err != nil {
		res.Report = append(res.Report, fmt.Sprintf("%-10s 중복 정리 실패: %v", "(정리)", err))
	} else if removed > 0 {
		res.Report = append(res.Report,
			fmt.Sprintf("%-10s 폴러 중복 %d행 삭제 (전일과 종가가 같아 등락률 0%%로 남던 행)", "(정리)", removed))
	}

	return res, nil
}

type datedClose struct {
	date  time.Time
	close float64
}

// indexBackfillCandles bounds one Toss request. 200 sessions comfortably covers
// a several-month window without paging.
const indexBackfillCandles = 200

func (s *IndexBackfillService) fromToss(ctx context.Context, symbol string, since time.Time) ([]datedClose, error) {
	if s.toss == nil {
		return nil, fmt.Errorf("toss not configured")
	}
	page, err := s.toss.GetMarketIndicatorCandles(ctx, toss.GetMarketIndicatorCandlesParams{
		Symbol:   symbol,
		Interval: toss.CandleInterval1d,
		Count:    indexBackfillCandles,
	})
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, nil
	}
	out := make([]datedClose, 0, len(page.Candles))
	for _, c := range page.Candles {
		ts, err := time.Parse(time.RFC3339, c.Timestamp)
		if err != nil {
			continue
		}
		v := parseDecimal(c.ClosePrice)
		if v <= 0 || ts.Before(since) {
			continue
		}
		out = append(out, datedClose{date: ts, close: v})
	}
	return sortedDated(out), nil
}

func (s *IndexBackfillService) fromFRED(ctx context.Context, seriesID string, since time.Time) ([]datedClose, error) {
	if s.fred == nil || !s.fred.Enabled() {
		return nil, fmt.Errorf("fred not configured")
	}
	obs, err := s.fred.SeriesHistory(ctx, seriesID, indexBackfillCandles)
	if err != nil {
		return nil, err
	}
	out := make([]datedClose, 0, len(obs))
	for _, o := range obs {
		if o.Date.Before(since) {
			continue
		}
		out = append(out, datedClose{date: o.Date, close: o.Value})
	}
	return sortedDated(out), nil
}

func (s *IndexBackfillService) fromUpbit(ctx context.Context, market string, since time.Time) ([]datedClose, error) {
	if s.upbit == nil || !s.upbit.Enabled() {
		return nil, fmt.Errorf("upbit not configured")
	}
	rows, err := s.upbit.DailyClosesDated(ctx, market, indexBackfillCandles)
	if err != nil {
		return nil, err
	}
	out := make([]datedClose, 0, len(rows))
	for _, r := range rows {
		if r.Date.Before(since) {
			continue
		}
		out = append(out, datedClose{date: r.Date, close: r.Close})
	}
	return sortedDated(out), nil
}

// fromStockCloses reads an ETF proxy's history straight out of the close
// projection — no API call, and it stays consistent with what the stock side
// shows for the same ticker.
func (s *IndexBackfillService) fromStockCloses(ctx context.Context, symbol string, since time.Time) ([]datedClose, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT trade_date, close_price
		  FROM stock_daily_closes
		 WHERE symbol = ? AND trade_date >= ?
		 ORDER BY trade_date`, symbol, since.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []datedClose
	for rows.Next() {
		var d time.Time
		var v float64
		if err := rows.Scan(&d, &v); err != nil {
			return nil, err
		}
		if v > 0 {
			out = append(out, datedClose{date: d, close: v})
		}
	}
	return out, rows.Err()
}

func sortedDated(in []datedClose) []datedClose {
	sort.Slice(in, func(i, j int) bool { return in[i].date.Before(in[j].date) })
	return in
}

// store writes the series with each row's previous close resolved from the row
// before it, so change rates are computed once at load rather than on read.
func (s *IndexBackfillService) store(
	ctx context.Context, indexID, currency, source string, points []datedClose,
) (int, error) {
	rows := make([]repository.IndexDailyCloseRow, 0, len(points))
	for i, p := range points {
		row := repository.IndexDailyCloseRow{
			IndexID:   indexID,
			TradeDate: p.date,
			Close:     p.close,
			Currency:  currency,
			Source:    source,
		}
		if i > 0 && points[i-1].close > 0 {
			prev := points[i-1].close
			row.PrevClose = sql.NullFloat64{Float64: prev, Valid: true}
			row.ChangeAmount = sql.NullFloat64{Float64: p.close - prev, Valid: true}
			row.ChangeRate = sql.NullFloat64{Float64: (p.close - prev) / prev * 100, Valid: true}
		}
		rows = append(rows, row)
	}
	if err := s.indices.UpsertBatch(ctx, rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// fromMacroRates reads 한국 금리 history from macro_rates (worker-asset의 ecos_rates가 채운다).
func (s *IndexBackfillService) fromMacroRates(ctx context.Context, seriesID string, since time.Time) ([]datedClose, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT period_date, value FROM macro_rates WHERE series_id = ? AND period_date >= ? ORDER BY period_date`,
		seriesID, since.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []datedClose
	for rows.Next() {
		var d time.Time
		var v float64
		if err := rows.Scan(&d, &v); err != nil {
			return nil, err
		}
		out = append(out, datedClose{date: d, close: v})
	}
	return out, rows.Err()
}
