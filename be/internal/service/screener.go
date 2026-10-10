package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
종목 스크리너 (V2 요구 4).

# 조회 경로에 외부 호출도 LLM도 없다

전부 `stock_scores` 한 표에서 나온다. 한 요청에 조회 두 번(깔때기 집계 1 + 결과 1)
이고, 조건 목록 화면은 세 번째 조회를 쓰되 **캐시한다**.

# 커버리지를 캐시하는 이유

조건 목록(`Catalog`)은 필드 23개의 측정 종목 수를 세는데, 그날 전체를 한 번 스캔한다.
점수는 **하루 한 번** 계산되므로 이 숫자는 하루 동안 변하지 않는다 — 화면을 열
때마다 다시 세는 것은 정확도가 아니라 부하만 사는 일이다(`stock_market_caps.go`의
스냅샷 캐시와 같은 판단).
*/

// ScreenerService answers screener queries.
type ScreenerService struct {
	repo    *repository.ScreenerRepo
	scores  *ScoreService
	screens *repository.UserScreenRepo

	mu      sync.RWMutex
	catalog map[string]screenCatalogEntry
	// results — 시세를 덮기 **전** 결과(깔때기·행·커버리지). 키에 거래일이 들어간다.
	results map[string]screenResultEntry
	// presetStats — 「점수일|시장」 → 프리셋 카드별 통과 수 · 앞 점수일 수 · 새로 든 종목(screener_presets_count.go).
	presetStats map[string][]presetStat
	refresh *quoteRefreshCoordinator
}

type screenResultEntry struct {
	data model.ScreenResult
	at   time.Time
}

/*
screenResultTTL · screenResultMax — 스크리닝 결과 캐시.

한 번의 스크리닝이 집계 조회 셋(깔때기·행·커버리지)이라 실측 ~300ms(09-25 운영 로그
335~736ms)였다. 재료는 하루 한 번 도는 점수 배치뿐이고 키에 거래일이 들어가므로, 같은
조건이면 같은 답이다. 시세는 캐시하지 않는다 — 요청마다 overlayLivePrices가 덮는다.
사용자 입력이 키라 무한히 늘 수 있어 상한을 두고, 넘치면 통째로 비운다(값싼 자료다).
*/
const (
	screenResultTTL = 15 * time.Minute
	screenResultMax = 128
)

type screenCatalogEntry struct {
	data model.ScreenCatalog
	at   time.Time
}

/*
screenCatalogTTL — 커버리지 캐시 수명.

점수 배치는 하루 한 번 돈다. 15분은 「배치가 막 돌았을 때 얼마나 빨리 새 숫자가
보이나」와 「같은 숫자를 몇 번 다시 세나」의 절충이고, 저장소의 다른 스냅샷 캐시와
같은 값이다.
*/
const screenCatalogTTL = 15 * time.Minute

func NewScreenerService(repo *repository.ScreenerRepo, scores *ScoreService) *ScreenerService {
	return &ScreenerService{
		repo: repo, scores: scores,
		catalog: map[string]screenCatalogEntry{},
		results: map[string]screenResultEntry{},
		refresh: newQuoteRefreshCoordinator(),
	}
}

/*
screenAssetTypes — 시장 탭을 `stocks.asset_type`으로.

랭킹과 **같은 구분**을 쓴다(`stock_score.go`의 domestic/overseas). 두 화면이 다른
우주를 보면 같은 종목이 한쪽에만 나타난다.
*/
func screenAssetTypes(market string) ([]string, string) {
	if strings.EqualFold(strings.TrimSpace(market), "overseas") {
		return overseasAssetTypes, "overseas"
	}
	return domesticAssetTypes, "domestic"
}

