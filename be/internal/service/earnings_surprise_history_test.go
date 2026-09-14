package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

// day()와 fptr()는 같은 패키지의 earnings_reaction_test.go / earnings_finnhub_test.go에 있다.

// `/stock/earnings`는 기간 순서로 오지 않는다 — 실측 TGT 응답이
// 2026-09-30, 2025-09-30, 2026-06-30, 2026-03-31 순이었다. 응답 순서를 믿으면
// 「최근 4분기」가 아니라 아무 분기나 집는다.
func TestSurpriseRows_sortsByPeriodEndDesc(t *testing.T) {
	got := surpriseRows("TGT", []FinnhubSurpriseRow{
		{Period: "2026-09-30", Quarter: 2, Year: 2027, Estimate: fptr(2.3095), Actual: fptr(2.46)},
		{Period: "2025-09-30", Quarter: 2, Year: 2026, Actual: fptr(2.46)},
		{Period: "2026-06-30", Quarter: 1, Year: 2027, Estimate: fptr(1.477), Actual: fptr(1.71)},
		{Period: "2026-03-31", Quarter: 4, Year: 2026, Estimate: fptr(2.177), Actual: fptr(2.44)},
	})

	want := []string{"2026-09-30", "2026-06-30", "2026-03-31", "2025-09-30"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].PeriodEnd.Format("2006-01-02") != w {
			t.Fatalf("row %d periodEnd = %s, want %s", i, got[i].PeriodEnd.Format("2006-01-02"), w)
		}
	}
}

// 분기 표기가 earnings_events와 어긋나면 같은 분기가 두 번 세어진다.
func TestSurpriseRows_normalizesQuarterToEventsVocabulary(t *testing.T) {
	got := surpriseRows("AAPL", []FinnhubSurpriseRow{
		{Period: "2026-06-30", Quarter: 3, Year: 2026, Estimate: fptr(1.9271), Actual: fptr(1.91)},
	})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].FiscalPeriod != "3Q" {
		t.Fatalf("fiscalPeriod = %q, want %q (earnings_events가 쓰는 표기)", got[0].FiscalPeriod, "3Q")
	}
	if got[0].Symbol != "AAPL" || got[0].Market != string(model.AssetUS) {
		t.Fatalf("symbol/market = %s/%s", got[0].Symbol, got[0].Market)
	}
}

func TestSurpriseRows_quarterZeroBecomesFY(t *testing.T) {
	got := surpriseRows("FOO", []FinnhubSurpriseRow{
		{Period: "2026-12-31", Quarter: 0, Year: 2026, Estimate: fptr(1), Actual: fptr(1.2)},
	})
	if len(got) != 1 || got[0].FiscalPeriod != "FY" {
		t.Fatalf("got %+v, want one FY row", got)
	}
}

// 날짜를 못 읽는 행은 버린다 — period가 비면 정렬 축이 없어서 zero time이 되고,
// 그러면 그 행이 목록 맨 뒤로 밀려 조용히 다른 분기를 밀어낸다.
func TestSurpriseRows_dropsUnparsableAndEmptyPeriod(t *testing.T) {
	got := surpriseRows("FOO", []FinnhubSurpriseRow{
		{Period: "not-a-date", Quarter: 1, Year: 2026, Estimate: fptr(1), Actual: fptr(1.2)},
		{Period: "2026-03-31", Quarter: 1, Year: 2026, Estimate: fptr(1), Actual: fptr(1.2)},
	})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (파싱 실패 행은 버린다)", len(got))
	}
}

// year가 0으로 오면 기간 종료일의 연도로 메운다.
func TestSurpriseRows_fallsBackToPeriodYear(t *testing.T) {
	got := surpriseRows("FOO", []FinnhubSurpriseRow{
		{Period: "2025-09-30", Quarter: 4, Year: 0, Estimate: fptr(1), Actual: fptr(1.2)},
	})
	if len(got) != 1 || got[0].FiscalYear != 2025 {
		t.Fatalf("fiscalYear = %v, want 2025", got)
	}
}

