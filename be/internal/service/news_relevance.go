package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// strongMarketTerms — 단독으로 증권/증시 기사로 인정 (가상자산 단독 키워드 제외).
var strongMarketTerms = []string{
	"코스피", "코스닥", "증시", "주식", "주가", "종목", "상장", "krx", "kosdaq", "kospi",
	"매수", "매도", "순매수", "거래량", "시가총액", "급등", "급락", "상한가", "하한가",
	"etf", "ipo", "공매도", "증권", "증권가", "지수", "선물", "옵션",
	"외국인 순매수", "외국인 매수", "외국인 매도", "외국인 투자",
	"기관 순매수", "기관 매수", "기관 매도",
	"테마주", "장중", "장마감", "장 시작", "장 마감",
}

// explicitDomesticStockTerms — 코인 매매 맥락의 매수·매도·배당과 구분되는 증시 앵커.
var explicitDomesticStockTerms = []string{
	"코스피", "코스닥", "증시", "주식", "주가", "종목", "상장", "krx", "kosdaq", "kospi",
	"시가총액", "급등", "급락", "상한가", "하한가",
	"etf", "ipo", "공매도", "증권", "증권가", "지수", "선물", "옵션",
	"외국인 순매수", "외국인 매수", "외국인 매도", "외국인 투자",
	"기관 순매수", "기관 매수", "기관 매도",
	"테마주", "장중", "장마감", "장 시작", "장 마감",
}

// corporateFinanceTerms — 기업 실적·공시 맥락.
var corporateFinanceTerms = []string{
	"실적", "영업이익", "순이익", "매출", "배당", "공시", "어닝", "실적발표", "컨센서스",
	"per", "pbr", "eps", "roe", "밸류에이션",
	"주주", "유상증자", "무상증자", "분기보고서",
}

// macroFinanceTerms — 거시·금융 정책 (증시에 직접 영향). 단독 허용은 제한.
var macroFinanceTerms = []string{
	"금리", "기준금리", "환율", "원달러", "유가", "채권", "한국은행", "cpi", "gdp", "물가",
}

// sectorTerms — 섹터 키워드 (단독으론 부족, 산업·기업 맥락과 함께일 때 허용).
var sectorTerms = []string{
	"2차전지", "반도체", "엔비디아", "nvda", "바이오", "자동차",
}

// industryContextTerms — 섹터 기사 허용을 위한 동반 맥락.
var industryContextTerms = []string{
	"산업", "전망", "수출", "수입", "수주", "기업", "업체", "법인", "양산", "공장", "capa",
	"가동률", "수익성", "투자", "지분", "인수", "합병", "m&a",
}

// ambiguousQueryTerms — 검색 키워드만으로는 증권 관련성을 증명하지 못하는 단어.
var ambiguousQueryTerms = map[string]struct{}{
	"ai": {}, "트럼프": {}, "외국인": {}, "기관": {},
	"바이오": {}, "자동차": {}, "반도체": {}, "2차전지": {},
	"엔비디아": {}, "nvda": {},
}

// societyGossipTerms — 연예·사회 스캔들 (기업명 언급만으로는 증권 기사 아님).
var societyGossipTerms = []string{
	"건물주", "비굴", "막장", "폭로", "논란", "구설수", "허세",
	"달라고 요구", "무조건 달라", "마케팅팀에 전화", "전화해 무조건",
	"유튜버", "인플루언서", "연예인", "셀럽",
}
var educationPolicyTerms = []string{
	"인재양성", "교육청", "과학고", "고등학교", "중학교", "대학교", "대학", "학생", "입학", "개학", "수능", "교육부",
	"산업 정책", "지원책",
}

// excludeTopicTerms — 아래 주제는 강한 증시·종목·실적 근거 없을 때 제외.
var excludeTopicTerms = []string{
	"교도소", "교정", "탈주", "폭염", "폭우", "호우", "경보 발령", "태풍", "기온",
	"여론조사", "지지율", "총선", "지방선거", "민선", "규제철폐", "시민",
	"교통사고", "살인", "체포", "기소", "재판", "피의자",
	"어린이날", "축제", "공연", "캠프",
	"승객", "관광객", "입국",
	"과학고", "고등학교", "중학교", "대학교", "교육청", "인재양성", "학생", "입학", "개학", "수능",
	"배우", "가수", "연예", "드라마", "영화", "아이돌",
	"축구", "야구", "올림픽", "월드컵", "e스포츠",
	"백신", "접종", "hpv", "병원", "의료", "질병", "감염",
	"결혼", "이혼", "스캔들", "불륜",
	"날씨", "미세먼지",
}

