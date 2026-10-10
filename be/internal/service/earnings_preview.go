package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
실적 「관전 포인트」 (2026-09-27 사용자 결정 C ②).

「향후 7일에 어떤 종목이 실적을 내고, 그 종목의 AI 점수·최근 기사·가격 흐름으로 보아 무엇을
봐야 하나」를 **매일 07:10 KST 배치**가 만들어 저장한다. 화면은 저장분만 읽는다(llm_spend_rule).

# 토큰 사전 조치 (사용자 요청 5 「토큰 사용량이 많아질 것 — 사전 조치」)

  1. **기능 버킷** `llm_feat_earnings`(하루 60콜 · 12만 토큰) — 키 버킷(종목 요약·브리핑과 공유)과
     별개로 이 기능의 몫을 떼어 둔다. 다 쓰면 이 기능만 규칙 문장으로 떨어진다.
  2. **사실표만** 넘긴다 — 숫자와 기사 제목 여섯. 콜당 ~1.5K 토큰.
  3. **다시 굽지 않는다** — 같은 사실표(해시)면 건너뛰고, 사실이 바뀌어도 AI 글이 3일 안이면
     그대로 둔다. 발표 하루 전에만 한 번 새로 쓴다(마지막 확인).
  4. 대상은 시장당 30종목 — 누군가 보유·관심인 종목이 먼저, 그다음 시가총액 순.
  5. 07:10 KST — Gemini가 매일 밤 503을 내는 시간대(22~03시)를 피한다.
*/

const (
	earningsPreviewWindowDays   = 7
	earningsPreviewMaxPerMarket = 30
	earningsPreviewFreshFor     = 72 * time.Hour
	earningsPreviewSlotHour     = 7
	earningsPreviewSlotMinute   = 10
)

type EarningsPreviewService struct {
	repo   *repository.EarningsPreviewRepo
	llm    *llm.Pool
	budget *apibudget.Manager
	runs   *repository.BatchJobRunRepo
	// calendar supplies 국내 **추정** 일정(작년 발표일 + 1년). 국내 다가오는 발표는 대부분 저장 행이
	// 아니라 읽을 때 추정되므로, 이것 없이는 국내 대상이 거의 비었다(2026-09-27 실측).
	calendar *EarningsCalendarService
	// digest — 실적 화면 맨 위 AI 요약(2026-10-09 요청 1). 이 배치 끝에서 시장마다 한 번.
	digest *EarningsDigestService
	now    func() time.Time
	sleep  func(context.Context, time.Duration) bool
}

func NewEarningsPreviewService(repo *repository.EarningsPreviewRepo, pool *llm.Pool, budget *apibudget.Manager) *EarningsPreviewService {
	return &EarningsPreviewService{repo: repo, llm: pool, budget: budget, now: time.Now, sleep: sleepCtx}
}

func (s *EarningsPreviewService) SetBatchRuns(r *repository.BatchJobRunRepo) { s.runs = r }

// SetCalendar wires the domestic estimate source.
func (s *EarningsPreviewService) SetCalendar(c *EarningsCalendarService) { s.calendar = c }

// SetDigest wires the page-top AI digest that runs at the end of each batch.
func (s *EarningsPreviewService) SetDigest(d *EarningsDigestService) { s.digest = d }

