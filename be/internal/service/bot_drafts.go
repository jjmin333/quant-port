package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
BotDraftService — 커뮤니티·투자 리포트 **초안** 생성기 (2026-10-16 사용자 요청).

3시간마다(KST 00·03·…·21시, 하루 8번) 그 시각의 저장분만 읽어 커뮤니티 5개 · 리포트 2개를 만든다.
LLM을 부르지 않는다 — 문장은 이미 배치가 써 둔 것(브리핑 · 종목 AI 요약 · 종가 AI 한 줄 · 실적 프리뷰)이고,
이 코드는 고르고 엮기만 한다. 초안은 바로 올라가지 않는다: 관리자가 고쳐서 게시 계정으로 올린다.

종목 고르기 — 후보마다 가중치를 쌓고 가중 랜덤으로 뽑는다.
  - 최신 브리핑의 「많이 언급된 종목」(국내·해외)   3 + 언급 수/10(최대 +5)
  - 최신 「오늘의 변화」 사건                     점수 급변·실적 3 · 공시·거래량·뉴스·내부자·가격 1~2
  - 2일 안에 실적 발표가 있는 종목(프리뷰 있음)    2
  - 최근 24시간 안에 같은 종류 초안이 있던 종목은 뺀다(회차마다 같은 종목 반복 금지).
리포트는 종목 AI 요약이 있는 후보에서만 뽑는다 — 긴 글의 몸통이 그것이다.
*/

const (
	botCommunityPerRun = 5
	botResearchPerRun  = 2
	botSlotHours       = 3
	botDraftTTL        = 24 * time.Hour
	// 긴 글 비율 — 커뮤니티는 짧은 글이 기본, 리포트는 긴 글이 기본.
	botCommunityLongShare = 0.4
	botResearchLongShare  = 0.7
)

type BotDraftService struct {
	repo       *repository.BotDraftRepo
	briefings  *repository.NewsBriefingRepo
	changes    *repository.ChangeEventRepo
	scores     *repository.StockScoreRepo
	narratives *repository.StockSummaryNarrativeRepo
	previews   *repository.EarningsPreviewRepo
	stocks     *repository.StockRepo
	now        func() time.Time
	rnd        *rand.Rand
}

func NewBotDraftService(db *sql.DB) *BotDraftService {
	return &BotDraftService{
		repo:       repository.NewBotDraftRepo(db),
		briefings:  repository.NewNewsBriefingRepo(db),
		changes:    repository.NewChangeEventRepo(db),
		scores:     repository.NewStockScoreRepo(db),
		narratives: repository.NewStockSummaryNarrativeRepo(db),
		previews:   repository.NewEarningsPreviewRepo(db),
		stocks:     repository.NewStockRepo(db),
		now:        time.Now,
		rnd:        rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x9e3779b97f4a7c15)),
	}
}

// BotNextSlot — now 다음의 생성 시각(KST 3시간 경계).
func BotNextSlot(now time.Time) time.Time {
	local := now.In(kstLoc())
	h := (local.Hour()/botSlotHours + 1) * botSlotHours
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, kstLoc()).Add(time.Duration(h) * time.Hour)
}

// botSlotOf — now가 속한 회차(KST 3시간 경계로 내림).
func botSlotOf(now time.Time) time.Time {
	local := now.In(kstLoc())
	h := local.Hour() / botSlotHours * botSlotHours
	return time.Date(local.Year(), local.Month(), local.Day(), h, 0, 0, 0, kstLoc())
}

type botCandidate struct {
	Symbol     string
	Name       string
	Market     string // domestic | us
	Weight     float64
	Mentions   int
	ChangeRate *float64
	Events     []string // 오늘의 변화 제목
	ScoreJump  string   // 점수 급변 제목(있으면)
	Upcoming   *botPreview
	// 채움(뽑힌 뒤)
	Score     *model.StockScoreView
	CloseAI   repository.BotCloseAI
	Narrative *repository.StockSummaryNarrative
	Articles  []repository.BotArticle
}

type botPreview struct {
	EventDate string
	Fiscal    string
	Headline  string
	Points    []string
	Risk      string
}

type botMarketMood struct {
	Mood    string
	Summary string
}