/*
normalizeScreenQuery — 요청을 **거절하거나** 안전한 모양으로 만든다.

화이트리스트에 없는 필드는 조용히 버리지 않고 **거절한다**. 버리면 사용자가 건 조건이
없는 채로 결과가 나오는데, 화면은 그 조건이 걸린 줄 안다 — 가장 나쁜 종류의 조용한
실패다 (저장소 규칙: 쓰기·조회 입력은 관용보다 거부).
*/
func normalizeScreenQuery(q model.ScreenQuery) (model.ScreenQuery, []model.ScreenField, []model.ScreenField, error) {
	var out model.ScreenQuery
	_, market := screenAssetTypes(q.Market)
	out.Market = market
	if sec := strings.TrimSpace(q.Sector); sec != "" {
		if !model.Sector(sec).Valid() {
			return out, nil, nil, invalidInput("알 수 없는 업종입니다: %s", sec)
		}
		out.Sector = sec
	}

	if len(q.Rules) > model.ScreenMaxRules {
		return out, nil, nil, invalidInput("조건은 최대 %d개까지 걸 수 있습니다", model.ScreenMaxRules)
	}

	seen := make(map[string]struct{}, len(q.Rules))
	var active []model.ScreenField
	var cells []model.ScreenField
	for _, rule := range q.Rules {
		field, ok := model.LookupScreenField(rule.Field)
		if !ok {
			return out, nil, nil, invalidInput("알 수 없는 조건입니다: %s", rule.Field)
		}
		if _, dup := seen[field.ID]; dup {
			return out, nil, nil, invalidInput("같은 조건을 두 번 걸 수 없습니다: %s", field.Label)
		}
		seen[field.ID] = struct{}{}

		if rule.Min != nil && rule.Max != nil && *rule.Min > *rule.Max {
			return out, nil, nil, invalidInput("%s의 최솟값이 최댓값보다 큽니다", field.Label)
		}
		/*
			시가총액은 화면이 **조(兆) 단위**로 보낸다(`ScreenField.Scale`). 저장된 값은
			원 단위라 여기서 되돌린다 — 화면과 DB 중 한 곳은 반드시 환산해야 하고,
			사용자가 읽는 숫자를 그대로 주고받는 쪽이 저장 조건도 사람이 읽을 수 있다.
		*/
		norm := model.ScreenRule{Field: field.ID}
		norm.Min = scaleScreenBound(rule.Min, field.Scale)
		norm.Max = scaleScreenBound(rule.Max, field.Scale)

		out.Rules = append(out.Rules, norm)
		cells = append(cells, field)
		if norm.Active() {
			active = append(active, field)
		}
	}

	if q.SortBy != "" {
		if _, ok := model.LookupScreenField(q.SortBy); !ok {
			return out, nil, nil, invalidInput("알 수 없는 정렬 기준입니다: %s", q.SortBy)
		}
		out.SortBy = q.SortBy
		out.SortAsc = q.SortAsc
	}

	if like := strings.ToUpper(strings.TrimSpace(q.Like)); like != "" {
		if !screenLikeSymbol.MatchString(like) {
			return out, nil, nil, invalidInput("기준 종목이 올바르지 않습니다: %s", q.Like)
		}
		out.Like = like
	}

	out.Limit = q.Limit
	if out.Limit <= 0 {
		out.Limit = model.ScreenDefaultLimit
	}
	if out.Limit > model.ScreenMaxLimit {
		out.Limit = model.ScreenMaxLimit
	}
	return out, active, cells, nil
}

// scaleScreenBound는 화면 단위를 저장 단위로 되돌린다. Scale이 0이면 그대로다.
func scaleScreenBound(v *float64, scale float64) *float64 {
	if v == nil {
		return nil
	}
	if scale <= 0 {
		return v
	}
	scaled := *v * scale
	return &scaled
}

