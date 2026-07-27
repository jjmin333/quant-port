package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

const snapshotMinInterval = 5 * time.Minute

func (s *PortfolioService) recordSnapshot(ctx context.Context, userID int64, summary *model.PortfolioSummary) {
	if s.snapshots == nil || summary == nil || userID <= 0 {
		return
	}
	_ = s.snapshots.InsertIfDue(ctx, repository.PortfolioSnapshot{
		UserID:         userID,
		RecordedAt:     time.Now(),
		TotalAssetsKRW: summary.TotalAssetsKRW,
		TotalCashKRW:   summary.TotalCashKRW,
		TotalProfitKRW: summary.TotalProfitKRW,
		ExchangeRate:   summary.ExchangeRate,
	}, snapshotMinInterval)
}

func (s *PortfolioService) GetHistory(ctx context.Context, rangeKey string, currentTotal float64) (*model.PortfolioHistory, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if s.snapshots == nil {
		return fallbackHistory(rangeKey, currentTotal), nil
	}

	now := time.Now()
	since, bucket := historyWindow(rangeKey, now)

	points, err := s.snapshots.ListSince(ctx, userID, since)
	if err != nil {
		return nil, err
	}

	dataSince, _ := s.snapshots.Earliest(ctx, userID)

	out := downsampleSnapshots(points, bucket, now.Location())
	if len(out) == 0 && currentTotal > 0 {
		out = []model.PortfolioHistoryPoint{{
			At:    now.Format(time.RFC3339),
			Label: "현재",
			Value: currentTotal,
		}}
	}

	h := &model.PortfolioHistory{
		Range:  rangeKey,
		Points: out,
	}
	if !dataSince.IsZero() {
		h.DataSince = dataSince.Format(time.RFC3339)
	}
	return h, nil
}

func historyWindow(rangeKey string, now time.Time) (since time.Time, bucket time.Duration) {
	switch rangeKey {
	case "1d":
		return now.Add(-24 * time.Hour), 15 * time.Minute
	case "1w":
		return now.Add(-7 * 24 * time.Hour), time.Hour
	case "1y":
		return now.Add(-365 * 24 * time.Hour), 7 * 24 * time.Hour
	default: // 1m
		return now.Add(-30 * 24 * time.Hour), 24 * time.Hour
	}
}

func downsampleSnapshots(points []repository.PortfolioSnapshot, bucket time.Duration, loc *time.Location) []model.PortfolioHistoryPoint {
	if len(points) == 0 {
		return nil
	}

	type bucketPoint struct {
		at    time.Time
		value float64
	}
	buckets := map[int64]bucketPoint{}

	for _, p := range points {
		t := p.RecordedAt.In(loc)
		key := t.Unix() / int64(bucket.Seconds())
		if bucket >= 24*time.Hour {
			key = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).Unix()
		}
		buckets[key] = bucketPoint{at: t, value: p.TotalAssetsKRW}
	}

	keys := make([]int64, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	out := make([]model.PortfolioHistoryPoint, 0, len(keys))
	for _, k := range keys {
		bp := buckets[k]
		out = append(out, model.PortfolioHistoryPoint{
			At:    bp.at.Format(time.RFC3339),
			Label: formatHistoryLabel(bp.at, bucket),
			Value: math.Round(bp.value),
		})
	}
	return out
}

func formatHistoryLabel(t time.Time, bucket time.Duration) string {
	switch {
	case bucket <= 15*time.Minute:
		return t.Format("15:04")
	case bucket <= time.Hour:
		return t.Format("1/2 15:04")
	case bucket <= 24*time.Hour:
		return t.Format("1/2")
	default:
		return t.Format("1/2")
	}
}

func fallbackHistory(rangeKey string, currentTotal float64) *model.PortfolioHistory {
	points := []model.PortfolioHistoryPoint{}
	if currentTotal > 0 {
		points = append(points, model.PortfolioHistoryPoint{
			At:    time.Now().Format(time.RFC3339),
			Label: "현재",
			Value: currentTotal,
		})
	}
	return &model.PortfolioHistory{Range: rangeKey, Points: points}
}

func normalizeHistoryRange(raw string) string {
	switch raw {
	case "1d", "day", "일":
		return "1d"
	case "1w", "week", "주":
		return "1w"
	case "1y", "year", "년":
		return "1y"
	default:
		return "1m"
	}
}

func ParseHistoryRange(raw string) (string, error) {
	r := normalizeHistoryRange(raw)
	if r == "" {
		return "1m", fmt.Errorf("invalid range")
	}
	return r, nil
}
