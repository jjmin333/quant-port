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
}

func NewIndexBackfillService(
	db *sql.DB,
	indices *repository.IndexDailyCloseRepo,
	tossClient *toss.Client,
	cfg *config.Config,
) *IndexBackfillService {
	return &IndexBackfillService{
		db:      db,
		indices: indices,
		toss:    tossClient,
		fred:    marketdata.NewFREDClient(cfg.MarketData.FRED.APIKey, &http.Client{Timeout: 20 * time.Second}, nil),
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
		case entry.source == indexSourceToss:
			source = "toss"
			points, err = s.fromToss(ctx, entry.symbol, since)
		case entry.source == indexSourceFRED:
			source = "fred"
			points, err = s.fromFRED(ctx, entry.symbol, since)
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
