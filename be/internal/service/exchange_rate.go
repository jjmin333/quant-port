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
	fxRedisKey = "market:fx:usd-krw:v1"
	/*
		환율 갱신 주기 — **15초에서 60초로** (2026-09-11 사용자 요청 7).

		15초는 하루 5,760호출이다. 그 값을 치르고 무엇을 사는지 실측했다 —
		60초 간격 세 표본이 1342.85 → 1343.00 → 1342.92, 즉 **분당 약 0.011%**다.
		화면에서 환율이 쓰이는 자리는 달러 자산의 원화 환산인데, 그 정밀도는 총액
		1천만 원에 1,200원 차이다. 15초와 60초의 차이는 **그것의 4분의 1**이라
		어느 화면에서도 보이지 않는다.

		`MARKET_INFO` 버킷은 시장 달력과 공유한다. 달력은 하루 몇 번이라 지금은
		여유가 있지만, 이 폴러가 그 창의 거의 전부를 쓰고 있었다는 사실 자체가
		위험이다 — 달력 조회가 한 번 늘면 곧바로 경합이 된다.
	*/
	fxPollInterval = time.Minute
	/*
		두 시장이 모두 닫혀 있을 때의 주기.

		글로벌 FX는 주중에 계속 움직이지만, **그 시간에 이 값을 보는 사람이 없다.**
		국내도 해외도 닫힌 구간은 KST 06:00~08:00과 주말이고, 그때 필요한 정밀도는
		「어제 환율이 대략 얼마였나」다. 10분이면 충분하고, 열리는 순간 위의 1분
		주기로 돌아온다.
	*/
	fxPollIntervalClosed = 10 * time.Minute
	fxRedisTTL           = 48 * time.Hour
	fxRateLimitBackoff   = 60 * time.Second
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
	// anyMarketLive는 세션 판정을 가진 쪽에서 배선한다(SetSessionProbe 머리글).
	anyMarketLive func(context.Context) bool
}

func NewExchangeRateService(tossClient *toss.Client, redisClient *redis.Client, fxRepo *repository.FxRateRepo) *ExchangeRateService {
	return &ExchangeRateService{toss: tossClient, redis: redisClient, fxRepo: fxRepo}
}

/*
SetSessionProbe — 「지금 어느 시장이든 열려 있나」를 묻는 자리.

주기를 세션에 맞추려면 이 답이 필요한데, 세션을 판정하는 쪽은 거래소 달력을 가진
`QuotesService`다. 여기서 시간을 다시 계산하지 않는 이유는 `lib/market-session`에
적힌 그대로다 — 두 번째 진실을 만들면 안 된다.

배선하지 않으면 **언제나 1분 주기**다(예전 15초보다는 느리고, 세션을 모를 때
10분으로 늦추지는 않는다). 모를 때 느려지는 것은 보이는 손해이고, 모를 때 조금 더
자주 받는 것은 안 보이는 비용이다.
*/
func (s *ExchangeRateService) SetSessionProbe(anyMarketLive func(context.Context) bool) {
	if s != nil {
		s.anyMarketLive = anyMarketLive
	}
}

// pollInterval picks the cadence for the next tick. See fxPollInterval.
func (s *ExchangeRateService) pollInterval(ctx context.Context) time.Duration {
	if s == nil || s.anyMarketLive == nil {
		return fxPollInterval
	}
	if s.anyMarketLive(ctx) {
		return fxPollInterval
	}
	return fxPollIntervalClosed
}

/*
Run — 갱신 루프.

**고정 티커가 아니라 매 회차 주기를 다시 고른다.** 세션 경계(09:00 · 15:30 ·
22:30/23:30)를 넘을 때 티커를 갈아 끼워야 하는데, `time.Ticker`의 주기는 바꿀 수
없으므로 타이머를 매번 새로 건다. 회차가 분 단위라 그 비용은 없다.
*/
func (s *ExchangeRateService) Run(ctx context.Context) {
	s.loadFromStorage(ctx)
	s.refresh(ctx)

	for {
		timer := time.NewTimer(s.pollInterval(ctx))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
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