// Run is one slot: 지난 초안 정리 → 후보 → 뽑기 → 글 → 저장.
func (s *BotDraftService) Run(ctx context.Context) (map[string]any, string, error) {
	now := s.now()
	slot := botSlotOf(now)
	pruned, err := s.repo.Prune(ctx, now.Add(-botDraftTTL))
	if err != nil {
		return nil, "", err
	}
	recent, err := s.repo.RecentSymbols(ctx, now.Add(-botDraftTTL))
	if err != nil {
		return nil, "", err
	}
	cands, moods, err := s.candidates(ctx, now)
	if err != nil {
		return nil, "", err
	}

	community := botPick(s.rnd, cands, botCommunityPerRun, func(c *botCandidate) bool {
		return !recent[repository.BotDraftCommunity+":"+c.Symbol]
	})
	taken := map[string]bool{}
	for _, c := range community {
		taken[c.Symbol] = true
	}
	// 리포트 후보는 AI 요약이 있어야 한다 — 미리 읽어 거른다(후보 수십 개, 행 하나씩).
	for _, c := range cands {
		if !taken[c.Symbol] && !recent[repository.BotDraftResearch+":"+c.Symbol] {
			c.Narrative, _ = s.narratives.Get(ctx, c.Symbol)
		}
	}
	research := botPick(s.rnd, cands, botResearchPerRun, func(c *botCandidate) bool {
		return !taken[c.Symbol] && !recent[repository.BotDraftResearch+":"+c.Symbol] &&
			c.Narrative != nil && strings.TrimSpace(c.Narrative.Overview) != ""
	})

	picked := append(append([]*botCandidate{}, community...), research...)
	if err := s.enrich(ctx, picked, now); err != nil {
		return nil, "", err
	}
	drafts := make([]repository.BotDraft, 0, len(picked))
	for _, c := range community {
		long := s.rnd.Float64() < botCommunityLongShare
		drafts = append(drafts, buildBotCommunityDraft(s.rnd, c, moods[c.Market], long, slot))
	}
	for _, c := range research {
		long := s.rnd.Float64() < botResearchLongShare
		drafts = append(drafts, buildBotResearchDraft(s.rnd, c, long, slot))
	}
	if err := s.repo.Insert(ctx, drafts); err != nil {
		return nil, "", err
	}
	summary := fmt.Sprintf("커뮤니티 %d · 리포트 %d (후보 %d, 정리 %d)", len(community), len(research), len(cands), pruned)
	log.Printf("bot drafts: %s", summary)
	return map[string]any{
		"community": len(community), "research": len(research), "candidates": len(cands), "pruned": pruned,
		"slot": slot.Format(time.RFC3339),
	}, summary, nil
}

func (s *BotDraftService) candidates(ctx context.Context, now time.Time) ([]*botCandidate, map[string]botMarketMood, error) {
	by := map[string]*botCandidate{}
	get := func(sym, name, market string) *botCandidate {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		c := by[sym]
		if c == nil {
			c = &botCandidate{Symbol: sym, Name: name, Market: market}
			by[sym] = c
		}
		if c.Name == "" {
			c.Name = name
		}
		return c
	}
	moods := map[string]botMarketMood{}

	// ① 브리핑 — 이 시각의 많이 언급된 종목.
	for _, market := range []string{"domestic", "us"} {
		b, err := s.briefings.Latest(ctx, market)
		if err != nil {
			return nil, nil, err
		}
		if b == nil || now.Sub(b.GeneratedAt) > 12*time.Hour {
			continue
		}
		moods[market] = botMarketMood{Mood: b.Mood, Summary: b.Summary}
		for _, m := range b.Facts.TopMentioned {
			if m.Symbol == "" {
				continue
			}
			c := get(m.Symbol, m.Name, market)
			c.Mentions = m.Mentions
			c.Weight += 3 + min(float64(m.Mentions)/10, 5)
			if m.HasQuote {
				cr := m.ChangeRate
				c.ChangeRate = &cr
			}
		}
	}

	// ② 오늘의 변화 — 가장 최근 감지일.
	if dates, err := s.changes.Dates(ctx, 1); err != nil {
		return nil, nil, err
	} else if len(dates) > 0 {
		events, err := s.changes.List(ctx, repository.ChangeListQuery{Date: dates[0], Limit: 300})
		if err != nil {
			return nil, nil, err
		}
		for _, e := range events {
			w, ok := botEventWeight[e.Kind]
			if !ok || (e.AssetType != "domestic" && e.AssetType != "us") {
				continue
			}
			c := get(e.Symbol, e.Name, e.AssetType)
			c.Weight += w
			if len(c.Events) < 4 && e.Title != "" {
				c.Events = append(c.Events, e.Title)
			}
			if e.Kind == "score_jump" && c.ScoreJump == "" {
				c.ScoreJump = e.Title
			}
		}
	}

	// ③ 2일 안 실적 발표(프리뷰가 있는 것).
	from := now.In(kstLoc()).Format("2006-01-02")
	to := now.In(kstLoc()).AddDate(0, 0, 2).Format("2006-01-02")
	for _, market := range []string{"domestic", "us"} {
		rows, err := s.previews.List(ctx, market, from, to)
		if err != nil {
			return nil, nil, err
		}
		for _, r := range rows {
			var body struct {
				Headline string   `json:"headline"`
				Points   []string `json:"points"`
				Risk     string   `json:"risk"`
			}
			if json.Unmarshal(r.Body, &body) != nil || body.Headline == "" {
				continue
			}
			c := get(r.Symbol, r.Name, market)
			c.Weight += 2
			c.Upcoming = &botPreview{EventDate: r.EventDate, Fiscal: r.FiscalLabel,
				Headline: body.Headline, Points: body.Points, Risk: body.Risk}
		}
	}

	out := make([]*botCandidate, 0, len(by))
	for _, c := range by {
		if c.Name == "" {
			c.Name = c.Symbol
		}
		out = append(out, c)
	}
	// 지도 순회 순서를 지운다 — 같은 난수면 같은 결과(시험).
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out, moods, nil
}

