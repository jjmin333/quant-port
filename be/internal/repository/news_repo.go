package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
)

type NewsArticleRecord struct {
	ID            string
	Market        string
	URLHash       string
	URL           string
	OriginalURL   string
	PortalURL     string
	Headline      string
	HeadlineKo    string
	Summary       string
	SummaryKo     string
	ImageURL      string
	PreviewMode   string
	SourceID      string
	PublisherID   string
	PublisherName string
	Tags          []string
	Symbols       []string
	Keywords      []string
	PublishedAt   time.Time
	ViewCount     int
	Highlighted   bool
	UpdatedAt     time.Time
	Topic         string // "" = stock
	AssetTags     []model.NewsAssetTag
}

type NewsListParams struct {
	Market   string
	SourceID string
	Sort     string
	Query    string
	Symbol   string
	Topic    string // "" = 전부
	Limit    int
}

type NewsRepo struct {
	db *sql.DB
}

func NewNewsRepo(db *sql.DB) *NewsRepo {
	return &NewsRepo{db: db}
}

/*
marshalJSONStringArray — 문자열 목록을 **항상 JSON 배열로** 만든다.

`json.Marshal`은 nil 슬라이스를 `[]`가 아니라 **`null`**로 쓴다. 그래서 종목을
하나도 못 찾은 기사는 `symbols_json`이 JSON `null`이 됐다 — 실측(2026-08-24)
93,469행 중 **28,470행**이 그 상태였다.

두 가지가 걸린다:

	① 「종목 없음」이 `[]`와 `null` 두 가지로 저장돼 의미가 갈린다.
	② **multi-valued index를 만들 수 없다.** MySQL의 `CAST(... AS ARRAY)`는
	   배열이나 SQL NULL만 받고 JSON `null`에서는 실패한다.

빈 배열로 통일하면 둘 다 사라진다. `tags`·`keywords`도 같은 코드 경로라
같은 문제를 갖고 있었으므로 함께 통과시킨다.
*/
func marshalJSONStringArray(items []string) []byte {
	if items == nil {
		items = []string{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return []byte("[]")
	}
	return raw
}

func (r *NewsRepo) Upsert(ctx context.Context, rec NewsArticleRecord) error {
	tags := marshalJSONStringArray(rec.Tags)
	symbols := marshalJSONStringArray(rec.Symbols)
	keywords := marshalJSONStringArray(rec.Keywords)
	previewMode := strings.TrimSpace(rec.PreviewMode)
	if previewMode == "" {
		previewMode = "summary"
	}
	publishedAt := NormalizeNewsPublishedAt(rec.PublishedAt)
	topic := rec.Topic
	if topic == "" {
		topic = model.NewsTopicStock
	}
	var assetTags any // NULL — 칩이 없는 기사가 대부분이다
	if len(rec.AssetTags) > 0 {
		b, _ := json.Marshal(rec.AssetTags)
		assetTags = string(b)
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO news_articles (
			id, market, url_hash, url, original_url, portal_url,
			headline, headline_ko, summary, summary_ko, image_url, preview_mode,
			source_id, publisher_id, publisher_name,
			tags_json, symbols_json, keywords_json,
			published_at, view_count, highlighted, topic, asset_tags_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			headline = VALUES(headline),
			headline_ko = IF(VALUES(headline_ko) != '', VALUES(headline_ko), headline_ko),
			summary = IF(VALUES(summary) != '', VALUES(summary), summary),
			summary_ko = IF(VALUES(summary_ko) != '', VALUES(summary_ko), summary_ko),
			original_url = IF(VALUES(original_url) != '', VALUES(original_url), original_url),
			portal_url = IF(VALUES(portal_url) != '', VALUES(portal_url), portal_url),
			publisher_id = IF(VALUES(publisher_id) != '', VALUES(publisher_id), publisher_id),
			publisher_name = IF(VALUES(publisher_name) != '', VALUES(publisher_name), publisher_name),
			image_url = IF(VALUES(image_url) != '', VALUES(image_url), image_url),
			preview_mode = IF(VALUES(preview_mode) != 'summary', VALUES(preview_mode), preview_mode),
			tags_json = VALUES(tags_json),
			symbols_json = VALUES(symbols_json),
			topic = VALUES(topic),
			asset_tags_json = VALUES(asset_tags_json),
			keywords_json = IF(JSON_LENGTH(VALUES(keywords_json)) > 0, VALUES(keywords_json), keywords_json),
			published_at = CASE
				WHEN VALUES(published_at) IS NULL OR VALUES(published_at) < '2000-01-01' THEN published_at
				WHEN published_at IS NULL OR published_at < '2000-01-01' THEN VALUES(published_at)
				WHEN VALUES(published_at) < published_at THEN VALUES(published_at)
				ELSE published_at
			END,
			updated_at = CURRENT_TIMESTAMP(3)
	`, rec.ID, rec.Market, rec.URLHash, rec.URL,
		nullIfEmpty(rec.OriginalURL), nullIfEmpty(rec.PortalURL),
		rec.Headline, nullIfEmpty(rec.HeadlineKo), nullIfEmpty(rec.Summary), nullIfEmpty(rec.SummaryKo),
		nullIfEmpty(rec.ImageURL), previewMode,
		rec.SourceID, nullIfEmpty(rec.PublisherID), nullIfEmpty(rec.PublisherName),
		tags, symbols, keywords, publishedAt, rec.ViewCount, rec.Highlighted, topic, assetTags)
	return err
}

func (r *NewsRepo) ExistsByURLHash(ctx context.Context, urlHash string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM news_articles WHERE url_hash = ?
	`, urlHash).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *NewsRepo) ExistsRecentHeadlineMatch(
	ctx context.Context,
	market, headline, headlineKo string,
	within time.Duration,
	sourceIDs ...string,
) (bool, error) {
	want := map[string]struct{}{}
	for _, key := range marketdata.NewsHeadlineMatchKeys(headline, headlineKo) {
		want[key] = struct{}{}
	}
	if len(want) == 0 {
		return false, nil
	}
	since := time.Now().UTC().Add(-within)

	query := `
		SELECT headline, COALESCE(headline_ko, '') FROM news_articles
		WHERE market = ? AND published_at >= ?
	`
	args := []any{market, since}
	if len(sourceIDs) == 1 && strings.TrimSpace(sourceIDs[0]) != "" {
		query += " AND source_id = ?"
		args = append(args, strings.TrimSpace(sourceIDs[0]))
	}
	query += " ORDER BY published_at DESC LIMIT 128"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var existing, existingKo string
		if err := rows.Scan(&existing, &existingKo); err != nil {
			return false, err
		}
		for _, key := range marketdata.NewsHeadlineMatchKeys(existing, existingKo) {
			if _, ok := want[key]; ok {
				return true, nil
			}
		}
	}
	return false, rows.Err()
}

func (r *NewsRepo) AppendKeywords(ctx context.Context, urlHash string, keywords []string) error {
	if r == nil || len(keywords) == 0 {
		return nil
	}
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT keywords_json FROM news_articles WHERE url_hash = ? LIMIT 1
	`, urlHash).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	existing := parseJSONStringSlice(raw)
	merged := mergeStringSlices(existing, keywords)
	if len(merged) == len(existing) {
		return nil
	}
	payload, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE news_articles SET keywords_json = ?, updated_at = CURRENT_TIMESTAMP(3)
		WHERE url_hash = ?
	`, payload, urlHash)
	return err
}

func parseJSONStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func mergeStringSlices(primary, extra []string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	for _, v := range primary {
		add(v)
	}
	for _, v := range extra {
		add(v)
	}
	return out
}

func (r *NewsRepo) PublishedAtByURLHash(ctx context.Context, urlHash string) (time.Time, error) {
	var at sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT published_at FROM news_articles WHERE url_hash = ? LIMIT 1
	`, urlHash).Scan(&at)
	if err != nil {
		return time.Time{}, err
	}
	if at.Valid {
		return at.Time, nil
	}
	return time.Time{}, nil
}

func (r *NewsRepo) IncrementViewCount(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE news_articles SET view_count = view_count + 1, updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, id)
	return err
}

func (r *NewsRepo) UpdateTranslation(ctx context.Context, id, headlineKo, summaryKo string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE news_articles
		SET headline_ko = ?, summary_ko = IF(? != '', ?, summary_ko), updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, headlineKo, summaryKo, summaryKo, id)
	return err
}

/*
translationMaxAttempts — 한 제목을 몇 번까지 다시 옮겨 볼 것인가.

번역 실패의 대부분은 그 제목 자체의 성질이다(고유명사 덩어리 · 숫자만 있는 제목).
일시적인 장애는 한 번의 재시도로 거의 다 걷힌다. 세 번은 그 둘 사이이고, 3에서
멈추면 최악의 낭비가 제목당 3회로 묶인다.

세는 자리는 서비스 쪽이다(`runUSTranslationBatch`) — 예산에 걸려 **못 보낸** 회차와
「보냈는데 쓸 수 없었다」를 거기서만 구분할 수 있다.
*/
const translationMaxAttempts = 3

/*
translationWindowDays — 얼마나 지난 기사까지 옮길 것인가.

두 가지를 한꺼번에 한다.

	· **비용** — 무료 등급은 월 200만 자다. 실측 신규 해외 기사가 하루 500~1,900건
	  (평균 제목 68자)이라 전량은 월 2.5M자로 한도를 넘는다. 오래된 기사를 빼는 것이
	  가장 덜 아픈 축소다 — 목록은 최신부터 읽히고, 3주 지난 제목을 여는 사람은 거의
	  없다.
	· **질의 비용** — 재시도 상한이 생기면서 「상한에 닿은 행」이 쌓인다. 창이 없으면
	  이 질의가 매 회차 그 무덤을 지나 24건을 찾아 뒤로 계속 걸어간다. 창을 두면
	  `idx_news_market_published`의 범위 훑기로 끝난다.

되돌리려면(전량 번역으로 가려면) 이 값을 늘리고 `translate_attempts`를 0으로 되돌린다.
*/
const translationWindowDays = 7

func (r *NewsRepo) ListMissingTranslation(ctx context.Context, market string, limit int) ([]NewsArticleRecord, error) {
	return r.ListMissingTranslationWindow(ctx, market, limit, translationWindowDays)
}

/*
ListMissingTranslationWindow는 창을 **부르는 쪽이 정하는** 판이다.

창은 원래 **문자에 돈이 붙는 엔진**을 전제로 한 절약이다(위 머리글). 자체 모델로
도는 동안에는 지킬 것이 없으므로, 한 번 비우는 일(`cmd/backfill-translations`)은
창을 통째로 열어 밀린 것을 전부 가져간다 — 2026-09-15 기준 7일 창 **밖**에만
578건이 남아 있었고, 그 행들은 창이 있는 한 영원히 안 걸린다.

주기 경로(`ListMissingTranslation`)의 기본값은 그대로 7일이다.
*/
func (r *NewsRepo) ListMissingTranslationWindow(
	ctx context.Context, market string, limit, windowDays int,
) ([]NewsArticleRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if windowDays <= 0 {
		windowDays = translationWindowDays
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+newsSelectCols+`
		FROM news_articles
		WHERE market = ?
		  AND published_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
		  AND (headline_ko IS NULL OR headline_ko = '')
		  AND headline != ''
		  AND translate_attempts < ?
		ORDER BY published_at DESC
		LIMIT ?
	`, market, windowDays, translationMaxAttempts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

/*
BumpTranslateAttempts marks headlines the engine answered but we could not use.

**한 문장으로 끝낸다.** 회차마다 스물넷이 실패할 수 있고 그때마다 UPDATE를 스물네
번 내면, 2분 주기가 DB에 내는 부하가 번역보다 커진다(운영 DB가 1 ECPU다).
*/
func (r *NewsRepo) BumpTranslateAttempts(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	q := `UPDATE news_articles SET translate_attempts = translate_attempts + 1
	      WHERE id IN (?` + strings.Repeat(",?", len(ids)-1) + `)`
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *NewsRepo) ListMissingImages(ctx context.Context, market string, limit int) ([]NewsArticleRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+newsSelectCols+`
		FROM news_articles
		WHERE market = ? AND (image_url IS NULL OR image_url = '')
		ORDER BY published_at DESC
		LIMIT ?
	`, market, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

func (r *NewsRepo) ListRecentForImageReprobe(ctx context.Context, market string, limit int) ([]NewsArticleRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+newsSelectCols+`
		FROM news_articles
		WHERE market = ? AND COALESCE(original_url, '') != ''
		ORDER BY published_at DESC
		LIMIT ?
	`, market, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

func (r *NewsRepo) UpdateImageURL(ctx context.Context, id, imageURL string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE news_articles SET image_url = ?, updated_at = CURRENT_TIMESTAMP(3) WHERE id = ?
	`, imageURL, id)
	return err
}