func concatTermLists(lists ...[]string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, list := range lists {
		for _, term := range list {
			key := strings.ToLower(strings.TrimSpace(term))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, term)
		}
	}
	return out
}

func newsTextBlob(title, summary string) string {
	return strings.ToLower(strings.TrimSpace(title + " " + summary))
}

func containsAnyTerm(blob string, terms []string) bool {
	for _, term := range terms {
		if containsWholeTerm(blob, term) {
			return true
		}
	}
	return false
}

func containsWholeTerm(blob, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	if strings.Contains(term, " ") || utf8.RuneCountInString(term) >= 4 {
		return strings.Contains(blob, term)
	}
	idx := 0
	for {
		pos := strings.Index(blob[idx:], term)
		if pos < 0 {
			return false
		}
		abs := idx + pos
		if financeTermBoundaryBefore(blob, abs) && financeTermBoundaryAfter(blob, abs+len(term)) {
			return true
		}
		idx = abs + len(term)
		if idx >= len(blob) {
			return false
		}
	}
}

func financeTermBoundaryBefore(blob string, pos int) bool {
	if pos == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(blob[:pos])
	return !isFinanceTermGlueRune(r)
}

func financeTermBoundaryAfter(blob string, pos int) bool {
	if pos >= len(blob) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(blob[pos:])
	return !isFinanceTermGlueRune(r)
}

func isFinanceTermGlueRune(r rune) bool {
	if r >= 0xAC00 && r <= 0xD7A3 {
		return true
	}
	if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
		return true
	}
	return false
}

func hasStrongMarketSignal(blob string) bool {
	return containsAnyTerm(blob, strongMarketTerms)
}

func hasCorporateFinanceSignal(blob string) bool {
	return containsAnyTerm(blob, corporateFinanceTerms)
}

func hasMacroFinanceSignal(blob string) bool {
	return containsAnyTerm(blob, macroFinanceTerms)
}

func hasFinanceContentSignal(blob string) bool {
	return hasStrongMarketSignal(blob) || hasCorporateFinanceSignal(blob)
}

// cryptoFinanceTerms — 가상자산 전문 매체 허용 및 btc 지수 태깅.
var cryptoFinanceTerms = []string{
	"비트코인", "bitcoin", "btc", "이더리움", "ethereum", "eth",
	"가상자산", "암호화폐", "cryptocurrency", "crypto", "블록체인", "blockchain",
	"디파이", "defi", "스테이블코인", "stablecoin", "usdt", "usdc",
	"리플", "xrp", "솔라나", "solana", "도지", "doge", "dogecoin",
	"업비트", "빗썸", "coinbase", "binance", "바이낸스",
	"nft", "web3", "채굴", "mining", "halving", "반감기",
}

func isCryptoNewsSource(sourceID string) bool {
	switch strings.ToLower(strings.TrimSpace(sourceID)) {
	case "tokenpost", "coinreaders", "blockmedia":
		return true
	default:
		return false
	}
}

func hasExplicitDomesticStockSignal(blob string) bool {
	if containsAnyTerm(blob, explicitDomesticStockTerms) {
		return true
	}
	return hasSectorWithIndustryContext(blob)
}

// hasStockMarketKeyword — 코인 전문 RSS용. 증시 앵커 또는 국내 종목·실적 맥락 필요.
func hasStockMarketKeyword(blob string) bool {
	if hasExplicitDomesticStockSignal(blob) {
		return true
	}
	if hasCorporateFinanceSignal(blob) && hasStrongMarketSignal(blob) {
		return true
	}
	return false
}

func isDomesticCryptoOnlyArticle(blob string) bool {
	if !hasCryptoFinanceSignal(blob) {
		return false
	}
	return !hasExplicitDomesticStockSignal(blob)
}

func hasCryptoFinanceSignal(blob string) bool {
	return containsAnyTerm(blob, cryptoFinanceTerms)
}

func hasStrictFinanceSignal(blob string) bool {
	return hasFinanceContentSignal(blob)
}

func matchesSocietyGossipTopic(blob string) bool {
	return containsAnyTerm(blob, societyGossipTerms)
}

func matchesEducationOrGeneralPolicy(blob string) bool {
	return containsAnyTerm(blob, educationPolicyTerms)
}