/*
Screen — 한 번의 스크리닝.

깔때기를 **먼저** 센다. 결과가 0줄이어도 화면은 「왜 0인지」를 말할 수 있어야 하고,
그 답이 깔때기에 들어 있다.
*/
func (s *ScreenerService) Screen(ctx context.Context, q model.ScreenQuery) (*model.ScreenResult, error) {
	if s == nil || s.repo == nil || s.scores == nil {
		return nil, invalidInput("스크리너를 사용할 수 없습니다")
	}
	norm, activeFields, cellFields, err := normalizeScreenQuery(q)
	if err != nil {
		return nil, err
	}

	tradeDate, err := s.scores.LatestScoredDate(ctx)
	if err != nil {
		return nil, err
	}
	if tradeDate == "" {
		// 점수가 아직 하나도 없는 상태 — 오류가 아니라 「아직 없음」이다 (요구 8).
		return &model.ScreenResult{Market: norm.Market, Rows: []model.ScreenRow{}}, nil
	}

	cacheKey := screenResultKey(tradeDate, norm)
	if cached, ok := s.cachedResult(cacheKey); ok {
		out := cached
		// 행은 복사해서 덮는다 — overlay는 행의 포인터 칸을 갈아 끼우므로 얕은 복사면 된다.
		out.Rows = append([]model.ScreenRow(nil), cached.Rows...)
		s.overlayLivePrices(ctx, out.Rows)
		return &out, nil
	}

	assetTypes, _ := screenAssetTypes(norm.Market)
	scope := repository.ScreenScope{AssetTypes: assetTypes, Sector: norm.Sector}

	funnel, err := s.repo.ScreenCounts(ctx, tradeDate, scope, norm.Rules)
	if err != nil {
		return nil, err
	}

	sortExpr := "sc.score"
	sortAsc := false
	if norm.SortBy != "" {
		if f, ok := model.LookupScreenField(norm.SortBy); ok {
			sortExpr = f.Expr
			sortAsc = norm.SortAsc
		}
	} else if norm.Like != "" {
		// 기준 종목의 값이 없으면(점수 없는 종목) 점수 순 그대로 — 조건은 이미 걸려 있다.
		if _, values, meta, err := s.Values(ctx, []string{norm.Like}); err == nil {
			if expr := screenLikeSortExpr(values[norm.Like], meta[norm.Like].Sector); expr != "" {
				sortExpr, sortAsc = expr, true
			}
		}
	}

	rows, err := s.repo.ScreenRows(ctx, tradeDate, scope, norm.Rules,
		cellFields, sortExpr, sortAsc, norm.Limit)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.ScreenRow{}
	}

	out := &model.ScreenResult{
		TradeDate: tradeDate,
		Market:    norm.Market,
		Funnel:    funnel,
		Rows:      rows,
		// 상한에 걸려 잘렸으면 말한다 — 안 말하면 「29종목뿐」으로 읽힌다.
		Truncated: funnel.Passed > len(rows),
	}

	/*
		커버리지는 **건 조건들만** 낸다. 목록 전체(23개)는 조건 화면이 따로 받으므로
		여기서 다시 세면 같은 스캔을 두 번 하게 된다.
	*/
	if len(activeFields) > 0 {
		counts, _, err := s.repo.ScreenCoverage(ctx, tradeDate, scope, activeFields)
		if err != nil {
			return nil, err
		}
		for _, f := range activeFields {
			out.Coverage = append(out.Coverage, model.ScreenFieldCoverage{
				Field: f.ID, Count: counts[f.ID],
			})
		}
	}
	s.rememberResult(cacheKey, *out)
	// 캐시에는 시세 덮기 전 값을 넣고, 응답은 복사본에 덮는다.
	out.Rows = append([]model.ScreenRow(nil), out.Rows...)
	s.overlayLivePrices(ctx, out.Rows)
	return out, nil
}

var screenLikeSymbol = regexp.MustCompile(`^[A-Z0-9][A-Z0-9.\-]{0,31}$`)

// screenLikeAxes — 「닮은꼴」 거리에 쓰는 축. 종합 점수는 축에서 나오므로 빼고 다섯만 센다.
var screenLikeAxes = []string{"axis.technical", "axis.quality", "axis.growth", "axis.valuation", "axis.sentiment"}

/*
screenLikeSortExpr — 「이 종목과 닮았거나 나은 종목」 정렬식(작을수록 앞) (2026-10-01 요청 1, 결정 B).

  - 축마다 기준보다 **모자란 만큼은 그대로**, 넘친 만큼은 1/4로 센다 — 「닮았는데 더 나은」이 앞에 온다.
  - 값이 없는 축은 기준과 같다고 본다. 비었다고 멀리 보내면 측정률 낮은 축이 순서를 정한다.
  - 같은 업종이 아니면 400(한 축 20점 모자람과 같은 거리)을 더한다 — 업종 먼저, 아주 닮은 다른 업종은 섞인다.

값은 DB에서 읽은 숫자, 업종은 `Sector.Valid()`를 통과한 고정 키라 식에 그대로 넣어도 된다.
ponytail: 가중치(1/4 · 400)는 눈대중이다. 사용자 반응을 보고 조정.
*/
func screenLikeSortExpr(ref map[string]float64, sector string) string {
	var terms []string
	for _, id := range screenLikeAxes {
		v, ok := ref[id]
		f, fok := model.LookupScreenField(id)
		if !ok || !fok {
			continue
		}
		n := strconv.FormatFloat(v, 'f', 3, 64)
		d := "(COALESCE(" + f.Expr + ", " + n + ") - " + n + ")"
		terms = append(terms, "POW(LEAST("+d+", 0), 2) + 0.25 * POW(GREATEST("+d+", 0), 2)")
	}
	if len(terms) == 0 {
		return ""
	}
	expr := strings.Join(terms, " + ")
	if model.Sector(sector).Valid() {
		expr = "(CASE WHEN s.symbol IN (SELECT ss.symbol FROM stock_sectors ss WHERE ss.sector = '" + sector +
			"') THEN 0 ELSE 400 END) + " + expr
	}
	return expr
}

