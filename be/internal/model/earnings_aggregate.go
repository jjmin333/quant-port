package model

import "sort"

// ComputeHitHistory summarizes a symbol's announcements (newest first).
//
// `limit`은 볼 분기 수. 0이면 전부.
func ComputeHitHistory(events []EarningsEvent, limit int) *EarningsHitHistory {
	pcts := make([]float64, 0, len(events))
	out := &EarningsHitHistory{}
	for _, ev := range events {
		if ev.Surprise == nil || ev.Surprise.Pct == nil {
			continue
		}
		out.Quarters++
		if *ev.Surprise.Pct >= 0 {
			out.Beats++
		} else {
			out.Misses++
		}
		pcts = append(pcts, *ev.Surprise.Pct)
		if limit > 0 && out.Quarters >= limit {
			break
		}
	}
	if out.Quarters == 0 {
		return nil
	}
	avg, median := meanAndMedian(pcts)
	out.AvgPct, out.MedianPct = &avg, &median
	return out
}

// meanAndMedian returns both, because either alone misleads here.
//
// 평균은 +6232% 한 건에 끌려가고(실측 ATN 인터내셔널), 중위만 주면 그 극단이
// 존재한다는 사실이 화면에서 사라진다. 둘을 나란히 두면 분포가 읽힌다.
func meanAndMedian(values []float64) (mean, median float64) {
	if len(values) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean = sum / float64(len(values))

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		median = sorted[mid]
	} else {
		median = (sorted[mid-1] + sorted[mid]) / 2
	}
	return mean, median
}

// BuildEarningsStats aggregates a window into the 상단 스코어보드.
//
// `material` decides which rows count toward the surprise statistics — the same
// 유의성 하한을 랭킹과 공유한다. 하한 없이 평균을 내면 컨센서스가 1억원인 종목
// 하나가 시장 전체의 「평균 서프라이즈」를 만든다.
func BuildEarningsStats(events []EarningsEvent, material func(EarningsSurprise) bool) EarningsStats {
	stats := EarningsStats{}
	pcts := make([]float64, 0, len(events))

	for _, ev := range events {
		stats.Total++
		switch ev.DateStatus {
		case EarningsDateEstimated:
			stats.Estimated++
		case EarningsDateScheduled:
			stats.Scheduled++
		default:
			stats.Confirmed++
		}

		s := ev.Surprise
		if s == nil || s.Pct == nil {
			continue
		}
		if material != nil && !material(*s) {
			continue
		}
		pct := *s.Pct
		stats.Compared++
		if pct >= 0 {
			stats.Beats++
		} else {
			stats.Misses++
		}
		pcts = append(pcts, pct)

		pick := &EarningsStatsPick{Symbol: ev.Symbol, Name: ev.Name, Pct: pct}
		if pick.Name == "" {
			pick.Name = ev.Symbol
		}
		if stats.Best == nil || pct > stats.Best.Pct {
			stats.Best = pick
		}
		if stats.Worst == nil || pct < stats.Worst.Pct {
			stats.Worst = pick
		}
	}

	if stats.Compared > 0 {
		rate := float64(stats.Beats) / float64(stats.Compared) * 100
		stats.BeatRate = &rate
		avg, median := meanAndMedian(pcts)
		stats.AvgPct, stats.MedianPct = &avg, &median
	}
	return stats
}
