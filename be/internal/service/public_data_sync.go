package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
PublicDataSync — 자산 확장 공공데이터 적재 잡 넷 (2026-10-01, worker-filings). 조회 경로는 외부를 부르지 않는다.

	re_trades   국토부 실거래 12종 × 전국 시군구(2026-10-02 사용자 결정 — 전에는 서울만 12종). 13개월 창,
	            **아파트 매매만 36개월**(단지 「기간 최고가」·3년 차트). 최근 3개월은 매일, 3~5개월 전은 주 1회,
	            그보다 오래된 달은 한 번만(신고 기한 30일 · 해제는 늦게 들어온다). 한 번에 호출 상한(limit, 기본 15,000 —
	            전국 유지보수가 하루 약 1만)까지 하고 나머지는 다음 날. 공공데이터포털 개발계정은 **API(종류)마다 하루 1만**이라
	            한도 초과 응답을 받은 종류는 그 회차에서 뺀다(다른 종류는 계속).
	re_index    한국부동산원 R-ONE 표 11개. 처음엔 전체 이력, 그 뒤엔 마지막 시점부터.
	ecos_rates  한국은행 ECOS 금리 21계열.
	onbid       캠코 온비드 부동산 공매 — 공고목록(30일 앞) → 공고상세(회차별 물건·감정가·최저가·결과) →
	            낙찰 회차만 결과상세(낙찰가). 오퍼레이션당 개발계정 일 1,000이라 상세는 하루 900까지.
*/
type PublicDataSync struct {
	client *marketdata.PublicDataClient
	repo   *repository.PublicDataRepo
	wealth *repository.WealthRepo
	values *WealthValuer
	now    func() time.Time
}

func NewPublicDataSync(client *marketdata.PublicDataClient, repo *repository.PublicDataRepo, wealth *repository.WealthRepo, values *WealthValuer) *PublicDataSync {
	return &PublicDataSync{client: client, repo: repo, wealth: wealth, values: values, now: time.Now}
}

func (s *PublicDataSync) sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func kstNow(now time.Time) time.Time { return now.In(time.FixedZone("KST", 9*3600)) }

// ── 실거래 ──────────────────────────────────────────────────────────────────

const (
	reTradeMonths    = 13
	reAptSaleMonths  = 36 // 아파트 매매만 — 단지 화면이 3년을 그린다
	reTradeKeepExtra = 12 // 창 밖으로 이만큼 더 두고 지운다(평가의 지수 보정 창)
)

func tradeMonths(k marketdata.RTMSKind) int {
	if k.Kind == "apt" && k.DealType == "sale" {
		return reAptSaleMonths
	}
	return reTradeMonths
}