func screenResultKey(tradeDate string, norm model.ScreenQuery) string {
	raw, _ := json.Marshal(norm)
	return tradeDate + "|" + string(raw)
}

func (s *ScreenerService) cachedResult(key string) (model.ScreenResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.results[key]
	if !ok || time.Since(entry.at) > screenResultTTL {
		return model.ScreenResult{}, false
	}
	return entry.data, true
}

func (s *ScreenerService) rememberResult(key string, data model.ScreenResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.results == nil || len(s.results) >= screenResultMax {
		s.results = map[string]screenResultEntry{}
	}
	s.results[key] = screenResultEntry{data: data, at: time.Now()}
}

/*
overlayLivePrices — 결과 행의 시세를 **랭킹과 같은 규칙**으로 맞춘다 (2026-09-23 QA P0-01).

# 왜 필요했나

이 화면과 AI 추천 랭킹은 같은 `stock_daily_closes`를 같은 모양의 LATERAL로 읽는다.
그런데 랭킹만 장중에 확정 종가를 실시간 값으로 덮고 있었다. 그래서 **장이 닫혀 있는
동안에는 두 화면의 숫자가 소수점까지 같고**(2026-09-23 02:05 실측: 리튬포어스 073570이
양쪽 다 2095원 · +1.6990291262135921), 열리는 순간 갈렸다 — 외부 QA가 같은 종목을
한 화면에서 +1.70%, 다른 화면에서 +0.49%로 본 것이 그것이다.

# 규칙을 여기 다시 적지 않는다

기준가를 무엇으로 고를지는 `list_price_overlay.go`가 정한다. 그 한 줄(「저장된 종가가
이미 그 시장의 오늘 것인가」)이 두 곳에서 갈리면 미국 애프터마켓에서만 한 세션씩
밀리는데, 그건 눈으로 안 갈린다.

시세를 못 구한 행은 **손대지 않는다.** 그 종목만 조용히 종가로 남는 편이 절반은
실시간이고 절반은 빈칸인 표보다 낫다.
*/
func (s *ScreenerService) overlayLivePrices(ctx context.Context, rows []model.ScreenRow) {
	if s == nil || s.scores == nil || len(rows) == 0 {
		return
	}
	in := make([]ListPriceInput, len(rows))
	for i, row := range rows {
		in[i] = ListPriceInput{
			Symbol:    row.Symbol,
			AssetType: row.AssetType,
			Close:     row.Close,
			PrevClose: row.PrevClose,
			PriceAsOf: row.PriceAsOf,
		}
	}
	for i, d := range s.scores.ResolveListPrices(ctx, in) {
		if d.BasePrice > 0 {
			base := d.BasePrice
			rows[i].BasePrice = &base
		}
		if !d.Live {
			continue
		}
		rate := d.ChangeRate
		price := d.Price
		rows[i].ChangeRate = &rate
		rows[i].Close = &price
		if d.Currency != "" {
			rows[i].Currency = d.Currency
		}
		rows[i].PriceAsOf = model.ScorePriceAsOfLive
	}
}

