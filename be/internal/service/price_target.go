package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/textutil"
)

type PriceTargetService struct {
	repo *repository.PriceTargetRepo
	// Optional read-only sources. A target must be savable with the market data
	// stack down, so a missing price costs the snapshot, never the row.
	quotes  *QuotesService
	closes  *repository.StockDailyCloseRepo
	candles *repository.CandleBarRepo
}

func NewPriceTargetService(repo *repository.PriceTargetRepo) *PriceTargetService {
	return &PriceTargetService{repo: repo}
}

func (s *PriceTargetService) SetQuotes(q *QuotesService)                       { s.quotes = q }
func (s *PriceTargetService) SetDailyCloses(c *repository.StockDailyCloseRepo) { s.closes = c }
func (s *PriceTargetService) SetCandles(c *repository.CandleBarRepo)           { s.candles = c }

func (s *PriceTargetService) List(ctx context.Context) ([]model.PriceTarget, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.PriceTarget{}
	}
	s.attachExtremes(ctx, rows)
	return rows, nil
}

// PriceTargetInput is what a client may send. There is no base price on it —
// "what it cost when I set this" is measured, not asserted.
type PriceTargetInput struct {
	Symbol       string
	TargetPrice  float64
	TargetPrice2 float64
	StopPrice    float64
	Currency     string
	Note         string
}

func (s *PriceTargetService) Create(ctx context.Context, in PriceTargetInput) (model.PriceTarget, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.PriceTarget{}, err
	}
	w, err := validatePriceTargetInput(in, true)
	if err != nil {
		return model.PriceTarget{}, err
	}
	s.captureBasePrice(ctx, &w)
	row, err := s.repo.Create(ctx, userID, w)
	if err != nil {
		return row, err
	}
	s.attachExtremes(ctx, []model.PriceTarget{row})
	return row, nil
}

func (s *PriceTargetService) Update(ctx context.Context, id int64, in PriceTargetInput) (model.PriceTarget, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.PriceTarget{}, err
	}
	// Symbol is not editable on update (the row is keyed by it), so it is not
	// required here — the placeholder keeps one validator for both paths.
	w, err := validatePriceTargetInput(in, false)
	if err != nil {
		return model.PriceTarget{}, err
	}
	row, err := s.repo.Update(ctx, userID, id, w)
	if err != nil {
		return row, err
	}
	s.attachExtremes(ctx, []model.PriceTarget{row})
	return row, nil
}

func (s *PriceTargetService) Delete(ctx context.Context, id int64) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, userID, id)
}

// captureBasePrice records what the symbol cost at registration.
func (s *PriceTargetService) captureBasePrice(ctx context.Context, w *repository.PriceTargetWrite) {
	if s.quotes != nil {
		if quote, ok := s.quotes.GetQuote(ctx, w.Symbol); ok && quote.CurrentPrice > 0 {
			w.BasePrice = quote.CurrentPrice
		}
	}
	if s.closes != nil {
		if rows, err := s.closes.List(ctx, w.Symbol, 1); err == nil && len(rows) > 0 {
			if w.BasePrice <= 0 {
				w.BasePrice = rows[0].Close
			}
			if !rows[0].TradeDate.IsZero() {
				w.BaseDate = rows[0].TradeDate.Format("2006-01-02")
			}
		}
	}
	if w.BasePrice <= 0 {
		w.BasePrice = 0
		w.BaseDate = ""
		return
	}
	if w.BaseDate == "" {
		w.BaseDate = time.Now().In(seoulLocation()).Format("2006-01-02")
	}
}

// attachExtremes fills reachedDate / peak / trough from stored daily bars.
//
// These are derived rather than stored so that editing a target re-answers
// "has it been reached" against the new number. A stored reached_at would keep
// answering for the target the user has already replaced.
//
// The registration day itself is excluded: that day's high can predate the
// moment the target was set, which would hand back a level already in the past.
func (s *PriceTargetService) attachExtremes(ctx context.Context, rows []model.PriceTarget) {
	if s.candles == nil {
		return
	}
	for i := range rows {
		t := &rows[i]
		if t.BaseDate == "" {
			continue
		}
		bars, err := s.candles.DailyExtremesBetween(ctx, t.Symbol, t.BaseDate, "")
		if err != nil || len(bars) == 0 {
			continue
		}
		var peak, trough float64
		for _, bar := range bars {
			if bar.Date <= t.BaseDate {
				continue
			}
			if bar.High > peak {
				peak = bar.High
			}
			if trough == 0 || (bar.Low > 0 && bar.Low < trough) {
				trough = bar.Low
			}
			if t.ReachedDate == "" && reachesTarget(t, bar) {
				t.ReachedDate = bar.Date
			}
		}
		if peak > 0 {
			t.PeakPrice = &peak
		}
		if trough > 0 {
			t.TroughPrice = &trough
		}
	}
}

// reachesTarget answers "did this bar touch the target".
//
// Direction comes from the base price, not from a user-declared stance: a
// target below where the stock was when it was set is a downside target, and
// waiting for the high to cross it would never resolve.
func reachesTarget(t *model.PriceTarget, bar repository.DailyExtreme) bool {
	if t.TargetPrice <= 0 {
		return false
	}
	if t.BasePrice == nil || *t.BasePrice <= 0 {
		return bar.High >= t.TargetPrice
	}
	if t.TargetPrice >= *t.BasePrice {
		return bar.High >= t.TargetPrice
	}
	return bar.Low > 0 && bar.Low <= t.TargetPrice
}

func validatePriceTargetInput(in PriceTargetInput, requireSymbol bool) (repository.PriceTargetWrite, error) {
	sym := strings.ToUpper(strings.TrimSpace(in.Symbol))
	if requireSymbol && sym == "" {
		return repository.PriceTargetWrite{}, fmt.Errorf("symbol required")
	}
	if in.TargetPrice <= 0 {
		return repository.PriceTargetWrite{}, fmt.Errorf("target price must be positive")
	}
	if in.TargetPrice2 < 0 || in.StopPrice < 0 {
		return repository.PriceTargetWrite{}, fmt.Errorf("prices must not be negative")
	}
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	if cur == "" {
		cur = "KRW"
	}
	if cur != "KRW" && cur != "USD" {
		return repository.PriceTargetWrite{}, fmt.Errorf("currency must be KRW or USD")
	}
	note := textutil.TruncateRunes(in.Note, 500)
	return repository.PriceTargetWrite{
		Symbol:       sym,
		TargetPrice:  in.TargetPrice,
		TargetPrice2: in.TargetPrice2,
		StopPrice:    in.StopPrice,
		Currency:     cur,
		Note:         note,
	}, nil
}
