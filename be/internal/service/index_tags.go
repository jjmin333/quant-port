package service

import (
	"fmt"
	"math"
	"strings"
)

/*
지수 상태 키워드.

「AI 56」을 대신한다. 그 점수의 문제는 정확도가 아니라 **되짚을 수 없다는 것**이었다:
지수 점수는 횡단면 랭킹이 아니라 절대 지표로 만든 값이라 코스피 56과 나스닥 62를
비교할 수 없는데 나란히 놓이면 반드시 비교하게 되고, 무엇을 보고 나온 수인지 알 수
없어 맞아도 배울 게 없고 틀려도 반박할 수 없었다.

여기서 내는 건 전부 **셀 안의 스파크라인을 보면 그 자리에서 확인되는 사실**이다.
「3일 연속↑」은 설명이 필요 없다. 해석어(강세·매수우위)는 결국 점수의 말바꿈이므로
쓰지 않는다.

코드로 저장하고 문구는 FE가 붙인다(`fe/src/lib/index-tags.ts`) — 표현을 다듬자고
배치를 다시 돌릴 이유는 없다. 숫자가 붙는 코드는 `코드:값` 꼴이다.

우선순위는 아래 배열 순서 그대로다. **드물수록 앞**에 온다: 오늘 처음 일어난 일이
늘 참인 상태보다 먼저 읽혀야 한다. 하나도 걸리지 않으면 빈 값을 낸다 — 「보통」 같은
칩을 두면 「AI 56」과 같은 실수를 반복하는 셈이다.
*/

// 화면에 나가는 최대 개수. 홈 스트립은 이 중 1개, 지수 모달은 3개를 쓴다.
const indexTagLimit = 3

const (
	// 최소 이 정도는 있어야 지표가 의미를 갖는다(20일선 + 여유).
	indexTagMinBars = 30
	// 52주 고저는 창이 얕으면 거짓말이 된다 — 반년 치는 있어야 한다.
	indexTagHighLowMinBars = 120
	// 변동성 비교는 긴 쪽 창이 채워져야 성립한다.
	indexTagVolMinBars = 100

	indexTagStreakMin      = 3    // 연속일수는 3일부터 말할 값이 된다
	indexTagMonthMovePct   = 5.0  // 20거래일 누적이 이만큼이면 「한 달 ±N%」
	indexTagSurgeSigma     = 2.0  // 급등락 판정: 60일 변동성의 몇 배인가
	indexTagSurgeFloorPct  = 1.0  // 변동성이 바닥이면 2σ가 0.2%인 날도 생긴다
	indexTagRangeBoundPct  = 3.0  // 20일 고저 폭이 평균 대비 이 안이면 박스권
	indexTagVolExpandRatio = 1.5  // 20일 변동성이 100일의 몇 배면 확대인가
	indexTagOverboughtRSI  = 70.0 // 과열
	indexTagOversoldRSI    = 30.0 // 과매도

	/*
		20일선 돌파로 인정할 최소 이격.

		이게 없으면 **조용한 구간마다 매일 켜진다**. 지수는 방향이 없을 때 20일선에
		달라붙어 오르내리므로, 부호만 보면 하루걸러 「회복」과 「이탈」이 번갈아 뜬다.
		테스트(`TestIndexTagsRangeBoundIsLastResort`)가 0.02% 지그재그에서 바로
		이 상태를 잡아냈다. 선을 **넘었다고 말하려면 넘어가 있어야 한다**.
	*/
	indexTagSMAMarginPct = 0.25
)

