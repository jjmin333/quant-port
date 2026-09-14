package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

// NewsBriefingService generates and serves the 「AI 시장 브리핑」 shown atop the
// 국내주식 / 해외주식 뉴스 화면.
//
// It follows the same spending rule as the per-symbol AI 요약: **the LLM is
// only ever called by the scheduler or an admin trigger.** An ordinary page
// load reads the last stored briefing and nothing else, however old it is.
// That is what keeps a day's cost equal to the number of slots the schedule
// planned instead of scaling with traffic.
type NewsBriefingService struct {
	news     *NewsStore
	indices  *MarketIndexStore
	quotes   *QuotesService
	repo     *repository.NewsBriefingRepo
	llm      *llm.Pool
	earnings *EarningsCalendarService
	toss     *toss.Client
	redis    *redis.Client
	// runs records each scheduled slot in the batch ledger — 건너뛴 슬롯까지
	// 포함해서. 없으면 감시는 「조용한 주말」과 「죽은 스케줄러」를 구분하지 못한다.
	runs *repository.BatchJobRunRepo
}

func NewNewsBriefingService(
	news *NewsStore,
	indices *MarketIndexStore,
	quotes *QuotesService,
	repo *repository.NewsBriefingRepo,
) *NewsBriefingService {
	return &NewsBriefingService{news: news, indices: indices, quotes: quotes, repo: repo}
}

// SetBatchRuns wires the batch ledger so every scheduled slot leaves a row.
func (s *NewsBriefingService) SetBatchRuns(repo *repository.BatchJobRunRepo) {
	if s != nil {
		s.runs = repo
	}
}

func (s *NewsBriefingService) SetLLM(pool *llm.Pool) {
	if s != nil {
		s.llm = pool
	}
}

func (s *NewsBriefingService) SetEarnings(c *EarningsCalendarService) {
	if s != nil {
		s.earnings = c
	}
}

func (s *NewsBriefingService) SetToss(c *toss.Client) {
	if s != nil {
		s.toss = c
	}
}

func (s *NewsBriefingService) SetRedis(r *redis.Client) {
	if s != nil {
		s.redis = r
	}
}

const (
	// briefingHeadlineCount is how many headlines the prompt carries. Beyond
	// this the model repeats itself and the input cost keeps climbing.
	briefingHeadlineCount = 40
	// briefingHeadlinesPerPublisher stops one prolific wire service from
	// filling the whole list and skewing what the briefing thinks the day was
	// about.
	briefingHeadlinesPerPublisher = 5
	/*
		「많이 언급된 종목」의 세 숫자.

		예전에는 이 목록을 **브리핑이 프롬프트에 실으려고 받아 둔 기사 200건**에서
		Go로 셌다. 그 200건이 덮는 시간이 국내 기준 한 시간 남짓이라, 「오늘 많이
		언급된 종목」이 사실 「방금 한 시간」이었다 — 뉴스 화면의 필터 칩이 같은
		이유로 이미 최근 1,000건 SQL 집계로 옮겨 가 있었는데(news_top_symbols.go)
		홈 브리핑만 옛 방식으로 남아, **같은 앱의 두 화면이 다른 하루를 말했다.**

		이제 둘이 같은 집계를 부른다. 다른 것은 문턱뿐이다:

		  - Min  25회 — 브리핑이 「많이 언급됐다」고 적을 때 2회는 많이가 아니다.
		    처음에는 10회로 뒀는데 그 문턱을 넘는 종목이 국내 19·해외 21개나 됐다
		    (실측 2026-09-03) — 그 정도면 「오늘 화제인 종목」이 아니라 「오늘 뉴스에
		    나온 종목」이다. 25회로 올리면 국내 8·해외 5개가 남고(같은 표본), 그게
		    사람이 훑어보고 「오늘은 이것들이었구나」 하는 크기다(사용자 결정).
		    **화면도 같은 수로 한 번 더 거른다** — 문턱을 바꾸면 예전 문턱으로 저장된
		    브리핑이 남아 있어서, 각주와 칩이 어긋나는 프레임이 생긴다.
		  - Count 30 — 문턱을 넘은 종목을 **다 준다**. 문턱을 넘긴 종목이 많은 날은
		    스무 개도 나오는데, 그걸 서버에서 잘라 버리면 화면이 「전체 보기」로
		    펼칠 것이 없다. 접는 일은 화면이 한다(market-briefing-card.tsx).
	*/
	briefingMentionMinCount = 25
	briefingMentionCount    = 30
	// briefingHistoryCount / briefingHistoryChars — how much of the previous
	// briefings the next prompt reads back. Ten truncated summaries of the
	// same morning cost more than the facts they were meant to support; three
	// plus the newest watchPoints is enough for 이어짐·반전.
	briefingHistoryCount = 3
	briefingHistoryChars = 150
	// briefingKeep is how many rows per market survive pruning.
	briefingKeep = 60
	// briefingBreakingWindow matches the front-end's 속보 definition.
	briefingBreakingWindow = 24 * time.Hour
)

