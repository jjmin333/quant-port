package marketdata

import (
	"net/url"
	"strings"
)

type Publisher struct {
	ID   string
	Name string
}

var publisherRules = []struct {
	Match string
	ID    string
	Name  string
}{
	{Match: "mk.co.kr", ID: "maeil", Name: "매일경제"},
	{Match: "hankyung.com", ID: "hankyung", Name: "한국경제"},
	{Match: "sedaily.com", ID: "sedaily", Name: "서울경제"},
	{Match: "yonhapnewstv.co.kr", ID: "yonhap", Name: "연합뉴스"},
	{Match: "yna.co.kr", ID: "yonhap", Name: "연합뉴스"},
	{Match: "fnnews.com", ID: "financial-news", Name: "파이낸셜뉴스"},
	{Match: "etnews.com", ID: "etnews", Name: "전자신문"},
	{Match: "mt.co.kr", ID: "mt", Name: "머니투데이"},
	{Match: "news.naver.com", ID: "naver-news", Name: "네이버뉴스"},
	{Match: "n.news.naver.com", ID: "naver-news", Name: "네이버뉴스"},
	{Match: "tf.co.kr", ID: "the-fact", Name: "더팩트"},
	{Match: "tokenpost.kr", ID: "tokenpost", Name: "토큰포스트"},
	{Match: "coinreaders.com", ID: "coinreaders", Name: "코인리더스"},
	{Match: "blockmedia.co.kr", ID: "blockmedia", Name: "블록미디어"},
	{Match: "kmaeil.com", ID: "kmaeil", Name: "경북매일"},
	{Match: "newspim.com", ID: "newspim", Name: "뉴스핌"},
	{Match: "bizwatch.co.kr", ID: "bizwatch", Name: "비즈워치"},
	{Match: "newsis.com", ID: "newsis", Name: "뉴시스"},
	{Match: "heraldcorp.com", ID: "herald", Name: "헤럴드경제"},
	{Match: "chosun.com", ID: "chosun", Name: "조선일보"},
	{Match: "joongang.co.kr", ID: "joongang", Name: "중앙일보"},
	{Match: "donga.com", ID: "donga", Name: "동아일보"},
	{Match: "hani.co.kr", ID: "hani", Name: "한겨레"},
	{Match: "kbs.co.kr", ID: "kbs", Name: "KBS"},
	{Match: "sbs.co.kr", ID: "sbs", Name: "SBS"},
	{Match: "mbn.co.kr", ID: "mbn", Name: "MBN"},
	{Match: "cnbc.com", ID: "cnbc", Name: "CNBC"},
	{Match: "marketwatch.com", ID: "marketwatch", Name: "MarketWatch"},
	{Match: "finance.yahoo.com", ID: "yahoo-finance", Name: "Yahoo Finance"},
	{Match: "yahoo.com", ID: "yahoo-finance", Name: "Yahoo Finance"},
	{Match: "financialjuice.com", ID: "financial-juice", Name: "FinancialJuice"},
	{Match: "reuters.com", ID: "reuters", Name: "Reuters"},
	{Match: "bloomberg.com", ID: "bloomberg", Name: "Bloomberg"},
	{Match: "investing.com", ID: "investing", Name: "Investing.com"},
	{Match: "seekingalpha.com", ID: "seekingalpha", Name: "Seeking Alpha"},
	{Match: "benzinga.com", ID: "benzinga", Name: "Benzinga"},
	{Match: "fool.com", ID: "motley-fool", Name: "Motley Fool"},
	{Match: "nasdaq.com", ID: "nasdaq", Name: "Nasdaq"},
}

// ResolvePublisher maps an article URL to the original media outlet.
func ResolvePublisher(rawURL string) Publisher {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return Publisher{ID: "unknown", Name: "기타"}
	}

	host := strings.ToLower(rawURL)
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Host != "" {
		host = strings.ToLower(parsed.Host)
	}
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")

	for _, rule := range publisherRules {
		if strings.Contains(host, rule.Match) {
			return Publisher{ID: rule.ID, Name: rule.Name}
		}
	}

	label := host
	if dot := strings.Index(label, "."); dot > 0 {
		label = label[:dot]
	}
	if label == "" {
		return Publisher{ID: "unknown", Name: "기타"}
	}
	return Publisher{ID: "unknown", Name: label}
}

// ReaderAllowedHost reports whether Reader API may fetch the article URL host.
func ReaderAllowedHost(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return false
	}
	host := strings.ToLower(rawURL)
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Host != "" {
		host = strings.ToLower(parsed.Host)
	}
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	if strings.Contains(host, "news.naver.com") || strings.Contains(host, "n.news.naver.com") {
		return false
	}
	for _, rule := range publisherRules {
		if strings.Contains(host, rule.Match) {
			return true
		}
	}
	return strings.HasSuffix(host, "reuters.com") ||
		strings.HasSuffix(host, "federalreserve.gov")
}