func (r *NewsRepo) List(ctx context.Context, params NewsListParams) ([]NewsArticleRecord, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		if params.Market == "us" {
			if limit > 300 {
				limit = 300
			}
		} else {
			limit = 200
		}
	}

	order := "published_at DESC"
	if params.Sort == "popular" {
		order = "view_count DESC, published_at DESC"
	}

	query := `
		SELECT `+newsSelectCols+`
		FROM news_articles
		WHERE market = ?
	`
	args := []any{params.Market}

	if params.Market == "us" {
		query += usTranslationReadySQL()
	}

	if src := strings.TrimSpace(params.SourceID); src != "" && src != "all" {
		query += " AND source_id = ?"
		args = append(args, src)
	}
	if sym := strings.ToUpper(strings.TrimSpace(params.Symbol)); sym != "" {
		// MEMBER OF 라야 idx_news_symbols_mv(multi-valued index)를 탄다 —
		// JSON_CONTAINS는 같은 결과를 주지만 인덱스를 못 쓴다(061 마이그레이션 참조).
		query += ` AND ? MEMBER OF (symbols_json->'$')`
		args = append(args, sym)
	}
	if topic := strings.TrimSpace(params.Topic); topic != "" {
		query += " AND topic = ?"
		args = append(args, topic)
	}
	if q := strings.TrimSpace(params.Query); q != "" {
		like := "%" + q + "%"
		query += " AND (headline LIKE ? OR summary LIKE ? OR headline_ko LIKE ? OR summary_ko LIKE ?)"
		args = append(args, like, like, like, like)
	}
	query += " ORDER BY " + order + " LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

