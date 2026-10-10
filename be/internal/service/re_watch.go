package service

import (
	"math"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

// WatchTradeSummary — 관심 단지 한 곳의 최근 거래 요약(2026-10-03 R3).
type WatchTradeSummary struct {
	Recent30 int                 `json:"recent30"`
	Last     *repository.ReTrade `json:"last,omitempty"`
	// NewHigh — 마지막 거래가 같은 평형(전용 정수 ㎡)의 이전 거래(3년)보다 비쌌다.
	NewHigh bool `json:"newHigh"`
}

/*
SummarizeWatchTrades — 단지별 최근 30일 거래 수 · 마지막 거래 · 신고가 여부. trades는 최신순.
해제된 거래는 세지 않는다. 직거래는 신고가 비교에서 뺀다(가족 간 거래 같은 값이 섞인다).
*/
func SummarizeWatchTrades(trades []repository.ReTrade, now time.Time) map[string]WatchTradeSummary {
	cut := now.AddDate(0, 0, -30).Format("2006-01-02")
	out := map[string]WatchTradeSummary{}
	type peakKey struct {
		key  string
		area int
	}
	// 오래된 것부터 보며 평형별 직전 최고가를 쌓는다 — 마지막 거래 차례에 그 최고가와 비교.
	peak := map[peakKey]int64{}
	lastIdx := map[string]int{}
	for i, t := range trades {
		if t.Cancelled || t.Price == nil {
			continue
		}
		if _, ok := lastIdx[t.ComplexKey]; !ok {
			lastIdx[t.ComplexKey] = i
		}
	}
	for i := len(trades) - 1; i >= 0; i-- {
		t := trades[i]
		if t.Cancelled || t.Price == nil {
			continue
		}
		s := out[t.ComplexKey]
		if t.DealDate >= cut {
			s.Recent30++
		}
		pk := peakKey{t.ComplexKey, int(math.Floor(t.Area + 1e-9))}
		prev, seen := peak[pk]
		if lastIdx[t.ComplexKey] == i {
			tt := t
			s.Last = &tt
			s.NewHigh = !t.Direct && seen && *t.Price > prev
		}
		if !t.Direct && *t.Price > prev {
			peak[pk] = *t.Price
		}
		out[t.ComplexKey] = s
	}
	return out
}