// RunTrades is the re_trades job. limit = 이번 회차 최대 호출 수(0이면 6,000).
func (s *PublicDataSync) RunTrades(ctx context.Context, limit int) (map[string]any, string, error) {
	if !s.client.DataEnabled() {
		return nil, "", fmt.Errorf("re_trades: DATA_GO_KR_API_KEY not configured")
	}
	if limit <= 0 {
		limit = 15000
	}
	result := map[string]any{}
	regionMsg := s.ensureRegions(ctx)
	result["regions"] = regionMsg

	regions, err := s.repo.Regions(ctx)
	if err != nil {
		return result, "", err
	}
	synced, err := s.repo.SyncedAt(ctx)
	if err != nil {
		return result, "", err
	}
	now := kstNow(s.now())
	type task struct {
		k      marketdata.RTMSKind
		region string
		ym     string
		age    int
	}
	var tasks []task
	for age := 0; age < reAptSaleMonths; age++ {
		ym := now.AddDate(0, -age, 0).Format("200601")
		for _, g := range regions {
			for _, k := range marketdata.RTMSKinds {
				if age >= tradeMonths(k) {
					continue
				}
				last, ok := synced[repository.SyncKey(k.Kind, k.DealType, g.LawdCd, ym)]
				stale := !ok
				switch {
				case age <= 2:
					stale = !ok || now.Sub(last) > 20*time.Hour
				case age <= 5:
					stale = !ok || now.Sub(last) > 7*24*time.Hour
				}
				if stale {
					tasks = append(tasks, task{k, g.LawdCd, ym, age})
				}
			}
		}
	}
	// 최근 달 먼저, 같은 달이면 12종 지역(사용자 지역 포함)이 먼저.
	tier := map[string]string{}
	for _, g := range regions {
		tier[g.LawdCd] = g.Tier
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].age != tasks[j].age {
			return tasks[i].age < tasks[j].age
		}
		return tier[tasks[i].region] == "full" && tier[tasks[j].region] != "full"
	})

	calls, slices, rows := 0, 0, 0
	dead := map[string]string{} // 활용신청이 안 된 종류 — 이번 회차엔 더 부르지 않는다
	var failures []string
	stopped, pauses := "", 0
	for _, t := range tasks {
		if calls >= limit || stopped != "" {
			break
		}
		if _, skip := dead[t.k.Service]; skip {
			continue
		}
		var all []marketdata.RTMSRow
		ok := true
		for page := 1; page <= 20; page++ {
			if calls > 0 {
				if err := s.sleep(ctx, 260*time.Millisecond); err != nil {
					return result, "", err
				}
			}
			calls++
			got, total, err := s.client.RTMSPage(ctx, t.k, t.region, t.ym, page)
			// 429는 **그 API(종류)** 의 하루 한도다 — 종류마다 개발계정 한도가 달라(10-02 상업·업무용이 먼저 막힘)
			// 그 종류만 이번 회차에서 빼고, 429가 건 버킷 차단(1분)이 풀리면 다른 종류를 계속한다.
			// 10-02 첫 일괄 적재가 429 한 번에 남은 2.9만 구간을 두고 통째로 멈췄었다.
			if err != nil && pauses < 15 && (strings.Contains(err.Error(), "status 429") || strings.Contains(err.Error(), "blocked until")) {
				pauses++
				if strings.Contains(err.Error(), "status 429") {
					dead[t.k.Service] = "429 — 이 API의 하루 한도로 보임"
				}
				if e := s.sleep(ctx, 65*time.Second); e != nil {
					return result, "", e
				}
				if _, skip := dead[t.k.Service]; skip {
					ok = false
					break
				}
				page--
				continue
			}
			if err != nil {
				ok = false
				if errors.Is(err, apibudget.ErrExhausted) {
					stopped = err.Error() // 우리 예산(molit_rtms 하루 상한)이 다 찼다 — 나머지는 다음 회차
					break
				}
				msg := err.Error()
				// 활용신청 안 됨 · 그 API의 하루 한도 초과 — 둘 다 이 회차엔 그 종류를 더 부르지 않는다.
				if strings.Contains(msg, "등록되지 않은") || strings.Contains(msg, "NOT_REGISTERED") ||
					strings.Contains(msg, "LIMITED_NUMBER_OF_SERVICE_REQUESTS") || strings.Contains(msg, "초과") {
					dead[t.k.Service] = msg
				} else if len(failures) < 20 {
					failures = append(failures, msg)
				}
				break
			}
			all = append(all, got...)
			if len(all) >= total || len(got) == 0 {
				break
			}
		}
		if !ok {
			continue
		}
		if err := s.repo.ReplaceTradeSlice(ctx, t.k.Kind, t.k.DealType, t.region, t.ym, all); err != nil {
			return result, "", err
		}
		slices++
		rows += len(all)
	}
	cutoff := now.AddDate(0, -(reTradeMonths + reTradeKeepExtra), 0).Format("200601")
	aptCutoff := now.AddDate(0, -(reAptSaleMonths + reTradeKeepExtra), 0).Format("200601")
	pruned, _ := s.repo.PruneTrades(ctx, cutoff, aptCutoff)

	revalued, rerr := s.values.RevalueAll(ctx)
	result["calls"], result["slices"], result["rows"], result["pending"] = calls, slices, rows, len(tasks)-slices
	result["pruned"], result["revalued"] = pruned, revalued
	if len(dead) > 0 {
		result["notRegistered"] = dead
	}
	if len(failures) > 0 {
		result["failures"] = failures
	}
	msg := fmt.Sprintf("%s · 호출 %d · 구간 %d · 거래 %d · 남은 구간 %d · 재평가 %d", regionMsg, calls, slices, rows, len(tasks)-slices, revalued)
	if stopped != "" {
		msg += " · 예산 소진으로 멈춤(" + truncateStr(stopped, 60) + ")"
		result["stopped"] = stopped
	}
	if rerr != nil {
		msg += " · 재평가 오류 " + rerr.Error()
	}
	if len(dead) > 0 {
		msg += fmt.Sprintf(" · 이번 회차 제외 %d종(활용신청 없음·하루 한도)", len(dead))
	}
	if slices == 0 && len(tasks) > 0 && (len(failures) > 0 || len(dead) > 0) {
		return result, "", fmt.Errorf("re_trades: every slice failed — %s", msg)
	}
	return result, msg, nil
}