var botEventWeight = map[string]float64{
	"score_jump":   3,
	"earnings":     3,
	"disclosure":   2,
	"volume_surge": 2,
	"news_event":   1.5,
	"insider":      1.5,
	"price_move":   1,
	"social":       1,
}

// botPick — 가중 랜덤, 중복 없이 n개.
func botPick(rnd *rand.Rand, cands []*botCandidate, n int, ok func(*botCandidate) bool) []*botCandidate {
	pool := make([]*botCandidate, 0, len(cands))
	for _, c := range cands {
		if c.Weight > 0 && ok(c) {
			pool = append(pool, c)
		}
	}
	out := []*botCandidate{}
	for len(out) < n && len(pool) > 0 {
		total := 0.0
		for _, c := range pool {
			total += c.Weight
		}
		x := rnd.Float64() * total
		i := 0
		for ; i < len(pool)-1; i++ {
			x -= pool[i].Weight
			if x < 0 {
				break
			}
		}
		out = append(out, pool[i])
		pool = append(pool[:i], pool[i+1:]...)
	}
	return out
}

func (s *BotDraftService) enrich(ctx context.Context, picked []*botCandidate, now time.Time) error {
	if len(picked) == 0 {
		return nil
	}
	syms := make([]string, 0, len(picked))
	for _, c := range picked {
		syms = append(syms, c.Symbol)
	}
	views, err := s.scores.LatestBatch(ctx, syms)
	if err != nil {
		return err
	}
	byScore := map[string]*model.StockScoreView{}
	for i := range views {
		byScore[strings.ToUpper(views[i].Symbol)] = &views[i]
	}
	closes, err := s.repo.LatestCloseAI(ctx, syms)
	if err != nil {
		return err
	}
	stocks, err := s.stocks.ListBySymbols(ctx, syms)
	if err != nil {
		return err
	}
	names := map[string]string{}
	for _, st := range stocks {
		names[strings.ToUpper(st.Symbol)] = st.Name
	}
	for _, c := range picked {
		c.Score = byScore[c.Symbol]
		c.CloseAI = closes[c.Symbol]
		if n := names[c.Symbol]; n != "" {
			c.Name = n
		}
		if c.Narrative == nil {
			c.Narrative, _ = s.narratives.Get(ctx, c.Symbol)
		}
		c.Articles, _ = s.repo.RecentArticles(ctx, c.Symbol, now.Add(-48*time.Hour), 2)
	}
	return nil
}

// ---------------------------------------------------------------- 글 (순수 함수)

