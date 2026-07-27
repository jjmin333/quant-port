package service

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

const (
	fxRedisKey         = "market:fx:usd-krw:v1"
	fxPollInterval     = 15 * time.Second
	fxRedisTTL         = 48 * time.Hour
	fxRateLimitBackoff = 60 * time.Second
)

type FxSnapshot struct {
	MidRate   float64   `json:"midRate"`
	BuyRate   float64   `json:"buyRate"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ExchangeRateService struct {
	toss   *toss.Client
	redis  *redis.Client
	fxRepo *repository.FxRateRepo

	mu           sync.RWMutex
	cached       FxSnapshot
	backoffUntil time.Time
}

func NewExchangeRateService(tossClient *toss.Client, redisClient *redis.Client, fxRepo *repository.FxRateRepo) *ExchangeRateService {
	return &ExchangeRateService{toss: tossClient, redis: redisClient, fxRepo: fxRepo}
}

func (s *ExchangeRateService) Run(ctx context.Context) {
	s.loadFromStorage(ctx)
	s.refresh(ctx)

	ticker := time.NewTicker(fxPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(ctx)
		}
	}
}

func (s *ExchangeRateService) loadFromStorage(ctx context.Context) {
	if s.redis != nil {
		if raw, err := s.redis.Get(ctx, fxRedisKey).Bytes(); err == nil && len(raw) > 0 {
			var snap FxSnapshot
			if json.Unmarshal(raw, &snap) == nil && snap.MidRate > 0 {
				s.mu.Lock()
				s.cached = snap
				s.mu.Unlock()
				return
			}
		}
	}
	if s.fxRepo != nil {
		rec, err := s.fxRepo.Get(ctx, "USD", "KRW")
		if err == nil && rec != nil && rec.MidRate > 0 {
			snap := FxSnapshot{
				MidRate:   rec.MidRate,
				BuyRate:   rec.BuyRate,
				UpdatedAt: rec.UpdatedAt,
			}
			s.mu.Lock()
			s.cached = snap
			s.mu.Unlock()
			s.persist(ctx, snap)
		}
	}
}

func (s *ExchangeRateService) refresh(ctx context.Context) {
	s.mu.RLock()
	if time.Now().Before(s.backoffUntil) {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	if s.toss == nil || !s.toss.Enabled() {
		return
	}

	rateResp, err := s.toss.GetExchangeRate(ctx, toss.GetExchangeRateParams{
		BaseCurrency:  toss.CurrencyUSD,
		QuoteCurrency: toss.CurrencyKRW,
	})
	if err != nil {
		if isRateLimitError(err) {
			s.mu.Lock()
			s.backoffUntil = time.Now().Add(fxRateLimitBackoff)
			s.mu.Unlock()
			log.Printf("exchange rate poller: rate limited, backing off %s", fxRateLimitBackoff)
		} else {
			log.Printf("exchange rate poller: %v", err)
		}
		return
	}

	mid := parseDecimal(rateResp.MidRate)
	if mid <= 0 {
		mid = parseDecimal(rateResp.Rate)
	}
	buy := parseDecimal(rateResp.Rate)
	if mid <= 0 || buy <= 0 {
		return
	}

	snap := FxSnapshot{
		MidRate:   mid,
		BuyRate:   buy,
		UpdatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	s.cached = snap
	s.backoffUntil = time.Time{}
	s.mu.Unlock()
	s.persist(ctx, snap)
}

func (s *ExchangeRateService) persist(ctx context.Context, snap FxSnapshot) {
	if s.redis != nil {
		if payload, err := json.Marshal(snap); err == nil {
			_ = s.redis.Set(ctx, fxRedisKey, payload, fxRedisTTL).Err()
		}
	}
	if s.fxRepo != nil {
		_ = s.fxRepo.Upsert(ctx, repository.FxRateRecord{
			BaseCurrency:  "USD",
			QuoteCurrency: "KRW",
			MidRate:       snap.MidRate,
			BuyRate:       snap.BuyRate,
			UpdatedAt:     snap.UpdatedAt,
		})
	}
}

func (s *ExchangeRateService) Get(ctx context.Context) (FxSnapshot, bool) {
	s.mu.RLock()
	snap := s.cached
	s.mu.RUnlock()
	if snap.MidRate > 0 {
		return snap, true
	}

	if s.redis != nil {
		if raw, err := s.redis.Get(ctx, fxRedisKey).Bytes(); err == nil && len(raw) > 0 {
			var cached FxSnapshot
			if json.Unmarshal(raw, &cached) == nil && cached.MidRate > 0 {
				s.mu.Lock()
				s.cached = cached
				s.mu.Unlock()
				return cached, true
			}
		}
	}
	if s.fxRepo != nil {
		rec, err := s.fxRepo.Get(ctx, "USD", "KRW")
		if err == nil && rec != nil && rec.MidRate > 0 {
			snap = FxSnapshot{
				MidRate:   rec.MidRate,
				BuyRate:   rec.BuyRate,
				UpdatedAt: rec.UpdatedAt,
			}
			s.mu.Lock()
			s.cached = snap
			s.mu.Unlock()
			return snap, true
		}
	}
	return FxSnapshot{}, false
}

func (s *ExchangeRateService) Ensure(ctx context.Context) (FxSnapshot, bool) {
	if snap, ok := s.Get(ctx); ok {
		return snap, true
	}
	s.refresh(ctx)
	return s.Get(ctx)
}

func (s *ExchangeRateService) MidBuy(ctx context.Context) (mid, buy float64, ok bool) {
	snap, ok := s.Ensure(ctx)
	if !ok {
		return 0, 0, false
	}
	return snap.MidRate, snap.BuyRate, true
}

func (s *ExchangeRateService) ToResponse(ctx context.Context) (*toss.ExchangeRateResponse, bool) {
	snap, ok := s.Ensure(ctx)
	if !ok {
		return nil, false
	}
	return &toss.ExchangeRateResponse{
		BaseCurrency:  toss.CurrencyUSD,
		QuoteCurrency: toss.CurrencyKRW,
		Rate:          strconv.FormatFloat(snap.BuyRate, 'f', 2, 64),
		MidRate:       strconv.FormatFloat(snap.MidRate, 'f', 2, 64),
	}, true
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "한도")
}