/*
Catalog — 조건 목록 + 그날의 커버리지.

사용자가 조건을 **누르기 전에** 「이건 9%만 측정됩니다」가 보여야 한다. 결과가 0이
된 뒤에 알려 주면 이미 늦다.
*/
func (s *ScreenerService) Catalog(ctx context.Context, market string) (*model.ScreenCatalog, error) {
	if s == nil || s.repo == nil || s.scores == nil {
		return nil, invalidInput("스크리너를 사용할 수 없습니다")
	}
	assetTypes, normMarket := screenAssetTypes(market)

	/*
		낡았으면 낡은 것을 주고 뒤에서 다시 센다 (09-25 운영 로그: 만료 뒤 첫 요청
		176~610ms). 커버리지는 하루 한 번 바뀌는 숫자라 15분 낡은 값은 문제가 없다.
	*/
	if cached, fresh, ok := s.cachedCatalogAllowStale(normMarket); ok {
		if !fresh && s.refresh != nil {
			s.refresh.runOnce("screener:catalog:"+normMarket, func(bg context.Context) {
				_, _ = s.buildCatalog(bg, assetTypes, normMarket)
			})
		}
		return &cached, nil
	}
	return s.buildCatalog(ctx, assetTypes, normMarket)
}

func (s *ScreenerService) buildCatalog(ctx context.Context, assetTypes []string, normMarket string) (*model.ScreenCatalog, error) {
	tradeDate, err := s.scores.LatestScoredDate(ctx)
	if err != nil {
		return nil, err
	}
	out := model.ScreenCatalog{TradeDate: tradeDate, Market: normMarket}
	if tradeDate == "" {
		// 점수가 없으면 커버리지도 없다. 필드 정의는 그대로 내보내 화면이 그려진다.
		for _, f := range model.ScreenFields {
			out.Fields = append(out.Fields, model.ScreenCatalogField{ScreenField: f})
		}
		return &out, nil
	}

	scope := repository.ScreenScope{AssetTypes: assetTypes}
	counts, inMarket, err := s.repo.ScreenCoverage(ctx, tradeDate, scope, model.ScreenFields)
	if err != nil {
		return nil, err
	}
	quart, err := s.repo.ScreenQuantiles(ctx, tradeDate, scope, model.ScreenFields)
	if err != nil {
		return nil, err
	}
	out.InMarket = inMarket
	for _, f := range model.ScreenFields {
		cf := model.ScreenCatalogField{ScreenField: f, Count: counts[f.ID], InMarket: inMarket}
		if q, ok := quart[f.ID]; ok {
			cf.Quartiles = &q
		}
		out.Fields = append(out.Fields, cf)
	}

	s.mu.Lock()
	s.catalog[normMarket] = screenCatalogEntry{data: out, at: time.Now()}
	s.mu.Unlock()
	return &out, nil
}

/*
Values — 종목들의 스크리너 값(가장 최근 점수일). 스크리너 「기준 종목」과 글 태그가 같이 쓴다 —
같은 지표를 같은 식으로 읽어야 글 속 「PER 101배」와 스크리너 칸의 101이 같은 숫자다.
*/
func (s *ScreenerService) Values(ctx context.Context, symbols []string) (string, map[string]map[string]float64, map[string]repository.ScreenSymbolMeta, error) {
	if s == nil || s.repo == nil || s.scores == nil {
		return "", nil, nil, nil
	}
	tradeDate, err := s.scores.LatestScoredDate(ctx)
	if err != nil || tradeDate == "" {
		return "", nil, nil, err
	}
	values, meta, err := s.repo.SymbolValues(ctx, tradeDate, symbols)
	return tradeDate, values, meta, err
}

// SymbolValues — 한 종목. 점수 행이 없으면 nil(404).
func (s *ScreenerService) SymbolValues(ctx context.Context, symbol string) (*model.ScreenSymbolValues, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	tradeDate, values, meta, err := s.Values(ctx, []string{symbol})
	if err != nil {
		return nil, err
	}
	m, ok := meta[symbol]
	if !ok {
		return nil, nil
	}
	market := "domestic"
	for _, t := range overseasAssetTypes {
		if m.AssetType == t {
			market = "overseas"
		}
	}
	return &model.ScreenSymbolValues{
		Symbol: symbol, Name: m.Name, AssetType: m.AssetType, Market: market, Sector: m.Sector,
		TradeDate: tradeDate, Values: values[symbol],
	}, nil
}

func (s *ScreenerService) cachedCatalogAllowStale(market string) (data model.ScreenCatalog, fresh, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.catalog[market]
	if !ok {
		return model.ScreenCatalog{}, false, false
	}
	return entry.data, time.Since(entry.at) <= screenCatalogTTL, true
}

/* ------------------------------------------------------------------------- */
/* 저장한 조건                                                                */
/* ------------------------------------------------------------------------- */