var botQuestions = []string{
	"여러분은 어떻게 보시나요?",
	"지금 들고 계신 분 있나요? 생각 궁금합니다.",
	"단기 이슈일까요, 흐름이 바뀌는 걸까요?",
	"이 구간 어떻게 대응하실지 의견 나눠 봐요.",
	"다음에 확인할 포인트가 뭐라고 보세요?",
	"놓친 부분이 있으면 댓글로 알려 주세요.",
}

func botChoice(rnd *rand.Rand, list []string) string { return list[rnd.IntN(len(list))] }

// botReason — 관리자 화면용 「왜 골랐나」 한 줄.
func botReason(c *botCandidate) string {
	parts := []string{}
	if c.Mentions > 0 {
		parts = append(parts, fmt.Sprintf("뉴스 언급 %d건", c.Mentions))
	}
	if c.ScoreJump != "" {
		parts = append(parts, c.ScoreJump)
	}
	for _, e := range c.Events {
		if e != c.ScoreJump && len(parts) < 3 {
			parts = append(parts, e)
		}
	}
	if c.Upcoming != nil {
		parts = append(parts, botDateLabel(c.Upcoming.EventDate)+" 실적 발표")
	}
	return strings.Join(parts, " · ")
}

func botDateLabel(ymd string) string {
	t, err := time.Parse("2006-01-02", ymd)
	if err != nil {
		return ymd
	}
	return fmt.Sprintf("%d/%d", int(t.Month()), t.Day())
}

func botTitle(rnd *rand.Rand, c *botCandidate) string {
	var pool []string
	switch {
	case c.Upcoming != nil && rnd.IntN(2) == 0:
		d := botDateLabel(c.Upcoming.EventDate)
		pool = []string{
			c.Name + " " + d + " 실적 발표, 관전 포인트 정리",
			c.Name + " 실적 앞두고 체크할 것들",
			d + " " + c.Name + " 실적, 어떻게 보세요?",
		}
	case c.ScoreJump != "":
		pool = []string{
			c.Name + " — " + c.ScoreJump,
			c.Name + " 점수가 크게 움직였네요",
			c.Name + ", 오늘 점수 변화 이유 정리",
		}
	case c.Mentions >= 10:
		pool = []string{
			fmt.Sprintf("오늘 뉴스에 %d번 나온 %s", c.Mentions, c.Name),
			c.Name + " 오늘 왜 이렇게 자주 보이나 했더니",
			"요즘 많이 언급되는 " + c.Name + ", 정리해 봤습니다",
		}
	case len(c.Events) > 0:
		pool = []string{c.Name + " — " + c.Events[0], c.Name + " 오늘 무슨 일이 있었나"}
	default:
		pool = []string{c.Name + " 짧게 정리", c.Name + " 지금 어디쯤일까요"}
	}
	return botChoice(rnd, pool)
}

func botScoreLine(v *model.StockScoreView) string {
	if v == nil || v.Score == nil {
		return ""
	}
	line := fmt.Sprintf("AI 점수 %d점", *v.Score)
	if v.Grade != "" {
		line += " (" + v.Grade + ")"
	}
	if v.PeerPct != nil && v.PeerGroup != "" {
		line += fmt.Sprintf(" · %s 상위 %d%%", botPeerLabel(v.PeerGroup), model.ReadPeerPct(*v.PeerPct))
	}
	return line
}

// botPeerLabel — 피어그룹 내부 키를 화면 이름으로(FE `lib/score-explain-api.ts` peerGroupLabel과 같은 규칙).
// `domestic:KOSPI:cap3` → 「국내 KOSPI 대형주」. 모르는 조각은 그대로 둔다.
func botPeerLabel(group string) string {
	if group == "*" {
		return "전체 시장"
	}
	labels := map[string]string{
		"domestic": "국내", "us": "미국", "etf": "ETF", "etn": "ETN",
		"cap1": "소형주", "cap2": "중형주", "cap3": "대형주",
		"energy": "에너지", "materials": "소재", "industrials": "산업재",
		"consumer_discretionary": "경기소비재", "consumer_staples": "필수소비재", "health_care": "헬스케어",
		"financials": "금융", "information_technology": "IT", "communication": "커뮤니케이션",
		"utilities": "유틸리티", "real_estate": "부동산",
	}
	parts := strings.Split(group, ":")
	for i, p := range parts {
		if l, ok := labels[p]; ok {
			parts[i] = l
		}
	}
	return strings.Join(parts, " ")
}