func hasSectorWithIndustryContext(blob string) bool {
	if !containsAnyTerm(blob, sectorTerms) {
		return false
	}
	return containsAnyTerm(blob, industryContextTerms) ||
		hasStrongMarketSignal(blob) ||
		hasCorporateFinanceSignal(blob) ||
		hasMacroFinanceSignal(blob)
}

func matchesExcludeTopic(blob string) bool {
	return containsAnyTerm(blob, excludeTopicTerms)
}

func isAmbiguousSearchTerm(term string) bool {
	_, ok := ambiguousQueryTerms[strings.ToLower(strings.TrimSpace(term))]
	return ok
}

func containsSearchTerm(blob, term string) bool {
	return containsWholeTerm(blob, term)
}

func hasRelevanceAnchor(title, blob string, hasSymbols bool) bool {
	titleLower := strings.ToLower(strings.TrimSpace(title))
	if hasStrictFinanceSignal(blob) {
		return true
	}
	if hasMacroFinanceSignal(blob) && hasStrongMarketSignal(blob) {
		return true
	}
	if hasMacroFinanceSignal(titleLower) {
		return true
	}
	return hasSectorWithIndustryContext(blob)
}

func passesKeywordSearchGate(title, blob string, searchTerms []string, linkedSymbol string, hasSymbols bool) bool {
	if linkedSymbol != "" {
		return true
	}
	if len(searchTerms) == 0 {
		return true
	}

	// 모호 검색어(AI·트럼프·외국인 등)로 찾은 기사 → 강한 증시/실적/종목 근거 필수
	for _, term := range searchTerms {
		if isAmbiguousSearchTerm(term) && containsSearchTerm(blob, term) {
			if !hasStrictFinanceSignal(blob) {
				return false
			}
		}
	}

	if matchesEducationOrGeneralPolicy(blob) && !hasStrictFinanceSignal(blob) {
		return false
	}

	// 키워드 검색 결과는 제목에 증시/실적/거시·증시 동반 신호가 있어야 저장
	titleLower := strings.ToLower(strings.TrimSpace(title))
	if hasStrictFinanceSignal(titleLower) {
		return true
	}
	if hasMacroFinanceSignal(titleLower) && hasStrongMarketSignal(titleLower) {
		return true
	}
	if hasSectorWithIndustryContext(titleLower) {
		return true
	}

	// 본문에만 키워드가 있고 제목이 비증권인 경우 차단
	for _, term := range searchTerms {
		if containsSearchTerm(blob, term) && !containsSearchTerm(titleLower, term) {
			return false
		}
	}

	return hasStrictFinanceSignal(blob) ||
		(hasMacroFinanceSignal(blob) && hasStrongMarketSignal(blob))
}

// isStockRelated decides whether a news item belongs on the domestic stock news feed.
func isStockRelated(
	ctx context.Context,
	st *NewsStore,
	title, summary string,
	linkedSymbol string,
	searchTerms []string,
	storedSymbols []string,
) bool {
	blob := newsTextBlob(title, summary)

	symbols := st.extractSymbols(ctx, title+" "+summary)
	for _, sym := range storedSymbols {
		symbols = appendUnique(symbols, sym)
	}
	if linkedSymbol != "" {
		symbols = appendUnique(symbols, linkedSymbol)
	}

	hasSymbols := len(symbols) > 0
	if isDomesticCryptoOnlyArticle(blob) && !hasSymbols {
		return false
	}

	hasStrong := hasStrongMarketSignal(blob)
	hasCorporate := hasCorporateFinanceSignal(blob)
	hasFinance := hasStrictFinanceSignal(blob)

	// 1) 비증권 주제 차단 — 강한 증시·종목·실적 근거가 없으면 섹터/산업 맥락만으로는 통과 불가
	if matchesExcludeTopic(blob) && !hasStrong && !hasSymbols && !hasCorporate {
		return false
	}

	// 1b) 사회·스캔들/허세 기사 — 기업명만 등장해도 증권 피드 제외
	if (matchesSocietyGossipTopic(blob) || matchesExcludeTopic(blob)) && hasSymbols && !hasFinance {
		return false
	}

	// 2) 종목/기업명 언급만으로는 허용하지 않음 — 증시·실적·섹터 전망 등 텍스트 근거 필요
	if hasSymbols && !hasFinanceContentSignal(blob) && !hasSectorWithIndustryContext(blob) {
		return false
	}

	// 3) 종목명 검색(네이버 등): 회사명이 제목에 있어도 anchor 없으면 차단
	if linkedSymbol != "" {
		if !hasRelevanceAnchor(title, blob, hasSymbols) {
			return false
		}
		return passesKeywordSearchGate(title, blob, searchTerms, linkedSymbol, hasSymbols)
	}

	// 4) 넓은 검색어(AI·외국인 등)로 찾은 기사 → 강한 금융 맥락 필수
	for _, term := range searchTerms {
		if isAmbiguousSearchTerm(term) && containsSearchTerm(blob, term) {
			if !hasStrictFinanceSignal(blob) {
				return false
			}
		}
	}

	if matchesEducationOrGeneralPolicy(blob) && !hasStrictFinanceSignal(blob) {
		return false
	}

	// 5) 키워드 검색·RSS 공통: relevance anchor + 키워드 검색 추가 게이트
	if !hasRelevanceAnchor(title, blob, hasSymbols) {
		return false
	}
	return passesKeywordSearchGate(title, blob, searchTerms, linkedSymbol, hasSymbols)
}