// Digest returns the stored page-top digest (nil before the first run or when not wired).
func (s *EarningsPreviewService) Digest(ctx context.Context, market string) (*repository.EarningsDigestRow, error) {
	if s.digest == nil {
		return nil, nil
	}
	return s.digest.Get(ctx, market)
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func earningsPreviewWindow(now time.Time) (string, string) {
	k := now.In(kstLoc())
	return k.Format("2006-01-02"), k.AddDate(0, 0, earningsPreviewWindowDays).Format("2006-01-02")
}

func kstLoc() *time.Location {
	if loc, err := time.LoadLocation("Asia/Seoul"); err == nil {
		return loc
	}
	return time.FixedZone("KST", 9*3600)
}

// List returns stored previews for the coming week (the page's data).
func (s *EarningsPreviewService) List(ctx context.Context, market string) ([]repository.EarningsPreviewRow, error) {
	from, to := earningsPreviewWindow(s.now())
	rows, err := s.repo.List(ctx, normalizePreviewMarket(market), from, to)
	if err != nil {
		return nil, err
	}
	return LatestPreviewPerSymbol(rows), nil
}

// LatestPreviewPerSymbol keeps one row per symbol — the most recently generated.
// 추정일(10/14)로 만든 뒤 확정일(10/10)이 들어오면 키(symbol, event_date)가 달라 두 줄이 된다.
func LatestPreviewPerSymbol(rows []repository.EarningsPreviewRow) []repository.EarningsPreviewRow {
	best := map[string]int{}
	for i, r := range rows {
		if j, ok := best[r.Symbol]; !ok || r.GeneratedAt.After(rows[j].GeneratedAt) {
			best[r.Symbol] = i
		}
	}
	out := make([]repository.EarningsPreviewRow, 0, len(best))
	for i, r := range rows {
		if best[r.Symbol] == i {
			out = append(out, r)
		}
	}
	return out
}

func normalizePreviewMarket(m string) string {
	if strings.EqualFold(strings.TrimSpace(m), "us") || strings.EqualFold(strings.TrimSpace(m), "overseas") {
		return "us"
	}
	return "domestic"
}

// RankEarningsPreviewTargets puts held/watched first, then market cap, and caps.
func RankEarningsPreviewTargets(ts []repository.EarningsPreviewTarget, max int) []repository.EarningsPreviewTarget {
	out := append([]repository.EarningsPreviewTarget(nil), ts...)
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := out[i].Held || out[i].Watched, out[j].Held || out[j].Watched
		if ai != aj {
			return ai
		}
		return out[i].MarketCap > out[j].MarketCap
	})
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// PreviewNeedsRegen decides whether a stored preview should be rewritten.
func PreviewNeedsRegen(exists bool, storedHash, newHash, storedSource string, age time.Duration, daysToEvent int) bool {
	if !exists {
		return true
	}
	if storedSource != "llm" {
		return true // 규칙 문장(한도·밤 503)이었으면 사실이 같아도 AI로 다시 시도한다
	}
	if storedHash == newHash {
		return false
	}
	if daysToEvent <= 1 && age >= 20*time.Hour {
		return true // 발표 직전 마지막 확인
	}
	return age >= earningsPreviewFreshFor
}

// RunBatchNow generates previews for one market's next seven days.
func (s *EarningsPreviewService) RunBatchNow(ctx context.Context, market string) (map[string]any, string, error) {
	market = normalizePreviewMarket(market)
	now := s.now()
	from, to := earningsPreviewWindow(now)
	targets, err := s.repo.Targets(ctx, market, from, to)
	if err != nil {
		return nil, "", err
	}
	if market == "domestic" && s.calendar != nil {
		targets = s.withDomesticEstimates(ctx, targets, from, to)
	}
	targets = RankEarningsPreviewTargets(targets, earningsPreviewMaxPerMarket)

	var client *llm.Client
	if s.llm != nil {
		client = s.llm.For(market)
	}
	llmOn := client != nil && client.Enabled()
	interval := time.Duration(0)
	if llmOn {
		interval = client.Limits().MinInterval
	}

	stats := map[string]any{"market": market, "window": from + "~" + to, "targets": len(targets)}
	var nLLM, nRule, nSkip, nFail int
	var tokens int64
	lastCall := time.Time{}
	for _, t := range targets {
		if ctx.Err() != nil {
			break
		}
		facts, err := s.repo.Facts(ctx, t, now.In(kstLoc()))
		if err != nil {
			log.Printf("earnings preview %s facts: %v", t.Symbol, err)
			nFail++
			continue
		}
		factsJSON, _ := json.Marshal(facts)
		sum := sha256.Sum256(factsJSON)
		hash := hex.EncodeToString(sum[:])

		storedHash, storedSource, at, exists, err := s.repo.Existing(ctx, t.Symbol, t.EventDate)
		if err != nil {
			nFail++
			continue
		}
		if !PreviewNeedsRegen(exists, storedHash, hash, storedSource, now.Sub(at), facts.DaysToEvent) {
			nSkip++
			continue
		}

		out := RuleEarningsPreview(facts)
		source, modelName := "rule", ""
		var used int64
		if llmOn {
			if wait := interval - time.Since(lastCall); !lastCall.IsZero() && wait > 0 {
				if !s.sleep(ctx, wait) {
					break
				}
			}
			est := client.EstimateEarningsPreviewTokens(string(factsJSON))
			if err := s.budget.CheckTokens(ctx, apibudget.ProviderLLMFeatEarnings, est); err != nil {
				log.Printf("earnings preview: feature budget — %v (나머지는 규칙 문장)", err)
				llmOn = false
			} else {
				lastCall = time.Now()
				res, err := client.GenerateEarningsPreview(ctx, string(factsJSON))
				switch {
				case err != nil:
					log.Printf("earnings preview %s llm: %v", t.Symbol, err)
					if llm.IsBudgetExhausted(err) {
						llmOn = false
					}
				case ValidEarningsPreview(res):
					s.budget.Record(ctx, apibudget.ProviderLLMFeatEarnings)
					s.budget.RecordTokens(ctx, apibudget.ProviderLLMFeatEarnings, res.TotalTokens)
					out, source, modelName, used = *res, "llm", res.Model, res.TotalTokens
				default:
					s.budget.Record(ctx, apibudget.ProviderLLMFeatEarnings)
					s.budget.RecordTokens(ctx, apibudget.ProviderLLMFeatEarnings, res.TotalTokens)
					log.Printf("earnings preview %s: 형식이 맞지 않아 규칙 문장으로", t.Symbol)
				}
			}
		}
		body, _ := json.Marshal(map[string]any{"headline": out.Headline, "points": out.Points, "risk": out.Risk, "dateStatus": t.DateStatus})
		if err := s.repo.Upsert(ctx, repository.EarningsPreviewUpsert{
			Symbol: t.Symbol, EventDate: t.EventDate, Market: market, FiscalLabel: t.FiscalLabel(),
			Body: body, Facts: factsJSON, Source: source, Model: modelName, InputHash: hash, Tokens: used,
		}); err != nil {
			nFail++
			continue
		}
		tokens += used
		if source == "llm" {
			nLLM++
		} else {
			nRule++
		}
	}
	digestNote := ""
	if s.digest != nil && ctx.Err() == nil {
		var used int64
		digestNote, used = s.digest.RunNow(ctx, market)
		tokens += used
		stats["digest"] = digestNote
	}
	stats["llm"], stats["rule"], stats["skipped"], stats["failed"], stats["tokens"] = nLLM, nRule, nSkip, nFail, tokens
	msg := fmt.Sprintf("%s %d종목 — AI %d · 규칙 %d · 그대로 %d · 실패 %d (토큰 %d)", market, len(targets), nLLM, nRule, nSkip, nFail, tokens)
	if digestNote != "" {
		msg += " · " + digestNote
	}
	return stats, msg, nil
}

