package service

import (
	"strings"
	"testing"
)

// ramp builds `n` closes rising (or falling) by `pct` each bar from `start`.
func ramp(start float64, n int, pct float64) []float64 {
	out := make([]float64, 0, n)
	v := start
	for i := 0; i < n; i++ {
		out = append(out, v)
		v *= 1 + pct/100
	}
	return out
}

// flat builds `n` closes that wobble by a hair. 짝수 자리를 위로 두어 **마지막
// 봉이 반드시 내림**으로 끝나게 한다 — 그래야 뒤에 붙이는 연속 상승의 길이가
// 붙인 개수와 정확히 같아진다.
func flat(start float64, n int) []float64 {
	out := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			out = append(out, start*1.0002)
		} else {
			out = append(out, start)
		}
	}
	return out
}

func tagsFor(closes []float64) []string {
	return indexTags(indexTechnicals(closes), closes)
}

func hasTag(tags []string, prefix string) bool {
	for _, t := range tags {
		if t == prefix || strings.HasPrefix(t, prefix+":") {
			return true
		}
	}
	return false
}

/*
지표 기반 키워드는 창이 차야 나온다.

예전에는 「10봉이면 아무 키워드도 없다」였다. 지금은 **누적 등락률만** 나온다 — 그 값은
20일선·RSI와 달리 얕은 창에서도 참이고, 빈 칸은 「조용하다」가 아니라 「아직 안 불러왔다」로
읽히기 때문이다(`TestIndexTagsShallowHistoryStillReportsMove`). 창이 필요한 것들은
여전히 나오지 않는다는 것이 여기서 지키는 계약이다.
*/
func TestIndexTagsNeedsHistory(t *testing.T) {
	tags := tagsFor(ramp(100, 10, 1))
	for _, tag := range tags {
		if !strings.HasPrefix(tag, "month:") && !strings.HasPrefix(tag, "week:") {
			t.Fatalf("10 bars must not produce indicator-based tags, got %v", tags)
		}
	}
}

func TestIndexTagsStreak(t *testing.T) {
	// 조용하다가 마지막 나흘만 오른다 — 연속 상승이 잡혀야 한다.
	closes := flat(100, 40)
	last := closes[len(closes)-1]
	for i := 0; i < 4; i++ {
		last *= 1.004
		closes = append(closes, last)
	}
	tags := tagsFor(closes)
	if !hasTag(tags, "streak_up") {
		t.Fatalf("expected streak_up, got %v", tags)
	}
	for _, tag := range tags {
		if tag == "streak_up:4" {
			return
		}
	}
	t.Fatalf("expected a 4-day run, got %v", tags)
}

func TestIndexTagsFlatDayBreaksStreak(t *testing.T) {
	closes := flat(100, 40)
	last := closes[len(closes)-1]
	for i := 0; i < 3; i++ {
		last *= 1.004
		closes = append(closes, last)
	}
	closes = append(closes, last) // 보합으로 끝난다
	if tags := tagsFor(closes); hasTag(tags, "streak_up") {
		t.Fatalf("a flat close must end the run, got %v", tags)
	}
}

func TestIndexTags52WeekHigh(t *testing.T) {
	// 1년 넘게 꾸준히 올랐으면 마지막 종가가 곧 신고가다.
	tags := tagsFor(ramp(100, 300, 0.2))
	if !hasTag(tags, "high_52w") {
		t.Fatalf("expected high_52w, got %v", tags)
	}
}

func TestIndexTagsShortHistorySkipsHighLow(t *testing.T) {
	// 60봉짜리 창에서 「52주 최고」를 말하면 거짓말이 된다.
	tags := tagsFor(ramp(100, 60, 0.2))
	if hasTag(tags, "high_52w") {
		t.Fatalf("52w tag must need a deeper window, got %v", tags)
	}
}

func TestIndexTagsPlunge(t *testing.T) {
	closes := flat(100, 60)
	closes = append(closes, closes[len(closes)-1]*0.94)
	tags := tagsFor(closes)
	if !hasTag(tags, "plunge") {
		t.Fatalf("expected plunge, got %v", tags)
	}
	if hasTag(tags, "surge") {
		t.Fatalf("surge and plunge must not coexist: %v", tags)
	}
}

