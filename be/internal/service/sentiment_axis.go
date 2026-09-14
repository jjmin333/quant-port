package service

import (
	"math"

	"github.com/enact/quant-port/be/internal/model"
)

/*
심리·수급 축 — 종합 점수의 다섯 번째 축.

# 왜 배치가 아니라 여기서 재는가

나머지 네 축(추세·수익성·성장·가치)은 MySQL에 이미 있는 값만으로 전 종목
17,900개를 한 번에 매긴다. 심리·수급의 재료는 그렇지 않다:

  · **뉴스 감성**은 기사별 라벨을 LLM이 붙인다. 전 종목에 돌릴 수 없다(비용).
  · **내부자 거래**는 DART/EDGAR에서 종목 단위로 받아 온다.

그래서 이 축은 **요약 파이프라인이 다녀간 종목에만** 붙는다 — 인기 종목 배치가
이미 그 둘을 모아 프롬프트에 넣고 있으므로, 같은 재료를 한 번 더 쓰는 것뿐이고
호출은 늘지 않는다(`docs/AGENT_STATE.md`의 LLM 지출 규칙).

# 왜 0점이 아니라 「없음」인가

배치가 다녀가지 않은 종목의 이 축은 `nil`이다 — 0이 아니다. 0으로 두면 뉴스가
없다는 이유만으로 심리 점수 최하위가 되고, 그 축의 가중치(기본 10%)만큼 종합
점수가 통째로 깎인다. `nil`이면 가중치에서 빠지고, 대신 커버리지가 낮아져
점수가 중립 쪽으로 조금 당겨진다 — 「모른다」의 올바른 값.

# 구성

  · 뉴스 감성 0.6 — 긍/부정 평균(-100..100)을 0..100으로 옮긴다.
  · 내부자 거래 0.4 — 최근 신고의 순매수 비율.

각 성분은 **재료가 얼마나 두터웠는지**를 confidence로 함께 낸다(기사 8건 ·
신고 5건이면 1.0). 기사 한 건으로 매긴 감성이 완전한 판정 행세를 하지 않도록,
이 값이 축 전체의 confidence가 되어 커버리지에 반영된다.
*/

// sentimentFullNews/Insider — confidence가 1.0에 닿는 표본 수.
//
// 뉴스 8건은 요약 프롬프트가 한 종목에 넣는 최대 12건의 3분의 2다. 내부자 5건은
// 분기 한 번꼴로 신고가 들어오는 국내 종목에서 대략 1년치에 해당한다. 둘 다
// 「이 정도면 방향을 말해도 된다」는 선이지 통계적 유의성은 아니다.
const (
	sentimentFullNews    = 8.0
	sentimentFullInsider = 5.0

	sentimentNewsWeight    = 0.6
	sentimentInsiderWeight = 0.4
)

// sentimentAxis returns the 심리·수급 score (0–100) and how complete the
// reading was (0–1). Both are zero-value when nothing measurable existed —
// callers must treat that as "no axis", never as a score of 0.
func sentimentAxis(
	news *model.NewsSentimentSummary,
	insider []model.InsiderTransaction,
) (score float64, confidence float64, ok bool) {
	var weighted, usedWeight, weightedConf float64

	if v, conf, has := newsSentimentComponent(news); has {
		weighted += v * sentimentNewsWeight
		weightedConf += conf * sentimentNewsWeight
		usedWeight += sentimentNewsWeight
	}
	if v, conf, has := insiderSentimentComponent(insider); has {
		weighted += v * sentimentInsiderWeight
		weightedConf += conf * sentimentInsiderWeight
		usedWeight += sentimentInsiderWeight
	}
	if usedWeight <= 0 {
		return 0, 0, false
	}
	return clamp01to100(weighted / usedWeight), weightedConf / usedWeight, true
}

// newsSentimentComponent maps the LLM-labelled article tally onto 0–100.
//
// AverageScore is already the server's own count over real article ids (see
// aggregateNewsSentiment) — the model never reports an aggregate itself, so
// this cannot be inflated by a hallucinated number.
func newsSentimentComponent(news *model.NewsSentimentSummary) (float64, float64, bool) {
	if news == nil || news.Total <= 0 {
		return 0, 0, false
	}
	// -100..100 → 0..100. 중립 기사만 있는 날은 정확히 50이 된다.
	score := clamp01to100((news.AverageScore + 100) / 2)
	conf := math.Min(1, float64(news.Total)/sentimentFullNews)
	return score, conf, true
}

/*
insiderSentimentComponent — 최근 내부자 신고의 순매수 방향.

**건수가 아니라 주식 수로 잰다.** 1주짜리 신고와 100만 주짜리 신고를 한 표씩
세면, 소액 매수 신고 여러 건이 대주주의 대량 매도를 덮어 버린다.

`SharesChange`가 없는 행은 방향을 말하지 않으므로 **빼고 센다** — 0으로 넣으면
없는 신고가 「중립 매매」로 잡힌다.
*/
func insiderSentimentComponent(txns []model.InsiderTransaction) (float64, float64, bool) {
	var buy, sell float64
	var counted int
	for _, t := range txns {
		if t.SharesChange == nil || *t.SharesChange == 0 {
			continue
		}
		counted++
		v := float64(*t.SharesChange)
		if v > 0 {
			buy += v
		} else {
			sell += -v
		}
	}
	if counted == 0 || buy+sell <= 0 {
		return 0, 0, false
	}

	/*
		순매수 비율 −1..1 → 0..100.

		극단(전부 매수·전부 매도)은 0/100이 아니라 **15/85**에 둔다. 내부자
		거래는 방향을 알려 주는 신호이지 그 자체로 회사의 심리 전부는 아니고,
		이 축 하나가 0이나 100으로 튀면 다섯 축을 평균한 종합 점수가 신고 두
		건에 5점씩 움직인다.
	*/
	net := (buy - sell) / (buy + sell)
	score := 50 + net*35
	conf := math.Min(1, float64(counted)/sentimentFullInsider)
	return clamp01to100(score), conf, true
}