func historyRow(symbol string, year int, period, periodEnd string, est, act float64) model.EarningsSurpriseRow {
	return model.EarningsSurpriseRow{
		Symbol: symbol, Name: symbol, Market: string(model.AssetUS),
		FiscalYear: year, FiscalPeriod: period, PeriodEnd: day(periodEnd),
		EPSEst: fptr(est), EPSAct: fptr(act), Currency: "USD", Source: "finnhub",
	}
}

// 이 테스트가 이 기능의 핵심이다. `/stock/earnings`가 주는 4분기에는 캘린더가
// 이미 넣어 둔 최근 분기가 **포함**되어 있다(실측 AAPL FY2026 3Q). 중복 가드가
// 없으면 그 분기가 두 번 세어져 적중률이 왜곡된다.
func TestMergeSurpriseHistory_skipsQuarterAlreadyFromCalendar(t *testing.T) {
	calendarEv := model.EarningsEvent{
		Symbol: "AAPL", Market: string(model.AssetUS), Date: "2026-07-30",
		DateStatus: model.EarningsDateConfirmed,
		FiscalYear: 2026, FiscalPeriod: "3Q",
		EPS: model.EarningsMetric{Estimate: fptr(1.9271), Actual: fptr(1.91)},
	}
	calendarEv.Surprise = calendarEv.HeadlineSurprise()

	seen := map[string]struct{}{
		model.EarningsPeriodMergeKey("AAPL", 2026, "3Q"): {},
	}
	bySymbol := map[string][]model.EarningsEvent{"AAPL": {calendarEv}}

	mergeSurpriseHistory([]model.EarningsSurpriseRow{
		historyRow("AAPL", 2026, "3Q", "2026-06-30", 1.9271, 1.91), // 중복
		historyRow("AAPL", 2026, "2Q", "2026-03-31", 1.9884, 2.01),
		historyRow("AAPL", 2026, "1Q", "2025-12-31", 2.7257, 2.84),
		historyRow("AAPL", 2025, "4Q", "2025-09-30", 1.8075, 1.85),
	}, seen, bySymbol)

	got := bySymbol["AAPL"]
	if len(got) != 4 {
		t.Fatalf("분기 수 = %d, want 4 (캘린더 1 + 이력 3, 중복 3Q는 한 번)", len(got))
	}
	periods := map[string]int{}
	for _, ev := range got {
		periods[ev.FiscalPeriod]++
	}
	if periods["3Q"] != 1 {
		t.Fatalf("3Q가 %d번 세어졌다 — 중복 가드가 깨졌다", periods["3Q"])
	}
}

// 기준선이 유의성 하한(EPS 0.10달러) 아래인 행은 캘린더 행과 똑같이 뺀다.
func TestMergeSurpriseHistory_appliesMaterialityGate(t *testing.T) {
	seen := map[string]struct{}{}
	bySymbol := map[string][]model.EarningsEvent{}

	mergeSurpriseHistory([]model.EarningsSurpriseRow{
		historyRow("TINY", 2026, "2Q", "2026-03-31", 0.01, 0.04), // +300%, 무의미
		historyRow("BIG", 2026, "2Q", "2026-03-31", 2.00, 2.20),
	}, seen, bySymbol)

	if _, ok := bySymbol["TINY"]; ok {
		t.Fatal("컨센서스 0.01달러 행이 통과했다 — materialSurprise 관문이 빠졌다")
	}
	if len(bySymbol["BIG"]) != 1 {
		t.Fatalf("BIG 분기 수 = %d, want 1", len(bySymbol["BIG"]))
	}
}

// 퍼센트를 만들 수 없는 행(기준치 0 이하)은 적중률 표본이 될 수 없다.
func TestMergeSurpriseHistory_skipsRowsWithoutPct(t *testing.T) {
	seen := map[string]struct{}{}
	bySymbol := map[string][]model.EarningsEvent{}

	mergeSurpriseHistory([]model.EarningsSurpriseRow{
		historyRow("LOSS", 2026, "2Q", "2026-03-31", -1.0, 0.5), // 흑자전환 — Pct 없음
	}, seen, bySymbol)

	if len(bySymbol) != 0 {
		t.Fatalf("Pct 없는 행이 통과했다: %+v", bySymbol)
	}
}