func TestIndexTagsRangeBoundIsLastResort(t *testing.T) {
	tags := tagsFor(flat(100, 60))
	if len(tags) != 1 || tags[0] != "range_bound" {
		t.Fatalf("a quiet series should yield only range_bound, got %v", tags)
	}
}

func TestIndexTagsRespectsLimit(t *testing.T) {
	// 1년 넘게 매일 오르면 신고가·연속·한 달·과열이 전부 걸린다.
	tags := tagsFor(ramp(100, 300, 0.4))
	if len(tags) > indexTagLimit {
		t.Fatalf("expected at most %d tags, got %v", indexTagLimit, tags)
	}
	// 드문 사건이 먼저 온다.
	if tags[0] != "high_52w" && tags[0] != "cross_golden" {
		t.Fatalf("rarest tag should lead, got %v", tags)
	}
}

func TestIndexTagsSMA20Recover(t *testing.T) {
	// 내리막 끝에 하루 크게 튀어 20일선을 되찾는 모양.
	closes := ramp(100, 60, -0.3)
	closes = append(closes, closes[len(closes)-1]*1.05)
	tags := tagsFor(closes)
	if !hasTag(tags, "sma20_recover") && !hasTag(tags, "surge") {
		t.Fatalf("expected a recovery or surge tag, got %v", tags)
	}
}

func TestIndexTagsEncodeRoundTrip(t *testing.T) {
	in := []string{"high_52w", "streak_up:4"}
	if got := decodeIndexTags(encodeIndexTags(in)); len(got) != 2 || got[1] != "streak_up:4" {
		t.Fatalf("round trip lost data: %v", got)
	}
	if got := decodeIndexTags(""); got != nil {
		t.Fatalf("empty column must decode to nil, got %v", got)
	}
	if got := decodeIndexTags(" , "); got != nil {
		t.Fatalf("blank entries must decode to nil, got %v", got)
	}
}

/*
조용한 지수도 빈손으로 돌려보내지 않는다.

박스권 문턱(20일 고저 폭 3%)을 비껴갈 만큼은 흔들리면서 위 여덟 층위에는 하나도
걸리지 않는 구간이 실제로 넓다 — 실측 08-31에 열세 칸 중 여덟이 그랬다. 빈 칸은
「조용하다」가 아니라 「아직 안 불러왔다」로 읽히므로, 마지막에 누적 등락률을 적는다.
*/
func TestIndexTagsFallsBackToCumulativeMove(t *testing.T) {
	/*
		네 봉짜리 주기로 오르내린다.

		한 층위씩 비껴가도록 고른 모양이다: 20일 고저 폭이 4.9%라 박스권(3%)이 아니고,
		주기가 바뀌므로 연속은 하루를 못 넘기며, 마지막 두 봉이 **둘 다 20일선 위**라
		선을 넘지 않는다. 하루 등락(−0.95%)은 이 계열의 2σ(≈6%)에 한참 못 미쳐 급락도
		아니다. 20거래일이 주기의 배수라 누적은 정확히 0% — 그래서 6번(±5% 이상)에도
		걸리지 않는다.
	*/
	cycle := []float64{100, 101, 105, 104}
	closes := make([]float64, 0, 60)
	for i := 0; i < 60; i++ {
		closes = append(closes, cycle[i%len(cycle)])
	}
	tags := tagsFor(closes)
	if len(tags) != 1 || !strings.HasPrefix(tags[0], "month:") {
		t.Fatalf("a quiet-but-not-boxed series should fall back to month:, got %v", tags)
	}
	// 부호는 값에 붙는다 — `month_up:0` 같은 자기모순을 만들지 않기 위해서다.
	if !strings.ContainsAny(tags[0], "+-") {
		t.Fatalf("month tag must carry a sign, got %q", tags[0])
	}
}

// 사건이 하나라도 있으면 마지막 층위는 열리지 않는다.
func TestIndexTagsFallbackYieldsToRealEvents(t *testing.T) {
	tags := tagsFor(ramp(100, 300, 0.4))
	for _, tag := range tags {
		if strings.HasPrefix(tag, "month:") || strings.HasPrefix(tag, "week:") {
			t.Fatalf("fallback tag must not appear alongside events: %v", tags)
		}
	}
}

