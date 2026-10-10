package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
AssetAIService — 부동산·금융상품 AI 해설 (2026-10-01 사용자 요청). worker-asset의 asset_ai 잡, 점수 잡 뒤.

키는 GEMINI_SUB_API_KEY 하나(버킷 llm_sub) + 기능 버킷 llm_feat_asset — 주식·뉴스의 KR/US 키를 쓰지 않는다.
키가 없거나 버킷이 차면 규칙 문장으로 채운다(화면 자리는 늘 찬다).

순서(싸고 많이 보이는 것부터):

	① 금리 한 줄(1콜)  ② 시군구 해설(전국, 6개씩 ~42콜)  ③ 단지 해설(보유 단지 + 시군구마다 점수 상위 5, 6개씩 ~210콜)
	④ 예·적금 우대조건 구조화(8개씩 ≤ 60콜/회)

**한도 안에서 매일 돌게**(2026-10-02 사용자): 입력 숫자를 반올림(지수 0.1%p · 단지 모멘텀 1%p · 건수·비율 5 단위)해
해시가 「실제로 달라졌을 때」만 다시 부른다. 입력 해시가 같고 이미 llm이면 건너뜀. 기능 버킷(llm_feat_asset 250콜)이
차면 나머지는 다음 날 — 이미 AI 글이 있던 대상은 규칙 문장으로 덮지 않고 옛 AI 글을 둔다(다음 회차 재시도).
전국 첫 채우기(~350콜)는 이틀, 그 뒤 하루 수십 콜.
*/
type AssetAIService struct {
	client *llm.Client
	budget *apibudget.Manager
	intel  *repository.AssetIntelRepo
	data   *repository.PublicDataRepo
	fin    *repository.FinProductRepo
	wealth *repository.WealthRepo
	// news — 부동산·금리·예적금·대출 기사 제목(2026-10-09 요청 5). nil이면 기사 없이 숫자만.
	news *repository.NewsRepo
	now  func() time.Time
}

// WithNews wires topic headlines into the briefs and returns s.
func (s *AssetAIService) WithNews(n *repository.NewsRepo) *AssetAIService {
	s.news = n
	return s
}

/*
topicNews — LLM 입력에 붙일 최근 기사 제목. 제목만(본문 0) · 60자 절단 · 같은 사건 하나 · 상한 n —
새 기사 블록이 콜당 ~400토큰을 넘지 않게(2026-10-09 요청 5 토큰 관리). 기사는 숫자 근거가 아니라 「무슨 이야기가
오가나」의 참고이고, 프롬프트도 그렇게 말한다.
*/
func (s *AssetAIService) topicNews(ctx context.Context, n int, topics ...string) []string {
	if s.news == nil {
		return nil
	}
	h, err := s.news.TopicHeadlines(ctx, topics, s.now().AddDate(0, 0, -3), n)
	if err != nil {
		log.Printf("asset ai: topic news: %v", err)
		return nil
	}
	return h
}

func NewAssetAIService(client *llm.Client, budget *apibudget.Manager, intel *repository.AssetIntelRepo, data *repository.PublicDataRepo,
	fin *repository.FinProductRepo, wealth *repository.WealthRepo) *AssetAIService {
	return &AssetAIService{client: client, budget: budget, intel: intel, data: data, fin: fin, wealth: wealth, now: time.Now}
}

const (
	assetAIRegionBatch  = 6
	assetAIAreaBatch    = 5
	assetAIComplexBatch = 6
	assetAICondsBatch   = 8
	assetAICondsCalls   = 60
	assetAIComplexPerRegion = 5
)

// q rounds v to the nearest step — 해시가 잔물결에 흔들리지 않게(같은 뜻이면 같은 입력).
func q(v, step float64) float64 { return math.Round(v/step) * step }

type aiRun struct {
	s        *AssetAIService
	ctx      context.Context
	on       bool
	lastCall time.Time
	calls    int
	tokens   int64
	fails    int
}

