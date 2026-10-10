package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

// 넘겨받은 봉이 1년보다 길어도 52주 밖의 값은 범위에 들지 않는다(SK하이닉스 162,700원 사례).
func TestWeek52RangeFromBars_ignoresBarsOlderThan52Weeks(t *testing.T) {
	bars := []model.Candle{
		{Time: "2026-09-23T00:00:00+09:00", High: 1910000, Low: 1836000},
		{Time: "2026-06-23T00:00:00+09:00", High: 3002000, Low: 2536000},
		{Time: "2025-10-01T00:00:00+09:00", High: 360000, Low: 335500},
		{Time: "2025-03-10T00:00:00+09:00", High: 170000, Low: 162700}, // 1년 반 전
	}
	high, low, ok := week52RangeFromBars(bars)
	if !ok || high != 3002000 || low != 335500 {
		t.Fatalf("got high=%v low=%v ok=%v, want 3002000/335500", high, low, ok)
	}
}