func passesDomesticCryptoSourceGate(
	ctx context.Context,
	st *NewsStore,
	sourceID, title, summary string,
	linkedSymbol string,
	searchTerms []string,
	storedSymbols []string,
) bool {
	if !isCryptoNewsSource(sourceID) {
		return true
	}
	blob := newsTextBlob(title, summary)
	symbols := st.extractSymbols(ctx, title+" "+summary)
	for _, sym := range storedSymbols {
		symbols = appendUnique(symbols, sym)
	}
	if linkedSymbol != "" {
		symbols = appendUnique(symbols, linkedSymbol)
	}
	if len(symbols) > 0 {
		return isStockRelated(ctx, st, title, summary, linkedSymbol, searchTerms, storedSymbols)
	}
	if !hasStockMarketKeyword(blob) {
		return false
	}
	if matchesExcludeTopic(blob) && !hasExplicitDomesticStockSignal(blob) {
		return false
	}
	return true
}

func (st *NewsStore) passesStockRelevance(
	ctx context.Context,
	item marketdata.RawNewsItem,
	searchTerms []string,
	linkedSymbol string,
) bool {
	if isCryptoNewsSource(item.SourceID) {
		return passesDomesticCryptoSourceGate(
			ctx, st, item.SourceID, item.Title, item.Description, linkedSymbol, searchTerms, nil,
		)
	}
	return isStockRelated(
		ctx,
		st,
		item.Title,
		item.Description,
		linkedSymbol,
		searchTerms,
		nil,
	)
}

func isStockRelatedRecord(ctx context.Context, st *NewsStore, rec repository.NewsArticleRecord) bool {
	linked := inferLinkedSymbolFromRecord(ctx, st, rec)
	if !passesDomesticCryptoSourceGate(
		ctx, st, rec.SourceID, rec.Headline, rec.Summary, linked, rec.Keywords, rec.Symbols,
	) {
		return false
	}
	return isStockRelated(
		ctx,
		st,
		rec.Headline,
		rec.Summary,
		linked,
		rec.Keywords,
		rec.Symbols,
	)
}

func isStockRelatedArticle(ctx context.Context, st *NewsStore, article model.NewsArticle) bool {
	linked := ""
	if len(article.Symbols) == 1 {
		linked = article.Symbols[0]
	}
	if !passesDomesticCryptoSourceGate(
		ctx, st, article.SourceID, article.Headline, article.Summary, linked, article.MatchedKeys, article.Symbols,
	) {
		return false
	}
	return isStockRelated(
		ctx,
		st,
		article.Headline,
		article.Summary,
		linked,
		article.MatchedKeys,
		article.Symbols,
	)
}

func inferLinkedSymbolFromRecord(ctx context.Context, st *NewsStore, rec repository.NewsArticleRecord) string {
	if len(rec.Symbols) != 1 || len(rec.Keywords) == 0 {
		return ""
	}
	sym := strings.ToUpper(strings.TrimSpace(rec.Symbols[0]))
	if sym == "" {
		return ""
	}
	st.rebuildNameIndex(ctx)
	st.mu.RLock()
	idx := st.nameIndex
	st.mu.RUnlock()
	for name, mapped := range idx {
		if mapped != sym {
			continue
		}
		for _, kw := range rec.Keywords {
			if strings.EqualFold(strings.TrimSpace(kw), name) {
				return sym
			}
		}
	}
	return ""
}
