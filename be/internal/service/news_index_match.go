package service

import (
	"sort"
	"strings"
)

// dashboardIndexRule maps headline/summary text to dashboard index component IDs
// (see fe/src/components/market/market-overview.tsx GROUPS).
type dashboardIndexRule struct {
	IDs   []string
	Terms []string
}

var dashboardIndexRules = []dashboardIndexRule{
	// 「서울 증시」·「Seoul stocks」는 국내 매체·외신이 코스피를 부르는 흔한 이름이다.
	// 이게 없으면 "서울 증시가 역대 최고일을 기록"이라고 쓴 기사에 코스피가 안 붙는다.
	{IDs: []string{"kospi"}, Terms: []string{"코스피", "kospi", "서울 증시", "서울증시", "seoul stocks", "seoul shares"}},
	{IDs: []string{"kosdaq"}, Terms: []string{"코스닥", "kosdaq"}},
	{IDs: []string{"usd-krw"}, Terms: []string{
		"원달러", "usd/krw", "usd-krw", "usdkrw", "원/달러", "달러-원", "달러환율", "원·달러",
	}},
	{IDs: []string{"sp500"}, Terms: []string{"s&p 500", "s&p500", "sp500", "sp 500", "s&p"}},
	{IDs: []string{"nasdaq"}, Terms: []string{"nasdaq", "나스닥"}},
	{IDs: []string{"nasdaq100", "nq100"}, Terms: []string{"nasdaq 100", "nasdaq100", "nq100", "나스닥100", "나스닥 100"}},
	{IDs: []string{"dow"}, Terms: []string{"dow jones", "dow", "djia", "다우"}},
	{IDs: []string{"vix"}, Terms: []string{"vix", "변동성 지수", "volatility index", "fear gauge"}},
	{IDs: []string{"us10y"}, Terms: []string{
		"미 국채 10년", "미국채 10년", "미 10년물", "10-year treasury", "10 year treasury",
		"10-year yield", "treasury yield", "dgs10",
	}},
	{IDs: []string{"sox"}, Terms: []string{"sox", "phlx semiconductor", "반도체 지수", "chip index"}},
	{IDs: []string{"btc"}, Terms: []string{
		"비트코인", "bitcoin", "btc", "이더리움", "ethereum", "eth",
		"가상자산", "암호화폐", "cryptocurrency", "crypto market", "블록체인", "blockchain",
	}},
	{IDs: []string{"gold"}, Terms: goldIndexTerms},
}

var goldIndexTerms = []string{
	"금값", "금 시세", "gold price", "gold spot", "spot gold", "국제금", "금값이", "금값은",
	"gold futures", "gold etf", "gold market", "gold rally", "gold falls", "gold rises",
	"국제 금", "금 가격", "금 시장",
}

var goldmanGuardTerms = []string{"goldman", "goldman sachs"}

func matchesGoldIndex(blob string) bool {
	if containsAnyTerm(blob, goldIndexTerms) {
		return true
	}
	idx := 0
	for {
		pos := strings.Index(blob[idx:], "gold")
		if pos < 0 {
			break
		}
		abs := idx + pos
		after := abs + 4
		if after < len(blob) && strings.HasPrefix(blob[after:], "man") {
			idx = after
			continue
		}
		if financeTermBoundaryBefore(blob, abs) && financeTermBoundaryAfter(blob, after) {
			return true
		}
		idx = abs + 4
	}
	return containsWholeTerm(blob, "금") &&
		(containsWholeTerm(blob, "국제") || containsWholeTerm(blob, "금값") ||
			containsWholeTerm(blob, "금 시세") || containsWholeTerm(blob, "금 가격"))
}

// MatchDashboardIndexIDs returns dashboard index IDs referenced in the article text.
func MatchDashboardIndexIDs(headline, summary string) []string {
	blob := newsTextBlob(headline, summary)
	seen := map[string]struct{}{}
	var out []string
	for _, rule := range dashboardIndexRules {
		matched := containsAnyTerm(blob, rule.Terms)
		if !matched {
			for _, id := range rule.IDs {
				if id == "gold" && matchesGoldIndex(blob) {
					matched = true
					break
				}
			}
		}
		if !matched {
			continue
		}
		for _, id := range rule.IDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// DashboardIndexTags converts index IDs to ingest tags for future dashboard modal linking.
func DashboardIndexTags(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		out = append(out, "index:"+id)
	}
	return out
}