func (r *NewsRepo) LatestUpdatedAt(ctx context.Context, market string) (time.Time, error) {
	var at sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT MAX(updated_at) FROM news_articles WHERE market = ?
	`, market).Scan(&at)
	if err != nil {
		return time.Time{}, err
	}
	if at.Valid {
		return at.Time, nil
	}
	return time.Time{}, nil
}

func (r *NewsRepo) Count(ctx context.Context, market string) (int, error) {
	query := `SELECT COUNT(1) FROM news_articles WHERE market = ?`
	args := []any{market}
	if market == "us" {
		query += usTranslationReadySQL()
	}
	var n int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

func usTranslationReadySQL() string {
	return " AND COALESCE(headline_ko, '') != ''"
}

func (r *NewsRepo) ListPage(ctx context.Context, market string, offset, limit int) ([]NewsArticleRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+newsSelectCols+`
		FROM news_articles
		WHERE market = ?
		ORDER BY id
		LIMIT ? OFFSET ?
	`, market, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNewsRows(rows)
}

// UpdateSymbols rewrites one article's symbols_json. Used by the retag pass
// that re-derives symbols after the extraction rules change — it must not touch
// updated_at, or every retagged row would look freshly ingested to the merge
// logic that orders the feed.
func (r *NewsRepo) UpdateSymbols(ctx context.Context, id string, symbols []string) error {
	if symbols == nil {
		symbols = []string{}
	}
	payload, err := json.Marshal(symbols)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE news_articles SET symbols_json = ? WHERE id = ?`, string(payload), id)
	return err
}

// UpdateMarket moves an article between domestic and US feeds.
func (r *NewsRepo) UpdateMarket(ctx context.Context, id, market string) error {
	market = strings.ToLower(strings.TrimSpace(market))
	if market != "us" && market != "domestic" {
		return fmt.Errorf("invalid market %q", market)
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE news_articles SET market = ?, updated_at = CURRENT_TIMESTAMP(3) WHERE id = ?`,
		market, id)
	return err
}

// UpdateTags rewrites one article's tags_json without touching updated_at.
func (r *NewsRepo) UpdateTags(ctx context.Context, id string, tags []string) error {
	if tags == nil {
		tags = []string{}
	}
	payload, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE news_articles SET tags_json = ? WHERE id = ?`, string(payload), id)
	return err
}