// call runs one LLM request under both buckets. false = 이번 회차엔 LLM을 더 쓰지 않는다.
func (r *aiRun) call(kind, input string, n int, fn func() (int64, error)) bool {
	if !r.on {
		return false
	}
	if err := r.s.budget.CheckTokens(r.ctx, apibudget.ProviderLLMFeatAsset, llm.EstimateAssetTokens(kind, input, n)); err != nil {
		log.Printf("asset ai: feature budget — %v (나머지는 규칙 문장)", err)
		r.on = false
		return false
	}
	if wait := r.s.client.Limits().MinInterval - time.Since(r.lastCall); !r.lastCall.IsZero() && wait > 0 && !sleepCtx(r.ctx, wait) {
		r.on = false
		return false
	}
	used, err := fn()
	// 간격(llm_sub 4초)에 막혔으면 한 번 기다렸다 다시 — 클라이언트가 503 뒤 안에서 재시도하면 버킷의
	// 마지막 호출이 우리가 잰 시각보다 늦다(2026-10-02, 첫 실행이 이것으로 전부 규칙 문장이 됐다).
	if err != nil && errors.Is(err, apibudget.ErrPacing) && sleepCtx(r.ctx, r.s.client.Limits().MinInterval) {
		used, err = fn()
	}
	r.lastCall = time.Now() // 끝난 시각부터 잰다
	r.calls++
	if err != nil {
		r.fails++
		log.Printf("asset ai %s: %v", kind, err)
		if llm.IsBudgetExhausted(err) {
			r.on = false
		}
		return false
	}
	r.tokens += used
	r.s.budget.Record(r.ctx, apibudget.ProviderLLMFeatAsset)
	r.s.budget.RecordTokens(r.ctx, apibudget.ProviderLLMFeatAsset, used)
	return true
}

