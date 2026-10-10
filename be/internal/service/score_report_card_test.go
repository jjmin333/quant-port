package service

import (
	"database/sql"
	"fmt"
	"math"
	"testing"

	"github.com/enact/quant-port/be/internal/repository"
)

func TestSpearmanPerfectAndReversed(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	if got := spearman(xs, []float64{10, 20, 30, 40, 50}); math.Abs(got-1) > 1e-9 {
		t.Fatalf("perfect = %v", got)
	}
	if got := spearman(xs, []float64{5, 4, 3, 2, 1}); math.Abs(got+1) > 1e-9 {
		t.Fatalf("reversed = %v", got)
	}
}

// 동점은 평균 순위를 나눠 갖는다 — 점수가 정수라 동점이 흔하다.
func TestAverageRanksTies(t *testing.T) {
	got := averageRanks([]float64{10, 20, 20, 30})
	want := []float64{1, 2.5, 2.5, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ranks = %v, want %v", got, want)
		}
	}
}

func TestTopNStats(t *testing.T) {
	xs := []float64{90, 80, 10, 20}
	ys := []float64{5, 3, -1, 1}
	excess, hit := topNStats(xs, ys, 2)
	// top 2 mean 4, all mean 2 → +2; median(sorted[2]) = 3 → only 5 beats it.
	if math.Abs(excess-2) > 1e-9 || hit != 0.5 {
		t.Fatalf("excess %v hit %v", excess, hit)
	}
}

// 짧은 표본은 의미 있다고 말하지 않는다.
func TestSummarizeAxisNotSignificantWhenShort(t *testing.T) {
	s := summarizeAxis("composite", nil)
	if s.Significant || s.MeanIC != nil || s.Days != 0 {
		t.Fatalf("empty axis should say nothing: %+v", s)
	}
}

// 성적표의 마지막 방식 변경일은 점수 배치의 산출 방식 판과 같아야 한다.
func TestMethodChangelogMatchesVersion(t *testing.T) {
	last := scoreMethodChanges[len(scoreMethodChanges)-1].Date
	if last != scoreMethodVersion {
		t.Fatalf("scoreMethodChanges ends at %s but scoreMethodVersion is %s — 둘을 함께 고쳐라", last, scoreMethodVersion)
	}
}

// 겹치는 5일 창을 보정한 t는 날짜 수로 낸 t보다 √5배 작아야 한다.
func TestSummarizeAxisOverlapAdjustedT(t *testing.T) {
	var rows []repository.ScoreICRow
	for i, v := range []float64{0.05, 0.02, 0.04, 0.01, 0.03, 0.06, 0.02, 0.05, 0.03, 0.04} {
		rows = append(rows, repository.ScoreICRow{TradeDate: fmt.Sprintf("2026-09-%02d", i+1), IC: sql.NullFloat64{Float64: v, Valid: true}})
	}
	s := summarizeAxis("composite", rows)
	mean, sd := meanSD([]float64{0.05, 0.02, 0.04, 0.01, 0.03, 0.06, 0.02, 0.05, 0.03, 0.04})
	naive := mean / (sd / math.Sqrt(10))
	if s.TStat == nil || math.Abs(*s.TStat-naive/math.Sqrt(5)) > 1e-3 {
		t.Fatalf("adjusted t = %v, want %.4f", s.TStat, naive/math.Sqrt(5))
	}
}