// withDomesticEstimates adds 국내 추정 일정 not already stored as events.
func (s *EarningsPreviewService) withDomesticEstimates(ctx context.Context, targets []repository.EarningsPreviewTarget, from, to string) []repository.EarningsPreviewTarget {
	loc := kstLoc()
	f, err1 := time.ParseInLocation("2006-01-02", from, loc)
	t, err2 := time.ParseInLocation("2006-01-02", to, loc)
	if err1 != nil || err2 != nil {
		return targets
	}
	est, err := s.calendar.EstimatedDomestic(ctx, f, t)
	if err != nil || len(est) == 0 {
		return targets
	}
	have := map[string]bool{}
	for _, x := range targets {
		have[x.Symbol] = true
	}
	var add []repository.EarningsPreviewTarget
	var syms []string
	for _, e := range est {
		if have[e.Symbol] || e.Date < from || e.Date > to {
			continue
		}
		have[e.Symbol] = true
		add = append(add, repository.EarningsPreviewTarget{
			Symbol: e.Symbol, Name: e.Name, Market: "domestic", EventDate: e.Date, DateStatus: "estimated",
			FiscalYear: e.FiscalYear, FiscalPeriod: e.FiscalPeriod, Currency: e.Currency,
		})
		syms = append(syms, e.Symbol)
	}
	meta, err := s.repo.TargetMeta(ctx, syms)
	if err == nil {
		for i := range add {
			if m, ok := meta[add[i].Symbol]; ok {
				if add[i].Name == "" {
					add[i].Name = m.Name
				}
				add[i].Held, add[i].Watched, add[i].MarketCap = m.Held, m.Watched, m.MarketCap
			}
		}
	}
	return append(targets, add...)
}

// ValidEarningsPreview rejects replies the page cannot show.
func ValidEarningsPreview(o *llm.EarningsPreviewLLMOutput) bool {
	if o == nil || strings.TrimSpace(o.Headline) == "" || len(o.Points) < 2 {
		return false
	}
	if len([]rune(o.Headline)) > 80 {
		return false
	}
	for _, p := range o.Points {
		if len([]rune(p)) > 140 {
			return false
		}
	}
	return true
}

