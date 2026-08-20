package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
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
	stocks   *repository.StockRepo
	repo     *repository.NewsBriefingRepo
	llm      *llm.Client
	earnings *EarningsCalendarService
	toss     *toss.Client
	redis    *redis.Client
}

func NewNewsBriefingService(
	news *NewsStore,
	indices *MarketIndexStore,
	quotes *QuotesService,
	stocks *repository.StockRepo,
	repo *repository.NewsBriefingRepo,
) *NewsBriefingService {
	return &NewsBriefingService{news: news, indices: indices, quotes: quotes, stocks: stocks, repo: repo}
}

func (s *NewsBriefingService) SetLLM(client *llm.Client) {
	if s != nil {
		s.llm = client
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
	// briefingMentionCount is how many symbols the 「오늘의 이슈 종목」 strip and
	// the prompt share.
	briefingMentionCount = 10
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
		Source:      "rules",
		Facts:       facts,
	}

	if s.llm != nil && s.llm.Enabled() {
		history, _ := s.repo.History(ctx, market, briefingHistoryCount, briefingHistoryChars)
		earnings := s.briefingEarnings(ctx, market)
		var flow *briefingFlowDTO
		if market != "us" {
			flow = s.briefingInvestorFlow(ctx)
		}
		factsJSON, err := s.promptJSON(market, facts, history, earnings, flow)
		if err == nil {
			out, err := s.llm.GenerateNewsBriefing(ctx, factsJSON)
			if err != nil {
				// 실패해도 카드를 비우지 않는다 — 규칙 기반으로 내려간다.
				log.Printf("news briefing %s: llm failed, falling back to rules: %v", market, err)
			} else if strings.TrimSpace(out.Summary) != "" {
				briefing.Source = "llm"
				briefing.Summary = out.Summary
				briefing.Mood = out.Mood
				briefing.MoodReason = out.MoodReason
				briefing.Highlights = out.Highlights
				briefing.WatchPoints = out.WatchPoints
			}
		}
	}

	if briefing.Source == "rules" {
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
	facts.TopMentioned = s.topMentioned(ctx, articles, market)
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

// topMentioned counts how often each symbol is tagged across the articles the
// screen is showing, then attaches the live change rate.
//
// The count comes from the same article list the front-end renders, so the
// strip and the sentence are looking at one set of facts.
func (s *NewsBriefingService) topMentioned(
	ctx context.Context,
	articles []model.NewsArticle,
	market string,
) []model.NewsBriefingMention {
	counts := map[string]int{}
	for _, a := range articles {
		seen := map[string]bool{}
		for _, sym := range a.Symbols {
			sym = strings.ToUpper(strings.TrimSpace(sym))
			if sym == "" || seen[sym] {
				continue
			}
			seen[sym] = true
			counts[sym]++
		}
	}
	if len(counts) == 0 {
		return nil
	}

	type pair struct {
		symbol string
		count  int
	}
	pairs := make([]pair, 0, len(counts))
	for sym, n := range counts {
		pairs = append(pairs, pair{sym, n})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].symbol < pairs[j].symbol
	})
	if len(pairs) > briefingMentionCount {
		pairs = pairs[:briefingMentionCount]
	}

	symbols := make([]string, 0, len(pairs))
	for _, p := range pairs {
		symbols = append(symbols, p.symbol)
	}

	names := map[string]string{}
	if s.stocks != nil {
		if rows, err := s.stocks.ListBySymbols(ctx, symbols); err == nil {
			for _, row := range rows {
				names[strings.ToUpper(row.Symbol)] = row.Name
			}
		}
	}
	quotes := map[string]model.QuoteSummary{}
	if s.quotes != nil {
		for _, q := range s.quotes.BatchQuoteSummaries(ctx, symbols) {
			quotes[strings.ToUpper(q.Symbol)] = q
		}
	}

	out := make([]model.NewsBriefingMention, 0, len(pairs))
	for _, p := range pairs {
		m := model.NewsBriefingMention{Symbol: p.symbol, Mentions: p.count, Name: names[p.symbol]}
		if q, ok := quotes[p.symbol]; ok {
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
			m.Name = p.symbol
		}
		out = append(out, m)
	}
	return out
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