/*
indexTags는 한 지수의 종가 이력에서 키워드 코드를 뽑는다.

`closes`는 오래된 것부터, `ti`는 같은 계열로 계산된 지표다(`indexTechnicals`).
둘 다 지수 점수 배치가 이미 메모리에 들고 있는 값이라 추가 쿼리도 API 호출도 없다.
*/
func indexTags(ti *TechnicalIndicators, closes []float64) []string {
	n := len(closes)
	if ti == nil || ti.LastClose == nil {
		return nil
	}
	/*
		봉이 얕으면 **사건 판정은 건너뛰고 누적 등락률만** 낸다.

		30봉 문턱은 20일선·RSI·변동성 때문에 있다 — 그 계산들은 창이 안 차면 거짓말이
		된다. 그런데 마지막 층위(아래 10번)의 누적 등락률은 그 창이 필요 없고, 5봉이면
		이미 참이다. 문턱을 함수 입구에 두는 바람에 **이력이 얕은 지수는 아무 칩도 못
		받았다** — 실측 08-31에 금(16봉)이 열세 칸 중 유일한 빈 칸이었고, 빈 칸은
		"조용하다"가 아니라 "아직 안 불러왔다"로 읽힌다.
	*/
	if n < indexTagMinBars {
		return cumulativeMoveTags(ti)
	}
	last := *ti.LastClose
	prevCloses := closes[:n-1]

	var tags []string
	add := func(code string) {
		if len(tags) < indexTagLimit {
			tags = append(tags, code)
		}
	}

	// 1. 골든/데드 크로스 — 몇 달에 한 번이라 걸리면 그날의 사건이다.
	if cross := smaCross(closes, 50, 200); cross != "" {
		add(cross)
	}

	// 2. 52주 신고가·신저가. 종가가 창의 끝값과 같은지만 본다 — 「부근」은
	//    사건이 아니라 상태라서 아래 층위로 내려간다.
	if n >= indexTagHighLowMinBars {
		window := closes
		if n > tiTradingDaysInYear {
			window = closes[n-tiTradingDaysInYear:]
		}
		hi, lo := seriesExtremes(window)
		switch {
		case last >= hi:
			add("high_52w")
		case last <= lo:
			add("low_52w")
		}
	}

	// 3. 급등·급락. 임계를 최근 변동성으로 잡는 이유는 지수마다 평소 진폭이
	//    다르기 때문이다 — 비트코인의 2%와 미 국채 10년의 2%는 다른 사건이다.
	if move, ok := lastMovePct(closes); ok {
		sigma := returnStdevPct(closes, 60)
		threshold := math.Max(sigma*indexTagSurgeSigma, indexTagSurgeFloorPct)
		if math.Abs(move) >= threshold {
			if move > 0 {
				add("surge")
			} else {
				add("plunge")
			}
		}
	}

	// 4. 20일선 돌파. 어제까지 **뚜렷이** 아래에 있다가 오늘 **뚜렷이** 위로
	//    올라온 날만 해당한다 — 이격 조건은 위 상수의 설명 참고.
	if ti.SMA20 != nil {
		if prevSMA := smaLast(prevCloses, 20); prevSMA != nil && len(prevCloses) > 0 {
			prev := prevCloses[len(prevCloses)-1]
			was := gapPct(prev, *prevSMA)
			now := gapPct(last, *ti.SMA20)
			switch {
			case was <= -indexTagSMAMarginPct && now >= indexTagSMAMarginPct:
				add("sma20_recover")
			case was >= indexTagSMAMarginPct && now <= -indexTagSMAMarginPct:
				add("sma20_break")
			}
		}
	}

	// 5. 연속 상승·하락.
	if dir, run := closeStreak(closes); run >= indexTagStreakMin {
		if dir > 0 {
			add(fmt.Sprintf("streak_up:%d", run))
		} else {
			add(fmt.Sprintf("streak_down:%d", run))
		}
	}

	// 6. 한 달 누적. 하루하루는 조용해도 20거래일을 모으면 이야기가 되는 구간이 있다.
	if ti.Return20D != nil && math.Abs(*ti.Return20D) >= indexTagMonthMovePct {
		if *ti.Return20D > 0 {
			add(fmt.Sprintf("month_up:%d", int(math.Round(*ti.Return20D))))
		} else {
			add(fmt.Sprintf("month_down:%d", int(math.Round(-*ti.Return20D))))
		}
	}

	// 7. 과열·과매도.
	if ti.RSI14 != nil {
		switch {
		case *ti.RSI14 >= indexTagOverboughtRSI:
			add("overbought")
		case *ti.RSI14 <= indexTagOversoldRSI:
			add("oversold")
		}
	}

	// 8. 변동성 확대.
	if n >= indexTagVolMinBars {
		short := returnStdevPct(closes, 20)
		long := returnStdevPct(closes, 100)
		if long > 0 && short >= long*indexTagVolExpandRatio {
			add("vol_expand")
		}
	}

	// 9. 박스권 — 「아무 일도 없었다」는 다른 할 말이 없을 때만 할 말이다.
	if len(tags) == 0 {
		if hi, lo := seriesExtremes(tail(closes, 20)); hi > 0 {
			mean := seriesMean(tail(closes, 20))
			if mean > 0 && (hi-lo)/mean*100 <= indexTagRangeBoundPct {
				add("range_bound")
			}
		}
	}

	/*
		10. 그래도 비었으면 **누적 등락률을 그냥 적는다**.

		실측(08-31): 열세 칸 중 여덟이 빈 칸이었다 — 코스피·코스닥·원/달러·나스닥·
		나스닥100·필라델피아 반도체·비트코인·금. 박스권이라기엔 흔들렸고 사건이라기엔
		조용한 구간이 생각보다 넓어서, 위 아홉 층위를 전부 비껴간다. 그런데 빈 칸은
		"조용하다"로 읽히지 않는다 — **아직 안 불러왔다**로 읽힌다.

		여기서 내는 건 해석이 아니라 사실이다. 「한 달 +4.6%」는 셀 안의 스파크라인을
		보면 그 자리에서 확인되고, 위 6번(`month_up`)과 **같은 값에 같은 문구**다.
		다른 건 문턱뿐이다 — 6번은 5% 넘게 움직여 그 자체가 이야기인 경우고, 여기는
		할 말이 그것밖에 없어서 적는 경우다. 「보통」이나 「중립」 같은 말바꿈은 여전히
		내지 않는다.

		20거래일이 없으면(원/달러처럼 이력이 얕은 지수) 5거래일로 내려간다. 둘 다
		없으면 빈 값 그대로다 — 없는 것을 지어내지는 않는다.
	*/
	if len(tags) == 0 {
		tags = cumulativeMoveTags(ti)
	}

	/*
		11. **화면에서 하나도 안 남는 경우를 막는다** (2026-09-03 사용자 신고).

		여기까지 오면 태그는 비어 있지 않다. 그런데 **화면은 여전히 빈 칸일 수 있다** —
		FE(`fe/src/lib/index-tags.ts`)가 「오늘 움직임과 어긋나는 당일 주장」을 지우기
		때문이다. 「3일 연속↓」은 마지막 마감까지의 사실이고 셀의 등락률은 지금 이
		순간이라, 장중에 반등하면 그 하나뿐이던 키워드가 사라진다.

		실측 2026-09-03: 열세 지수 중 넷(나스닥·나스닥100 선물·S&P 500·다우)이 태그가
		`streak_down:3` 하나뿐이었다. 그날 그 넷이 오르면 네 칸이 동시에 빈다.

		그래서 **지워질 수 없는 태그가 하나도 없으면** 누적 등락률을 뒤에 붙인다. 이건
		지난 20거래일에 대한 말이라 오늘 방향과 어긋날 수가 없다. 상한(3개) 안에서
		마지막 자리에만 들어가므로, 지울 것이 없는 날에는 화면(1~2개)에 나오지도 않는다.
	*/
	if !hasSurvivingIndexTag(tags) {
		for _, code := range cumulativeMoveTags(ti) {
			add(code)
		}
	}

	return tags
}