// ensureRegions refreshes the 시군구 list (주 1회) and seeds Seoul + user regions as 'full'.
func (s *PublicDataSync) ensureRegions(ctx context.Context) string {
	regions, _ := s.repo.Regions(ctx)
	msg := fmt.Sprintf("지역 %d", len(regions))
	// 전국 목록은 법정동코드 API로. 활용신청 전이면 서울 25구만으로 돈다.
	if len(regions) < 100 || kstNow(s.now()).Weekday() == time.Monday {
		if list, err := s.client.Regions(ctx); err == nil && len(list) > 100 {
			if err := s.repo.UpsertRegions(ctx, list, "all"); err == nil {
				msg = fmt.Sprintf("지역 %d(법정동코드)", len(list))
			}
		} else if err != nil {
			msg += " · 법정동코드 불가(" + truncateStr(err.Error(), 60) + ")"
		}
	}
	_ = s.repo.UpsertRegions(ctx, marketdata.SeoulRegions, "full")
	if assets, err := s.wealth.AssetsOfType(ctx, "real_estate"); err == nil {
		for _, a := range assets {
			if d := parseReDetail(a.Detail); d.LawdCd != "" {
				_ = s.repo.PromoteRegion(ctx, d.LawdCd)
			}
		}
	}
	return msg
}

