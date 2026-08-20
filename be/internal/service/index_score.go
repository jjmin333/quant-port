package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// Index scoring.
//
// An index has no balance sheet, so the four statement-derived axes simply do
// not exist for it. Rather than pretend otherwise, the index score is
// technical-only and labelled as such — the same treatment ETFs get in the
// stock batch.
//
// It is also NOT cross-sectionally ranked: twelve indices spanning equities,
// FX, crypto and gold are not peers, and ranking KOSPI against bitcoin would
// produce a number with no meaning. The absolute indicator curves are used
// directly, which is honest for a series read on its own terms.

// IndexScoreService computes daily scores and a rule-based summary for each
// tracked index.
type IndexScoreService struct {
	db      *sql.DB
	indices *repository.IndexDailyCloseRepo
}

func NewIndexScoreService(db *sql.DB, indices *repository.IndexDailyCloseRepo) *IndexScoreService {
	return &IndexScoreService{db: db, indices: indices}
}

// indexScoreHistory is how many closes to load per index. 260 sessions covers a
// 52-week window with lead-in, matching the stock batch.
const indexScoreHistory = 260

// indexCloseSeries is one index's ordered close history, oldest first.
type indexCloseSeries struct {
	indexID string
	dates   []string
	closes  []float64
}

// loadIndexSeries reads every tracked index's recent closes in one pass.
func (s *IndexScoreService) loadIndexSeries(ctx context.Context) (map[string]*indexCloseSeries, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT index_id, DATE_FORMAT(trade_date, '%Y-%m-%d'), close_value
		  FROM index_daily_closes
		 ORDER BY index_id, trade_date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]*indexCloseSeries{}
	for rows.Next() {
		var id, date string
		var close float64
		if err := rows.Scan(&id, &date, &close); err != nil {
			return nil, err
		}
		if close <= 0 {
			continue
		}
		series, ok := out[id]
		if !ok {
			series = &indexCloseSeries{indexID: id}
			out[id] = series
		}
		series.dates = append(series.dates, date)
		series.closes = append(series.closes, close)
		if len(series.closes) > indexScoreHistory {
			series.dates = series.dates[1:]
			series.closes = series.closes[1:]
		}
	}
	return out, rows.Err()
}

// indexTechnicals builds indicators from a close-only series. Highs and lows
// fall back to the close, which is correct for an index quoted as a single
// level — there is no intraday range stored for it.
func indexTechnicals(closes []float64) *TechnicalIndicators {
	if len(closes) == 0 {
		return nil
	}
	candles := make([]model.Candle, 0, len(closes))
	for _, c := range closes {
		candles = append(candles, model.Candle{Open: c, High: c, Low: c, Close: c})
	}
	return ComputeTechnicalIndicators(candles)
}

// scoreIndexSeries produces the 0–100 reading for one index.
func scoreIndexSeries(ti *TechnicalIndicators, bars int) *int {
	if ti == nil || bars < 15 {
		return nil
	}
	in := &scoreInput{ti: ti, barCount: bars}
	raw, confidence := technicalAxis(in)
	if raw == nil {
		return nil
	}
	// Same shrink rule as the stock batch: a thin history must not yield an
	// extreme reading.
	v := applyShrink(*raw, confidence)
	score := int(clamp01to100(v) + 0.5)
	return &score
}

// indexSummaryText is the rule-based one-paragraph read stored beside the score.
// Written from computed values only — no LLM, so every index gets one.
func indexSummaryText(name string, ti *TechnicalIndicators) string {
	if ti == nil || ti.LastClose == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if ti.Return5D != nil {
		parts = append(parts, fmt.Sprintf("최근 5거래일 %s", signedPercent(*ti.Return5D)))
	}
	if ti.Return20D != nil {
		parts = append(parts, fmt.Sprintf("20거래일 %s", signedPercent(*ti.Return20D)))
	}
	if ti.SMA20 != nil {
		pos := "위"
		if *ti.LastClose < *ti.SMA20 {
			pos = "아래"
		}
		parts = append(parts, fmt.Sprintf("20일선 %s", pos))
	}
	if ti.RSI14 != nil {
		parts = append(parts, fmt.Sprintf("RSI %.1f(%s)", *ti.RSI14, classifyRSI(*ti.RSI14)))
	}
	if ti.PctFromHigh52Week != nil {
		parts = append(parts, fmt.Sprintf("52주 최고 대비 %s", signedPercent(*ti.PctFromHigh52Week)))
	}
	if len(parts) == 0 {
		return ""
	}
	return name + " — " + strings.Join(parts, ", ") + "."
}

// RecomputeAll scores every index for its most recent stored close.
func (s *IndexScoreService) RecomputeAll(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("index score service not configured")
	}
	series, err := s.loadIndexSeries(ctx)
	if err != nil {
		return 0, err
	}

	updated := 0
	for id, sr := range series {
		if len(sr.closes) == 0 {
			continue
		}
		ti := indexTechnicals(sr.closes)
		score := scoreIndexSeries(ti, len(sr.closes))
		summary := indexSummaryText(indexDisplayName(id), ti)
		if score == nil && summary == "" {
			continue
		}

		var grade any
		var scoreVal any
		if score != nil {
			scoreVal = *score
			grade = model.ScoreGrade(*score)
		}
		latest := sr.dates[len(sr.dates)-1]
		res, err := s.db.ExecContext(ctx, `
			UPDATE index_daily_closes
			   SET score = ?, grade = ?, ai_summary = ?
			 WHERE index_id = ? AND trade_date = ?`,
			scoreVal, grade, summary, id, latest)
		if err != nil {
			log.Printf("index score %s: %v", id, err)
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			updated++
		}
	}
	return updated, nil
}

// indexDisplayName resolves the Korean label used in the summary sentence.
func indexDisplayName(id string) string {
	for _, entry := range marketIndexCatalog {
		if entry.id == id {
			return entry.name
		}
	}
	return strings.ToUpper(id)
}


// IndexScoreSummary is one index's latest score and rule-based read, for the
// dashboard cards.
type IndexScoreSummary struct {
	IndexID   string `json:"indexId"`
	TradeDate string `json:"tradeDate"`
	Score     *int   `json:"score"`
	Grade     string `json:"grade,omitempty"`
	AISummary string `json:"aiSummary,omitempty"`
}

// LatestAll returns every index's newest scored row in one query.
//
// One call rather than one per index: the dashboard renders a dozen cards at
// once, and twelve round trips for a dozen small rows is the kind of thing that
// makes an overview feel slow for no reason.
func (s *IndexScoreService) LatestAll(ctx context.Context) ([]IndexScoreSummary, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.index_id, DATE_FORMAT(d.trade_date, '%Y-%m-%d'), d.score, d.grade, d.ai_summary
		  FROM index_daily_closes d
		  JOIN (
		      SELECT index_id, MAX(trade_date) AS newest
		        FROM index_daily_closes
		       WHERE ai_summary <> '' OR score IS NOT NULL
		       GROUP BY index_id
		  ) latest ON latest.index_id = d.index_id AND latest.newest = d.trade_date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]IndexScoreSummary, 0, 12)
	for rows.Next() {
		var (
			item    IndexScoreSummary
			score   sql.NullInt64
			grade   sql.NullString
			summary sql.NullString
		)
		if err := rows.Scan(&item.IndexID, &item.TradeDate, &score, &grade, &summary); err != nil {
			return nil, err
		}
		if score.Valid {
			v := int(score.Int64)
			item.Score = &v
		}
		item.Grade = grade.String
		item.AISummary = summary.String
		out = append(out, item)
	}
	return out, rows.Err()
}