/*
sameDayIndexTagCodes — **오늘 하루에 대한 주장**이라 FE의 모순 필터에 지워질 수 있는
코드들.

`fe/src/lib/index-tags.ts`의 `TagSpec.sameDay`와 같은 목록이다. 두 곳에 나뉘어 있는
이유는 판정이 FE에서 일어나기 때문이다 — 등락률은 30초마다 갱신되는 라이브 값이라
배치 시점에는 알 수 없다. 여기서는 「지워질 수 있는가」만 알면 되므로 목록만 맞춰 둔다.
한쪽에 새 코드를 더하면 다른 쪽도 같이 본다.
*/
var sameDayIndexTagCodes = map[string]struct{}{
	"cross_golden":  {},
	"cross_dead":    {},
	"high_52w":      {},
	"low_52w":       {},
	"surge":         {},
	"plunge":        {},
	"sma20_recover": {},
	"sma20_break":   {},
	"streak_up":     {},
	"streak_down":   {},
}

// hasSurvivingIndexTag reports whether at least one tag is safe from the
// contradiction filter — i.e. is not a claim about today.
func hasSurvivingIndexTag(tags []string) bool {
	for _, tag := range tags {
		name := tag
		if at := strings.Index(tag, ":"); at >= 0 {
			name = tag[:at]
		}
		if _, sameDay := sameDayIndexTagCodes[name]; !sameDay {
			return true
		}
	}
	return false
}

/*
cumulativeMoveTags는 누적 등락률 하나를 낸다 — 마지막 층위이자, 이력이 얕은 지수가
받을 수 있는 유일한 칩이다.

20거래일이 없으면 5거래일로 내려간다. 둘 다 없으면 빈 값 그대로다 — 없는 것을 지어내지는
않는다.
*/
func cumulativeMoveTags(ti *TechnicalIndicators) []string {
	switch {
	case ti.Return20D != nil:
		return []string{signedMoveTag("month", *ti.Return20D)}
	case ti.Return5D != nil:
		return []string{signedMoveTag("week", *ti.Return5D)}
	}
	return nil
}