func (r *NewsRepo) DeleteByIDs(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var total int64
	for start := 0; start < len(ids); start += 100 {
		end := start + 100
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		query := "DELETE FROM news_articles WHERE id IN (?" + strings.Repeat(",?", len(chunk)-1) + ")"
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		res, err := r.db.ExecContext(ctx, query, args...)
		if err != nil {
			return total, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// newsSelectCols — scanNewsRow와 짝인 열 목록. 조회마다 같은 목록을 따로 적으면 열을 하나 더할 때 어긋난다
// (2026-10-09: topic · asset_tags_json을 더하며 다섯 벌을 하나로).
const newsSelectCols = `id, market, url_hash, url, COALESCE(original_url, ''), COALESCE(portal_url, ''),
			headline, COALESCE(headline_ko, ''), COALESCE(summary, ''), COALESCE(summary_ko, ''),
			COALESCE(image_url, ''), COALESCE(preview_mode, 'summary'),
			source_id, COALESCE(publisher_id, ''), COALESCE(publisher_name, ''),
			tags_json, symbols_json, keywords_json, published_at, view_count, highlighted, updated_at,
			topic, asset_tags_json`

func scanNewsRows(rows *sql.Rows) ([]NewsArticleRecord, error) {
	var out []NewsArticleRecord
	for rows.Next() {
		rec, err := scanNewsRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	return out, rows.Err()
}

func scanNewsRow(scanner interface {
	Scan(dest ...any) error
}) (*NewsArticleRecord, error) {
	var rec NewsArticleRecord
	var tagsRaw, symbolsRaw, keywordsRaw, assetTagsRaw []byte
	if err := scanner.Scan(
		&rec.ID, &rec.Market, &rec.URLHash, &rec.URL, &rec.OriginalURL, &rec.PortalURL,
		&rec.Headline, &rec.HeadlineKo, &rec.Summary, &rec.SummaryKo, &rec.ImageURL, &rec.PreviewMode,
		&rec.SourceID, &rec.PublisherID, &rec.PublisherName,
		&tagsRaw, &symbolsRaw, &keywordsRaw, &rec.PublishedAt, &rec.ViewCount, &rec.Highlighted, &rec.UpdatedAt,
		&rec.Topic, &assetTagsRaw,
	); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(assetTagsRaw, &rec.AssetTags)
	_ = json.Unmarshal(tagsRaw, &rec.Tags)
	_ = json.Unmarshal(symbolsRaw, &rec.Symbols)
	_ = json.Unmarshal(keywordsRaw, &rec.Keywords)
	rec.PublishedAt = NormalizeNewsPublishedAt(rec.PublishedAt)
	return &rec, nil
}

/*
KeywordArticleCounts — 검색어별로 **실제로 들어온 기사 수**를 센다.

수집은 검색어를 돌아가며 부른다(`service.NewsStore.nextKeywords`). 그 큐에는
「코스피」처럼 하루 수십 건을 내는 말과, 7일 동안 한 건도 못 낸 보유 종목이 **같은
자리**를 차지하고 있었다. 큐가 225개라 한 바퀴가 25분이고, 그래서 시장 뉴스가 평균
12.5분 늦게 들어왔다(실측 2026-09-17: naver 수집 지연 평균 723초).

그래서 차선을 가르는 데 쓸 값이 필요하다 — **무엇이 뉴스를 내는 말인가.** 그 답은
추측할 것이 아니라 이미 우리 표에 있다.

`keywords_json`의 **첫 칸**만 센다. 수집 경로가 기사를 넣을 때 자기 검색어를 그
자리에 적기 때문이다(`ingestItem(ctx, item, []string{kw.Text}, …)`). 뒤 칸은 나중에
붙는 추출 키워드라 「이 검색어가 낸 기사」와는 다른 뜻이다.

기준은 `published_at`이다 — `(market, published_at)` 인덱스를 그대로 타고, 이 값이
묻는 것은 「그 말이 얼마나 자주 기사를 만드는가」라 발행 시각이 더 맞다.
*/
func (r *NewsRepo) KeywordArticleCounts(
	ctx context.Context,
	market string,
	since time.Time,
) (map[string]int, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT JSON_UNQUOTE(JSON_EXTRACT(keywords_json, '$[0]')) AS kw, COUNT(*)
		FROM news_articles
		WHERE market = ? AND published_at >= ? AND keywords_json IS NOT NULL
		GROUP BY kw
	`, market, since.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var kw sql.NullString
		var n int
		if err := rows.Scan(&kw, &n); err != nil {
			return nil, err
		}
		key := strings.ToLower(strings.TrimSpace(kw.String))
		if key == "" {
			continue
		}
		out[key] += n
	}
	return out, rows.Err()
}

// NewsTopicRow is the slice of an article the topic retag needs.
type NewsTopicRow struct {
	ID, Headline, Summary, Topic string
	AssetTags                    []model.NewsAssetTag
}

// TopicRetagCandidates — 국내 · 종목 없음 기사 전부(주제 포함 — 분류 규칙이 바뀌면 되돌리기도 한다, 2026-10-09).
func (r *NewsRepo) TopicRetagCandidates(ctx context.Context, since time.Time, limit int) ([]NewsTopicRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, headline, COALESCE(summary, ''), topic, asset_tags_json
		  FROM news_articles
		 WHERE market = 'domestic' AND published_at >= ?
		   AND (symbols_json IS NULL OR JSON_LENGTH(symbols_json) = 0)
		 ORDER BY published_at DESC
		 LIMIT ?`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NewsTopicRow
	for rows.Next() {
		var x NewsTopicRow
		var tags []byte
		if err := rows.Scan(&x.ID, &x.Headline, &x.Summary, &x.Topic, &tags); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(tags, &x.AssetTags)
		out = append(out, x)
	}
	return out, rows.Err()
}

// UpdateTopic sets one article's topic and asset chips.
func (r *NewsRepo) UpdateTopic(ctx context.Context, id, topic string, tags []model.NewsAssetTag) error {
	var raw any
	if len(tags) > 0 {
		b, _ := json.Marshal(tags)
		raw = string(b)
	}
	_, err := r.db.ExecContext(ctx, `UPDATE news_articles SET topic = ?, asset_tags_json = ? WHERE id = ?`, topic, raw, id)
	return err
}

// TopicHeadlines — 국내 주제 기사 최근 제목(최신순). 같은 사건이 여러 매체로 오므로 앞 18자가 같으면 하나만,
// 60자에서 자른다 — LLM 입력 토큰 관리(2026-10-09 요청 5).
func (r *NewsRepo) TopicHeadlines(ctx context.Context, topics []string, since time.Time, limit int) ([]string, error) {
	if len(topics) == 0 || limit <= 0 {
		return nil, nil
	}
	args := []any{since}
	for _, t := range topics {
		args = append(args, t)
	}
	args = append(args, limit*4)
	rows, err := r.db.QueryContext(ctx, `
		SELECT headline FROM news_articles
		 WHERE market = 'domestic' AND published_at >= ? AND topic IN (?`+strings.Repeat(",?", len(topics)-1)+`)
		 ORDER BY published_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		h = strings.TrimSpace(h)
		rs := []rune(strings.Join(strings.Fields(h), ""))
		key := string(rs[:min(18, len(rs))])
		if h == "" || seen[key] || len(out) >= limit {
			continue
		}
		seen[key] = true
		if hr := []rune(h); len(hr) > 60 {
			h = string(hr[:60]) + "…"
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// RegionNews is one 시군구's recent topic coverage (자산 칩 기준).
type RegionNews struct {
	Count  int
	Latest string
}

// RegionNewsStats — 시군구(자산 칩 region)별 기사 수와 최신 제목. 부동산 점수의 「근거」 칸(2026-10-09 결정 A — 점수식은 그대로).
func (r *NewsRepo) RegionNewsStats(ctx context.Context, since time.Time) (map[string]RegionNews, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT jt.k, COUNT(*),
		       SUBSTRING_INDEX(GROUP_CONCAT(n.headline ORDER BY n.published_at DESC SEPARATOR '\n'), '\n', 1)
		  FROM news_articles n
		  JOIN JSON_TABLE(n.asset_tags_json, '$[*]' COLUMNS (kind VARCHAR(10) PATH '$.kind', k VARCHAR(20) PATH '$.key')) jt
		 WHERE n.market = 'domestic' AND n.topic <> 'stock' AND n.published_at >= ? AND jt.kind = 'region'
		 GROUP BY jt.k`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]RegionNews{}
	for rows.Next() {
		var k string
		var v RegionNews
		if err := rows.Scan(&k, &v.Count, &v.Latest); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