func hashOf(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Run is the asset_ai job.
func (s *AssetAIService) Run(ctx context.Context) (map[string]any, string, error) {
	run := &aiRun{s: s, ctx: ctx, on: s.client != nil && s.client.Enabled()}
	result := map[string]any{"llm": run.on}
	steps := []struct {
		name string
		fn   func(*aiRun) (int, int, error)
	}{
		{"rates", s.ratesNote},
		{"re_area", s.areaNotes},
		{"briefs", s.briefs},
		{"re_region", s.regionNotes},
		{"re_complex", s.complexNotes},
		{"fin_conds", s.finConds},
	}
	var parts []string
	for _, st := range steps {
		llmN, ruleN, err := st.fn(run)
		if err != nil {
			return result, "", fmt.Errorf("%s: %w", st.name, err)
		}
		result[st.name] = map[string]int{"llm": llmN, "rule": ruleN}
		parts = append(parts, fmt.Sprintf("%s AI %d·규칙 %d", st.name, llmN, ruleN))
	}
	result["calls"], result["tokens"], result["fails"] = run.calls, run.tokens, run.fails
	msg := fmt.Sprintf("%s · 호출 %d · 토큰 %d", strings.Join(parts, " · "), run.calls, run.tokens)
	if !run.on && s.client == nil {
		msg += " · LLM 키 없음(GEMINI_SUB_API_KEY)"
	}
	return result, msg, nil
}

func (s *AssetAIService) model() string {
	if s.client == nil {
		return ""
	}
	return s.client.Model()
}

// ── ① 금리 ──────────────────────────────────────────────────────────────────

// rateRows — 금리 계열별 [이름, 현재, 1주 전, 3개월 전]과 가장 늦은 기준일. 금리 한 줄과 홈 브리핑이 같이 쓴다.
func (s *AssetAIService) rateRows(ctx context.Context, keys []string) ([][]any, string) {
	now := kstToday(s.now())
	var rows [][]any
	date := ""
	for _, d := range RateSeriesDefs {
		if !containsString(keys, d.Key) {
			continue
		}
		pts, err := s.data.RateSeries(ctx, d.ID(), now.AddDate(0, -4, 0))
		if err != nil || len(pts) == 0 {
			continue
		}
		last := pts[len(pts)-1]
		at := func(days int) float64 {
			cut := now.AddDate(0, 0, -days).Format("2006-01-02")
			v := pts[0].Value
			for _, p := range pts {
				if p.Date <= cut {
					v = p.Value
				}
			}
			return v
		}
		rows = append(rows, []any{d.Label, last.Value, at(7), at(91)})
		if last.Date > date {
			date = last.Date
		}
	}
	return rows, date
}

func (s *AssetAIService) ratesNote(r *aiRun) (int, int, error) {
	rows, date := s.rateRows(r.ctx, []string{"base", "cd91", "ktb3", "ktb10", "corpAA", "deposit1y", "mortgage", "creditLoan"})
	if len(rows) == 0 {
		return 0, 0, nil
	}
	facts := map[string]any{"d": date, "r": rows}
	input, _ := json.Marshal(facts)
	h := hashOf(facts)
	prev, _ := s.intel.NoteHashes(r.ctx, "rates")
	if p, ok := prev["kr"]; ok && p[0] == h && p[1] == "llm" {
		return 0, 0, nil
	}
	body := RuleRatesNote(rows)
	source, model := "rule", ""
	var used int64
	r.call("rates", string(input), 1, func() (int64, error) {
		out, u, err := s.client.GenerateRatesNote(r.ctx, string(input))
		if err == nil && out.Note != "" {
			body, source, model, used = map[string]any{"note": out.Note, "points": out.Points}, "llm", s.model(), u
		}
		return u, err
	})
	err := s.intel.UpsertNote(r.ctx, repository.AINote{Kind: "rates", Key: "kr", InputHash: h, Body: rawJSON(body), Source: source, Model: model, Tokens: int(used)})
	if source == "llm" {
		return 1, 0, err
	}
	return 0, 1, err
}

// RuleRatesNote — LLM이 없을 때의 금리 한 줄.
func RuleRatesNote(rows [][]any) map[string]any {
	parts := []string{}
	for _, r := range rows {
		name, _ := r[0].(string)
		now, _ := r[1].(float64)
		m3, _ := r[3].(float64)
		if name == "한국은행 기준금리" || name == "국고채 3년" || name == "주택담보대출 평균" {
			dir := "보합"
			if d := now - m3; d >= 0.05 {
				dir = fmt.Sprintf("3개월 새 %.2f%%p 상승", d)
			} else if d <= -0.05 {
				dir = fmt.Sprintf("3개월 새 %.2f%%p 하락", -d)
			}
			parts = append(parts, fmt.Sprintf("%s %.2f%%(%s)", name, now, dir))
		}
	}
	return map[string]any{"note": strings.Join(parts, " · "), "points": []string{}}
}

// ── ② 시군구 · ③ 단지 ───────────────────────────────────────────────────────

type noteTarget struct {
	key  string
	in   map[string]any
	rule map[string]any
}

// notesFor runs batched notes for targets, skipping unchanged llm notes.
func (s *AssetAIService) notesFor(r *aiRun, kind, llmKind string, targets []noteTarget, batch int) (int, int, error) {
	prev, err := s.intel.NoteHashes(r.ctx, kind)
	if err != nil {
		return 0, 0, err
	}
	var todo []noteTarget
	for _, t := range targets {
		if p, ok := prev[t.key]; ok && p[0] == hashOf(t.in) && p[1] == "llm" {
			continue
		}
		todo = append(todo, t)
	}
	nLLM, nRule := 0, 0
	for start := 0; start < len(todo); start += batch {
		chunk := todo[start:min(start+batch, len(todo))]
		ins := make([]map[string]any, len(chunk))
		for i, t := range chunk {
			ins[i] = t.in
		}
		input, _ := json.Marshal(ins)
		got := map[string]llm.AssetNoteItem{}
		var used int64
		r.call(llmKind, string(input), len(chunk), func() (int64, error) {
			items, u, err := s.client.GenerateAssetNotes(r.ctx, llmKind, string(input), len(chunk))
			for _, it := range items {
				if it.Note != "" {
					got[it.Key] = it
				}
			}
			used = u
			return u, err
		})
		for _, t := range chunk {
			body, source, model, tok := t.rule, "rule", "", 0
			if it, ok := got[t.key]; ok {
				body, source, model = map[string]any{"note": it.Note, "tags": it.Tags}, "llm", s.model()
				tok = int(used) / max(1, len(got))
				nLLM++
			} else if p, ok := prev[t.key]; ok && p[1] == "llm" {
				continue // 옛 AI 글이 규칙 문장보다 낫다 — 해시가 달라 다음 회차에 다시 부른다
			} else {
				nRule++
			}
			if err := s.intel.UpsertNote(r.ctx, repository.AINote{Kind: kind, Key: t.key, InputHash: hashOf(t.in),
				Body: rawJSON(body), Source: source, Model: model, Tokens: tok}); err != nil {
				return nLLM, nRule, err
			}
		}
	}
	return nLLM, nRule, nil
}

func factsMap(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	return m
}

func num(m map[string]any, k string) float64 {
	v, _ := m[k].(float64)
	return v
}

func (s *AssetAIService) regionNotes(r *aiRun) (int, int, error) {
	regions, err := s.intel.TopScores(r.ctx, "re_region", "", "", "score", 1000)
	if err != nil {
		return 0, 0, err
	}
	// 전국 — 거래 많은 지역부터(예산이 모자라면 큰 시장이 먼저 찬다).
	sort.Slice(regions, func(i, j int) bool {
		return num(factsMap(regions[i].Facts), "recentVol") > num(factsMap(regions[j].Facts), "recentVol")
	})
	complexes, err := s.intel.TopScores(r.ctx, "re_complex", "", "", "momentumPct", 200000)
	if err != nil {
		return 0, 0, err
	}
	topBy := map[string][][]any{}
	for _, c := range complexes {
		f := factsMap(c.Facts)
		if num(f, "trades6m") < 3 || len(topBy[c.Group]) >= 3 {
			continue
		}
		topBy[c.Group] = append(topBy[c.Group], []any{c.Name, q(num(f, "momentumPct"), 1)})
	}
	targets := make([]noteTarget, 0, len(regions))
	for _, g := range regions {
		f := factsMap(g.Facts)
		in := map[string]any{"k": g.Key, "n": g.Name, "w1": q(num(f, "w1Pct"), 0.1), "w4": q(num(f, "w4Pct"), 0.1), "w12": q(num(f, "w12Pct"), 0.1),
			"j4": q(num(f, "jeonse4Pct"), 0.1), "vol": q(num(f, "volTrendPct"), 5), "top": topBy[g.Key]}
		// 그 시군구 이름이 제목에 든 최근 기사 하나(있을 때만 — 없는 지역의 해시는 그대로라 다시 부르지 않는다).
		if t, _ := f["newsTitle"].(string); t != "" {
			in["news"] = clipRunes(t, 50)
		}
		targets = append(targets, noteTarget{key: g.Key, in: in, rule: RuleRegionNote(g.Name, f)})
	}
	return s.notesFor(r, "re_region", "region", targets, assetAIRegionBatch)
}

// areaNotes — 전국·시도 해설(2026-10-02 사용자: 「전국·서울처럼 큰 틀에서도 AI 요약」). 키 = 시도 코드, 전국 "00".
// 16개 남짓이라 한 회차 3~4콜. 하위 시군구의 오름·내림 수와 많이 오른·내린 곳을 같이 줘 「어디가 끌었나」를 쓰게 한다.
func (s *AssetAIService) areaNotes(r *aiRun) (int, int, error) {
	sidos, err := s.intel.TopScores(r.ctx, "re_sido", "", "", "score", 100)
	if err != nil {
		return 0, 0, err
	}
	nation, err := s.intel.TopScores(r.ctx, "re_nation", "", "", "score", 1)
	if err != nil {
		return 0, 0, err
	}
	var targets []noteTarget
	for _, a := range append(nation, sidos...) {
		f := factsMap(a.Facts)
		in := map[string]any{"k": a.Key, "n": a.Name, "w1": q(num(f, "w1Pct"), 0.05), "w4": q(num(f, "w4Pct"), 0.05), "w12": q(num(f, "w12Pct"), 0.1),
			"j4": q(num(f, "jeonse4Pct"), 0.05), "vol": q(num(f, "volTrendPct"), 5), "up": num(f, "up"), "dn": num(f, "down"), "sub": num(f, "regions"),
			"top": roundPairs(f["top"]), "bot": roundPairs(f["bottom"])}
		if a.Key == "00" {
			in["sido"] = roundPairs(f["sidos"])
		}
		targets = append(targets, noteTarget{key: a.Key, in: in, rule: RuleAreaNote(a.Name, f)})
	}
	return s.notesFor(r, "re_area", "area", targets, assetAIAreaBatch)
}

// roundPairs — [[이름, %]] 의 %를 0.1 단위로(해시가 잔물결에 흔들리지 않게).
func roundPairs(v any) [][]any {
	list, _ := v.([]any)
	out := [][]any{}
	for _, x := range list {
		p, _ := x.([]any)
		if len(p) == 2 {
			f, _ := p[1].(float64)
			out = append(out, []any{p[0], q(f, 0.1)})
		}
	}
	return out
}

// RuleAreaNote — 전국·시도 규칙 문장(LLM이 없거나 한도가 찼을 때).
func RuleAreaNote(name string, f map[string]any) map[string]any {
	note := RuleRegionNote(name, f)["note"].(string)
	if n := num(f, "regions"); n > 0 {
		note += fmt.Sprintf(" 시군구 %.0f곳 중 4주 상승 %.0f·하락 %.0f.", n, num(f, "up"), num(f, "down"))
	}
	return map[string]any{"note": note, "tags": RuleRegionNote(name, f)["tags"]}
}

// RuleRegionNote — 지수·거래량으로 만든 지역 한 줄.
func RuleRegionNote(name string, f map[string]any) map[string]any {
	w4 := num(f, "w4Pct")
	dir := "보합"
	switch {
	case w4 >= 0.3:
		dir = "상승"
	case w4 <= -0.3:
		dir = "하락"
	}
	note := fmt.Sprintf("%s 아파트 매매지수 4주 %+.2f%%(%s), 12주 %+.2f%%.", name, w4, dir, num(f, "w12Pct"))
	if v := num(f, "volTrendPct"); math.Abs(v) >= 10 {
		note += fmt.Sprintf(" 거래량은 직전 3개월보다 %+.0f%%.", v)
	}
	return map[string]any{"note": note, "tags": []string{dir}}
}

func (s *AssetAIService) complexNotes(r *aiRun) (int, int, error) {
	held := map[string]bool{}
	if assets, err := s.wealth.AssetsOfType(r.ctx, "real_estate"); err == nil {
		for _, a := range assets {
			if d := parseReDetail(a.Detail); d.ComplexKey != "" {
				held[d.ComplexKey] = true
			}
		}
	}
	all, err := s.intel.TopScores(r.ctx, "re_complex", "", "", "score", 200000)
	if err != nil {
		return 0, 0, err
	}
	// 시군구마다 점수 상위 5(6개월 거래 6건 이상) — 거래 많은 단지부터 부른다.
	perRegion := map[string]int{}
	var top []repository.AssetScore
	for _, c := range all {
		if num(factsMap(c.Facts), "trades6m") < 6 || perRegion[c.Group] >= assetAIComplexPerRegion {
			continue
		}
		perRegion[c.Group]++
		top = append(top, c)
	}
	sort.SliceStable(top, func(i, j int) bool {
		return num(factsMap(top[i].Facts), "trades6m") > num(factsMap(top[j].Facts), "trades6m")
	})
	keys := make([]string, 0, len(held))
	for k := range held {
		keys = append(keys, k)
	}
	heldScores, err := s.intel.Scores(r.ctx, "re_complex", keys)
	if err != nil {
		return 0, 0, err
	}
	seen := map[string]bool{}
	var targets []noteTarget
	add := func(c repository.AssetScore) {
		if seen[c.Key] {
			return
		}
		seen[c.Key] = true
		f := factsMap(c.Facts)
		in := map[string]any{"k": c.Key, "n": c.Name, "r": f["umdNm"], "mom": q(num(f, "momentumPct"), 1), "tr": q(num(f, "trades6m"), 5),
			"jr": q(num(f, "jeonseRatio"), 5), "pk": q(num(f, "peakPct"), 5), "pr": q(num(f, "premiumPct"), 5), "by": f["buildYear"]}
		targets = append(targets, noteTarget{key: c.Key, in: in, rule: RuleComplexNote(c.Name, f)})
	}
	for _, c := range heldScores {
		add(c)
	}
	for _, c := range top {
		add(c)
	}
	return s.notesFor(r, "re_complex", "complex", targets, assetAIComplexBatch)
}

// RuleComplexNote — 실거래 흐름으로 만든 단지 한 줄.
func RuleComplexNote(name string, f map[string]any) map[string]any {
	note := fmt.Sprintf("최근 3개월 ㎡당 가격은 그 전 9개월보다 %+.1f%%, 6개월 거래 %.0f건.", num(f, "momentumPct"), num(f, "trades6m"))
	if jr := num(f, "jeonseRatio"); jr > 0 {
		note += fmt.Sprintf(" 전세가율 %.0f%%.", jr)
	}
	return map[string]any{"note": note, "tags": []string{}}
}

// ── ④ 우대조건 ──────────────────────────────────────────────────────────────

func (s *AssetAIService) finConds(r *aiRun) (int, int, error) {
	prev, err := s.intel.NoteHashes(r.ctx, "fin_conds")
	if err != nil {
		return 0, 0, err
	}
	scores, err := s.intel.TopScores(r.ctx, "fin_product", "", "", "score", 5000)
	if err != nil {
		return 0, 0, err
	}
	rank := map[string]int{}
	for i, sc := range scores {
		rank[sc.Key] = i
	}
	today := kstToday(s.now()).Format("20060102")
	type item struct {
		id   string
		in   map[string]any
		rank int
	}
	var todo []item
	for _, kind := range []string{"deposit", "saving"} {
		list, err := s.fin.List(r.ctx, kind, today)
		if err != nil {
			return 0, 0, err
		}
		for _, p := range list {
			var b map[string]any
			_ = json.Unmarshal(p.Base, &b)
			text := marketdata.FinStr(b, "spcl_cnd")
			if text == "" || text == "없음" || utf8.RuneCountInString(text) < 4 {
				continue
			}
			rt := ProductRates(kind, p.Options)
			id := fmt.Sprint(p.ID)
			in := map[string]any{"id": id, "n": p.ProductName, "b": rt.Base, "m": rt.Max, "t": clipRunes(text, 400)}
			if pv, ok := prev[id]; ok && pv[0] == hashOf(in) {
				continue
			}
			rk, ok := rank[id]
			if !ok {
				rk = 1 << 20
			}
			todo = append(todo, item{id, in, rk})
		}
	}
	sort.Slice(todo, func(i, j int) bool { return todo[i].rank < todo[j].rank })
	nLLM, calls := 0, 0
	for start := 0; start < len(todo) && calls < assetAICondsCalls && r.on; start += assetAICondsBatch {
		chunk := todo[start:min(start+assetAICondsBatch, len(todo))]
		ins := make([]map[string]any, len(chunk))
		for i, t := range chunk {
			ins[i] = t.in
		}
		input, _ := json.Marshal(ins)
		calls++
		var got []llm.FinCondsItem
		var used int64
		r.call("conds", string(input), len(chunk), func() (int64, error) {
			items, u, err := s.client.GenerateFinConds(r.ctx, string(input), len(chunk))
			got, used = items, u
			return u, err
		})
		byID := map[string]llm.FinCondsItem{}
		for _, it := range got {
			byID[it.ID] = it
		}
		for _, t := range chunk {
			it, ok := byID[t.id]
			if !ok {
				continue // 다음 회차에 다시(규칙 대체는 없다 — 원문이 화면에 그대로 있다)
			}
			if err := s.intel.UpsertNote(r.ctx, repository.AINote{Kind: "fin_conds", Key: t.id, InputHash: hashOf(t.in),
				Body: rawJSON(it), Source: "llm", Model: s.model(), Tokens: int(used) / max(1, len(got))}); err != nil {
				return nLLM, 0, err
			}
			nLLM++
		}
	}
	return nLLM, 0, nil
}

func clipRunes(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
