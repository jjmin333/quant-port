package model

import (
	"strings"
)

// screenFieldExtras — 필드마다 ⓘ 설명과 태그 이름. 정의 표를 한 줄씩 늘리지 않으려고 따로 둔다.
var screenFieldExtras = map[string]struct {
	help    string
	aliases []string
}{
	"score":             {"AI가 추세·수익성·성장·가치·심리 다섯 축을 합친 0~100점. 높을수록 좋다.", []string{"점수", "종합점수", "종합", "AI점수", "score"}},
	"axis.technical":    {"가격 흐름(이동평균·모멘텀·52주 위치). 높을수록 오르는 흐름이 강하다.", []string{"추세점수", "기술", "모멘텀"}},
	"axis.quality":      {"ROE·이익률·부채 등 돈을 잘 버는 체질. 높을수록 좋다.", []string{"수익성점수", "퀄리티"}},
	"axis.growth":       {"매출·이익이 작년보다 얼마나 늘었나. 높을수록 빠르게 큰다.", []string{"성장점수", "성장성"}},
	"axis.valuation":    {"PER·PBR·PSR로 본 싼 정도. 높을수록 이익·자산에 비해 싸다.", []string{"가치점수", "밸류"}},
	"axis.sentiment":    {"뉴스 논조와 외국인·기관 수급. 높을수록 분위기가 좋다. 측정 종목이 적다.", []string{"심리", "수급", "심리수급", "심리·수급"}},
	"roe":               {"자기자본이익률 — 주주 돈 100원으로 1년에 몇 원을 벌었나. 높을수록 좋다.", []string{"ROE", "자기자본이익률"}},
	"operatingMargin":   {"매출 중 영업이익 비율. 높을수록 본업이 남는 장사다.", []string{"영업이익률", "OPM"}},
	"netProfitMargin":   {"매출 중 순이익 비율. 높을수록 좋다.", []string{"순이익률"}},
	"revenueGrowthYoY":  {"매출이 작년보다 몇 % 늘었나.", []string{"매출성장", "매출성장률"}},
	"epsGrowthYoY":      {"주당순이익이 작년보다 몇 % 늘었나.", []string{"EPS성장", "EPS성장률"}},
	"debtRatio":         {"부채 ÷ 자본. 낮을수록 빚 부담이 적다(100% = 빚과 자본이 같다). 기본 칸이 「이하」.", []string{"부채비율"}},
	"currentRatio":      {"유동자산 ÷ 유동부채. 1배보다 크면 1년 안에 갚을 돈이 충분하다.", []string{"유동비율"}},
	"per":               {"주가 ÷ 주당순이익. 낮을수록 이익에 비해 싸다. 적자면 값이 없다. 기본 칸이 「이하」.", []string{"PER", "주가수익비율"}},
	"pbr":               {"주가 ÷ 주당순자산. 1배 아래면 장부상 자산보다 싸게 거래된다.", []string{"PBR", "주가순자산비율"}},
	"psr":               {"시가총액 ÷ 매출. 적자 기업의 싼 정도를 볼 때 쓴다. 낮을수록 싸다.", []string{"PSR"}},
	"marketCap":         {"회사 전체 가치(주가 × 주식 수). 국내는 조 원, 해외는 조 달러 단위.", []string{"시가총액", "시총"}},
	"close":             {"가장 최근 종가.", []string{"주가", "가격", "종가"}},
	"rsi14":             {"14일 RSI. 70 위는 과열, 30 아래는 과매도로 흔히 읽는다.", []string{"RSI"}},
	"pctFromHigh52Week": {"52주 최고가에서 몇 % 아래인가(0에 가까울수록 신고가 근처).", []string{"신고가대비", "52주고가대비"}},
	"pctFromLow52Week":  {"52주 최저가보다 몇 % 위인가.", []string{"신저가대비", "52주저가대비"}},
	"return5d":          {"최근 5거래일 수익률.", []string{"5일수익률"}},
	"return20d":         {"최근 20거래일 수익률.", []string{"20일수익률", "한달수익률"}},
	"return60d":         {"최근 60거래일 수익률.", []string{"60일수익률", "석달수익률"}},
	"daysToEarnings":    {"다음 실적 발표까지 남은 날(국내는 작년 발표일로 추정 포함). 14 이하 = 2주 안.", []string{"실적D", "실적일", "실적발표"}},
	"scoreChange":       {"직전 점수일 대비 종합 점수 변화.", []string{"점수변화"}},
	"flowNetPct":        {"최근 외국인·기관 순매수 ÷ 거래대금. 플러스면 사는 쪽.", []string{"순매수", "외국인기관"}},
	"analystRating":     {"애널리스트 평균 의견 1(강력매도)~5(강력매수). 해외는 매수·보유·매도 명수(Finnhub), 국내는 네이버 증권 컨센서스.", []string{"의견", "투자의견", "애널리스트"}},
	"targetPrice":       {"국내 애널리스트 평균 목표가(네이버 증권). 해외는 무료 출처가 없어 비어 있습니다.", []string{"목표가", "목표주가"}},
	"targetUpside":      {"평균 목표가가 지금 종가보다 몇 % 위인가(국내만). 높을수록 애널리스트가 더 오를 여지를 본다.", []string{"목표가여력", "상승여력", "괴리율"}},
	"socialMentions":    {"Reddit 주식 커뮤니티에서 최근 24시간 언급된 횟수(ApeWisdom, 미국 종목만). 많다고 좋은 것이 아니다 — 과열 신호로도 읽는다.", []string{"레딧", "레딧언급", "reddit"}},
}

func init() {
	for i := range ScreenFields {
		if x, ok := screenFieldExtras[ScreenFields[i].ID]; ok {
			ScreenFields[i].Help = x.help
			ScreenFields[i].Aliases = x.aliases
		}
		screenFieldByID[ScreenFields[i].ID] = ScreenFields[i] // 패키지 변수 초기화가 init보다 먼저라 다시 담는다
	}
	for _, f := range ScreenFields {
		screenFieldByAlias[mentionAliasKey(f.Label)] = f.ID
		for _, a := range f.Aliases {
			screenFieldByAlias[mentionAliasKey(a)] = f.ID
		}
	}
}

var screenFieldByAlias = map[string]string{}

func mentionAliasKey(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", ""))
}

/*
MatchScreenFieldAlias — 글자 run의 **가장 긴 앞부분**이 지표 이름이면 그 필드와 룬 수.
「점수가」→ score, 2. 못 찾으면 ("", 0).
*/
func MatchScreenFieldAlias(run string) (string, int) {
	runes := []rune(run)
	for n := len(runes); n > 0; n-- {
		if id, ok := screenFieldByAlias[mentionAliasKey(string(runes[:n]))]; ok {
			return id, n
		}
	}
	return "", 0
}