// 같은 이력이 두 번 들어와도 한 번만 센다(재실행·중복 응답 방어).
func TestMergeSurpriseHistory_dedupsWithinHistoryItself(t *testing.T) {
	seen := map[string]struct{}{}
	bySymbol := map[string][]model.EarningsEvent{}

	row := historyRow("FOO", 2026, "2Q", "2026-03-31", 1.0, 1.1)
	mergeSurpriseHistory([]model.EarningsSurpriseRow{row, row}, seen, bySymbol)

	if len(bySymbol["FOO"]) != 1 {
		t.Fatalf("분기 수 = %d, want 1", len(bySymbol["FOO"]))
	}
}

// 두 출처의 날짜 축이 다르다(발표일 vs 기간종료일). 섞어 정렬해도 분기 순서가
// 뒤집히면 안 된다 — ComputeHitHistory가 앞에서 8개를 자르기 때문이다.
func TestSortByRecency_mixesAnnouncementAndPeriodEnd(t *testing.T) {
	calendarQ3 := model.EarningsEvent{
		Symbol: "AAPL", Date: "2026-07-30", FiscalYear: 2026, FiscalPeriod: "3Q",
	}
	historyQ2 := historyRow("AAPL", 2026, "2Q", "2026-03-31", 1.9884, 2.01).AsEvent()
	historyQ1 := historyRow("AAPL", 2026, "1Q", "2025-12-31", 2.7257, 2.84).AsEvent()

	events := []model.EarningsEvent{historyQ1, calendarQ3, historyQ2}
	sortByRecency(events)

	want := []string{"3Q", "2Q", "1Q"}
	for i, w := range want {
		if events[i].FiscalPeriod != w {
			t.Fatalf("위치 %d = %s, want %s (정렬: %+v)", i, events[i].FiscalPeriod, w,
				[]string{events[0].FiscalPeriod, events[1].FiscalPeriod, events[2].FiscalPeriod})
		}
	}
}

// 이력 행은 발표일을 모른다. 그 값이 캘린더로 새어 나가면 분기말에 가짜 발표가
// 뭉치므로, EarningsEvent.Date는 반드시 비어 있어야 한다.
func TestAsEvent_neverCarriesAnAnnouncementDate(t *testing.T) {
	ev := historyRow("AAPL", 2026, "2Q", "2026-03-31", 1.9884, 2.01).AsEvent()
	if ev.Date != "" {
		t.Fatalf("Date = %q — 이력 행에는 발표일이 없어야 한다", ev.Date)
	}
	if ev.PeriodEnd != "2026-03-31" {
		t.Fatalf("PeriodEnd = %q, want 2026-03-31", ev.PeriodEnd)
	}
	if ev.Surprise == nil || ev.Surprise.Pct == nil {
		t.Fatal("Surprise가 계산되지 않았다")
	}
	if ev.Surprise.Metric != model.EarningsMetricEPS {
		t.Fatalf("metric = %q, want eps (해외 헤드라인 지표)", ev.Surprise.Metric)
	}
}

func TestFiscalPeriodFromQuarter(t *testing.T) {
	cases := map[int]string{1: "1Q", 2: "2Q", 3: "3Q", 4: "4Q", 0: "FY", 5: "FY", -1: "FY"}
	for q, want := range cases {
		if got := model.FiscalPeriodFromQuarter(q); got != want {
			t.Fatalf("quarter %d = %q, want %q", q, got, want)
		}
	}
}

// --- HTTP 에러 분류 ---------------------------------------------------------
//
// 이 셋의 구분이 잡의 안전장치 전부다. 401/403은 즉시 중단(재시도해도 3,000번
// 거절), 429는 1회 재시도(기다리면 풀림), 그 밖은 그 종목만 건너뛴다.
// 분류가 어긋나면 한도 문제에 한 시간을 태우거나, 풀릴 오류에 종목을 버린다.

func stubClient(t *testing.T, status int, body string) *EarningsClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := NewEarningsClient(EarningsConfig{FinnhubAPIKey: "test-key"}, nil)
	c.baseURL = srv.URL
	return c
}