/*
signedMoveTag는 누적 등락률을 `코드:부호값` 꼴로 적는다 (`month:+5`, `week:-2`).

부호를 값에 넣는 이유는 `month_up`/`month_down`처럼 코드를 둘로 쪼개면 **0% 근처가
갈 곳이 없기 때문**이다. 반올림해서 0이 되는 날 `month_up:0`은 "올랐다"고 말하면서
0을 보여 준다. 여기서는 `month:+0`이 그냥 "한 달 0%"로 읽힌다.
*/
func signedMoveTag(code string, pct float64) string {
	return fmt.Sprintf("%s:%+d", code, int(math.Round(pct)))
}

// encodeIndexTags/decodeIndexTags — 한 칸에 쉼표로 잇는다. 코드 3개면 넉넉히
// VARCHAR(255) 안이라 별도 표를 만들 이유가 없다.
func encodeIndexTags(tags []string) string { return strings.Join(tags, ",") }

func decodeIndexTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// smaCross reports a moving-average crossover that happened on the last bar.
func smaCross(closes []float64, fast, slow int) string {
	if len(closes) < slow+1 {
		return ""
	}
	prev := closes[:len(closes)-1]
	nowFast, nowSlow := smaLast(closes, fast), smaLast(closes, slow)
	wasFast, wasSlow := smaLast(prev, fast), smaLast(prev, slow)
	if nowFast == nil || nowSlow == nil || wasFast == nil || wasSlow == nil {
		return ""
	}
	switch {
	case *wasFast <= *wasSlow && *nowFast > *nowSlow:
		return "cross_golden"
	case *wasFast >= *wasSlow && *nowFast < *nowSlow:
		return "cross_dead"
	}
	return ""
}

// closeStreak returns the direction (+1/-1) and length of the run ending at the
// last bar. Unchanged closes end a run — a flat day is not momentum.
func closeStreak(closes []float64) (dir, run int) {
	if len(closes) < 2 {
		return 0, 0
	}
	last := closes[len(closes)-1]
	prev := closes[len(closes)-2]
	switch {
	case last > prev:
		dir = 1
	case last < prev:
		dir = -1
	default:
		return 0, 0
	}
	for i := len(closes) - 1; i > 0; i-- {
		diff := closes[i] - closes[i-1]
		if (dir > 0 && diff > 0) || (dir < 0 && diff < 0) {
			run++
			continue
		}
		break
	}
	return dir, run
}

// lastMovePct is the last bar's percent change.
func lastMovePct(closes []float64) (float64, bool) {
	if len(closes) < 2 {
		return 0, false
	}
	prev := closes[len(closes)-2]
	if prev <= 0 {
		return 0, false
	}
	return (closes[len(closes)-1] - prev) / prev * 100, true
}

// returnStdevPct is the standard deviation of the last `bars` daily returns, in
// percent. Returns 0 when the window is too thin to mean anything.
func returnStdevPct(closes []float64, bars int) float64 {
	if bars < 2 || len(closes) < bars+1 {
		return 0
	}
	window := closes[len(closes)-bars-1:]
	rets := make([]float64, 0, bars)
	for i := 1; i < len(window); i++ {
		if window[i-1] <= 0 {
			continue
		}
		rets = append(rets, (window[i]-window[i-1])/window[i-1]*100)
	}
	if len(rets) < 2 {
		return 0
	}
	var sum float64
	for _, r := range rets {
		sum += r
	}
	mean := sum / float64(len(rets))
	var sq float64
	for _, r := range rets {
		sq += (r - mean) * (r - mean)
	}
	return math.Sqrt(sq / float64(len(rets)-1))
}

// gapPct is how far `value` sits above (+) or below (-) `base`, in percent.
func gapPct(value, base float64) float64 {
	if base == 0 {
		return 0
	}
	return (value - base) / base * 100
}

func seriesExtremes(values []float64) (high, low float64) {
	if len(values) == 0 {
		return 0, 0
	}
	high, low = values[0], values[0]
	for _, v := range values[1:] {
		if v > high {
			high = v
		}
		if v < low {
			low = v
		}
	}
	return high, low
}

func seriesMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func tail(values []float64, n int) []float64 {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}
