package model

import (
	"regexp"
	"strings"
)

/*
NewsEventType — 기사 제목이 말하는 **사건의 종류**. 변화 감지(`변화` 메뉴)가 「오늘 이
종목에 무슨 일이 있었나」를 감성 점수 하나가 아니라 사건 이름으로 말하게 한다.

# 왜 규칙인가

감성 모델(KR-FinBert-SC · FinBERT)은 긍정·부정만 낸다. 사건 종류는 제목에 쓰이는
낱말이 거의 정해져 있어(「목표가 상향」·「유상증자」·「beats estimates」) 규칙이 모델보다
정확하고, 틀렸을 때 왜 틀렸는지 한 줄로 되짚을 수 있다. 모르는 제목은 빈 값이다 —
억지로 칸에 넣지 않는다.

**먼저 맞는 규칙이 이긴다.** 「실적 발표 후 목표가 상향」은 목표가 쪽이다 — 증권사
의견 변화가 실적보다 구체적인 사건이라서다. 순서가 곧 우선순위다.
*/
type NewsEventType string

const (
	NewsEventNone         NewsEventType = ""
	NewsEventTargetUp     NewsEventType = "target_up"
	NewsEventTargetDown   NewsEventType = "target_down"
	NewsEventEarnings     NewsEventType = "earnings"
	NewsEventContract     NewsEventType = "contract"
	NewsEventCapitalRaise NewsEventType = "capital_raise"
	NewsEventMnA          NewsEventType = "mna"
	NewsEventShareholder  NewsEventType = "shareholder_return"
	NewsEventLegal        NewsEventType = "legal"
	NewsEventTradingHalt  NewsEventType = "trading_halt"
)

var newsEventLabels = map[NewsEventType]string{
	NewsEventTargetUp:     "목표가 상향",
	NewsEventTargetDown:   "목표가 하향",
	NewsEventEarnings:     "실적",
	NewsEventContract:     "수주·계약",
	NewsEventCapitalRaise: "자금 조달",
	NewsEventMnA:          "인수·합병",
	NewsEventShareholder:  "주주환원",
	NewsEventLegal:        "소송·규제",
	NewsEventTradingHalt:  "거래정지·관리",
}

// Label is the Korean display name ("" for none).
func (e NewsEventType) Label() string { return newsEventLabels[e] }

type newsEventRule struct {
	event NewsEventType
	re    *regexp.Regexp
}

// 한국어와 영어를 한 목록에 둔다 — 제목 언어를 먼저 가를 필요가 없다.
var newsEventRules = []newsEventRule{
	{NewsEventTradingHalt, regexp.MustCompile(`거래정지|관리종목|상장폐지|trading halt|delist`)},
	{NewsEventTargetUp, regexp.MustCompile(`목표(주)?가[^,.]{0,12}(상향|올|↑)|price target (raised|hiked|lifted|increased)|raises? (its )?price target|upgrade[sd]?\b`)},
	{NewsEventTargetDown, regexp.MustCompile(`목표(주)?가[^,.]{0,12}(하향|내|낮|↓)|price target (cut|lowered|reduced)|(cuts?|lowers?) (its )?price target|downgrade[sd]?\b`)},
	{NewsEventCapitalRaise, regexp.MustCompile(`유상증자|전환사채|\bCB\b|신주인수권|\bBW\b|교환사채|public offering|secondary offering|convertible notes?|share offering`)},
	{NewsEventMnA, regexp.MustCompile(`인수|합병|M&A|매각|지분 ?취득|acqui(re|res|red|sition)|merger|to buy .{0,30} for \$`)},
	{NewsEventShareholder, regexp.MustCompile(`자사주|배당|소각|buyback|repurchase|dividend`)},
	{NewsEventContract, regexp.MustCompile(`수주|공급계약|단일판매|계약 ?체결|wins? .{0,20}contract|awarded|signs? .{0,20}(deal|agreement)`)},
	{NewsEventEarnings, regexp.MustCompile(`실적|영업이익|매출|순이익|어닝|잠정|분기 ?(최대|사상)|earnings|quarterly results|revenue|\bEPS\b|beats? (estimates|expectations)|misses? (estimates|expectations)|guidance`)},
	{NewsEventLegal, regexp.MustCompile(`소송|과징금|제재|압수수색|기소|lawsuit|sues?\b|probe|investigation|fined?\b|SEC charges`)},
}

// ClassifyNewsEvent returns the first matching event type for a headline.
func ClassifyNewsEvent(headline string) NewsEventType {
	h := strings.ToLower(strings.TrimSpace(headline))
	if h == "" {
		return NewsEventNone
	}
	for _, r := range newsEventRules {
		if r.re.MatchString(h) {
			return r.event
		}
	}
	return NewsEventNone
}

// NewsEventControlChange — 최대주주 변경. 공시에만 있는 종류다.
const NewsEventControlChange NewsEventType = "control_change"

func init() { newsEventLabels[NewsEventControlChange] = "최대주주 변경" }

/*
ClassifyDisclosureEvent — DART 공시 제목을 같은 사건 종류로.

**절차성 공시는 사건이 아니다.** 소유상황보고서·대량보유·기업설명회·주총 소집 같은
제목은 하루 수백 건이고, 그것을 「변화」로 올리면 진짜 사건이 묻힌다. 그래서 여기는
허용 목록이다 — 맞지 않는 제목은 빈 값이다.

「거래정지**해제**」는 정지가 아니다 — 풀린 것을 정지로 올리면 방향이 뒤집힌다.
*/
func ClassifyDisclosureEvent(title string) NewsEventType {
	t := strings.ReplaceAll(strings.TrimSpace(title), " ", "")
	switch {
	case t == "":
		return NewsEventNone
	case strings.Contains(t, "해제"):
		return NewsEventNone
	case disclosureHalt.MatchString(t):
		return NewsEventTradingHalt
	case strings.Contains(t, "최대주주변경"):
		return NewsEventControlChange
	case disclosureCapital.MatchString(t):
		return NewsEventCapitalRaise
	case disclosureMnA.MatchString(t):
		return NewsEventMnA
	case disclosureShareholder.MatchString(t):
		return NewsEventShareholder
	case strings.Contains(t, "단일판매") || strings.Contains(t, "공급계약"):
		return NewsEventContract
	case disclosureEarnings.MatchString(t):
		return NewsEventEarnings
	case strings.Contains(t, "소송등의제기"):
		return NewsEventLegal
	}
	return NewsEventNone
}

var (
	disclosureHalt        = regexp.MustCompile(`매매거래정지|관리종목지정|상장폐지`)
	disclosureCapital     = regexp.MustCompile(`유상증자결정|전환사채권발행결정|신주인수권부사채권발행결정|교환사채권발행결정`)
	disclosureMnA         = regexp.MustCompile(`타법인주식및출자증권(취득|양수)결정|회사합병결정|회사분할결정|영업양수결정|주식교환`)
	disclosureShareholder = regexp.MustCompile(`자기주식취득결정|자기주식소각결정|주식소각결정|현금ㆍ현물배당결정|현금·현물배당결정`)
	disclosureEarnings    = regexp.MustCompile(`영업\(잠정\)실적|매출액또는손익구조`)
)
