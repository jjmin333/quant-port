package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

const (
	journalTitleMaxLen = 200
	journalBodyMaxLen  = 10_000
	journalTagMaxLen   = 20
	journalMaxTags     = 6
)

type JournalService struct {
	repo *repository.JournalNoteRepo
	// Read-only price sources for the write-time snapshot. Both are optional:
	// a note must be savable even when the market data stack is down, so a
	// missing price costs the snapshot, never the note.
	quotes *QuotesService
	closes *repository.StockDailyCloseRepo
}

func NewJournalService(repo *repository.JournalNoteRepo) *JournalService {
	return &JournalService{repo: repo}
}

func (s *JournalService) SetQuotes(q *QuotesService)                       { s.quotes = q }
func (s *JournalService) SetDailyCloses(c *repository.StockDailyCloseRepo) { s.closes = c }

func (s *JournalService) List(ctx context.Context, symbol, topic, q string) ([]model.JournalNote, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	q = strings.TrimSpace(q)
	notes, err := s.repo.List(ctx, userID, symbol, model.TopicFilter(topic), q)
	if err != nil {
		return nil, err
	}
	return notes, nil
}

func (s *JournalService) Create(ctx context.Context, in JournalNoteInput) (model.JournalNote, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.JournalNote{}, err
	}
	w, err := validateJournalFields(in)
	if err != nil {
		return model.JournalNote{}, err
	}
	// Only Create captures the price. Once written it is never revisited —
	// see repository.JournalNoteRepo.Update.
	s.capturePrice(ctx, &w)
	return s.repo.Create(ctx, userID, w)
}

func (s *JournalService) Update(ctx context.Context, id int64, in JournalNoteInput) (model.JournalNote, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.JournalNote{}, err
	}
	w, err := validateJournalFields(in)
	if err != nil {
		return model.JournalNote{}, err
	}
	return s.repo.Update(ctx, userID, id, w)
}

func (s *JournalService) Delete(ctx context.Context, id int64) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, userID, id)
}

// JournalNoteInput is what a client may send. Price fields are absent by
// design: the author does not get to state what the stock was worth.
type JournalNoteInput struct {
	Title      string
	Topic      string // 빈 값: 새 메모는 주식, 수정은 그대로
	Body       string
	Symbol     string
	SymbolName string
	Tags       []string
	Pinned     bool
}

// capturePrice fills the write-time snapshot, live quote first and the stored
// daily close second.
//
// The close is not merely a fallback: outside trading hours the live quote is
// the previous session's number anyway, and the stored close carries the
// trading day it belongs to. A note written at 21:00 must not be stamped with
// tomorrow's date.
func (s *JournalService) capturePrice(ctx context.Context, w *repository.JournalNoteWrite) {
	if w.Symbol == "" {
		return
	}
	if s.quotes != nil {
		if quote, ok := s.quotes.GetQuote(ctx, w.Symbol); ok && quote.CurrentPrice > 0 {
			w.PriceAtWrite = quote.CurrentPrice
			w.PriceCurrency = string(quote.Currency)
			if quote.Name != "" && w.SymbolName == "" {
				w.SymbolName = quote.Name
			}
		}
	}
	if s.closes != nil {
		if rows, err := s.closes.List(ctx, w.Symbol, 1); err == nil && len(rows) > 0 {
			if w.PriceAtWrite <= 0 {
				w.PriceAtWrite = rows[0].Close
				if w.PriceCurrency == "" {
					w.PriceCurrency = rows[0].Currency
				}
			}
			if !rows[0].TradeDate.IsZero() {
				w.PriceDate = rows[0].TradeDate.Format("2006-01-02")
			}
		}
	}
	if w.PriceAtWrite <= 0 {
		// Nothing usable — leave every price field empty rather than record a
		// zero that the UI would have to special-case as "0원".
		*w = clearPrice(*w)
		return
	}
	if w.PriceCurrency == "" {
		w.PriceCurrency = string(inferJournalCurrency(w.Symbol))
	}
	if w.PriceDate == "" {
		w.PriceDate = time.Now().In(seoulLocation()).Format("2006-01-02")
	}
}

func clearPrice(w repository.JournalNoteWrite) repository.JournalNoteWrite {
	w.PriceAtWrite = 0
	w.PriceCurrency = ""
	w.PriceDate = ""
	return w
}

func inferJournalCurrency(symbol string) model.QuoteCurrency {
	if isSixDigitCode(symbol) {
		return model.CurrencyKRW
	}
	return model.CurrencyUSD
}

func isSixDigitCode(symbol string) bool {
	if len(symbol) != 6 {
		return false
	}
	for _, r := range symbol {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validateJournalFields(in JournalNoteInput) (repository.JournalNoteWrite, error) {
	if in.Topic != "" {
		if _, ok := model.ParseTopic(in.Topic); !ok {
			return repository.JournalNoteWrite{}, invalidInput("알 수 없는 주제입니다")
		}
	}
	w := repository.JournalNoteWrite{
		Title:      strings.TrimSpace(in.Title),
		Topic:      in.Topic,
		Body:       strings.TrimSpace(in.Body),
		Symbol:     strings.ToUpper(strings.TrimSpace(in.Symbol)),
		SymbolName: strings.TrimSpace(in.SymbolName),
		Tags:       normalizeJournalTags(in.Tags),
		Pinned:     in.Pinned,
	}
	if w.Symbol == "" {
		w.SymbolName = ""
	}
	if w.Title == "" {
		w.Title = "무제"
	}
	if w.Body == "" {
		return repository.JournalNoteWrite{}, fmt.Errorf("body required")
	}
	if titleRunes := []rune(w.Title); len(titleRunes) > journalTitleMaxLen {
		w.Title = string(titleRunes[:journalTitleMaxLen])
	}
	if len([]rune(w.Body)) > journalBodyMaxLen {
		return repository.JournalNoteWrite{}, fmt.Errorf("body too long (max %d chars)", journalBodyMaxLen)
	}
	return w, nil
}

// normalizeJournalTags trims, drops blanks and case-insensitive duplicates, and
// caps both tag length and count. Deduping matters more than it looks: a filter
// list built from user tags turns "관찰" and "관찰 " into two separate rows.
func normalizeJournalTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if runes := []rune(tag); len(runes) > journalTagMaxLen {
			tag = string(runes[:journalTagMaxLen])
		}
		key := strings.ToLower(tag)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
		if len(out) >= journalMaxTags {
			break
		}
	}
	return out
}