func botAxesLine(v *model.StockScoreView) string {
	if v == nil {
		return ""
	}
	labels := map[model.ScoreAxis]string{
		model.AxisTechnical: "기술", model.AxisQuality: "퀄리티", model.AxisGrowth: "성장",
		model.AxisValuation: "가치", model.AxisSentiment: "심리",
	}
	parts := []string{}
	for _, ax := range model.ScoreAxes {
		if p := v.Breakdown.Get(ax); p != nil {
			parts = append(parts, fmt.Sprintf("%s %.0f", labels[ax], *p))
		}
	}
	return strings.Join(parts, " · ")
}

func botChangeLine(c *botCandidate) string {
	rate := c.ChangeRate
	if rate == nil && c.CloseAI.TradeDate != "" {
		r := c.CloseAI.ChangeRate
		rate = &r
	}
	if rate == nil {
		return ""
	}
	return fmt.Sprintf("등락률 %+.2f%%", *rate)
}

// botFirstSentences — 저장된 AI 문단을 n문장까지(짧은 글용).
func botFirstSentences(text string, n int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	out := []string{}
	for _, part := range strings.SplitAfter(text, ". ") {
		out = append(out, strings.TrimSpace(part))
		if len(out) >= n {
			break
		}
	}
	return clipBotText(strings.Join(out, " "), 400)
}

func clipBotText(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:max])) + "…"
}

// buildBotCommunityDraft — 커뮤니티 본문은 일반 글(마크다운 아님). `#`·`@`는 태그로 읽히니 쓰지 않는다.
func buildBotCommunityDraft(rnd *rand.Rand, c *botCandidate, mood botMarketMood, long bool, slot time.Time) repository.BotDraft {
	var b strings.Builder
	// 종가 AI 한 줄은 실제로는 여러 문단이다 — 머리는 두 문장까지(나머지는 긴 글의 섹션들이 말한다).
	lead := botFirstSentences(c.CloseAI.Summary, 2)
	if lead == "" && c.Narrative != nil {
		lead = botFirstSentences(c.Narrative.Overview, 2)
	}
	if lead == "" && len(c.Events) > 0 {
		lead = c.Name + " — " + strings.Join(c.Events, ", ")
	}
	facts := []string{}
	for _, f := range []string{botChangeLine(c), botScoreLine(c.Score)} {
		if f != "" {
			facts = append(facts, f)
		}
	}
	if c.Mentions > 0 {
		facts = append(facts, fmt.Sprintf("오늘 뉴스 언급 %d건", c.Mentions))
	}
	if c.Upcoming != nil {
		facts = append(facts, botDateLabel(c.Upcoming.EventDate)+" 실적 발표 예정")
	}

	if !long {
		if lead != "" {
			b.WriteString(lead + "\n\n")
		}
		for _, f := range facts {
			b.WriteString("· " + f + "\n")
		}
		b.WriteString("\n" + botChoice(rnd, botQuestions))
	} else {
		if mood.Mood != "" {
			// 종목 이름 뒤에 조사를 붙이지 않는다(받침에 따라 이/가가 갈린다).
			fmt.Fprintf(&b, "오늘 시장 분위기는 %s 쪽입니다. 그 안에서 눈에 띈 종목, %s 정리해 봤어요.\n\n", mood.Mood, c.Name)
		}
		b.WriteString("■ 오늘 무슨 일이\n")
		if lead != "" {
			b.WriteString(lead + "\n")
		}
		for _, e := range c.Events {
			b.WriteString("· " + e + "\n")
		}
		for _, f := range facts {
			b.WriteString("· " + f + "\n")
		}
		if c.Narrative != nil && c.Narrative.Overview != "" {
			b.WriteString("\n■ AI 요약\n" + clipBotText(c.Narrative.Overview, 700) + "\n")
		}
		if axes := botAxesLine(c.Score); axes != "" {
			b.WriteString("\n■ 점수로 보면\n" + botScoreLine(c.Score) + "\n" + axes + "\n")
		}
		if c.Upcoming != nil {
			b.WriteString("\n■ 실적 관전 포인트\n" + c.Upcoming.Headline + "\n")
			for _, p := range c.Upcoming.Points {
				b.WriteString("· " + p + "\n")
			}
		}
		if c.Narrative != nil && c.Narrative.News != "" {
			b.WriteString("\n■ 뉴스 흐름\n" + clipBotText(c.Narrative.News, 500) + "\n")
		}
		b.WriteString("\n" + botChoice(rnd, botQuestions))
	}
	return repository.BotDraft{
		Kind: repository.BotDraftCommunity, SlotAt: slot, Symbol: c.Symbol, SymbolName: c.Name, Market: c.Market,
		Length: botLength(long), Reason: botReason(c), Title: clipBotText(botTitle(rnd, c), 90),
		Body: botStripTags(strings.TrimSpace(b.String())), Articles: c.Articles,
	}
}