func itoa(n int) string { return strconv.Itoa(n) }

// formatSignedPercent renders a change rate the way the UI does, so the
// rule-based prose and the tiles beside it read as one voice.
func formatSignedPercent(rate float64) string {
	sign := "+"
	if rate < 0 {
		sign = ""
	}
	return sign + strconv.FormatFloat(rate, 'f', 2, 64) + "%"
}

func briefingScheduleFor(market string) calendar.ScheduleID {
	if market == "us" {
		return calendar.ScheduleUS
	}
	return calendar.ScheduleKRX
}

// Latest returns the stored briefing for a market. It never calls the LLM.
func (s *NewsBriefingService) Latest(ctx context.Context, market string) (*model.NewsMarketBriefing, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.Latest(ctx, market)
}

// Generate builds one briefing and stores it. skipWhenQuiet makes the
// scheduled path skip markets where no article has arrived since the last run —
// a quiet night must not spend a call restating the same headlines. The admin
// trigger passes false so a manual run always produces something.
func (s *NewsBriefingService) Generate(ctx context.Context, market string, skipWhenQuiet bool) (*model.NewsMarketBriefing, bool, error) {
	if s == nil || s.news == nil {
		return nil, false, nil
	}

	facts := s.buildFacts(ctx, market)
	if facts.ArticleCount == 0 {
		return nil, false, nil
	}

	if skipWhenQuiet {
		if prev, err := s.repo.Latest(ctx, market); err == nil && prev != nil {
			if !facts.LatestPublishedAt.IsZero() &&
				!facts.LatestPublishedAt.After(prev.Facts.LatestPublishedAt) {
				return prev, false, nil
			}
		}
	}

	briefing := model.NewsMarketBriefing{
		Market:      market,
		GeneratedAt: time.Now().UTC(),
		Source:      model.BriefingSourceRules,
		Facts:       facts,
	}

	client := s.llm.For(market)
	if client.Enabled() {
		history, _ := s.repo.History(ctx, market, briefingHistoryCount, briefingHistoryChars)
		earnings := s.briefingEarnings(ctx, market)
		var flow *briefingFlowDTO
		if market != "us" {
			flow = s.briefingInvestorFlow(ctx)
		}
		factsJSON, err := s.promptJSON(market, facts, history, earnings, flow)
		if err == nil {
			if out := s.briefingNarrative(ctx, client, market, factsJSON); out != nil {
				briefing.Source = model.BriefingSourceLLM
				briefing.Summary = out.Summary
				briefing.Mood = out.Mood
				briefing.MoodReason = out.MoodReason
				briefing.Highlights = out.Highlights
				briefing.WatchPoints = out.WatchPoints
			}
		} else {
			log.Printf("news briefing %s: prompt build failed: %v", market, err)
		}
	}

	// 문장을 못 얻었을 때의 차선은 **직전 AI 문장을 물려받는 것**이고, 그마저
	// 없을 때가 규칙 문장이다. 실패 한 번에 카드가 「AI 분석 없음」으로 바뀌던
	// 것이 이 두 줄로 바뀐 지점이다.
	if briefing.Source == model.BriefingSourceRules && !s.carryBriefingNarrative(ctx, &briefing) {
		applyRuleBasedBriefing(&briefing)
	}

	if s.repo != nil {
		if err := s.repo.Insert(ctx, briefing); err != nil {
			return nil, false, err
		}
		if err := s.repo.Prune(ctx, market, briefingKeep); err != nil {
			log.Printf("news briefing %s: prune failed: %v", market, err)
		}
	}
	return &briefing, true, nil
}