/*
이력이 얕은 지수도 칩을 받는다.

30봉 문턱은 20일선·RSI·변동성 계산이 거짓말이 되지 않게 하려고 있는 것이지, 누적
등락률까지 막으라는 뜻이 아니었다. 실측 08-31에 금(16봉)이 열세 칸 중 유일하게 빈
칸이었는데, 빈 칸은 「조용하다」가 아니라 「아직 안 불러왔다」로 읽힌다.
*/
func TestIndexTagsShallowHistoryStillReportsMove(t *testing.T) {
	tags := tagsFor(ramp(100, 16, 0.5))
	if len(tags) != 1 {
		t.Fatalf("a 16-bar series should yield exactly one tag, got %v", tags)
	}
	if !strings.HasPrefix(tags[0], "month:") && !strings.HasPrefix(tags[0], "week:") {
		t.Fatalf("expected a cumulative-move tag, got %v", tags)
	}
	// 얕은 창에서 계산되는 지표 기반 키워드는 여전히 나오지 않는다.
	for _, forbidden := range []string{"high_52w", "range_bound", "overbought", "vol_expand"} {
		if hasTag(tags, forbidden) {
			t.Fatalf("%s must not appear on a 16-bar series: %v", forbidden, tags)
		}
	}
}

// 5거래일조차 없으면 지어내지 않는다.
func TestIndexTagsTooShortYieldsNothing(t *testing.T) {
	if got := tagsFor(ramp(100, 3, 1)); got != nil {
		t.Fatalf("3 bars should yield no tags, got %v", got)
	}
}

/*
사용자 신고 (2026-09-03): 나스닥100 선물·S&P 500 등의 셀에 키워드가 하나도 없었다.

원인은 태그가 비어서가 아니라, 그 지수들의 태그가 `streak_down:3` **하나뿐**이라
FE의 모순 필터(오늘 오르는 중이면 「연속↓」을 지운다)에 통째로 지워졌기 때문이다.
그래서 이제 지워질 수 없는 태그가 하나도 없으면 누적 등락률을 뒤에 붙인다.
*/
func TestIndexTagsAlwaysLeaveOneSurvivingTag(t *testing.T) {
	pct := func(v float64) *float64 { return &v }

	cases := []struct {
		name string
		tags []string
		want bool // 지워질 수 없는 태그가 하나라도 있는가
	}{
		{"연속 하락 하나뿐", []string{"streak_down:3"}, false},
		{"52주 최고 하나뿐", []string{"high_52w"}, false},
		{"당일 주장 + 한 달 누적", []string{"streak_down:3", "month_down:9"}, true},
		{"박스권", []string{"range_bound"}, true},
		{"누적 등락", []string{"month:+4"}, true},
	}
	for _, tc := range cases {
		if got := hasSurvivingIndexTag(tc.tags); got != tc.want {
			t.Errorf("%s: hasSurvivingIndexTag(%v) = %v, want %v", tc.name, tc.tags, got, tc.want)
		}
	}

	// 연속 하락만 걸리는 계열: 마지막에 누적 등락률이 따라붙어야 한다.
	closes := make([]float64, 0, 60)
	for i := 0; i < 57; i++ {
		closes = append(closes, 100)
	}
	closes = append(closes, 99.9, 99.8, 99.7) // 3일 연속 하락, 폭은 작게

	ti := &TechnicalIndicators{LastClose: pct(99.7), Return20D: pct(-0.3)}
	got := indexTags(ti, closes)

	if len(got) == 0 {
		t.Fatal("no tags at all")
	}
	if !hasSurvivingIndexTag(got) {
		t.Fatalf("every tag can be filtered out on screen: %v", got)
	}
}

// 지울 수 없는 태그가 이미 있으면 **덧붙이지 않는다** — 같은 말을 두 번 적을 이유가 없다.
func TestIndexTagsDoesNotDuplicateCumulativeMove(t *testing.T) {
	pct := func(v float64) *float64 { return &v }

	closes := make([]float64, 0, 60)
	for i := 0; i < 40; i++ {
		closes = append(closes, 100)
	}
	// 20거래일 동안 크게 밀려 month_down이 걸리는 계열
	for i := 0; i < 20; i++ {
		closes = append(closes, 100-float64(i)*0.6)
	}

	ti := &TechnicalIndicators{LastClose: pct(closes[len(closes)-1]), Return20D: pct(-11)}
	got := indexTags(ti, closes)

	months := 0
	for _, tag := range got {
		if strings.HasPrefix(tag, "month") {
			months++
		}
	}
	if months > 1 {
		t.Fatalf("cumulative move appears %d times: %v", months, got)
	}
}
