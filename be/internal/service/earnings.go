package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
)

type EarningsConfig struct {
	FinnhubAPIKey string
}

func (c EarningsConfig) Enabled() bool {
	return c.FinnhubAPIKey != ""
}

type EarningsClient struct {
	cfg        EarningsConfig
	httpClient *http.Client
	budget     *apibudget.Manager
	// provider — 어느 한도 버킷으로 셀지. 기본은 라이브 폴러와 같은
	// ProviderFinnhub이고, 배치 워커는 UseBatchBudget()으로 전용 버킷으로 옮긴다.
	provider apibudget.Provider
	// baseURL — 테스트에서 스텁 서버를 물리기 위한 이음매. 비어 있으면
	// finnhubBaseURL. 지금은 FetchSurpriseHistory만 쓴다. 캘린더 경로는 운영에서
	// 돌고 있는 코드라 이유 없이 건드리지 않았다 — 옮길 때는 함께 옮긴다.
	baseURL string
}

// finnhubBaseURL is the live host. 테스트만 이 값을 바꾼다.
const finnhubBaseURL = "https://finnhub.io/api/v1"

// redactFinnhubToken strips the API key out of an error before it is logged.
//
// **이게 없으면 API 키가 로그 파일에 평문으로 남는다.** Finnhub은 토큰을
// 쿼리스트링으로 받는데, transport 오류에서 Go가 돌려주는 `*url.Error`는
// **요청 URL을 통째로** 메시지에 담는다. 실측(2026-08-21 백필): 타임아웃 한 건이
//
//	Get "https://finnhub.io/api/v1/stock/earnings?symbol=VLY&token=<진짜 키>":
//	context deadline exceeded
//
// 를 그대로 로그에 찍었다. 로그 파일은 커밋되기도 하고 공유되기도 한다.
//
// 토큰을 헤더로 옮기는 편이 더 낫지만, 그건 운영 중인 캘린더 배치의 인증 방식을
// 바꾸는 일이다. 여기서는 **동작을 바꾸지 않고** 새는 곳만 막는다.
func redactFinnhubToken(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	redacted := finnhubTokenPattern.ReplaceAllString(msg, "token=REDACTED")
	if redacted == msg {
		return err
	}
	return errors.New(redacted)
}

// token= 뒤의 값 — 따옴표·`&`·공백 앞까지.
var finnhubTokenPattern = regexp.MustCompile(`token=[^&"\s]*`)

func (e *EarningsClient) base() string {
	if e.baseURL != "" {
		return e.baseURL
	}
	return finnhubBaseURL
}

func NewEarningsClient(cfg EarningsConfig, budget *apibudget.Manager) *EarningsClient {
	return &EarningsClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		budget:     budget,
		provider:   apibudget.ProviderFinnhub,
	}
}

// UseBatchBudget switches this client to the batch-only quota bucket.
//
// ProviderFinnhub의 MinInterval 5분은 **라이브 ETF 시세 폴러** 간격이다. 하루
// 한 번 도는 실적 캘린더 배치가 그 버킷을 쓰면, 폴러가 방금 호출했다는 이유로
// 배치가 실패한다(첫 실행이 실제로 그렇게 죽었다). 같은 API 키·같은 실제
// 호출량이고, 나누는 것은 "누가 언제 부르는지"다 — ProviderFinnhubMigrate가
// 카탈로그 배치를 위해 같은 일을 한다.
func (e *EarningsClient) UseBatchBudget() *EarningsClient {
	if e != nil {
		e.provider = apibudget.ProviderFinnhubEarnings
	}
	return e
}

func (e *EarningsClient) UpcomingForSymbols(ctx context.Context, symbols []string) ([]model.BriefingEvent, error) {
	if !e.cfg.Enabled() || len(symbols) == 0 {
		return nil, nil
	}
	all, err := e.FetchCalendar(ctx)
	if err != nil {
		return nil, err
	}
	held := map[string]struct{}{}
	for _, s := range symbols {
		held[strings.ToUpper(strings.TrimSpace(s))] = struct{}{}
	}
	var out []model.BriefingEvent
	for _, ev := range all {
		if _, ok := held[strings.ToUpper(ev.Symbol)]; ok {
			out = append(out, ev)
		}
	}
	return out, nil
}

