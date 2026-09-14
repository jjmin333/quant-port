package model

import "time"

/*
AI 추천 랭킹의 두 손잡이 — **무엇으로 줄 세울까(축)** 와 **언제의 값으로(기간)**.

실시간 차트(마켓 레이더)가 「거래대금/거래량 × 실시간~1년」을 고르는 것과 같은 자리다.
이 화면은 그 자리를 점수로 바꾼다: 종합 점수와 다섯 축, 그리고 그 기간 안의 최고점.
*/

// ScoreRankingAxis is what a ranking sorts by. "composite" is the 종합 점수 —
// the axes are ScoreAxis values, kept as a separate type so an API parameter
// can carry "composite" too.
type ScoreRankingAxis string

const (
	RankAxisComposite ScoreRankingAxis = "composite"
	RankAxisTechnical ScoreRankingAxis = "technical"
	RankAxisQuality   ScoreRankingAxis = "quality"
	RankAxisGrowth    ScoreRankingAxis = "growth"
	RankAxisValuation ScoreRankingAxis = "valuation"
	RankAxisSentiment ScoreRankingAxis = "sentiment"
)

// ScoreRankingAxes is the display order: 종합 first, then the five axes in the
// same order the score card draws them.
var ScoreRankingAxes = []ScoreRankingAxis{
	RankAxisComposite,
	RankAxisTechnical,
	RankAxisQuality,
	RankAxisGrowth,
	RankAxisValuation,
	RankAxisSentiment,
}

var ScoreRankingAxisLabels = map[ScoreRankingAxis]string{
	RankAxisComposite: "종합",
	RankAxisTechnical: "추세",
	RankAxisQuality:   "수익성",
	RankAxisGrowth:    "성장",
	RankAxisValuation: "가치",
	RankAxisSentiment: "심리·수급",
}

// Axis maps a ranking axis onto the breakdown axis it reads. Composite has no
// breakdown axis — it is the weighted total — so the second return is false.
func (a ScoreRankingAxis) Axis() (ScoreAxis, bool) {
	switch a {
	case RankAxisTechnical:
		return AxisTechnical, true
	case RankAxisQuality:
		return AxisQuality, true
	case RankAxisGrowth:
		return AxisGrowth, true
	case RankAxisValuation:
		return AxisValuation, true
	case RankAxisSentiment:
		return AxisSentiment, true
	}
	return "", false
}

func ParseScoreRankingAxis(raw string) ScoreRankingAxis {
	for _, axis := range ScoreRankingAxes {
		if string(axis) == raw {
			return axis
		}
	}
	return RankAxisComposite
}

/*
ScoreRankingDuration — 어느 기간의 점수로 줄을 세울까.

`realtime`은 **가장 최근 배치일의 점수**다. 점수는 하루 한 번 계산되므로 시세처럼
초 단위로 움직이지 않지만, 실시간 차트와 버튼 구성을 맞추기 위해 같은 이름을 쓴다
(사용자 결정 2026-09-03) — 뜻은 「지금 화면에 있는 점수」다.

나머지는 그 기간 안에서 **한 번이라도 도달한 최고 점수**로 줄을 세운다. 「최근 한 달
사이에 가장 좋았던 종목」을 묻는 것이고, 오늘 값만 보는 realtime과 다른 답이 나온다.
*/
type ScoreRankingDuration string

const (
	RankDurationRealtime ScoreRankingDuration = "realtime"
	RankDuration1D       ScoreRankingDuration = "1d"
	RankDuration1W       ScoreRankingDuration = "1w"
	RankDuration1M       ScoreRankingDuration = "1mo"
)

var ScoreRankingDurations = []ScoreRankingDuration{
	RankDurationRealtime,
	RankDuration1D,
	RankDuration1W,
	RankDuration1M,
}

var ScoreRankingDurationLabels = map[ScoreRankingDuration]string{
	RankDurationRealtime: "실시간",
	RankDuration1D:       "1일",
	RankDuration1W:       "1주",
	RankDuration1M:       "1개월",
}

/*
LookbackDays — 최신 점수일에서 **며칠을 거슬러** 창을 잡을까. 달력 일수다.

거래일이 아니라 달력으로 세는 이유: 점수 배치가 매일 도는 것이 아니라서(실측
2026-08-07~09-02 사이 27일 중 16일만 점수가 있다) 「N거래일」은 종목마다 다른 날짜를
가리키게 된다. 달력 창은 어느 종목에나 같은 구간을 뜻한다.

`realtime`은 0 — 최신 점수일 하루뿐이다.
*/
func (d ScoreRankingDuration) LookbackDays() int {
	switch d {
	case RankDuration1D:
		return 1
	case RankDuration1W:
		return 7
	case RankDuration1M:
		return 30
	}
	return 0
}

func ParseScoreRankingDuration(raw string) ScoreRankingDuration {
	for _, d := range ScoreRankingDurations {
		if string(d) == raw {
			return d
		}
	}
	return RankDurationRealtime
}

// ScoreRankingView is one stored leaderboard as the API returns it.
type ScoreRankingView struct {
	Market   string               `json:"market"`
	Axis     ScoreRankingAxis     `json:"axis"`
	Duration ScoreRankingDuration `json:"duration"`
	// TradeDate is the latest scored date the list is built from.
	TradeDate string `json:"tradeDate"`
	// FromDate is the start of the window. Equal to TradeDate for realtime.
	FromDate   string               `json:"fromDate"`
	ComputedAt time.Time            `json:"computedAt"`
	Items      []StockScoreListItem `json:"items"`
}
