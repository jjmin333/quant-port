package service

import "testing"

// 일봉 한 번으로 등락·흐름선을 만든다(2026-10-07) — 최신이 먼저 온다.
func TestChangeAndSparklineFromCloses(t *testing.T) {
	closes := []float64{3.30, 3.25, 3.20} // 오늘 3.30, 어제 3.25
	ch, cr, ok := changeFromCloses(closes)
	if !ok || ch < 0.0499 || ch > 0.0501 || cr < 1.53 || cr > 1.54 {
		t.Fatalf("change %v rate %v ok %v", ch, cr, ok)
	}
	sp := sparklineFromCloses(closes)
	if len(sp) != 3 || sp[0] != 3.20 || sp[2] != 3.30 {
		t.Fatalf("sparkline should be oldest-first: %v", sp)
	}
	if _, _, ok := changeFromCloses([]float64{3.3}); ok {
		t.Fatal("one candle has no change")
	}
	if sparklineFromCloses([]float64{3.3}) != nil {
		t.Fatal("one point draws nothing")
	}
}