// SetScreens wires the saved-condition store. Nil means saving is unavailable.
func (s *ScreenerService) SetScreens(repo *repository.UserScreenRepo) {
	if s != nil {
		s.screens = repo
	}
}

func (s *ScreenerService) ListScreens(ctx context.Context, userID int64) ([]model.UserScreen, error) {
	if s == nil || s.screens == nil {
		return []model.UserScreen{}, nil
	}
	out, err := s.screens.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []model.UserScreen{}
	}
	return out, nil
}

/*
SaveScreen — 조건을 이름 붙여 저장한다.

**저장 전에 조회와 같은 검증을 통과시킨다.** 저장은 되는데 실행은 거절되는 조건을
남기면, 사용자는 목록에서 그것을 누를 때마다 오류를 본다.
*/
func (s *ScreenerService) SaveScreen(ctx context.Context, userID int64, in model.UserScreen) (*model.UserScreen, error) {
	if s == nil || s.screens == nil {
		return nil, invalidInput("조건 저장을 사용할 수 없습니다")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, invalidInput("조건 이름을 입력해 주세요")
	}
	if len([]rune(name)) > model.UserScreenNameMax {
		return nil, invalidInput("조건 이름은 %d자까지 입력할 수 있습니다", model.UserScreenNameMax)
	}

	norm, active, _, err := normalizeScreenQuery(model.ScreenQuery{
		Market: in.Market, Rules: in.Rules, SortBy: in.SortBy, SortAsc: in.SortAsc,
	})
	if err != nil {
		return nil, err
	}
	if len(active) == 0 {
		// 거르는 조건이 하나도 없으면 저장할 것이 없다 — 랭킹 전체와 같은 목록이 된다.
		return nil, invalidInput("거르는 조건을 하나 이상 넣어 주세요")
	}

	/*
		상한 검사는 **새 이름일 때만** 뜻이 있다. 기존 조건을 고치는 것은 개수를
		늘리지 않으므로, 20개를 채운 사용자가 자기 조건을 수정하지 못하면 안 된다.
	*/
	existing, err := s.screens.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	isNew := true
	for _, e := range existing {
		if e.Name == name {
			isNew = false
			break
		}
	}
	if isNew && len(existing) >= repository.UserScreenLimit {
		return nil, invalidInput("저장한 조건은 %d개까지입니다. 하나를 지우고 다시 시도해 주세요",
			repository.UserScreenLimit)
	}

	/*
		저장하는 값은 **화면이 보낸 단위 그대로**다 — 조회용으로 환산한 `norm.Rules`가
		아니다.

		실측으로 드러난 왕복 결함: 시가총액 「1조」를 보내면 조회용 환산이 1e12(원)을
		만드는데, 그것을 그대로 저장하면 화면이 다시 불러 보낼 때 1e12에 또 1e12가
		곱해진다. 결과는 언제나 0이고, 사용자는 자기가 저장한 조건이 고장 났다고 읽는다.

		**환산은 실행 지점 한 곳에만 둔다**(`Screen`). 덤으로 저장된 JSON이 사람이
		읽을 수 있는 숫자로 남는다 — `{"field":"marketCap","min":1}`.
	*/
	stored := make([]model.ScreenRule, 0, len(in.Rules))
	for _, rule := range in.Rules {
		field, ok := model.LookupScreenField(rule.Field)
		if !ok {
			continue // 위 검증을 이미 통과했으므로 여기 오지 않는다.
		}
		stored = append(stored, model.ScreenRule{Field: field.ID, Min: rule.Min, Max: rule.Max})
	}

	saved := model.UserScreen{
		Name: name, Market: norm.Market, Rules: stored,
		SortBy: norm.SortBy, SortAsc: norm.SortAsc,
	}
	id, err := s.screens.Upsert(ctx, userID, saved)
	if err != nil {
		return nil, err
	}
	saved.ID = id
	return &saved, nil
}

func (s *ScreenerService) DeleteScreen(ctx context.Context, userID, id int64) error {
	if s == nil || s.screens == nil {
		return invalidInput("조건 저장을 사용할 수 없습니다")
	}
	if err := s.screens.Delete(ctx, userID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return invalidInput("저장한 조건을 찾을 수 없습니다")
		}
		return err
	}
	return nil
}