// FinnhubEarningsRow is one /calendar/earnings entry, fully parsed.
//
// **예전 파서는 date·symbol·quarter만 읽고 나머지를 버렸다.** 그 결과 앱 화면에
// "무료 데이터 소스에서는 컨센서스·서프라이즈를 제공하지 않는다"는 문장이 올라가
// 있었는데, 사실은 응답에 처음부터 실려 오고 있었다. 실측(2026-07-20~08-01,
// 1,446행)에서 epsActual이 1,135행(78%)에 있었고 hour(bmo/amc/dmh)도 채워져 있다.
type FinnhubEarningsRow struct {
	Symbol string
	Date   string // "2026-08-03" — 미국 세션 달력일. KST로 환산하지 않는다.
	// "bmo"(장전) | "amc"(장후) | "dmh"(장중) | "" (미공개)
	Hour    string
	Quarter int
	Year    int

	EPSEstimate     *float64
	EPSActual       *float64
	RevenueEstimate *float64
	RevenueActual   *float64
}

// FetchCalendar keeps the 2주 창 + BriefingEvent 형태를 그대로 유지한다.
// 호출자(MarketDataStore.refreshEarnings)는 종목별 "다가오는
// 일정" 문구만 쓰므로, 그들을 위해 응답을 넓힐 이유가 없다.
//
// ⚠️ 이 2주 창은 실적 시즌에 1,500행 상한에 닿을 수 있다(실측 13일 = 1,446행).
// 그때는 창의 앞부분이 빠진다 — 이 경로는 "내 종목이 곧 발표하나"를 거르는
// 데만 쓰이므로 일부 누락이 기능을 깨지는 않지만, 정확한 일정이 필요한 화면은
// `earnings_events` 표를 읽어야 한다(배치가 조각내어 온전히 채운다).
func (e *EarningsClient) FetchCalendar(ctx context.Context) ([]model.BriefingEvent, error) {
	if !e.cfg.Enabled() {
		return nil, nil
	}
	// KST "today", not the container's own zone (UTC in Docker) — consistent
	// with every other "오늘" calendar boundary in this codebase.
	nowKST := time.Now().In(model.KST)
	rows, err := e.FetchCalendarRange(ctx, nowKST, nowKST.Add(14*24*time.Hour))
	if err != nil {
		return nil, err
	}
	out := make([]model.BriefingEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.BriefingEvent{
			Date:   formatBriefDate(row.Date),
			Symbol: row.Symbol,
			Event:  row.Label(),
		})
	}
	return out, nil
}

// Label renders the Korean event name shown in 「다가오는 일정」 문구.
func (r FinnhubEarningsRow) Label() string {
	if r.Quarter > 0 {
		return fmt.Sprintf("Q%d 실적발표", r.Quarter)
	}
	return "실적발표"
}

// finnhubCalendarRowCap is the hard row ceiling on /calendar/earnings.
//
// **응답이 잘려도 200으로 온다.** 그리고 잘릴 때 남는 것은 구간의 **뒤쪽**이다 —
// 실측으로 확인했다:
//
//	2026-07-03 ~ 2026-10-31 → 1500행, 실제 범위 10-07 ~ 10-30
//	2026-07-03 ~ 2026-08-31 → 1500행, 실제 범위 08-10 ~ 08-31
//	2026-07-03 ~ 2026-09-30 → 1500행, 실제 범위 08-12 ~ 09-30
//
// 즉 넓게 한 번 부르면 앞쪽 몇 달이 조용히 사라진다. 처음 구현이 정확히 그렇게
// 4개월 창에서 10월 3주치만 담았다. 그래서 구간을 쪼개 부른다.
const finnhubCalendarRowCap = 1500

// finnhubCalendarChunkDays keeps each request well under the row cap.
//
// 실적 시즌 밀도가 기준이다 — 2026-07-20~08-01(13일)이 1,446행이었으니 하루
// 100행 남짓이다. 7일이면 700행 안팎으로 상한의 절반이고, 비수기에는 200행이다.
const finnhubCalendarChunkDays = 7

// FinnhubCalendarChunkCount is how many HTTP calls FetchCalendarChunked makes.
//
// 구간은 양끝 포함. −45/+75 = 121일 → 18조각. −49/+77 = 127일 → 19조각(하루가
// 7로 나누어떨어지지 않아 1일짜리 조각이 하나 더 생긴다). 같은 18조각을 주 단위에
// 맞추려면 −49/+76 (126일)이다.
func FinnhubCalendarChunkCount(from, to time.Time) int {
	a := calendarDate(from)
	b := calendarDate(to)
	if b.Before(a) {
		return 0
	}
	days := int(b.Sub(a)/(24*time.Hour)) + 1
	return (days + finnhubCalendarChunkDays - 1) / finnhubCalendarChunkDays
}

func calendarDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// FetchCalendarChunked walks a wide window in small requests and merges them.
//
// 쪼개는 것은 취향이 아니라 필요다(finnhubCalendarRowCap 참고). 호출 간 간격은
// 이 클라이언트의 한도 버킷에서 읽어 **스스로 기다린다** — 기다리면 풀리는
// 제약을 실패로 다루면 배치가 첫 조각만 넣고 끝난다.
func (e *EarningsClient) FetchCalendarChunked(ctx context.Context, from, to time.Time) ([]FinnhubEarningsRow, error) {
	if !e.cfg.Enabled() {
		return nil, nil
	}
	pace := time.Duration(0)
	if e.budget != nil {
		pace = e.budget.MinInterval(e.provider)
	}

	// 조각 경계·소스 정정으로 같은 발표가 두 번 오고, Finnhub은 같은 날·같은
	// 분기에 회계연도만 다른 행도 둘 다 준다(TGT 2026 2Q + 2027 2Q). unique
	// key에 연도가 들어 있어 그대로 넣으면 캘린더에 두 줄이 된다. 한 번에 모은 뒤
	// mergeFinnhubCalendar가 (symbol, date, quarter)로 접는다.
	var out []FinnhubEarningsRow
	var firstErr error

	first := true
	for start := from; !start.After(to); start = start.AddDate(0, 0, finnhubCalendarChunkDays) {
		end := start.AddDate(0, 0, finnhubCalendarChunkDays-1)
		if end.After(to) {
			end = to
		}
		if !first && pace > 0 {
			select {
			case <-ctx.Done():
				return mergeFinnhubCalendar(out), ctx.Err()
			case <-time.After(pace):
			}
		}
		first = false

		rows, err := e.FetchCalendarRange(ctx, start, end)
		if err != nil {
			// 한 조각이 실패해도 나머지를 받는다. 전체를 되돌리면 429 한 번에
			// 그날 일정이 통째로 안 들어온다.
			log.Printf("finnhub earnings chunk %s~%s: %v",
				start.Format("2006-01-02"), end.Format("2006-01-02"), err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(rows) >= finnhubCalendarRowCap {
			// 조각 하나가 상한에 닿았다는 것은 그 조각의 앞부분이 잘렸다는 뜻이다.
			// 조용히 넘기면 데이터가 빠진 것을 아무도 모르므로 로그로 남긴다.
			log.Printf("finnhub earnings chunk %s~%s hit the %d-row cap — 앞부분이 잘렸을 수 있음 (chunk 기간을 줄여야 함)",
				start.Format("2006-01-02"), end.Format("2006-01-02"), finnhubCalendarRowCap)
		}
		out = append(out, rows...)
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return mergeFinnhubCalendar(out), nil
}

// FetchCalendarRange returns US earnings entries between two dates in **one**
// request.
//
// ⚠️ 넓은 구간에 쓰지 말 것 — 1,500행 상한에 걸려 앞쪽이 조용히 사라진다
// (finnhubCalendarRowCap). 여러 주를 받아야 하면 FetchCalendarChunked를 쓴다.
func (e *EarningsClient) FetchCalendarRange(ctx context.Context, from, to time.Time) ([]FinnhubEarningsRow, error) {
	if !e.cfg.Enabled() {
		return nil, nil
	}
	if e.budget != nil {
		if err := e.budget.Check(ctx, e.provider); err != nil {
			return nil, err
		}
	}
	u := fmt.Sprintf(
		"https://finnhub.io/api/v1/calendar/earnings?from=%s&to=%s&token=%s",
		from.Format("2006-01-02"), to.Format("2006-01-02"),
		url.QueryEscape(e.cfg.FinnhubAPIKey),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := e.httpClient.Do(req)
	if err != nil {
		// URL에 토큰이 들어 있어 그대로 올리면 로그에 키가 남는다.
		return nil, redactFinnhubToken(err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if e.budget != nil {
			e.budget.BlockFor(ctx, e.provider, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("finnhub earnings: status 429")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub earnings: status %d", res.StatusCode)
	}

	var payload struct {
		EarningsCalendar []struct {
			Date            string   `json:"date"`
			Symbol          string   `json:"symbol"`
			Hour            string   `json:"hour"`
			Quarter         float64  `json:"quarter"`
			Year            float64  `json:"year"`
			EPSEstimate     *float64 `json:"epsEstimate"`
			EPSActual       *float64 `json:"epsActual"`
			RevenueEstimate *float64 `json:"revenueEstimate"`
			RevenueActual   *float64 `json:"revenueActual"`
		} `json:"earningsCalendar"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if e.budget != nil {
		e.budget.Record(ctx, e.provider)
	}

	out := make([]FinnhubEarningsRow, 0, len(payload.EarningsCalendar))
	for _, row := range payload.EarningsCalendar {
		sym := strings.ToUpper(strings.TrimSpace(row.Symbol))
		if sym == "" {
			continue
		}
		out = append(out, FinnhubEarningsRow{
			Symbol:          sym,
			Date:            strings.TrimSpace(row.Date),
			Hour:            normalizeEarningsHour(row.Hour),
			Quarter:         int(row.Quarter),
			Year:            int(row.Year),
			EPSEstimate:     row.EPSEstimate,
			EPSActual:       row.EPSActual,
			RevenueEstimate: row.RevenueEstimate,
			RevenueActual:   row.RevenueActual,
		})
	}
	return mergeFinnhubCalendar(out), nil
}

// mergeFinnhubCalendar keeps one row per (symbol, date, quarter).
//
// Finnhub은 Target처럼 회계연도 끝(1월)인 종목을 같은 발표에 대해 연도만 다르게
// 두 줄로 준다. 한쪽은 컨센서스가 있고 한쪽은 실적만 있다. 연도를 키에 넣으면
// 둘 다 살아 캘린더가 같은 종목을 두 번 그린다. 분기가 다른 같은 날(2Q와 FY)은
// 별개 발표이므로 키에서 연도만 빼고 분기는 남긴다.
func mergeFinnhubCalendar(rows []FinnhubEarningsRow) []FinnhubEarningsRow {
	if len(rows) < 2 {
		return rows
	}
	idx := make(map[string]int, len(rows))
	out := make([]FinnhubEarningsRow, 0, len(rows))
	for _, row := range rows {
		key := row.Symbol + "|" + row.Date + "|" + fmt.Sprintf("%d", row.Quarter)
		i, ok := idx[key]
		if !ok {
			idx[key] = len(out)
			out = append(out, row)
			continue
		}
		out[i] = mergeFinnhubEarnings(out[i], row)
	}
	return out
}

func mergeFinnhubEarnings(a, b FinnhubEarningsRow) FinnhubEarningsRow {
	winner, loser := a, b
	if preferFinnhubEarnings(b, a) {
		winner, loser = b, a
	}
	out := winner
	if out.Hour == "" {
		out.Hour = loser.Hour
	}
	out.EPSEstimate = firstFloatPtr(out.EPSEstimate, loser.EPSEstimate)
	out.EPSActual = firstFloatPtr(out.EPSActual, loser.EPSActual)
	out.RevenueEstimate = firstFloatPtr(out.RevenueEstimate, loser.RevenueEstimate)
	out.RevenueActual = firstFloatPtr(out.RevenueActual, loser.RevenueActual)
	return out
}

// preferFinnhubEarnings reports whether cand is the row to keep when two
// Finnhub lines describe the same print. Estimates first — the mis-tagged year
// is usually the actuals-only copy. Then actuals, then the higher year
// (Target FY ends in January, so 2027 2Q is the real label for an August print).
func preferFinnhubEarnings(cand, cur FinnhubEarningsRow) bool {
	if c, u := finnhubEstimateCount(cand), finnhubEstimateCount(cur); c != u {
		return c > u
	}
	if c, u := finnhubActualCount(cand), finnhubActualCount(cur); c != u {
		return c > u
	}
	return cand.Year > cur.Year
}

func finnhubEstimateCount(row FinnhubEarningsRow) int {
	n := 0
	if row.EPSEstimate != nil {
		n++
	}
	if row.RevenueEstimate != nil {
		n++
	}
	return n
}

func finnhubActualCount(row FinnhubEarningsRow) int {
	n := 0
	if row.EPSActual != nil {
		n++
	}
	if row.RevenueActual != nil {
		n++
	}
	return n
}

func firstFloatPtr(a, b *float64) *float64 {
	if a != nil {
		return a
	}
	return b
}

// normalizeEarningsHour keeps only the three values the app knows how to label.
// 실측 분포에서 288/1446이 빈 문자열이었고, 그건 "미공개"라는 뜻이므로 그대로
// 빈 값으로 둔다 — 모르는 값을 장전/장후 중 하나로 찍으면 거짓말이 된다.
func normalizeEarningsHour(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case model.EarningsSessionBeforeOpen:
		return model.EarningsSessionBeforeOpen
	case model.EarningsSessionAfterClose:
		return model.EarningsSessionAfterClose
	case model.EarningsSessionDuringHours:
		return model.EarningsSessionDuringHours
	default:
		return ""
	}
}

func formatBriefDate(raw string) string {
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return raw
	}
	return t.Format("1/2")
}
