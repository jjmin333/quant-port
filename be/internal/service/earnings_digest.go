package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
실적 화면 맨 위 「AI 실적 요약」 (2026-10-09 사용자 요청 1, docs/mockups/requests-1009-v1.html ①).

실적 관전 포인트 배치(`EarningsPreviewService.RunBatchNow`) 끝에서 시장마다 한 번 만든다 — 같은 LLM 풀,
같은 기능 버킷(`llm_feat_earnings`), 같은 슬롯 장부. 슬롯: 07:10(둘 다) · 18:50(국내, 18:30 수집 뒤) ·
21:30(해외, 장 전 발표 뒤 · Gemini 밤 503 창 전). 조회는 저장분만 읽는다(llm_spend_rule).

  - 사실표는 규칙이 만든다: 최근 7일 발표(서프라이즈 · 시가총액 상위 6) · 통계 · 다가오는 대형 발표 5.
  - 사실표(오늘 날짜 제외)의 해시가 같고 이미 AI 글이면 부르지 않는다 — 시즌 밖엔 하루 0~1콜.
  - 모델 글에 사실표에 없는 숫자(소수 또는 32 이상)가 있으면 버리고 규칙 문장을 쓴다.
*/

const (
	earningsDigestLookbackDays = 6 // 오늘 포함 7일
	earningsDigestRecentMax    = 6
	earningsDigestUpcomingMax  = 5
)

type EarningsDigestService struct {
	cal    *EarningsCalendarService
	repo   *repository.EarningsDigestRepo
	llm    *llm.Pool
	budget *apibudget.Manager
	now    func() time.Time
}

func NewEarningsDigestService(cal *EarningsCalendarService, repo *repository.EarningsDigestRepo, pool *llm.Pool, budget *apibudget.Manager) *EarningsDigestService {
	return &EarningsDigestService{cal: cal, repo: repo, llm: pool, budget: budget, now: time.Now}
}

// EarningsDigestEvent is one compact line of the facts table.
type EarningsDigestEvent struct {
	Name   string   `json:"name"`
	Date   string   `json:"date"`
	Status string   `json:"status,omitempty"` // upcoming: scheduled | estimated
	Metric string   `json:"metric,omitempty"` // 영업이익 | EPS
	Est    string   `json:"est,omitempty"`
	Act    string   `json:"act,omitempty"`
	Pct    *float64 `json:"pct,omitempty"`
	Label  string   `json:"label,omitempty"` // 흑자전환 등(퍼센트 없음)
}

// EarningsDigestFacts is what the model reads (and what the 「근거 숫자 보기」 shows).
type EarningsDigestFacts struct {
	Market   string                `json:"market"`
	Today    string                `json:"today"`
	Released int                   `json:"released"`
	Beats    int                   `json:"beats"`
	Misses   int                   `json:"misses"`
	AvgPct   *float64              `json:"avgPct,omitempty"`
	Recent   []EarningsDigestEvent `json:"recent"`
	Upcoming []EarningsDigestEvent `json:"upcoming"`
}

// Get returns the stored digest (nil before the first run).
func (s *EarningsDigestService) Get(ctx context.Context, market string) (*repository.EarningsDigestRow, error) {
	return s.repo.Get(ctx, normalizePreviewMarket(market))
}

// Facts builds the rule-made table for one market from the calendar service.
func (s *EarningsDigestService) Facts(ctx context.Context, market string) (*EarningsDigestFacts, error) {
	now := s.now().In(kstLoc())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, kstLoc())
	from := today.AddDate(0, 0, -earningsDigestLookbackDays)
	rows, _, err := s.cal.Calendar(ctx, from, today, market, nil, 0)
	if err != nil {
		return nil, err
	}
	stats, err := s.cal.Stats(ctx, from, today, market)
	if err != nil {
		return nil, err
	}
	f := &EarningsDigestFacts{
		Market: market, Today: today.Format("2006-01-02"),
		Released: stats.Confirmed, Beats: stats.Beats, Misses: stats.Misses,
	}
	if stats.AvgPct != nil {
		f.AvgPct = round1Ptr(*stats.AvgPct)
	}
	f.Recent = digestRecent(rows, market == string(model.AssetUS))
	for _, ev := range s.cal.upcomingBig(ctx, today.AddDate(0, 0, 1), market) {
		if len(f.Upcoming) == earningsDigestUpcomingMax {
			break
		}
		f.Upcoming = append(f.Upcoming, EarningsDigestEvent{Name: ev.Name, Date: ev.Date, Status: ev.DateStatus})
	}
	if f.Upcoming == nil {
		f.Upcoming = []EarningsDigestEvent{}
	}
	return f, nil
}