func TestFetchSurpriseHistory_403IsNotEntitledAndAborts(t *testing.T) {
	c := stubClient(t, http.StatusForbidden, `{"error":"You don't have access to this resource."}`)
	_, err := c.FetchSurpriseHistory(context.Background(), "AAPL")
	if !errorsIsNotEntitled(err) {
		t.Fatalf("err = %v — 403은 잡을 중단시켜야 한다", err)
	}
	if errorsIsRateLimited(err) {
		t.Fatal("403이 rate limit으로 분류됐다")
	}
}

func TestFetchSurpriseHistory_401IsNotEntitled(t *testing.T) {
	c := stubClient(t, http.StatusUnauthorized, `{"error":"Invalid API key"}`)
	_, err := c.FetchSurpriseHistory(context.Background(), "AAPL")
	if !errorsIsNotEntitled(err) {
		t.Fatalf("err = %v — 401도 중단 대상이다", err)
	}
}

func TestFetchSurpriseHistory_429IsRetryable(t *testing.T) {
	c := stubClient(t, http.StatusTooManyRequests, ``)
	_, err := c.FetchSurpriseHistory(context.Background(), "AAPL")
	if !errorsIsRateLimited(err) {
		t.Fatalf("err = %v — 429는 재시도 대상이다", err)
	}
	if errorsIsNotEntitled(err) {
		t.Fatal("429가 중단 사유로 분류됐다 — 기다리면 풀리는 오류다")
	}
}

// 500은 둘 다 아니다 — 그 종목만 건너뛰고 다음 실행에서 다시 받는다.
func TestFetchSurpriseHistory_500IsNeitherAbortNorRetry(t *testing.T) {
	c := stubClient(t, http.StatusInternalServerError, ``)
	_, err := c.FetchSurpriseHistory(context.Background(), "AAPL")
	if err == nil {
		t.Fatal("500인데 에러가 없다")
	}
	if errorsIsNotEntitled(err) || errorsIsRateLimited(err) {
		t.Fatalf("500이 특수 분류로 샜다: %v", err)
	}
}

