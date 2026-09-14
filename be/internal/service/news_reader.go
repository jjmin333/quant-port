package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	"github.com/redis/go-redis/v9"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

const (
	readerCachePrefix   = "news:reader:v1:"
	readerKoCachePrefix = "news:reader:ko:v1:"
	readerCacheTTL      = time.Hour
	readerKoCacheTTL    = 24 * time.Hour
)

type ReaderExtractOptions struct {
	Market    string
	ArticleID string
}

type NewsReaderService struct {
	httpClient *http.Client
	redis      *redis.Client
	transRepo  *repository.NewsReaderTranslationRepo
	newsRepo   *repository.NewsRepo
}

func NewNewsReaderService(
	redisClient *redis.Client,
	transRepo *repository.NewsReaderTranslationRepo,
	newsRepo *repository.NewsRepo,
) *NewsReaderService {
	return &NewsReaderService{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		redis:      redisClient,
		transRepo:  transRepo,
		newsRepo:   newsRepo,
	}
}

func (s *NewsReaderService) Extract(ctx context.Context, rawURL string, opts ReaderExtractOptions) (*model.NewsReaderContent, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	if err := validateReaderURL(rawURL); err != nil {
		return nil, err
	}

	market := strings.ToLower(strings.TrimSpace(opts.Market))
	if market == "" {
		market = "domestic"
	}

	if market == "us" {
		if cached, err := s.loadKoCache(ctx, rawURL); err == nil && cached != nil {
			return cached, nil
		}
		if stored, err := s.loadStoredTranslation(ctx, rawURL); err == nil && stored != nil {
			s.saveKoCache(ctx, rawURL, stored)
			return stored, nil
		}
	}

	base, err := s.extractEnglish(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if market != "us" {
		return base, nil
	}

	// 저장된 번역이 없으면 원문을 그대로 준다 — 아래 주석 참고.
	return base, nil
}

func (s *NewsReaderService) extractEnglish(ctx context.Context, rawURL string) (*model.NewsReaderContent, error) {
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, readerCacheKey(rawURL)).Bytes(); err == nil && len(cached) > 0 {
			var out model.NewsReaderContent
			if json.Unmarshal(cached, &out) == nil {
				return &out, nil
			}
		}
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	if isKoreanNewsURL(rawURL) {
		req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9,en;q=0.8")
	} else {
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	article, err := readability.FromReader(strings.NewReader(string(body)), parsed)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	out := &model.NewsReaderContent{
		URL:         rawURL,
		Title:       strings.TrimSpace(article.Title),
		Byline:      strings.TrimSpace(article.Byline),
		Excerpt:     strings.TrimSpace(article.Excerpt),
		SiteName:    strings.TrimSpace(article.SiteName),
		ImageURL:    strings.TrimSpace(article.Image),
		TextContent: strings.TrimSpace(article.TextContent),
		ContentHTML: strings.TrimSpace(article.Content),
		FetchedAt:   time.Now(),
	}
	if article.PublishedTime != nil && !article.PublishedTime.IsZero() {
		out.PublishedAt = *article.PublishedTime
	}
	if out.Title == "" && out.TextContent == "" {
		return nil, fmt.Errorf("no readable content extracted")
	}

	if s.redis != nil {
		if payload, err := json.Marshal(out); err == nil {
			_ = s.redis.Set(ctx, readerCacheKey(rawURL), payload, readerCacheTTL).Err()
		}
	}
	return out, nil
}

/*
**리더 본문은 더 이상 번역하지 않는다.**

여기 있던 `translateUSReader`는 Papago의 웹 번역(HTML 구조를 지키며 통째로 옮기는
엔드포인트)을 불렀다. NCP 자격증명이 만료됐고 갱신하지 않기로 했으므로(2026-09-03)
이 경로는 어차피 오류만 내고 있었다 — 호출자가 그 오류를 삼키고 원문을 돌려주므로
화면에는 티가 안 났다. 안 도는 코드를 남겨 두면 다음 사람이 「왜 번역이 안 되지」를
여기서부터 파게 된다.

자체 모델로 대신하지 않는 이유는 성격이 달라서다. 제목은 배치라 한 건에 몇 초가
걸려도 되지만, 리더 본문은 **사람이 화면을 보며 기다리고** 기사 한 건이 2,000단어다.
2 OCPU ARM에서 소형 모델로 돌리면 분 단위가 걸린다(be/docs/TRANSLATION_SELFHOST.md §1).

**이미 저장된 번역은 그대로 쓴다** — `loadStoredTranslation`이 읽는 그 행들은
Papago가 살아 있을 때 만든 진짜 번역이라 버릴 이유가 없다. 새로 만들지 않을 뿐이다.
*/
func (s *NewsReaderService) resolveReaderTitleKo(ctx context.Context, articleID, fallback string) string {
	articleID = strings.TrimSpace(articleID)
	if articleID != "" && s.newsRepo != nil {
		if rec, err := s.newsRepo.GetByID(ctx, articleID); err == nil {
			if ko := strings.TrimSpace(rec.HeadlineKo); ko != "" {
				return ko
			}
		}
	}
	// 저장된 한국어 제목이 없으면 원문 제목을 그대로 쓴다. 여기서 번역기를 부르지
	// 않는 이유는 리더를 여는 사람이 그 몇 초를 기다리게 되기 때문이다 — 제목 번역은
	// 2분마다 도는 배치의 몫이고, 그 배치가 끝나면 다음 열람부터 한국어로 보인다.
	return strings.TrimSpace(fallback)
}

func (s *NewsReaderService) resolveReaderExcerptKo(ctx context.Context, articleID, fallback string) string {
	articleID = strings.TrimSpace(articleID)
	if articleID != "" && s.newsRepo != nil {
		if rec, err := s.newsRepo.GetByID(ctx, articleID); err == nil {
			if ko := strings.TrimSpace(rec.SummaryKo); ko != "" {
				return ko
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func (s *NewsReaderService) loadStoredTranslation(ctx context.Context, rawURL string) (*model.NewsReaderContent, error) {
	if s.transRepo == nil {
		return nil, fmt.Errorf("translation repo not configured")
	}
	rec, err := s.transRepo.GetByURLHash(ctx, newsURLHash(rawURL))
	if err != nil {
		return nil, err
	}
	base, err := s.extractEnglish(ctx, rawURL)
	if err != nil {
		base = &model.NewsReaderContent{
			URL:       rawURL,
			FetchedAt: time.Now(),
		}
	}
	out := *base
	out.Title = rec.TitleKo
	out.Excerpt = rec.ExcerptKo
	out.ContentHTML = rec.ContentHTMLKo
	if text := strings.TrimSpace(rec.TextContentKo); text != "" {
		out.TextContent = text
	} else if text := stripHTMLToText(rec.ContentHTMLKo); text != "" {
		out.TextContent = text
	}
	out.OriginalTitle = base.Title
	out.OriginalExcerpt = base.Excerpt
	out.Translated = true
	return &out, nil
}

func (s *NewsReaderService) loadKoCache(ctx context.Context, rawURL string) (*model.NewsReaderContent, error) {
	if s.redis == nil {
		return nil, fmt.Errorf("redis unavailable")
	}
	cached, err := s.redis.Get(ctx, readerKoCacheKey(rawURL)).Bytes()
	if err != nil || len(cached) == 0 {
		return nil, err
	}
	var out model.NewsReaderContent
	if err := json.Unmarshal(cached, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *NewsReaderService) saveKoCache(ctx context.Context, rawURL string, content *model.NewsReaderContent) {
	if s.redis == nil || content == nil {
		return
	}
	payload, err := json.Marshal(content)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, readerKoCacheKey(rawURL), payload, readerKoCacheTTL).Err()
}

func readerHTMLForTranslation(base *model.NewsReaderContent) string {
	if base == nil {
		return ""
	}
	if html := strings.TrimSpace(base.ContentHTML); html != "" {
		return html
	}
	text := strings.TrimSpace(base.TextContent)
	if text == "" {
		return ""
	}
	paras := strings.Split(text, "\n\n")
	var b strings.Builder
	b.WriteString("<article>")
	if title := strings.TrimSpace(base.Title); title != "" {
		b.WriteString("<h1>")
		b.WriteString(htmlEscape(title))
		b.WriteString("</h1>")
	}
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		b.WriteString("<p>")
		b.WriteString(htmlEscape(p))
		b.WriteString("</p>")
	}
	b.WriteString("</article>")
	return b.String()
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

func stripHTMLToText(html string) string {
	html = strings.TrimSpace(html)
	if html == "" {
		return ""
	}
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func validateReaderURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported url scheme")
	}
	if !marketdata.ReaderAllowedHost(rawURL) {
		return fmt.Errorf("reader host not allowed: %s", parsed.Hostname())
	}
	return nil
}

func isKoreanNewsURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.HasSuffix(host, ".kr") ||
		strings.Contains(host, "hankyung") ||
		strings.Contains(host, "mk.co.kr")
}

func readerCacheKey(rawURL string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(rawURL))))
	return readerCachePrefix + hex.EncodeToString(sum[:16])
}

func readerKoCacheKey(rawURL string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(rawURL))))
	return readerKoCachePrefix + hex.EncodeToString(sum[:16])
}