// RuleEarningsPreview is the no-LLM text — facts in sentence frames.
func RuleEarningsPreview(f *repository.EarningsPreviewFacts) llm.EarningsPreviewLLMOutput {
	when := fmt.Sprintf("D-%d", f.DaysToEvent)
	if f.DaysToEvent <= 0 {
		when = "오늘"
	}
	if f.DateStatus == "estimated" {
		when += "(추정)"
	}
	out := llm.EarningsPreviewLLMOutput{Headline: fmt.Sprintf("%s %s 실적 발표 %s", f.Name, f.Fiscal, when)}
	if n := len(f.Surprises); n > 0 {
		beats := 0
		for _, s := range f.Surprises {
			if s.Pct > 0 {
				beats++
			}
		}
		out.Points = append(out.Points, fmt.Sprintf("지난 %d번 중 %d번 예상을 넘었습니다 — 이번에도 이어지는지", n, beats))
	}
	if r := f.EPSRevision; r != nil {
		dir := "올랐습니다 — 높아진 눈높이를 넘는지"
		if r.Pct < 0 {
			dir = "내렸습니다 — 낮아진 눈높이에서 반등 여지가 있는지"
		}
		out.Points = append(out.Points, fmt.Sprintf("EPS 추정치가 한 달 새 %+.1f%% %s", r.Pct, dir))
	}
	if f.Score != nil {
		line := fmt.Sprintf("AI 점수 %d점", *f.Score)
		if f.ScoreDelta != nil && *f.ScoreDelta != 0 {
			line += fmt.Sprintf("(전일 %+d)", *f.ScoreDelta)
		}
		out.Points = append(out.Points, line+" — 발표가 점수의 어느 축을 바꿀지")
	}
	if len(out.Points) < 3 && len(f.Headlines) > 0 {
		out.Points = append(out.Points, "최근 기사: "+f.Headlines[0])
	}
	if len(out.Points) > 3 {
		out.Points = out.Points[:3]
	}
	switch {
	case f.Return20d != nil && *f.Return20d >= 10:
		out.Risk = fmt.Sprintf("최근 20거래일 %+.1f%% — 기대가 먼저 반영됐을 수 있습니다.", *f.Return20d)
	case f.Return20d != nil && *f.Return20d <= -10:
		out.Risk = fmt.Sprintf("최근 20거래일 %+.1f%% — 눈높이가 낮아진 상태에서의 발표입니다.", *f.Return20d)
	case f.Pos52w != nil && *f.Pos52w >= 90:
		out.Risk = "52주 고점 근처에서 맞는 발표입니다."
	default:
		out.Risk = "발표 직후 변동성이 커질 수 있습니다."
	}
	return out
}

// Run schedules the batch through the queue, so manual and scheduled runs share one executor and one ledger.
// 07:10 KST 두 시장 · 18:50 국내(18:30 수집 뒤) · 21:30 해외(장 전 발표 뒤) — 뒤의 둘은 관전 포인트를
// 거의 건너뛰고(같은 사실표·3일 유지) 맨 위 AI 요약만 새로 쓴다(2026-10-09 요청 1).
func (s *EarningsPreviewService) Run(ctx context.Context) {
	if s.runs == nil {
		return
	}
	gate := NewSlotGate()
	for {
		slot, ids := NextEarningsPreviewSlot(s.now())
		if !WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		for _, id := range ids {
			if busy, err := s.runs.HasPending(ctx, string(id)); err == nil && !busy {
				if _, err := s.runs.EnqueueAs(ctx, string(id), repository.TriggerSchedule, nil); err != nil {
					log.Printf("earnings preview: enqueue %s: %v", id, err)
				}
			}
		}
	}
}

// earningsPreviewSlots — KST 시:분과 그 슬롯에 넣는 잡.
var earningsPreviewSlots = []struct {
	hour, minute int
	ids          []model.BatchJobID
}{
	{earningsPreviewSlotHour, earningsPreviewSlotMinute, []model.BatchJobID{model.BatchEarningsPreviewDomestic, model.BatchEarningsPreviewOverseas}},
	{18, 50, []model.BatchJobID{model.BatchEarningsPreviewDomestic}},
	{21, 30, []model.BatchJobID{model.BatchEarningsPreviewOverseas}},
}

// NextEarningsPreviewSlot is the next slot strictly after now, with the jobs it enqueues.
func NextEarningsPreviewSlot(now time.Time) (time.Time, []model.BatchJobID) {
	k := now.In(kstLoc())
	var best time.Time
	var ids []model.BatchJobID
	for _, sl := range earningsPreviewSlots {
		slot := time.Date(k.Year(), k.Month(), k.Day(), sl.hour, sl.minute, 0, 0, kstLoc())
		if !slot.After(k) {
			slot = slot.AddDate(0, 0, 1)
		}
		if best.IsZero() || slot.Before(best) {
			best, ids = slot, sl.ids
		}
	}
	return best, ids
}
