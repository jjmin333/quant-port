package marketdata

import (
	"net/url"
	"regexp"
	"strings"
)

// naverPressByOID — Naver news portal press codes (oid).
// When originallink points to a wire (e.g. newspim.com), oid from the portal URL is the displayed outlet.
var naverPressByOID = map[string]Publisher{
	"001": {ID: "yonhap", Name: "연합뉴스"},
	"003": {ID: "newsis", Name: "뉴시스"},
	"008": {ID: "mt", Name: "머니투데이"},
	"009": {ID: "maeil", Name: "매일경제"},
	"011": {ID: "sedaily", Name: "서울경제"},
	"014": {ID: "financial-news", Name: "파이낸셜뉴스"},
	"015": {ID: "hankyung", Name: "한국경제"},
	"016": {ID: "herald", Name: "헤럴드경제"},
	"018": {ID: "etnews", Name: "전자신문"},
	"019": {ID: "mbn", Name: "MBN"},
	"020": {ID: "donga", Name: "동아일보"},
	"021": {ID: "donga", Name: "문화일보"},
	"022": {ID: "donga", Name: "세계일보"},
	"023": {ID: "chosun", Name: "조선일보"},
	"025": {ID: "joongang", Name: "중앙일보"},
	"028": {ID: "hani", Name: "한겨레"},
	"030": {ID: "etnews", Name: "전자신문"},
	"031": {ID: "unknown", Name: "아이뉴스24"},
	"052": {ID: "yonhap", Name: "YTN"},
	"055": {ID: "sbs", Name: "SBS"},
	"056": {ID: "kbs", Name: "KBS"},
	"057": {ID: "mbn", Name: "MBN"},
	"079": {ID: "newsis", Name: "노컷뉴스"},
	"081": {ID: "donga", Name: "서울신문"},
	"119": {ID: "herald", Name: "데일리안"},
	"215": {ID: "the-fact", Name: "더팩트"},
	"227": {ID: "hankyung", Name: "아시아경제"},
	"277": {ID: "the-fact", Name: "더팩트"},
	"366": {ID: "chosun", Name: "조선비즈"},
	"421": {ID: "yonhap", Name: "뉴스1"},
	"437": {ID: "yonhap", Name: "JTBC"},
	"448": {ID: "yonhap", Name: "TV조선"},
}

var naverArticlePathOID = regexp.MustCompile(`/article/(\d{3})/`)

// ParseNaverPressOID extracts oid from Naver news portal URLs.
func ParseNaverPressOID(rawURL string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false
	}
	if !isNaverNewsPortalURL(rawURL) {
		return "", false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	if oid := strings.TrimSpace(parsed.Query().Get("oid")); oid != "" {
		return padOID(oid), true
	}
	if m := naverArticlePathOID.FindStringSubmatch(parsed.Path); len(m) > 1 {
		return m[1], true
	}
	return "", false
}

func padOID(oid string) string {
	oid = strings.TrimSpace(oid)
	for len(oid) < 3 {
		oid = "0" + oid
	}
	if len(oid) > 3 {
		oid = oid[len(oid)-3:]
	}
	return oid
}

func isNaverNewsPortalURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "news.naver.com") || strings.Contains(lower, "n.news.naver.com")
}

// PublisherFromNaverPortal resolves the outlet from a Naver portal link (oid).
func PublisherFromNaverPortal(portalURL string) (Publisher, bool) {
	oid, ok := ParseNaverPressOID(portalURL)
	if !ok {
		return Publisher{}, false
	}
	if pub, ok := naverPressByOID[oid]; ok {
		return pub, true
	}
	return Publisher{ID: "unknown", Name: "언론사 " + oid}, true
}

// wireHosts — originallink often points here; prefer Naver oid when portal link exists.
var wireHosts = []string{
	"newspim.com",
	"newsis.com",
	"yna.co.kr",
}

func isWireHost(host string) bool {
	host = strings.ToLower(host)
	for _, w := range wireHosts {
		if strings.Contains(host, w) {
			return true
		}
	}
	return false
}

func hostOf(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	h := strings.ToLower(parsed.Host)
	h = strings.TrimPrefix(h, "www.")
	h = strings.TrimPrefix(h, "m.")
	return h
}

// ResolvePublisherFromArticle picks the best outlet label for a Naver-sourced article.
// Portal oid wins when originallink is a wire/redistributor URL.
func ResolvePublisherFromArticle(portalURL, originalURL string) Publisher {
	portalURL = strings.TrimSpace(portalURL)
	originalURL = strings.TrimSpace(originalURL)

	if portalURL != "" {
		if pub, ok := PublisherFromNaverPortal(portalURL); ok {
			if originalURL == "" || isWireHost(hostOf(originalURL)) || isNaverNewsPortalURL(originalURL) {
				return pub
			}
		}
	}

	primary := originalURL
	if primary == "" {
		primary = portalURL
	}
	pub := ResolvePublisher(primary)

	// Still a raw host slug — try portal oid as fallback.
	if pub.ID == "unknown" && portalURL != "" {
		if byPortal, ok := PublisherFromNaverPortal(portalURL); ok {
			return byPortal
		}
	}
	return pub
}