// digestRecent keeps announced rows with a surprise, biggest companies first.
func digestRecent(rows []model.EarningsEvent, usd bool) []EarningsDigestEvent {
	picked := make([]model.EarningsEvent, 0, len(rows))
	for _, ev := range rows {
		if ev.DateStatus == model.EarningsDateConfirmed && ev.Surprise != nil {
			picked = append(picked, ev)
		}
	}
	capOf := func(ev model.EarningsEvent) float64 {
		if ev.MarketCap == nil {
			return 0
		}
		return *ev.MarketCap
	}
	sort.SliceStable(picked, func(i, j int) bool { return capOf(picked[i]) > capOf(picked[j]) })
	out := []EarningsDigestEvent{}
	for _, ev := range picked {
		if len(out) == earningsDigestRecentMax {
			break
		}
		sp := ev.Surprise
		fmtv := formatKRWBrief
		metric := "영업이익"
		if usd || sp.Metric == "eps" {
			fmtv, metric = func(v float64) string { return "$" + formatUSDBrief(v) }, "EPS"
		}
		line := EarningsDigestEvent{Name: ev.Name, Date: ev.Date, Metric: metric,
			Est: fmtv(sp.Estimate), Act: fmtv(sp.Actual), Label: sp.Label}
		if sp.Pct != nil && math.Abs(*sp.Pct) < briefingSurpriseBugPct*5 {
			line.Pct = round1Ptr(*sp.Pct)
		}
		out = append(out, line)
	}
	return out
}

// RuleEarningsDigest is the no-LLM text.
func RuleEarningsDigest(f *EarningsDigestFacts) llm.EarningsDigestLLMOutput {
	out := llm.EarningsDigestLLMOutput{}
	if len(f.Recent) == 0 {
		out.Headline = "최근 7일 실적 발표가 없습니다"
		out.Points = []string{"서프라이즈를 계산할 발표가 아직 없습니다."}
	} else {
		top := f.Recent[0]
		out.Headline = fmt.Sprintf("%s %s %s(예상 %s)%s", top.Name, top.Metric, top.Act, top.Est, pctPhrase(top))
		if f.Beats+f.Misses > 0 {
			out.Points = append(out.Points, fmt.Sprintf("최근 7일 발표 %d곳 중 %d곳 예상 상회, %d곳 하회", f.Beats+f.Misses, f.Beats, f.Misses))
		}
		for _, e := range f.Recent[1:] {
			if len(out.Points) == 3 {
				break
			}
			out.Points = append(out.Points, fmt.Sprintf("%s %s %s(예상 %s)%s", e.Name, e.Metric, e.Act, e.Est, pctPhrase(e)))
		}
	}
	if len(f.Upcoming) > 0 {
		names := make([]string, 0, 3)
		for _, u := range f.Upcoming[:min(3, len(f.Upcoming))] {
			tag := ""
			if u.Status == model.EarningsDateEstimated {
				tag = " 추정"
			}
			names = append(names, fmt.Sprintf("%s(%s%s)", u.Name, shortDate(u.Date), tag))
		}
		out.Watch = "다가오는 대형 발표: " + strings.Join(names, " · ")
	}
	return out
}

func pctPhrase(e EarningsDigestEvent) string {
	switch {
	case e.Label != "":
		return " — " + e.Label
	case e.Pct == nil:
		return ""
	case *e.Pct >= 0:
		return fmt.Sprintf(" — 예상 %+.1f%% 상회", *e.Pct)
	default:
		return fmt.Sprintf(" — 예상 %.1f%% 하회", *e.Pct)
	}
}

func shortDate(d string) string {
	if t, err := time.Parse("2006-01-02", d); err == nil {
		return fmt.Sprintf("%d/%d", int(t.Month()), t.Day())
	}
	return d
}