// 빈 배열은 정상 응답이다(그 종목에 이력이 없을 뿐). 에러로 만들면 실패 카운터가
// 오르고 연속 실패 차단기가 엉뚱하게 잡을 멈춘다.
func TestFetchSurpriseHistory_emptyArrayIsNotAnError(t *testing.T) {
	c := stubClient(t, http.StatusOK, `[]`)
	rows, err := c.FetchSurpriseHistory(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("빈 배열에 에러: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(rows))
	}
}

func TestFetchSurpriseHistory_parsesRows(t *testing.T) {
	c := stubClient(t, http.StatusOK, `[
		{"actual":1.91,"estimate":1.9271,"period":"2026-06-30","quarter":3,"year":2026,"symbol":"AAPL"},
		{"actual":2.01,"estimate":1.9884,"period":"2026-03-31","quarter":2,"year":2026,"symbol":"AAPL"}
	]`)
	rows, err := c.FetchSurpriseHistory(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Period != "2026-06-30" || rows[0].Quarter != 3 || rows[0].Year != 2026 {
		t.Fatalf("row0 = %+v", rows[0])
	}
	if rows[0].Estimate == nil || *rows[0].Estimate != 1.9271 {
		t.Fatalf("estimate = %v", rows[0].Estimate)
	}
}

// --- 토큰 유출 방지 ---------------------------------------------------------
//
// 실측으로 겪은 사고다. 백필 중 VLY 타임아웃 한 건이 Go의 *url.Error를 통해
// API 키를 평문으로 로그 파일에 남겼다. Finnhub은 토큰을 쿼리스트링으로 받고,
// url.Error는 요청 URL을 통째로 메시지에 담는다.

func TestRedactFinnhubToken_stripsKeyFromTransportError(t *testing.T) {
	raw := errors.New(
		`Get "https://finnhub.io/api/v1/stock/earnings?symbol=VLY&token=sekrit-abc123": ` +
			`context deadline exceeded (Client.Timeout exceeded while awaiting headers)`)

	got := redactFinnhubToken(raw).Error()
	if strings.Contains(got, "sekrit-abc123") {
		t.Fatalf("키가 그대로 남았다: %s", got)
	}
	if !strings.Contains(got, "token=REDACTED") {
		t.Fatalf("REDACTED 치환이 안 됐다: %s", got)
	}
	// 나머지 정보는 진단에 필요하므로 남아야 한다.
	for _, want := range []string{"symbol=VLY", "context deadline exceeded"} {
		if !strings.Contains(got, want) {
			t.Fatalf("진단 정보 %q가 사라졌다: %s", want, got)
		}
	}
}

func TestRedactFinnhubToken_passthroughAndNil(t *testing.T) {
	if redactFinnhubToken(nil) != nil {
		t.Fatal("nil은 nil이어야 한다")
	}
	plain := errors.New("finnhub earnings: status 500")
	if got := redactFinnhubToken(plain); got != plain {
		t.Fatalf("토큰이 없으면 원래 에러를 그대로 돌려야 한다 (errors.Is 체인 보존): %v", got)
	}
}

// 새니타이저가 errors.Is 체인을 끊지 않는지 — 429/403 분류가 이 체인에 달려 있다.
func TestFetchSurpriseHistory_classificationSurvivesRedaction(t *testing.T) {
	c := stubClient(t, http.StatusTooManyRequests, ``)
	_, err := c.FetchSurpriseHistory(context.Background(), "AAPL")
	if !errorsIsRateLimited(err) {
		t.Fatalf("429 분류가 깨졌다: %v", err)
	}
}

// --- 시총 타이브레이크 가지치기 ---------------------------------------------

func consistency(sym string, beats, quarters int) model.EarningsConsistency {
	return model.EarningsConsistency{
		Symbol: sym,
		History: model.EarningsHitHistory{
			Beats: beats, Quarters: quarters, Misses: quarters - beats,
		},
	}
}

// 6위와 같은 등급은 통째로 남아야 한다 — 그 안의 순서를 시총이 정하기 때문이다.
// 해외는 실측 479종목이 (100%, 4분기)로 묶여 있어 이 그룹이 곧 후보 전체다.
func TestTrimToTieGroup_keepsWholeTieGroupAtTheCut(t *testing.T) {
	rows := []model.EarningsConsistency{}
	for i := range 10 { // 전부 4/4 동률
		rows = append(rows, consistency(fmt.Sprintf("P%d", i), 4, 4))
	}
	rows = append(rows, consistency("WORSE", 3, 4)) // 75% — 6위 안에 못 든다

	got := trimToTieGroup(rows)
	if len(got) != 10 {
		t.Fatalf("len = %d, want 10 (동률 10개는 남고 열등한 1개만 잘린다)", len(got))
	}
	for _, r := range got {
		if r.Symbol == "WORSE" {
			t.Fatal("적중률이 낮은 종목이 남았다 — 시총이 아무리 커도 6위 안에 못 든다")
		}
	}
}

// 표본이 많은 쪽이 위라는 규칙은 가지치기에서도 지켜져야 한다.
func TestTrimToTieGroup_prefersMoreQuartersAtSameRate(t *testing.T) {
	rows := []model.EarningsConsistency{}
	for i := range 6 {
		rows = append(rows, consistency(fmt.Sprintf("Q4_%d", i), 4, 4))
	}
	rows = append(rows, consistency("Q8", 8, 8)) // 같은 100%지만 표본 8분기

	got := trimToTieGroup(rows)
	if got[0].Symbol != "Q8" {
		t.Fatalf("1위 = %s, want Q8 (같은 비율이면 표본이 많은 쪽)", got[0].Symbol)
	}
}

func TestTrimToTieGroup_shortListUntouched(t *testing.T) {
	rows := []model.EarningsConsistency{consistency("A", 4, 4), consistency("B", 3, 4)}
	if got := trimToTieGroup(rows); len(got) != 2 {
		t.Fatalf("len = %d, want 2 (6개 이하는 자르지 않는다)", len(got))
	}
}