const (
	// briefingRetryWait is the pause before the one extra attempt. The floor is
	// the provider's own spacing: the likeliest fast failure is another LLM
	// batch (종목 AI 요약) having just taken this key's MinInterval slot — the
	// counters live in Redis, so the worker containers and the API share them.
	// Waiting that out is exactly what makes the second attempt worth making.
	briefingRetryWait = 5 * time.Second
	// briefingCarryWindow is how stale an inherited narrative may be. Past
	// this the sentences are describing a different session than the numbers
	// printed beside them, and a plain rules paragraph is the honest answer.
	briefingCarryWindow = 3 * time.Hour
	// briefingCarryDepth is how far back to look for one. Slots are 30–60
	// minutes, so this comfortably covers the window above.
	briefingCarryDepth = 8
)

// briefingNarrative asks the model for this slot's prose, with **one** extra
// attempt when the first failure is the kind a second try could survive.
//
// Not retried:
//   - 한도 초과(llm.IsBudgetExhausted) — 다시 물어도 같은 대답이고, 통과한다면
//     쓰지 않기로 정한 예산을 쓰는 것이다.
//   - 503 — chatCompletion이 이미 한 번 다시 걸었다. 여기서 또 걸면 한 슬롯에
//     네 번이 된다.
//
// Retried: MinInterval 거절, 타임아웃, 그 밖의 5xx, JSON 파싱 실패, 그리고
// **빈 summary**. 마지막 것이 특히 중요하다 — 모델이 형식만 맞는 빈 응답을 준
// 갈래는 예전에 로그를 한 줄도 남기지 않아, 카드가 규칙으로 내려간 이유를
// 사후에 확인할 수 없었다.
func (s *NewsBriefingService) briefingNarrative(
	ctx context.Context,
	client *llm.Client,
	market string,
	factsJSON string,
) *llm.NewsBriefingLLMOutput {
	for attempt := 1; attempt <= 2; attempt++ {
		out, err := client.GenerateNewsBriefing(ctx, factsJSON)
		switch {
		case err != nil:
			if attempt == 1 && !llm.IsBudgetExhausted(err) && !llm.IsUnavailable(err) {
				log.Printf("news briefing %s: llm failed (%v) — retrying once", market, err)
				if !s.waitBeforeBriefingRetry(ctx, client) {
					return nil
				}
				continue
			}
			log.Printf("news briefing %s: llm failed, no narrative this slot: %v", market, err)
			return nil
		case strings.TrimSpace(out.Summary) == "":
			if attempt == 1 {
				log.Printf("news briefing %s: llm returned an empty summary — retrying once", market)
				if !s.waitBeforeBriefingRetry(ctx, client) {
					return nil
				}
				continue
			}
			log.Printf("news briefing %s: llm returned an empty summary twice", market)
			return nil
		default:
			return out
		}
	}
	return nil
}