var digestNumberRE = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)

/*
DigestNumbersGrounded — 모델 글의 숫자가 사실표에 있는가. 소수이거나 32 이상인 숫자만 본다
(「3곳 중 2곳」·「10월 14일」 같은 작은 정수는 문장 구조라 통과). 계산해서 만든 비율(「2.3배」)은 걸린다 —
요약이 사실표보다 많이 아는 척하면 안 된다.
*/
func DigestNumbersGrounded(text string, factsJSON []byte) bool {
	facts := strings.ReplaceAll(string(factsJSON), ",", "")
	for _, m := range digestNumberRE.FindAllString(text, -1) {
		n := strings.ReplaceAll(m, ",", "")
		if !strings.Contains(n, ".") {
			if v, err := strconv.Atoi(n); err == nil && v < 32 {
				continue
			}
		}
		if !strings.Contains(facts, n) {
			return false
		}
	}
	return true
}

func validEarningsDigest(o *llm.EarningsDigestLLMOutput, factsJSON []byte) bool {
	if o == nil || o.Headline == "" || len(o.Points) == 0 || len([]rune(o.Headline)) > 90 {
		return false
	}
	all := o.Headline + " " + o.Watch
	for _, p := range o.Points {
		if len([]rune(p)) > 140 {
			return false
		}
		all += " " + p
	}
	return DigestNumbersGrounded(all, factsJSON)
}

// RunNow builds and stores one market's digest. Returns a one-line note for the batch ledger.
func (s *EarningsDigestService) RunNow(ctx context.Context, market string) (string, int64) {
	market = normalizePreviewMarket(market)
	facts, err := s.Facts(ctx, market)
	if err != nil {
		log.Printf("earnings digest %s facts: %v", market, err)
		return "AI 요약 실패(사실표)", 0
	}
	factsJSON, _ := json.Marshal(facts)
	hashFacts := *facts
	hashFacts.Today = ""
	hb, _ := json.Marshal(hashFacts)
	sum := sha256.Sum256(hb)
	hash := hex.EncodeToString(sum[:])

	if stored, err := s.repo.Get(ctx, market); err == nil && stored != nil && stored.InputHash == hash && stored.Source == "llm" {
		return "AI 요약 그대로(사실 같음)", 0
	}

	out := RuleEarningsDigest(facts)
	source, modelName := "rule", ""
	var used int64
	var client *llm.Client
	if s.llm != nil {
		client = s.llm.For(market)
	}
	// 관전 포인트 배치 바로 뒤에 돈다 — 제공자 최소 간격을 지키지 않으면 이 콜이 거절된다.
	if client != nil && client.Enabled() && sleepCtx(ctx, client.Limits().MinInterval) {
		est := client.EstimateEarningsDigestTokens(string(factsJSON))
		if err := s.budget.CheckTokens(ctx, apibudget.ProviderLLMFeatEarnings, est); err != nil {
			log.Printf("earnings digest %s: feature budget — %v (규칙 문장)", market, err)
		} else if res, err := client.GenerateEarningsDigest(ctx, string(factsJSON)); err != nil {
			log.Printf("earnings digest %s llm: %v", market, err)
		} else {
			s.budget.Record(ctx, apibudget.ProviderLLMFeatEarnings)
			s.budget.RecordTokens(ctx, apibudget.ProviderLLMFeatEarnings, res.TotalTokens)
			used = res.TotalTokens
			if validEarningsDigest(res, factsJSON) {
				out, source, modelName = *res, "llm", res.Model
			} else {
				log.Printf("earnings digest %s: 형식·숫자 검증 실패 — 규칙 문장", market)
			}
		}
	}
	body, _ := json.Marshal(map[string]any{"headline": out.Headline, "points": out.Points, "watch": out.Watch})
	if err := s.repo.Upsert(ctx, repository.EarningsDigestRow{
		Market: market, Body: body, Facts: factsJSON, Source: source, Model: modelName, InputHash: hash, Tokens: used,
	}); err != nil {
		log.Printf("earnings digest %s save: %v", market, err)
		return "AI 요약 저장 실패", used
	}
	if source == "llm" {
		return "AI 요약 갱신", used
	}
	return "AI 요약 규칙 문장", used
}