// buildBotResearchDraft — 리포트 본문은 마크다운. 방향·목표가는 넣지 않는다(게시할 때 관리자가 정한다).
func buildBotResearchDraft(rnd *rand.Rand, c *botCandidate, long bool, slot time.Time) repository.BotDraft {
	n := c.Narrative
	if n == nil {
		n = &repository.StockSummaryNarrative{}
	}
	var b strings.Builder
	section := func(title, body string) {
		if body = strings.TrimSpace(body); body != "" {
			b.WriteString("## " + title + "\n\n" + body + "\n\n")
		}
	}
	why := []string{}
	if lead := botFirstSentences(c.CloseAI.Summary, 2); lead != "" {
		why = append(why, lead)
	}
	why = append(why, c.Events...)
	if c.Mentions > 0 {
		why = append(why, fmt.Sprintf("최근 뉴스 언급 %d건", c.Mentions))
	}
	if c.Upcoming != nil {
		why = append(why, botDateLabel(c.Upcoming.EventDate)+" "+c.Upcoming.Fiscal+" 실적 발표 예정")
	}
	whyMD := ""
	for _, w := range why {
		whyMD += "- " + strings.TrimSpace(w) + "\n"
	}
	scoreMD := ""
	if line := botScoreLine(c.Score); line != "" {
		scoreMD = line
		if axes := botAxesLine(c.Score); axes != "" {
			scoreMD += "\n\n" + axes
		}
	}

	if !long {
		section("한 줄 요약", botFirstSentences(n.Overview, 2))
		section("지금 보는 이유", whyMD)
		section("점수", scoreMD)
	} else {
		section("요약", n.Overview)
		section("지금 보는 이유", whyMD)
		section("점수", scoreMD)
		section("밸류에이션", n.Valuation)
		section("기술적 흐름", n.Technical)
		section("내부자·수급", n.Insider)
		section("뉴스 흐름", n.News)
		if c.Upcoming != nil {
			pts := c.Upcoming.Headline + "\n\n"
			for _, p := range c.Upcoming.Points {
				pts += "- " + p + "\n"
			}
			section("실적 관전 포인트", pts)
		}
		risk := ""
		if c.Upcoming != nil && c.Upcoming.Risk != "" {
			risk = "- " + c.Upcoming.Risk + "\n"
		}
		section("위험 요인", risk)
	}
	titles := []string{
		c.Name + ": " + botTitleHook(c),
		c.Name + " 점검 — " + botTitleHook(c),
	}
	return repository.BotDraft{
		Kind: repository.BotDraftResearch, SlotAt: slot, Symbol: c.Symbol, SymbolName: c.Name, Market: c.Market,
		Length: botLength(long), Reason: botReason(c), Title: clipBotText(botChoice(rnd, titles), 120),
		Body: strings.TrimSpace(b.String()), Articles: c.Articles,
	}
}

func botTitleHook(c *botCandidate) string {
	switch {
	case c.Upcoming != nil:
		return "실적 발표 앞두고 볼 것"
	case c.ScoreJump != "":
		return c.ScoreJump
	case c.Mentions >= 10:
		return "뉴스가 몰린 이유"
	case c.Score != nil && c.Score.Score != nil:
		return fmt.Sprintf("AI 점수 %d점의 근거", *c.Score.Score)
	}
	return "지금 위치 정리"
}

func botLength(long bool) string {
	if long {
		return "long"
	}
	return "short"
}

// botStripTags — 커뮤니티 본문에서 `#`·`@`가 태그로 읽히지 않게 전각으로.
func botStripTags(s string) string {
	return strings.NewReplacer("#", "＃", "@", "＠").Replace(s)
}
