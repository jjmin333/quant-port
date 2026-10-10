package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/redis/go-redis/v9"
)

const (
	goldBaselineKeyFmt  = "quotes:gold:baseline:v1"
	goldSparklineKeyFmt = "quotes:gold:sparkline:v1"
	goldSparklineMax    = 20
)

type goldDayBaseline struct {
	Date string  `json:"date"`
	Open float64 `json:"open"`
}

type goldSparklineState struct {
	Hour   string    `json:"hour"`
	Points []float64 `json:"points"`
}

func enrichGoldQuote(ctx context.Context, redis *redis.Client, q marketdata.Quote) marketdata.Quote {
	if q.Value <= 0 {
		return q
	}
	q.Change, q.ChangeRate = goldDayChange(ctx, redis, q.Value)
	q.Sparkline = goldSparkline(ctx, redis, q.Value)
	return q
}

func goldDayChange(ctx context.Context, redis *redis.Client, price float64) (change, changeRate float64) {
	kst := time.FixedZone("KST", 9*3600)
	today := time.Now().In(kst).Format("2006-01-02")

	var baseline goldDayBaseline
	if redisGetJSON(ctx, redis, goldBaselineKeyFmt, &baseline) && baseline.Date == today && baseline.Open > 0 {
		change = price - baseline.Open
		changeRate = (change / baseline.Open) * 100
		return change, changeRate
	}
	redisSetJSON(ctx, redis, goldBaselineKeyFmt, goldDayBaseline{Date: today, Open: price}, 48*time.Hour)
	return 0, 0
}

func goldSparkline(ctx context.Context, redis *redis.Client, price float64) []float64 {
	kst := time.FixedZone("KST", 9*3600)
	hour := time.Now().In(kst).Format("2006-01-02T15")

	var state goldSparklineState
	redisGetJSON(ctx, redis, goldSparklineKeyFmt, &state)

	shouldAppend := len(state.Points) == 0 ||
		state.Hour != hour ||
		state.Points[len(state.Points)-1] != price
	if shouldAppend {
		state.Hour = hour
		state.Points = append(state.Points, price)
		if len(state.Points) > goldSparklineMax {
			state.Points = state.Points[len(state.Points)-goldSparklineMax:]
		}
		redisSetJSON(ctx, redis, goldSparklineKeyFmt, state, 7*24*time.Hour)
	}

	var baseline goldDayBaseline
	if redisGetJSON(ctx, redis, goldBaselineKeyFmt, &baseline) && baseline.Open > 0 {
		if len(state.Points) == 0 {
			return []float64{baseline.Open, price}
		}
		if state.Points[0] != baseline.Open {
			return append([]float64{baseline.Open}, state.Points...)
		}
		if len(state.Points) == 1 {
			return []float64{baseline.Open, price}
		}
	}
	return state.Points
}

func redisGetJSON(ctx context.Context, redis *redis.Client, key string, dest any) bool {
	if redis == nil {
		return false
	}
	raw, err := redis.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dest) == nil
}

func redisSetJSON(ctx context.Context, redis *redis.Client, key string, value any, ttl time.Duration) {
	if redis == nil {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = redis.Set(ctx, key, payload, ttl).Err()
}
