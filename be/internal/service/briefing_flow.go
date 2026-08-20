package service

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

const (
	briefingFlowCacheKey = "briefing:investor-flow:v1:KOSPI"
	briefingFlowCacheTTL = 5 * time.Minute
)

// briefingFlowDTO is the compact 외국인·기관 순매수 line (억원). Domestic
// briefings only — the US call must not hit Toss for this.
type briefingFlowDTO struct {
	D      string `json:"d"`
	FrNet  int    `json:"frNet"`
	InsNet int    `json:"insNet"`
	P      bool   `json:"p,omitempty"` // 장중 잠정
}

func (s *NewsBriefingService) briefingInvestorFlow(ctx context.Context) *briefingFlowDTO {
	if s == nil || s.toss == nil {
		return nil
	}
	if cached := s.readFlowCache(ctx); cached != nil {
		return cached
	}
	resp, err := s.toss.GetInvestorTrading(ctx, toss.GetInvestorTradingParams{
		Symbol:   "KOSPI",
		Interval: toss.InvestorInterval1d,
		Count:    1,
	})
	if err != nil {
		log.Printf("news briefing flow: toss: %v", err)
		return s.readFlowCache(ctx)
	}
	if resp == nil || len(resp.Records) == 0 {
		return nil
	}
	rec := resp.Records[0]
	dto := &briefingFlowDTO{
		D:      shortBriefingDate(rec.Date),
		FrNet:  netEok(rec.Foreigner.BuyAmount, rec.Foreigner.SellAmount),
		InsNet: netEok(rec.Institution.BuyAmount, rec.Institution.SellAmount),
		P:      flowIsPreliminary(rec.Date),
	}
	s.writeFlowCache(ctx, dto)
	return dto
}

func flowIsPreliminary(date string) bool {
	now := time.Now().In(seoulLocation())
	if date != now.Format("2006-01-02") {
		return false
	}
	return calendar.InRegularSession(briefingScheduleFor("domestic"), now)
}

func netEok(buy, sell string) int {
	b, _ := strconv.ParseFloat(strings.TrimSpace(buy), 64)
	s, _ := strconv.ParseFloat(strings.TrimSpace(sell), 64)
	return int(math.Round((b - s) / 1e8))
}

func (s *NewsBriefingService) readFlowCache(ctx context.Context) *briefingFlowDTO {
	if s.redis == nil {
		return nil
	}
	raw, err := s.redis.Get(ctx, briefingFlowCacheKey).Bytes()
	if err != nil {
		if err != redis.Nil {
			log.Printf("news briefing flow cache get: %v", err)
		}
		return nil
	}
	var dto briefingFlowDTO
	if json.Unmarshal(raw, &dto) != nil {
		return nil
	}
	return &dto
}

func (s *NewsBriefingService) writeFlowCache(ctx context.Context, dto *briefingFlowDTO) {
	if s.redis == nil || dto == nil {
		return
	}
	raw, err := json.Marshal(dto)
	if err != nil {
		return
	}
	if err := s.redis.Set(ctx, briefingFlowCacheKey, raw, briefingFlowCacheTTL).Err(); err != nil {
		log.Printf("news briefing flow cache set: %v", err)
	}
}