// waitBeforeBriefingRetry sleeps out the retry pause, or reports false when the
// context was cancelled while waiting (shutdown — the slot is simply skipped).
func (s *NewsBriefingService) waitBeforeBriefingRetry(ctx context.Context, client *llm.Client) bool {
	wait := briefingRetryWait
	if gap := client.Limits().MinInterval; gap+time.Second > wait {
		wait = gap + time.Second
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// carryBriefingNarrative inherits the last real AI narrative when this slot
// could not produce one, and reports whether it found one.
//
// Only the prose is inherited. The number tiles, the session label and the
// timestamp stay this slot's own — they are counted, not written, so freezing
// them would make a working screen look stuck for an hour. What the card owes
// the reader instead is CarriedFrom: 「문장은 몇 시 것인가」.
//
// Chained carries keep pointing at the *original* slot, so the 3-hour window is
// measured from when the sentences were actually written and a long outage
// cannot walk a morning briefing into the evening one slot at a time.
func (s *NewsBriefingService) carryBriefingNarrative(ctx context.Context, b *model.NewsMarketBriefing) bool {
	if s == nil || s.repo == nil {
		return false
	}
	recent, err := s.repo.Recent(ctx, b.Market, briefingCarryDepth)
	if err != nil {
		log.Printf("news briefing %s: carry lookup failed: %v", b.Market, err)
		return false
	}
	for _, prev := range recent {
		if prev.Source != model.BriefingSourceLLM && prev.Source != model.BriefingSourceCarry {
			continue
		}
		if strings.TrimSpace(prev.Summary) == "" {
			continue
		}
		origin := prev.GeneratedAt
		if prev.CarriedFrom != nil {
			origin = *prev.CarriedFrom
		}
		if !briefingCarryAllowed(origin, b.GeneratedAt) {
			return false
		}

		b.Source = model.BriefingSourceCarry
		b.Summary = prev.Summary
		b.Mood = prev.Mood
		b.MoodReason = prev.MoodReason
		b.Highlights = prev.Highlights
		b.WatchPoints = prev.WatchPoints
		b.CarriedFrom = &origin
		log.Printf("news briefing %s: carried the %s narrative forward",
			b.Market, origin.In(seoulLocation()).Format("15:04"))
		return true
	}
	return false
}

// briefingCarryAllowed reports whether a narrative written at origin may still
// stand beside facts counted at now.
//
// Two limits, and the second is not implied by the first: 3시간이 지나면 문장이
// 옆의 숫자와 다른 장을 말하고, **날이 바뀌면** 3시간 이내여도 다른 날 이야기다
// (어제 18:00 장 마감 문장 옆에 오늘 08:00 개장 전 숫자).
func briefingCarryAllowed(origin, now time.Time) bool {
	if origin.IsZero() || now.Before(origin) {
		return false
	}
	if now.Sub(origin) > briefingCarryWindow {
		return false
	}
	seoul := seoulLocation()
	o, n := origin.In(seoul), now.In(seoul)
	return o.Year() == n.Year() && o.YearDay() == n.YearDay()
}

// RunBatchNow is the 배치 관리 manual trigger. Unlike the schedule it never
// skips a quiet market — an admin pressing 실행 wants a new briefing, and being
// told "nothing changed" would look like a failure.
func (s *NewsBriefingService) RunBatchNow(ctx context.Context, market string) (map[string]any, string, error) {
	briefing, generated, err := s.Generate(ctx, market, false)
	if err != nil {
		return nil, "", err
	}
	if briefing == nil {
		return nil, "", fmt.Errorf("%s 뉴스 기사가 없어 브리핑을 만들 수 없습니다", briefingMarketLabel(market))
	}
	result := map[string]any{
		"market":        market,
		"source":        briefing.Source,
		"mood":          briefing.Mood,
		"articleCount":  briefing.Facts.ArticleCount,
		"breakingCount": briefing.Facts.BreakingCount,
		"summaryChars":  len([]rune(briefing.Summary)),
		"generated":     generated,
	}
	if briefing.CarriedFrom != nil {
		result["carriedFrom"] = briefing.CarriedFrom.In(seoulLocation()).Format("15:04")
	}
	return result, briefing.GeneratedAt.Format(time.RFC3339), nil
}

// buildFacts counts everything the briefing states as fact. The same counts
// drive the card's number tiles, so the prose and the tiles can never disagree.
func (s *NewsBriefingService) buildFacts(ctx context.Context, market string) model.NewsBriefingFacts {
	articles, _ := s.news.List(ctx, model.NewsListFilter{
		Market: market,
		Sort:   "latest",
		Limit:  200,
	})

	now := time.Now()
	facts := model.NewsBriefingFacts{
		Session:      string(calendar.SessionPhase(briefingScheduleFor(market), now)),
		ArticleCount: len(articles),
	}

	for _, a := range articles {
		if a.PublishedAt.After(facts.LatestPublishedAt) {
			facts.LatestPublishedAt = a.PublishedAt
		}
		if isBreakingArticle(a, now) {
			facts.BreakingCount++
		}
	}

	facts.Indices = s.briefingIndices(ctx, market)
	facts.TopMentioned, facts.MentionSample = s.topMentioned(ctx, market)
	facts.Headlines = briefingHeadlines(articles, facts.TopMentioned, market)
	return facts
}

// isBreakingArticle mirrors the front-end's 속보 rule so the count on the card
// matches the badges in the list.
func isBreakingArticle(a model.NewsArticle, now time.Time) bool {
	if a.PublishedAt.IsZero() || now.Sub(a.PublishedAt) > briefingBreakingWindow {
		return false
	}
	for _, tag := range a.Tags {
		if tag == "속보" {
			return true
		}
	}
	h := strings.TrimSpace(a.Headline)
	return strings.HasPrefix(h, "[속보]") || strings.HasPrefix(h, "【속보】") || strings.HasPrefix(h, "속보")
}

var briefingIndexOrder = map[string][]string{
	"domestic": {"kospi", "kosdaq", "usd-krw", "us10y", "sp500", "nasdaq"},
	"us":       {"nasdaq", "sp500", "dow", "us10y", "usd-krw", "kospi"},
}

func (s *NewsBriefingService) briefingIndices(ctx context.Context, market string) []model.NewsBriefingIndex {
	if s.indices == nil {
		return nil
	}
	all, _ := s.indices.List(ctx)
	byID := make(map[string]model.MarketIndex, len(all))
	for _, idx := range all {
		byID[idx.ID] = idx
	}
	order := briefingIndexOrder[market]
	if len(order) == 0 {
		order = briefingIndexOrder["domestic"]
	}
	var out []model.NewsBriefingIndex
	for _, id := range order {
		idx, ok := byID[id]
		if !ok || idx.Value == 0 {
			continue
		}
		out = append(out, model.NewsBriefingIndex{
			ID:         idx.ID,
			Name:       idx.Name,
			Value:      idx.Value,
			ChangeRate: idx.ChangeRate,
		})
	}
	return out
}

/*
topMentioned은 최근 1,000건에서 briefingMentionMinCount회 이상 언급된 종목을 세고, 각 종목에 지금 시세를
붙인다. 두 번째 반환값은 **실제로 센 기사 수**다 — 화면 각주가 「최근 N건 중」이라고
적을 때 요청한 수가 아니라 있었던 수를 써야 한다.

세는 일은 SQL이 한다(repository.TopSymbols). 옛 방식은 프롬프트용 기사 200건을 Go로
훑는 것이었는데, 표본을 1,000건으로 넓히려면 그 200건을 1,000건으로 올려야 하고 그건
**기사 본문·이미지 판정까지 딸린 1,000건을 메모리에 올리는 일**이다. 집계에 필요한
것은 심볼 목록뿐이라 그 값을 치를 이유가 없다.

이름은 SQL이 stocks를 조인해 함께 준다 — 국내 화면에 엔비디아가 올라오던 것도 그
조인이 막는다(asset_type='domestic').
*/
func (s *NewsBriefingService) topMentioned(
	ctx context.Context,
	market string,
) ([]model.NewsBriefingMention, int) {
	if s.news == nil {
		return nil, 0
	}
	top, err := s.news.TopSymbolsAbove(ctx, market, briefingMentionMinCount, briefingMentionCount)
	if err != nil || len(top.Symbols) == 0 {
		return nil, top.ArticleCount
	}

	symbols := make([]string, 0, len(top.Symbols))
	for _, row := range top.Symbols {
		symbols = append(symbols, strings.ToUpper(strings.TrimSpace(row.Symbol)))
	}

	quotes := map[string]model.QuoteSummary{}
	if s.quotes != nil {
		for _, q := range s.quotes.BatchQuoteSummaries(ctx, symbols) {
			quotes[strings.ToUpper(q.Symbol)] = q
		}
	}

	out := make([]model.NewsBriefingMention, 0, len(top.Symbols))
	for i, row := range top.Symbols {
		sym := symbols[i]
		m := model.NewsBriefingMention{Symbol: sym, Mentions: row.Count, Name: row.Name}
		if q, ok := quotes[sym]; ok {
			if m.Name == "" {
				m.Name = q.Name
			}
			// 가격이 0이면 시세를 못 붙인 것이다. 0%를 내려보내면 브리핑이
			// "보합"이라고 쓸 수 있으므로 없다고 표시한다.
			if q.CurrentPrice > 0 {
				m.ChangeRate = q.ChangeRate
				m.HasQuote = true
			}
		}
		if m.Name == "" {
			m.Name = sym
		}
		out = append(out, m)
	}
	return out, top.ArticleCount
}

// promptJSON is the exact user message the model sees. Everything it may cite
// is in here; nothing else is.
func (s *NewsBriefingService) promptJSON(
	market string,
	facts model.NewsBriefingFacts,
	history []model.NewsBriefingHistoryEntry,
	earnings []briefingEarningDTO,
	flow *briefingFlowDTO,
) (string, error) {
	type historyDTO struct {
		At          string   `json:"at"`
		Mood        string   `json:"mood,omitempty"`
		Summary     string   `json:"summary"`
		WatchPoints []string `json:"watchPoints,omitempty"`
	}
	hist := make([]historyDTO, 0, len(history))
	// 오래된 것부터 넣어 모델이 시간 순서대로 읽게 한다. watchPoints는 가장
	// 최근 1건에만 실어, 어제 보기로 한 것이 이어졌는지를 묻는다.
	for i := len(history) - 1; i >= 0; i-- {
		h := history[i]
		row := historyDTO{
			At:      h.GeneratedAt.In(seoulLocation()).Format("01-02 15:04"),
			Mood:    h.Mood,
			Summary: h.Summary,
		}
		if i == 0 {
			row.WatchPoints = h.WatchPoints
		}
		hist = append(hist, row)
	}

	payload := map[string]any{
		"market":            market,
		"marketLabel":       briefingMarketLabel(market),
		"generatedAt":       time.Now().In(seoulLocation()).Format("2006-01-02 15:04 (Mon)"),
		"session":           facts.Session,
		"articleCount":      facts.ArticleCount,
		"breakingCount":     facts.BreakingCount,
		"indices":           facts.Indices,
		"topMentioned":      facts.TopMentioned,
		"headlines":         facts.Headlines,
		"previousBriefings": hist,
	}
	if len(earnings) > 0 {
		payload["earnings"] = earnings
	}
	if flow != nil {
		payload["flow"] = flow
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func briefingMarketLabel(market string) string {
	if market == "us" {
		return "해외 주식 (미국 증시)"
	}
	return "국내 주식 (코스피·코스닥)"
}

// applyRuleBasedBriefing fills the narrative without an LLM — used when the LLM
// is disabled, over budget, or failed. It states only what was counted, so it
// is never wrong, just plainer.
func applyRuleBasedBriefing(b *model.NewsMarketBriefing) {
	facts := b.Facts
	var sb strings.Builder

	sb.WriteString(briefingMarketLabel(b.Market))
	sb.WriteString(" — ")
	sb.WriteString(facts.Session)
	sb.WriteString(". 최근 기사 ")
	sb.WriteString(itoa(facts.ArticleCount))
	sb.WriteString("건")
	if facts.BreakingCount > 0 {
		sb.WriteString(", 그중 속보 ")
		sb.WriteString(itoa(facts.BreakingCount))
		sb.WriteString("건")
	}
	sb.WriteString(".")

	if len(facts.Indices) > 0 {
		sb.WriteString(" 지수는 ")
		parts := make([]string, 0, len(facts.Indices))
		for _, idx := range facts.Indices {
			parts = append(parts, idx.Name+" "+formatSignedPercent(idx.ChangeRate))
		}
		sb.WriteString(strings.Join(parts, ", "))
		sb.WriteString(".")
	}

	if len(facts.TopMentioned) > 0 {
		top := facts.TopMentioned[0]
		sb.WriteString(" 오늘 가장 많이 언급된 종목은 ")
		sb.WriteString(top.Name)
		sb.WriteString("(")
		sb.WriteString(itoa(top.Mentions))
		sb.WriteString("건)")
		if top.HasQuote {
			sb.WriteString("이며 ")
			sb.WriteString(formatSignedPercent(top.ChangeRate))
			sb.WriteString(" 움직였습니다.")
		} else {
			sb.WriteString("입니다.")
		}
	}

	b.Summary = sb.String()
	b.Mood = ruleBasedMood(facts.Indices)
	b.MoodReason = "지수 등락만으로 판단한 값입니다 (AI 분석 없음)."

	for _, h := range facts.Headlines {
		if len(b.Highlights) >= 4 {
			break
		}
		b.Highlights = append(b.Highlights, h.Title)
	}
}

// ruleBasedMood reads the market's own indices only — the first two entries of
// the ordered list, which are that market's headline indices.
func ruleBasedMood(indices []model.NewsBriefingIndex) string {
	if len(indices) == 0 {
		return "중립"
	}
	limit := 2
	if len(indices) < limit {
		limit = len(indices)
	}
	var sum float64
	for _, idx := range indices[:limit] {
		sum += idx.ChangeRate
	}
	avg := sum / float64(limit)
	switch {
	case avg >= 0.5:
		return "강세"
	case avg <= -0.5:
		return "약세"
	default:
		return "중립"
	}
}
