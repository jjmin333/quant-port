package service

import "sort"

// Cross-sectional normalization for the 종합 점수.
//
// Absolute thresholds are unusable here. Measured on the live universe
// (docs/STOCK_SCORE_DESIGN.md §2.2) ROE spans -14,957% … +227,162% and EPS YoY
// reaches 2.28e9%. Any "ROE above 15 scores full marks" rule hands a perfect
// score to shell companies with a near-zero denominator.
//
// So every raw metric is turned into a rank percentile inside its peer group,
// after winsorizing the tails. Rank is bounded by construction, immune to the
// magnitude of an outlier (a 2-billion-percent value is simply "first"), and
// self-recalibrates as the universe changes.

// metricSample is one symbol's raw value for a single metric.
type metricSample struct {
	symbol string
	value  float64
}

// winsorizeBounds returns the p1/p99 cut points for a sorted slice. Small
// samples get no clipping: with a handful of points the "outlier" may be the
// only real signal, and clipping would erase it.
func winsorizeBounds(sorted []float64) (lo, hi float64) {
	n := len(sorted)
	if n == 0 {
		return 0, 0
	}
	if n < 20 {
		return sorted[0], sorted[n-1]
	}
	loIdx := int(float64(n-1) * 0.01)
	hiIdx := int(float64(n-1) * 0.99)
	return sorted[loIdx], sorted[hiIdx]
}

// percentileRanks maps each sample to 0..100 by rank within the group.
//
// Ties share the average rank of their run, so a group where most symbols hold
// the same value (common for debt_ratio = 0) does not hand an arbitrary
// advantage to whichever symbol the sort happened to place first.
//
// `higherIsBetter=false` inverts the result for metrics where a low reading is
// the good one (PER, PBR, PSR, 부채비율).
func percentileRanks(samples []metricSample, higherIsBetter bool) map[string]float64 {
	out := make(map[string]float64, len(samples))
	n := len(samples)
	if n == 0 {
		return out
	}
	if n == 1 {
		// A lone symbol has nothing to be ranked against; neutral is the only
		// honest answer.
		out[samples[0].symbol] = 50
		return out
	}

	values := make([]float64, n)
	for i, s := range samples {
		values[i] = s.value
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	lo, hi := winsorizeBounds(sorted)

	clipped := make([]metricSample, n)
	for i, s := range samples {
		v := s.value
		if v < lo {
			v = lo
		}
		if v > hi {
			v = hi
		}
		clipped[i] = metricSample{symbol: s.symbol, value: v}
	}

	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return clipped[order[a]].value < clipped[order[b]].value
	})

	// Average ranks across tie runs.
	i := 0
	for i < n {
		j := i + 1
		for j < n && clipped[order[j]].value == clipped[order[i]].value {
			j++
		}
		avgRank := float64(i+j-1) / 2.0
		pct := avgRank / float64(n-1) * 100
		if !higherIsBetter {
			pct = 100 - pct
		}
		for k := i; k < j; k++ {
			out[clipped[order[k]].symbol] = pct
		}
		i = j
	}
	return out
}

// axisScore averages the available normalized metrics for one axis. Missing
// metrics are skipped rather than counted as zero — the project-wide rule that
// absent data must never read as a bad signal.
func axisScore(parts []float64) *float64 {
	if len(parts) == 0 {
		return nil
	}
	sum := 0.0
	for _, p := range parts {
		sum += p
	}
	v := sum / float64(len(parts))
	return &v
}

// growthRate guards the negative-base trap: EPS going -10 → +1 is not "+110%
// growth", it is a turnaround whose percentage is meaningless. Measured on the
// live universe this path is exactly what produced EPS YoY = 2.28e9%.
//
// Returns ok=false when the reported growth cannot be trusted, so the caller
// drops the metric instead of feeding a nonsense number into the ranking.
func growthRate(reported *float64) (float64, bool) {
	if reported == nil {
		return 0, false
	}
	v := *reported
	// A swing beyond ±1000% is, in this dataset, always a base-effect artifact
	// rather than real operating growth.
	if v > 1000 || v < -1000 {
		return 0, false
	}
	return v, true
}

// shrinkFactor pulls a score toward neutral when little data backed it.
//
// Renormalizing weights alone is not enough: a symbol with only RSI available
// would otherwise reach 90 on the strength of one indicator. Shrinking by
// coverage makes an extreme score structurally impossible without broad data.
func shrinkFactor(coverage, shrinkFull float64) float64 {
	if shrinkFull <= 0 {
		return 1
	}
	f := coverage / shrinkFull
	if f > 1 {
		return 1
	}
	if f < 0 {
		return 0
	}
	return f
}

// applyShrink maps a raw 0..100 score toward the neutral 50 by `factor`.
func applyShrink(raw, factor float64) float64 {
	return 50 + (raw-50)*factor
}

// capTercile splits a peer market into 3 size buckets. With no sector data in
// the schema (verified: no sector/industry column exists anywhere), market cap
// is the most effective peer refinement available today — large caps and
// micro caps have entirely different PBR/부채비율 distributions.
func capTercile(sortedCaps []float64, cap float64) int {
	n := len(sortedCaps)
	if n < 30 || cap <= 0 {
		return 0
	}
	t1 := sortedCaps[n/3]
	t2 := sortedCaps[2*n/3]
	switch {
	case cap <= t1:
		return 1
	case cap <= t2:
		return 2
	default:
		return 3
	}
}