func truncateStr(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// ── R-ONE 지수 ───────────────────────────────────────────────────────────────

// RoneTable — 적재하는 R-ONE 통계표.
type RoneTable struct {
	ID, Cycle, Key, Label, Unit string
}

// RoneTables — 화면 이름(Key)으로 찾는다. 첫 표(주간 매매)가 평가의 지수 보정에 쓰인다.
var RoneTables = []RoneTable{
	{"T244183132827305", "WK", "apt_sale_weekly", "아파트 매매가격지수(주간)", "지수"},
	{"T247713133046872", "WK", "apt_jeonse_weekly", "아파트 전세가격지수(주간)", "지수"},
	{"A_2024_00178", "MM", "apt_real_trade", "아파트 실거래가격지수", "지수"},
	{"A_2024_00045", "MM", "apt_sale_monthly", "아파트 매매가격지수(월간)", "지수"},
	{"A_2024_00055", "MM", "apt_rent_monthly", "아파트 월세가격지수", "지수"},
	{"A_2024_00156", "MM", "apt_conversion", "아파트 전월세 전환율", "%"},
	{"A_2024_00615", "MM", "offi_sale", "오피스텔 매매가격지수", "지수"},
	{"TT244233134228593", "QY", "office_rent", "오피스 임대가격지수", "지수"},
	{"T242963134993964", "QY", "retail_rent", "통합 상가 임대가격지수", "지수"},
	{"TT244763134428698", "QY", "office_vacancy", "오피스 공실률", "%"},
	{"T249633134845544", "QY", "retail_vacancy", "중대형 상가 공실률", "%"},
}

const roneWeeklySale = "T244183132827305"
// roneWeeklyJeonse — 아파트 전세가격지수(주간).
const roneWeeklyJeonse = "T247713133046872"

// RunIndex is the re_index job.
func (s *PublicDataSync) RunIndex(ctx context.Context) (map[string]any, string, error) {
	if !s.client.RoneEnabled() {
		return nil, "", fmt.Errorf("re_index: REB_RONE_API_KEY not configured")
	}
	result := map[string]any{}
	var parts, failures []string
	calls := 0
	call := func(t RoneTable, q url.Values, page int) ([]marketdata.RoneRow, int, error) {
		if calls > 0 {
			if err := s.sleep(ctx, 600*time.Millisecond); err != nil {
				return nil, 0, err
			}
		}
		calls++
		return s.client.RoneData(ctx, t.ID, t.Cycle, q, page)
	}
	for _, t := range RoneTables {
		latest, err := s.repo.LatestIndexPeriod(ctx, t.ID)
		if err != nil {
			return result, "", err
		}
		written := 0
		if latest == "" {
			// 처음: 전체 이력.
			for page := 1; page <= 400; page++ {
				rows, total, err := call(t, nil, page)
				if err != nil {
					failures = append(failures, err.Error())
					break
				}
				if err := s.repo.UpsertIndex(ctx, toIndexRows(t, rows)); err != nil {
					return result, "", err
				}
				written += len(rows)
				if len(rows) == 0 || page*1000 >= total {
					break
				}
			}
		} else {
			for _, p := range periodsSince(t.Cycle, latest, kstNow(s.now())) {
				rows, _, err := call(t, url.Values{"WRTTIME_IDTFR_ID": {p}}, 1)
				if err != nil {
					failures = append(failures, err.Error())
					break
				}
				if err := s.repo.UpsertIndex(ctx, toIndexRows(t, rows)); err != nil {
					return result, "", err
				}
				written += len(rows)
			}
		}
		now, _ := s.repo.LatestIndexPeriod(ctx, t.ID)
		result[t.Key] = map[string]any{"rows": written, "latest": now}
		parts = append(parts, fmt.Sprintf("%s %d(%s)", t.Key, written, now))
	}
	matched := s.matchRoneRegions(ctx)
	result["calls"], result["regionsMatched"] = calls, matched
	msg := fmt.Sprintf("호출 %d · 지역 매칭 %d · %s", calls, matched, strings.Join(parts, " · "))
	if len(failures) > 0 {
		result["failures"] = failures
		msg += fmt.Sprintf(" · 실패 %d (%s)", len(failures), truncateStr(failures[0], 80))
		if len(failures) >= len(RoneTables) {
			return result, "", fmt.Errorf("re_index: %s", msg)
		}
	}
	return result, msg, nil
}

func toIndexRows(t RoneTable, rows []marketdata.RoneRow) []repository.ReIndexRow {
	out := make([]repository.ReIndexRow, 0, len(rows))
	for _, r := range rows {
		d, ok := periodDate(t.Cycle, r.Period, r.PeriodDesc)
		if !ok {
			continue
		}
		out = append(out, repository.ReIndexRow{StatblID: t.ID, ClsID: r.ClsID, ItmID: r.ItmID, Period: r.Period,
			PeriodDate: d, ClsName: r.ClsName, ClsFull: r.ClsFull, ItmName: r.ItmName, Value: r.Value})
	}
	return out
}

// periodDate — 주간은 WRTTIME_DESC(2026-09-21), 월간 YYYYMM → 1일, 분기 YYYY0Q → 분기 첫날.
func periodDate(cycle, period, desc string) (time.Time, bool) {
	switch cycle {
	case "WK":
		t, err := time.Parse("2006-01-02", desc)
		return t, err == nil
	case "MM":
		t, err := time.Parse("200601", period)
		return t, err == nil
	case "QY":
		if len(period) != 6 {
			return time.Time{}, false
		}
		y, err1 := strconv.Atoi(period[:4])
		q, err2 := strconv.Atoi(period[4:])
		if err1 != nil || err2 != nil || q < 1 || q > 4 {
			return time.Time{}, false
		}
		return time.Date(y, time.Month((q-1)*3+1), 1, 0, 0, 0, 0, time.UTC), true
	}
	return time.Time{}, false
}

// periodsSince lists the stored latest period (정정 반영) and every later one up to now.
func periodsSince(cycle, latest string, now time.Time) []string {
	out := []string{latest}
	switch cycle {
	case "WK":
		y, _ := strconv.Atoi(latest[:4])
		w, _ := strconv.Atoi(latest[4:])
		// ISO 주의 월요일.
		d := isoWeekMonday(y, w)
		for d = d.AddDate(0, 0, 7); !d.After(now) && len(out) < 60; d = d.AddDate(0, 0, 7) {
			yy, ww := d.ISOWeek()
			out = append(out, fmt.Sprintf("%04d%02d", yy, ww))
		}
	case "MM":
		t, err := time.Parse("200601", latest)
		if err != nil {
			return out
		}
		for t = t.AddDate(0, 1, 0); !t.After(now) && len(out) < 36; t = t.AddDate(0, 1, 0) {
			out = append(out, t.Format("200601"))
		}
	case "QY":
		d, ok := periodDate("QY", latest, "")
		if !ok {
			return out
		}
		for d = d.AddDate(0, 3, 0); !d.After(now) && len(out) < 12; d = d.AddDate(0, 3, 0) {
			out = append(out, fmt.Sprintf("%04d0%d", d.Year(), (int(d.Month())-1)/3+1))
		}
	}
	return out
}

func isoWeekMonday(year, week int) time.Time {
	// 1월 4일은 늘 ISO 1주차다.
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	monday := jan4.AddDate(0, 0, -((int(jan4.Weekday()) + 6) % 7))
	return monday.AddDate(0, 0, (week-1)*7)
}

// SidoShort — R-ONE 지역 전체 이름의 첫 칸(서울·경기·충북 …).
func SidoShort(sido string) string {
	m := map[string]string{"충청북도": "충북", "충청남도": "충남", "전라북도": "전북", "전북특별자치도": "전북",
		"전라남도": "전남", "경상북도": "경북", "경상남도": "경남", "강원도": "강원", "강원특별자치도": "강원",
		"제주특별자치도": "제주", "세종특별자치시": "세종"}
	if v, ok := m[sido]; ok {
		return v
	}
	if r := []rune(sido); len(r) >= 2 {
		return string(r[:2])
	}
	return sido
}

// matchRoneRegions stores each region's class id in the weekly sale index (평가의 지수 보정용).
func (s *PublicDataSync) matchRoneRegions(ctx context.Context) int {
	regions, err := s.repo.Regions(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, g := range regions {
		if id, ok := s.roneCls(ctx, roneWeeklySale, g); ok {
			if err := s.repo.SetRoneCls(ctx, g.LawdCd, id); err == nil {
				n++
			}
		}
	}
	return n
}

// roneCls finds a region's class id in a table: 구 이름(마지막 낱말) + 시도, 없으면 시 이름.
func (s *PublicDataSync) roneCls(ctx context.Context, statblID string, g repository.ReRegion) (int, bool) {
	return RoneClsFor(ctx, s.repo, statblID, g.Sido, g.Name)
}

// RoneClsFor is shared with the API (지역 지수 화면).
func RoneClsFor(ctx context.Context, repo *repository.PublicDataRepo, statblID, sido, name string) (int, bool) {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return 0, false
	}
	hint := SidoShort(sido)
	if id, ok, err := repo.IndexClsByName(ctx, statblID, fields[len(fields)-1], hint); err == nil && ok {
		return id, true
	}
	if len(fields) > 1 {
		if id, ok, err := repo.IndexClsByName(ctx, statblID, fields[0], hint); err == nil && ok {
			return id, true
		}
	}
	// 시도 전체(세종 등).
	if id, ok, err := repo.IndexClsByName(ctx, statblID, hint, ""); err == nil && ok {
		return id, true
	}
	return 0, false
}

// ── ECOS 금리 ────────────────────────────────────────────────────────────────

// RateSeries — 적재하는 금리 계열. ID = 표/항목.
type RateSeriesDef struct {
	Stat, Item, Cycle, Key, Label string
	Years                         float64 // 국고채 만기(곡선용), 아니면 0
}

var RateSeriesDefs = []RateSeriesDef{
	{"722Y001", "0101000", "M", "base", "한국은행 기준금리", 0},
	{"817Y002", "010101000", "D", "call", "콜금리(1일)", 0},
	{"817Y002", "010901000", "D", "kofr", "KOFR", 0},
	{"817Y002", "010502000", "D", "cd91", "CD(91일)", 0},
	{"817Y002", "010503000", "D", "cp91", "CP(91일)", 0},
	{"817Y002", "010190000", "D", "ktb1", "국고채 1년", 1},
	{"817Y002", "010195000", "D", "ktb2", "국고채 2년", 2},
	{"817Y002", "010200000", "D", "ktb3", "국고채 3년", 3},
	{"817Y002", "010200001", "D", "ktb5", "국고채 5년", 5},
	{"817Y002", "010210000", "D", "ktb10", "국고채 10년", 10},
	{"817Y002", "010220000", "D", "ktb20", "국고채 20년", 20},
	{"817Y002", "010230000", "D", "ktb30", "국고채 30년", 30},
	{"817Y002", "010300000", "D", "corpAA", "회사채 3년 AA-", 0},
	{"817Y002", "010320000", "D", "corpBBB", "회사채 3년 BBB-", 0},
	{"121Y002", "BEABAA2118", "M", "deposit1y", "정기예금(1년) 평균", 0},
	{"121Y002", "BEABAA212", "M", "saving", "정기적금 평균", 0},
	{"121Y006", "BECBLA0302", "M", "mortgage", "주택담보대출 평균", 0},
	{"121Y006", "BECBLA030201", "M", "mortgageFixed", "고정형 주담대", 0},
	{"121Y006", "BECBLA030202", "M", "mortgageVar", "변동형 주담대", 0},
	{"121Y006", "BECBLA03041", "M", "jeonseLoan", "전세자금대출 평균", 0},
	{"121Y006", "BECBLA03051", "M", "creditLoan", "일반신용대출 평균", 0},
}

func (d RateSeriesDef) ID() string { return d.Stat + "/" + d.Item }

// RunRates is the ecos_rates job.
func (s *PublicDataSync) RunRates(ctx context.Context) (map[string]any, string, error) {
	if !s.client.EcosEnabled() {
		return nil, "", fmt.Errorf("ecos_rates: ECOS_API_KEY not configured")
	}
	now := kstNow(s.now())
	result := map[string]any{}
	var failures []string
	total := 0
	for i, d := range RateSeriesDefs {
		if i > 0 {
			if err := s.sleep(ctx, 600*time.Millisecond); err != nil {
				return result, "", err
			}
		}
		latest, err := s.repo.LatestRatePeriod(ctx, d.ID())
		if err != nil {
			return result, "", err
		}
		var from, to string
		if d.Cycle == "D" {
			from, to = now.AddDate(-5, 0, 0).Format("20060102"), now.Format("20060102")
			if t, err := time.Parse("20060102", latest); err == nil {
				from = t.AddDate(0, 0, -7).Format("20060102")
			}
		} else {
			from, to = "200901", now.Format("200601")
			if t, err := time.Parse("200601", latest); err == nil {
				from = t.AddDate(0, -3, 0).Format("200601")
			}
		}
		obs, err := s.client.EcosSeries(ctx, d.Stat, d.Cycle, d.Item, from, to)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		points := make([]repository.IndexPoint, 0, len(obs))
		periods := make([]string, 0, len(obs))
		for _, o := range obs {
			layout := "20060102"
			if d.Cycle == "M" {
				layout = "200601"
			}
			t, err := time.Parse(layout, o.Time)
			if err != nil {
				continue
			}
			points = append(points, repository.IndexPoint{Date: t.Format("2006-01-02"), Value: o.Value})
			periods = append(periods, o.Time)
		}
		if err := s.repo.UpsertRates(ctx, d.ID(), points, periods); err != nil {
			return result, "", err
		}
		total += len(points)
		result[d.Key] = len(points)
	}
	msg := fmt.Sprintf("계열 %d · 값 %d", len(RateSeriesDefs)-len(failures), total)
	if len(failures) > 0 {
		result["failures"] = failures
		msg += fmt.Sprintf(" · 실패 %d (%s)", len(failures), truncateStr(failures[0], 80))
		if len(failures) == len(RateSeriesDefs) {
			return result, "", fmt.Errorf("ecos_rates: %s", msg)
		}
	}
	return result, msg, nil
}

// ── 온비드 ──────────────────────────────────────────────────────────────────

// onbidPrptDivs — 재산유형 전부(압류·국유·기타일반·공유·금융권담보·유입·수탁·공공개발·파산).
const onbidPrptDivs = "0007,0010,0005,0002,0003,0006,0008,0011,0013"

// RunOnbid is the onbid job. limit = 공고상세 호출 상한(0이면 900).
func (s *PublicDataSync) RunOnbid(ctx context.Context, limit int) (map[string]any, string, error) {
	if !s.client.DataEnabled() {
		return nil, "", fmt.Errorf("onbid: DATA_GO_KR_API_KEY not configured")
	}
	if limit <= 0 {
		limit = 900
	}
	now := kstNow(s.now())
	result := map[string]any{}
	calls := 0
	page := func(service string, q url.Values) ([]map[string]any, int, error) {
		if calls > 0 {
			if err := s.sleep(ctx, 1100*time.Millisecond); err != nil {
				return nil, 0, err
			}
		}
		calls++
		return s.client.OnbidPage(ctx, service, q)
	}

	// ① 공고목록 — 오늘부터 30일 안에 개찰하는 부동산 공고.
	notices := 0
	for p := 1; p <= 80; p++ {
		items, total, err := page("OnbidPbancListSrvc2/getPbancList2", url.Values{
			"pageNo": {strconv.Itoa(p)}, "numOfRows": {"100"}, "cltrTypeCd": {"0001"}, "prptDivCd": {onbidPrptDivs},
			"opbdDtStart": {now.Format("20060102")}, "opbdDtEnd": {now.AddDate(0, 0, 30).Format("20060102")},
		})
		if err != nil {
			return result, "", err
		}
		list := make([]repository.OnbidNotice, 0, len(items))
		for _, it := range items {
			list = append(list, repository.OnbidNotice{
				PbancMngNo: marketdata.AnyStr(it, "pbancMngNo"), Name: marketdata.AnyStr(it, "onbidPbancNm"),
				OrgName: marketdata.AnyStr(it, "orgNm"), PrptDiv: marketdata.AnyStr(it, "prptDivNm"),
				OpbdAt: onbidTime(marketdata.AnyStr(it, "cltrOpbdDt")),
			})
		}
		if err := s.repo.UpsertNotices(ctx, list); err != nil {
			return result, "", err
		}
		notices += len(items)
		if len(items) == 0 || p*100 >= total {
			break
		}
	}
	reopened, _ := s.repo.ReopenNotices(ctx)

	// ② 공고상세 — 회차별 물건.
	todo, err := s.repo.NoticesTodo(ctx, limit)
	if err != nil {
		return result, "", err
	}
	itemsN, details := 0, 0
	var failures []string
	for _, no := range todo {
		ok := true
		for p := 1; p <= 10; p++ {
			items, total, err := page("OnbidPbancCltrDtlSrvc2/getPbancCltrInf2", url.Values{
				"pageNo": {strconv.Itoa(p)}, "numOfRows": {"100"}, "pbancMngNo": {no},
			})
			if err != nil {
				ok = false
				if len(failures) < 10 {
					failures = append(failures, err.Error())
				}
				break
			}
			rows := make([]repository.OnbidItem, 0, len(items))
			for _, it := range items {
				rows = append(rows, onbidItem(it))
			}
			if err := s.repo.UpsertOnbidItems(ctx, rows); err != nil {
				return result, "", err
			}
			itemsN += len(rows)
			if len(items) == 0 || p*100 >= total {
				break
			}
		}
		details++
		if ok {
			_ = s.repo.MarkNoticeDone(ctx, no)
		}
	}

	// ③ 결과상세 — 낙찰 회차의 낙찰가.
	results := 0
	todoRes, err := s.repo.ResultTodo(ctx, limit)
	if err != nil {
		return result, "", err
	}
	for _, it := range todoRes {
		items, _, err := page("OnbidCltrBidRsltDtlSrvc2/getCltrBidRsltDtl2", url.Values{
			"pageNo": {"1"}, "numOfRows": {"10"}, "cltrMngNo": {it.CltrMngNo}, "pbctCdtnNo": {strconv.FormatInt(it.PbctCdtnNo, 10)},
		})
		if err != nil {
			if len(failures) < 10 {
				failures = append(failures, err.Error())
			}
			continue
		}
		var win *int64
		var bidders *int
		status := "낙찰"
		for _, r := range items {
			if v, ok := marketdata.AnyInt64(r, "scfbAmt"); ok && v > 0 {
				win = &v
			}
			if v, ok := marketdata.AnyInt64(r, "vldBddrNope"); ok {
				n := int(v)
				bidders = &n
			}
			if st := onbidStatus(marketdata.AnyStr(r, "pbctStatNm")); st != "" {
				status = st
			}
		}
		if err := s.repo.SetOnbidResult(ctx, it.CltrMngNo, it.PbctCdtnNo, status, win, bidders); err != nil {
			return result, "", err
		}
		results++
	}

	result["calls"], result["notices"], result["reopened"] = calls, notices, reopened
	result["details"], result["items"], result["results"], result["pendingNotices"] = details, itemsN, results, len(todo)-details
	msg := fmt.Sprintf("호출 %d · 공고 %d · 상세 %d(회차 %d) · 낙찰가 %d", calls, notices, details, itemsN, results)
	if len(failures) > 0 {
		result["failures"] = failures
		msg += fmt.Sprintf(" · 실패 %d (%s)", len(failures), truncateStr(failures[0], 80))
	}
	return result, msg, nil
}

func onbidTime(s string) *time.Time {
	if len(s) < 12 {
		return nil
	}
	t, err := time.ParseInLocation("200601021504", s[:12], time.FixedZone("KST", 9*3600))
	if err != nil {
		return nil
	}
	return &t
}

// onbidStatus keeps only terminal results (낙찰·유찰·취소). 진행 중이면 "".
func onbidStatus(s string) string {
	switch {
	case strings.Contains(s, "낙찰"):
		return "낙찰"
	case strings.Contains(s, "유찰"):
		return "유찰"
	case strings.Contains(s, "취소"):
		return "취소"
	}
	return ""
}

func onbidItem(it map[string]any) repository.OnbidItem {
	o := repository.OnbidItem{
		CltrMngNo:  marketdata.AnyStr(it, "cltrMngNo"),
		PbancMngNo: marketdata.AnyStr(it, "pbancMngNo"),
		Name:       marketdata.AnyStr(it, "onbidCltrNm"),
		Address:    marketdata.AnyStr(it, "cltrAdr"),
		UsageL:     marketdata.AnyStr(it, "cltrUsgLclsCtgrNm"),
		UsageM:     marketdata.AnyStr(it, "cltrUsgMclsCtgrNm"),
		UsageS:     marketdata.AnyStr(it, "cltrUsgSclsCtgrNm"),
		PrptDiv:    marketdata.AnyStr(it, "prptDivNm"),
		DspsMethod: marketdata.AnyStr(it, "dspsMthodNm"),
		BidStart:   onbidTime(marketdata.AnyStr(it, "cltrBidBgngDt")),
		BidEnd:     onbidTime(marketdata.AnyStr(it, "cltrBidEndDt")),
		Result:     onbidStatus(marketdata.AnyStr(it, "pbctStatNm")),
	}
	if v, ok := marketdata.AnyInt64(it, "pbctCdtnNo"); ok {
		o.PbctCdtnNo = v
	}
	if v, ok := marketdata.AnyInt64(it, "onbidCltrno"); ok {
		o.OnbidCltrNo = &v
	}
	if v, ok := marketdata.AnyInt64(it, "usbdNft"); ok {
		o.FailCount = int(v)
	}
	if v, ok := marketdata.AnyInt64(it, "apslEvlAmt"); ok && v > 0 {
		o.Appraisal = &v
	}
	// 최저입찰가는 「비공개」처럼 글자로 올 수 있다 — 숫자일 때만.
	if v, ok := marketdata.AnyInt64(it, "lowstBidPrcIndctCont"); ok && v > 0 {
		o.MinBid = &v
	}
	if o.Appraisal != nil && o.MinBid != nil {
		r := float64(*o.MinBid) * 100 / float64(*o.Appraisal)
		o.MinBidRatio = &r
	}
	if f := strings.Fields(o.Address); len(f) > 0 {
		o.Sido = SidoShort(f[0])
	}
	return o
}
