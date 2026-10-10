package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
ChangeDetectService — 「변화」 메뉴의 재료. 점수 배치 끝에 하루 한 번, 그날 전 종목에서
일어난 일을 규칙으로 골라 `stock_change_events`에 둔다.

# 차별점 둘 (docs/COMPETITIVE_ANALYSIS.md D1 · D4)

 1. **데이터가 만든 변화는 올리지 않는다.** AI 점수가 축이 생기거나 사라져서, 또는 지표의
    비교 집단이 바뀌어서(섹터 피어가 켜진 날) 움직인 것은 회사에 일어난 일이 아니다(V2_PLAN §0). 비교 기준이 통째로 바뀐 날(섹터 피어를
    켠 날처럼 종목의 15% 이상이 한꺼번에 뛰는 날)도 마찬가지다.
 2. **설명할 수 없는 급변을 숨기지 않는다.** 같은 날 그 종목에 사건도 기사도 없으면
    `explained=false` — 화면이 「설명 불가」라고 그대로 말한다.

# LLM을 부르지 않는다

전부 저장된 표의 규칙이다. 조회는 저장분만 읽는다(LLM 지출 규칙).
*/
type ChangeDetectService struct {
	repo *repository.ChangeEventRepo
}

func NewChangeDetectService(repo *repository.ChangeEventRepo) *ChangeDetectService {
	return &ChangeDetectService{repo: repo}
}

// 문턱. 각 값의 근거는 옆 주석에 있다 — 바꿀 때 그 근거부터 다시 잰다.
const (
	// 등락은 종목 자신의 90일 일간 변동성의 2.5배. 하루 8%가 예사인 종목과 1%도 드문
	// 종목에 같은 잣대를 대면 한쪽은 전부 급등이고 다른 쪽은 아무 일도 없다(타임라인과 같은 생각).
	changePriceSigmas = 2.5
	changePriceMinPct = 4.0
	changePriceMaxPct = 15.0
	// 거래량은 20세션 평균의 5배.
	changeVolumeRatio = 5.0
	// AI 점수는 ±8 이상.
	changeScoreDelta = 8
	// 한 날 비교 종목의 15% 이상이 문턱을 넘으면 그것은 시장이 아니라 기준이 바뀐 날이다.
	changeScoreMassShare = 0.15
	// 내부자 순매매가 20세션 평균 거래량의 20% 이상.
	changeInsiderVolumeShare = 0.20
	// 실적 서프라이즈 ±5% 이상만 방향을 붙인다.
	changeEarningsSurprisePct = 5.0
	// 등락 이벤트의 「설명」으로 치는 기사 수(사건 이름이 없어도).
	changeExplainNewsCount = 3
	// 소형주 문턱 — 관심·보유 종목은 크기와 무관하게 본다.
	changeMinCapDomestic = 50e9  // 500억 원
	changeMinCapUS       = 300e6 // 3억 달러
	changeRetentionDays  = 90

	/*
		중요도 상한 — 종류마다 문턱 대비 배수를 쓰는데, 꼬리가 긴 종류(거래량 270배,
		내부자 「평균 거래량의 2928%」)가 목록 맨 위를 통째로 가져갔다(2026-09-23 실측).
		상한을 두면 급등락·공시·실적이 같은 줄에서 겨룬다.
	*/
	changeImportanceMax = 5.0
)

// Run detects one day's changes and replaces that day's rows.
func (s *ChangeDetectService) Run(ctx context.Context, tradeDate string) (map[string]any, string, error) {
	if s == nil || s.repo == nil {
		return nil, "", fmt.Errorf("change detect not configured")
	}
	day, err := time.ParseInLocation("2006-01-02", tradeDate, model.KST)
	if err != nil {
		return nil, "", fmt.Errorf("change detect date %q: %w", tradeDate, err)
	}
	universe, err := s.repo.Universe(ctx, tradeDate)
	if err != nil {
		return nil, "", err
	}
	inScope := func(sym string) (repository.ChangeUniverseRow, bool) {
		u, ok := universe[sym]
		if !ok {
			return u, false
		}
		if u.Tracked {
			return u, true
		}
		if u.AssetType == "us" {
			return u, u.MarketCap >= changeMinCapUS
		}
		return u, u.MarketCap >= changeMinCapDomestic
	}

	var events []repository.ChangeEvent
	counts := map[string]int{}
	add := func(e repository.ChangeEvent) {
		events = append(events, e)
		counts[e.Kind]++
	}
	from := day.AddDate(0, 0, -1).Format("2006-01-02")

	// ── 사건류 먼저: 등락의 「설명」 여부가 이것들로 정해진다 ──────────────────
	hasEvent := map[string]bool{}

	newsRows, err := s.repo.NewsEventsBetween(ctx, day.AddDate(0, 0, -1), day.AddDate(0, 0, 1))
	if err != nil {
		return nil, "", err
	}
	for _, e := range groupNewsEvents(newsRowsAboutTheirSymbol(newsRows, universe)) {
		if _, ok := inScope(e.Symbol); ok {
			add(e)
			hasEvent[e.Symbol] = true
		}
	}

	discs, err := s.repo.DisclosuresBetween(ctx, from, tradeDate)
	if err != nil {
		return nil, "", err
	}
	for _, e := range groupDisclosures(discs) {
		if _, ok := inScope(e.Symbol); ok {
			add(e)
			hasEvent[e.Symbol] = true
		}
	}

	earnings, err := s.repo.EarningsBetween(ctx, from, tradeDate)
	if err != nil {
		return nil, "", err
	}
	for _, r := range earnings {
		if _, ok := inScope(r.Symbol); !ok {
			continue
		}
		add(earningsEvent(r))
		hasEvent[r.Symbol] = true
	}

	sessions, err := s.repo.LatestSessions(ctx, tradeDate)
	if err != nil {
		return nil, "", err
	}
	avgVol := make(map[string]float64, len(sessions))
	for _, p := range sessions {
		avgVol[p.Symbol] = p.AvgVolume
	}

	insiders, err := s.repo.InsiderNetBetween(ctx, from, tradeDate)
	if err != nil {
		return nil, "", err
	}
	for _, r := range insiders {
		if _, ok := inScope(r.Symbol); !ok {
			continue
		}
		if e, ok := insiderEvent(r, avgVol[r.Symbol]); ok {
			add(e)
			hasEvent[r.Symbol] = true
		}
	}

	// ── 레딧 언급 (설명에는 안 넣는다 — socialEvent 머리글) ──────────────────
	social, err := s.repo.SocialBetween(ctx, tradeDate, day.AddDate(0, 0, 4).Format("2006-01-02"), tradeDate)
	if err != nil {
		log.Printf("change detect social: %v", err) // 표가 아직 없거나 비어도 다른 감지는 돈다
	}
	for _, r := range social {
		if _, ok := inScope(r.Symbol); !ok {
			continue
		}
		if e, ok := socialEvent(r); ok {
			add(e)
		}
	}

	// ── 등락 · 거래량 ────────────────────────────────────────────────────────
	newsCounts, err := s.repo.NewsCountsBetween(ctx, day.AddDate(0, 0, -1), day.AddDate(0, 0, 1))
	if err != nil {
		return nil, "", err
	}
	unexplained := 0
	for _, p := range sessions {
		if _, ok := inScope(p.Symbol); !ok {
			continue
		}
		explained := hasEvent[p.Symbol] || newsCounts[p.Symbol] >= changeExplainNewsCount
		for _, e := range sessionEvents(p, explained, newsCounts[p.Symbol]) {
			add(e)
			if e.Kind == "price_move" && !explained {
				unexplained++
			}
		}
	}

	// ── AI 점수 ──────────────────────────────────────────────────────────────
	jumps, compared, err := s.repo.ScoreJumps(ctx, tradeDate, changeScoreDelta)
	if err != nil {
		return nil, "", err
	}
	scoreNote := ""
	if compared > 0 && float64(len(jumps)) > changeScoreMassShare*float64(compared) {
		// 기준이 바뀐 날 — 한 종목의 변화가 아니다. 전부 올리면 목록이 이것으로 덮인다.
		scoreNote = fmt.Sprintf("점수 급변 %d/%d종목 — 비교 기준이 바뀐 날로 보고 올리지 않음", len(jumps), compared)
		log.Printf("change detect %s: %s", tradeDate, scoreNote)
	} else {
		dataDriven, peerDriven, methodDriven := 0, 0, 0
		for _, j := range jumps {
			if _, ok := inScope(j.Symbol); !ok {
				continue
			}
			if !j.SameAxes {
				dataDriven++
				continue
			}
			if !j.SameMethod {
				methodDriven++
				continue
			}
			if !j.SamePeers {
				peerDriven++
				continue
			}
			add(scoreJumpEvent(j, tradeDate))
		}
		var notes []string
		if dataDriven > 0 {
			notes = append(notes, fmt.Sprintf("축 유무로 바뀐 점수 %d종목 제외", dataDriven))
		}
		if methodDriven > 0 {
			notes = append(notes, fmt.Sprintf("산출 방식이 바뀐 점수 %d종목 제외", methodDriven))
		}
		if peerDriven > 0 {
			notes = append(notes, fmt.Sprintf("비교 집단이 바뀐 점수 %d종목 제외", peerDriven))
		}
		scoreNote = strings.Join(notes, " · ")
	}

	// ── 이미 올린 일은 다시 올리지 않는다 (휴장일 회차) ─────────────────────
	seen, err := s.repo.SeenBefore(ctx, tradeDate)
	if err != nil {
		return nil, "", err
	}
	fresh := events[:0]
	for _, e := range events {
		if !seen[repository.ChangeEventKey(e.Symbol, e.Kind, e.Subtype, e.EventDate)] {
			fresh = append(fresh, e)
		}
	}
	dropped := len(events) - len(fresh)
	events = fresh
	for k := range counts {
		counts[k] = 0
	}
	unexplained = 0
	for _, e := range events {
		counts[e.Kind]++
		if e.Kind == "price_move" && e.Explained != nil && !*e.Explained {
			unexplained++
		}
	}

	for i := range events {
		events[i].MarketCap = universe[events[i].Symbol].MarketCap
	}
	if err := s.repo.ReplaceDay(ctx, tradeDate, events); err != nil {
		return nil, "", err
	}
	if err := s.repo.PruneBefore(ctx, changeRetentionDays); err != nil {
		log.Printf("change detect prune: %v", err)
	}

	result := map[string]any{"date": tradeDate, "events": len(events), "byKind": counts,
		"unexplainedMoves": unexplained, "repeatsDropped": dropped}
	if scoreNote != "" {
		result["scoreNote"] = scoreNote
	}
	msg := fmt.Sprintf("%s 변화 %d건 (등락 %d · 설명 불가 %d · 거래량 %d · 점수 %d · 뉴스 %d · 공시 %d · 실적 %d · 내부자 %d)",
		tradeDate, len(events), counts["price_move"], unexplained, counts["volume_surge"], counts["score_jump"],
		counts["news_event"], counts["disclosure"], counts["earnings"], counts["insider"])
	if counts["social"] > 0 {
		msg += fmt.Sprintf(" · 레딧 %d", counts["social"])
	}
	if scoreNote != "" {
		msg += " · " + scoreNote
	}
	return result, msg, nil
}

// sessionEvents turns one session row into price/volume events.
func sessionEvents(p repository.PriceSessionRow, explained bool, newsCount int) []repository.ChangeEvent {
	var out []repository.ChangeEvent
	threshold := math.Min(changePriceMaxPct, math.Max(changePriceMinPct, changePriceSigmas*p.StdDev))
	if math.Abs(p.ChangeRate) >= threshold {
		dir := 1
		word := "급등"
		if p.ChangeRate < 0 {
			dir, word = -1, "급락"
		}
		mag := p.ChangeRate
		exp := explained
		out = append(out, repository.ChangeEvent{
			EventDate: p.Date, Symbol: p.Symbol, Kind: "price_move", Direction: dir,
			Magnitude: &mag, Importance: capImportance(math.Abs(p.ChangeRate) / threshold),
			Title:     fmt.Sprintf("%s %+.2f%%", word, p.ChangeRate),
			Explained: &exp,
			Detail: map[string]any{
				"thresholdPct": round3(threshold), "volatilityPct": round3(p.StdDev), "newsCount": newsCount,
			},
		})
	}
	if p.AvgVolume > 0 && p.Volume >= changeVolumeRatio*p.AvgVolume {
		ratio := p.Volume / p.AvgVolume
		exp := explained
		dir := 0
		if p.ChangeRate > 0 {
			dir = 1
		} else if p.ChangeRate < 0 {
			dir = -1
		}
		out = append(out, repository.ChangeEvent{
			EventDate: p.Date, Symbol: p.Symbol, Kind: "volume_surge", Direction: dir,
			// 거래량만 튄 날은 등락보다 덜 말한다 — 상한을 절반으로.
			Magnitude: &ratio, Importance: capImportance(math.Min(ratio/changeVolumeRatio, changeImportanceMax/2)),
			Title:     fmt.Sprintf("거래량 %.1f배 (등락 %+.2f%%)", ratio, p.ChangeRate),
			Explained: &exp,
			Detail:    map[string]any{"avgVolume": math.Round(p.AvgVolume), "volume": p.Volume, "newsCount": newsCount},
		})
	}
	return out
}

func scoreJumpEvent(j repository.ScoreJumpRow, tradeDate string) repository.ChangeEvent {
	delta := float64(j.Score - j.PrevScore)
	dir := 1
	if delta < 0 {
		dir = -1
	}
	return repository.ChangeEvent{
		EventDate: tradeDate, Symbol: j.Symbol, Kind: "score_jump", Direction: dir,
		Magnitude: &delta, Importance: capImportance(math.Abs(delta) / changeScoreDelta),
		Title:  fmt.Sprintf("AI 점수 %+d (%d → %d)", j.Score-j.PrevScore, j.PrevScore, j.Score),
		Detail: map[string]any{"prevDate": j.PrevDate},
	}
}

/*
newsRowsAboutTheirSymbol — 기사가 **그 종목 이야기일 때만** 그 종목의 사건으로 센다
(2026-09-26 외부 방문자 점검).

기사의 종목 태그는 「본문에 나온 종목 전부」다. 그래서 엔비디아의 「최근 변화」에
「목표가 상향 · 코어위브 주식 상향 조정」「목표가 하향 · 팔란티어 …」가 섰다 — 태그가
[CRWV, NVDA] · [PLTR, NVDA]였다. 이 한 줄은 종목 페이지 요약(검색 결과 설명)과 「오늘의
변화」에 그대로 나가므로, 남의 목표가 변경이 그 종목의 사실처럼 읽힌다.

태깅 자체는 뉴스 필터·종목 뉴스 탭이 쓰는 넓은 그물이라 건드리지 않는다. 여기서만 좁힌다:
제목(원문 또는 번역)이 그 회사를 부르고 있어야 한다 — 한글 이름 · 영문 이름의 핵심어 ·
티커 중 하나. 제목에 이름이 없는 「배당주 3선」 같은 모음 기사도 여기서 빠진다.
*/
func newsRowsAboutTheirSymbol(rows []repository.NewsEventRow, universe map[string]repository.ChangeUniverseRow) []repository.NewsEventRow {
	out := rows[:0:0]
	for _, r := range rows {
		u, ok := universe[r.Symbol]
		if !ok {
			continue
		}
		if headlineNamesCompany(u, r.Headline, r.Original) {
			out = append(out, r)
		}
	}
	return out
}

// 영문 이름에서 회사를 가리키지 않는 꼬리말 — 「NVIDIA CORP」의 CORP.
var englishNameNoise = map[string]bool{
	"inc": true, "corp": true, "corporation": true, "co": true, "company": true, "ltd": true,
	"limited": true, "plc": true, "holdings": true, "holding": true, "group": true, "the": true,
	"class": true, "cl": true, "sa": true, "nv": true, "ag": true, "se": true, "adr": true,
	"ads": true, "lp": true, "llc": true, "trust": true, "&": true,
}

// englishCoreName은 「NVIDIA CORP」→「nvidia」, 「TG THERAPEUTICS INC」→「tg therapeutics」.
// 첫 낱말이 너무 짧으면(3자 이하) 다음 낱말까지 붙인다 — 「tg」만으로는 아무 제목에나 걸린다.
func englishCoreName(name string) string {
	fields := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '-' || r == '/' || r == '(' || r == ')'
	})
	var words []string
	for _, f := range fields {
		if englishNameNoise[f] {
			continue
		}
		words = append(words, f)
		if len(words) == 1 && len(f) >= 4 {
			break
		}
		if len(words) == 2 {
			break
		}
	}
	return strings.Join(words, " ")
}

func headlineNamesCompany(u repository.ChangeUniverseRow, headlines ...string) bool {
	koName := strings.ReplaceAll(strings.TrimSpace(u.Name), " ", "")
	core := englishCoreName(u.EnglishName)
	for _, h := range headlines {
		if h == "" {
			continue
		}
		if len([]rune(koName)) >= 2 && strings.Contains(strings.ReplaceAll(h, " ", ""), koName) {
			return true
		}
		lower := strings.ToLower(h)
		if len(core) >= 3 && containsWordPrefix(lower, core) {
			return true
		}
		if sym := strings.ToLower(u.Symbol); len(sym) >= 2 && containsWholeWord(lower, sym) {
			return true
		}
	}
	return false
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// containsWordPrefix: needle이 낱말 머리에서 시작하면 참(「nvidia's」는 「nvidia」로 잡는다).
func containsWordPrefix(s, needle string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], needle)
		if j < 0 {
			return false
		}
		at := i + j
		if at == 0 {
			return true
		}
		prev, _ := utf8.DecodeLastRuneInString(s[:at])
		if !isWordRune(prev) {
			return true
		}
		i = at + 1
	}
}

// containsWholeWord: 앞뒤가 모두 낱말 경계여야 참 — 티커 「ai」가 「said」에 걸리지 않게.
func containsWholeWord(s, needle string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], needle)
		if j < 0 {
			return false
		}
		at := i + j
		end := at + len(needle)
		prevOK := at == 0
		if !prevOK {
			prev, _ := utf8.DecodeLastRuneInString(s[:at])
			prevOK = !isWordRune(prev)
		}
		nextOK := end == len(s)
		if !nextOK {
			next, _ := utf8.DecodeRuneInString(s[end:])
			nextOK = !isWordRune(next)
		}
		if prevOK && nextOK {
			return true
		}
		i = at + 1
	}
}

// groupNewsEvents folds articles into one event per (symbol, event type, day).
func groupNewsEvents(rows []repository.NewsEventRow) []repository.ChangeEvent {
	type agg struct {
		n        int
		sum      float64
		headline string
		best     float64
		date     string
	}
	byKey := map[string]*agg{}
	var keys []string
	for _, r := range rows {
		k := r.Symbol + "|" + r.EventType + "|" + r.Date
		a := byKey[k]
		if a == nil {
			a = &agg{date: r.Date, best: -1}
			byKey[k] = a
			keys = append(keys, k)
		}
		a.n++
		a.sum += r.Score
		// 대표 제목은 감성이 가장 뚜렷한 것 — 중립 제목은 사건을 잘 말하지 못한다.
		if math.Abs(r.Score) > a.best {
			a.best = math.Abs(r.Score)
			a.headline = r.Headline
		}
	}
	sort.Strings(keys)
	out := make([]repository.ChangeEvent, 0, len(keys))
	for _, k := range keys {
		parts := strings.SplitN(k, "|", 3)
		sym, ev := parts[0], model.NewsEventType(parts[1])
		a := byKey[k]
		avg := a.sum / float64(a.n)
		dir := sentimentDirection(avg)
		switch ev {
		case model.NewsEventTargetUp:
			dir = 1
		case model.NewsEventTargetDown:
			dir = -1
		}
		n := float64(a.n)
		out = append(out, repository.ChangeEvent{
			EventDate: a.date, Symbol: sym, Kind: "news_event", Subtype: string(ev), Direction: dir,
			Magnitude: &n, Importance: round3(1 + math.Min(float64(a.n), 10)*0.1),
			Title:  fmt.Sprintf("%s · %s", ev.Label(), a.headline),
			Detail: map[string]any{"articles": a.n, "avgSentiment": round3(avg)},
		})
	}
	return out
}

// groupDisclosures keeps notable filings, one event per (symbol, type, day).
func groupDisclosures(rows []repository.ChangeDisclosureRow) []repository.ChangeEvent {
	seen := map[string]bool{}
	var out []repository.ChangeEvent
	for _, r := range rows {
		ev := model.ClassifyDisclosureEvent(r.Title)
		if ev == model.NewsEventNone {
			continue
		}
		k := r.Symbol + "|" + string(ev) + "|" + r.Date
		if seen[k] {
			continue
		}
		seen[k] = true
		imp := 1.5
		if ev == model.NewsEventControlChange || ev == model.NewsEventTradingHalt {
			imp = 2.0
		}
		out = append(out, repository.ChangeEvent{
			EventDate: r.Date, Symbol: r.Symbol, Kind: "disclosure", Subtype: string(ev),
			Importance: imp, Title: fmt.Sprintf("공시 · %s: %s", ev.Label(), r.Title),
			Detail: map[string]any{"receiptNo": r.ReceiptNo},
		})
	}
	return out
}

/*
earningsEvent — 컨센서스가 있으면 서프라이즈, 없으면 「발표」만.

**예상이 없으면 서프라이즈를 만들지 않는다**(타임라인과 같은 규칙). 0%로 적으면
「예상대로였다」가 되는데 사실은 「비교할 예상이 없었다」다. 비교 기준은 EPS → 영업이익 →
매출 순서로 처음 있는 것.
*/
func earningsEvent(r repository.EarningsRow) repository.ChangeEvent {
	e := repository.ChangeEvent{EventDate: r.Date, Symbol: r.Symbol, Kind: "earnings", Importance: 1.0}
	candidates := []struct {
		name  string
		estV  float64
		estOK bool
		actV  float64
		actOK bool
	}{
		{"EPS", r.EPSEst.Float64, r.EPSEst.Valid, r.EPSAct.Float64, r.EPSAct.Valid},
		{"영업이익", r.OpEst.Float64, r.OpEst.Valid, r.OpAct.Float64, r.OpAct.Valid},
		{"매출", r.RevEst.Float64, r.RevEst.Valid, r.RevAct.Float64, r.RevAct.Valid},
	}
	for _, c := range candidates {
		if !c.actOK || !c.estOK || c.estV == 0 {
			continue
		}
		pct := (c.actV - c.estV) / math.Abs(c.estV) * 100
		e.Magnitude = &pct
		e.Detail = map[string]any{"basis": c.name, "estimate": c.estV, "actual": c.actV}
		if math.Abs(pct) < changeEarningsSurprisePct {
			e.Title = fmt.Sprintf("실적 · %s 예상 부합 (%+.1f%%)", c.name, pct)
			return e
		}
		if pct > 0 {
			e.Direction = 1
			e.Title = fmt.Sprintf("실적 · %s 서프라이즈 %+.1f%%", c.name, pct)
		} else {
			e.Direction = -1
			e.Title = fmt.Sprintf("실적 · %s 쇼크 %+.1f%%", c.name, pct)
		}
		e.Subtype = "surprise"
		e.Importance = round3(math.Min(4, math.Abs(pct)/changeEarningsSurprisePct))
		return e
	}
	e.Title = "실적 발표 · 비교할 예상 없음"
	return e
}

func insiderEvent(r repository.InsiderNetRow, avgVolume float64) (repository.ChangeEvent, bool) {
	if avgVolume <= 0 || math.Abs(r.Net) < changeInsiderVolumeShare*avgVolume {
		return repository.ChangeEvent{}, false
	}
	share := math.Abs(r.Net) / avgVolume
	dir, word := 1, "순매수"
	if r.Net < 0 {
		dir, word = -1, "순매도"
	}
	net := r.Net
	return repository.ChangeEvent{
		EventDate: r.Date, Symbol: r.Symbol, Kind: "insider", Direction: dir,
		Magnitude: &net, Importance: capImportance(math.Min(share/changeInsiderVolumeShare, changeImportanceMax/2)),
		Title:  fmt.Sprintf("내부자 %s %s주 (평균 거래량의 %.0f%%)", word, formatShareCount(math.Abs(r.Net)), share*100),
		Detail: map[string]any{"filings": r.Count},
	}, true
}

func sentimentDirection(avg float64) int {
	switch {
	case avg >= 0.2:
		return 1
	case avg <= -0.2:
		return -1
	}
	return 0
}

func formatShareCount(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

func capImportance(v float64) float64 { return round3(math.Min(v, changeImportanceMax)) }

/*
socialEvent — 레딧 언급 급증·과열 (2026-09-28 사용자 요청 7, 1단계). 점수에는 아직 안 쓴다.

	급증  24시간 언급 20회 이상이면서 전일의 3배 이상(전일 0이면 30회 이상), 또는
	      순위가 50위 밖(또는 없음)에서 10위 안으로 뛰어든 경우
	과열  급증 + 5일 수익률 15% 이상 또는 RSI 75 이상 — 개인 관심이 가격을 이미 끌어올린 상태

「설명」(hasEvent)에는 넣지 않는다 — 레딧의 수다는 급등락의 원인을 말해 주지 않는다.
*/
func socialEvent(r repository.ChangeSocialRow) (repository.ChangeEvent, bool) {
	if r.Mentions < 20 {
		return repository.ChangeEvent{}, false
	}
	surge := (r.Prev == 0 && r.Mentions >= 30) || (r.Prev > 0 && r.Mentions >= 3*r.Prev)
	jump := r.Rank > 0 && r.Rank <= 10 && (r.RankPrev == nil || *r.RankPrev > 50)
	if !surge && !jump {
		return repository.ChangeEvent{}, false
	}
	hot := (r.Return5d != nil && *r.Return5d >= 15) || (r.RSI != nil && *r.RSI >= 75)
	n := float64(r.Mentions)
	e := repository.ChangeEvent{
		EventDate: r.Date, Symbol: r.Symbol, Kind: "social", Subtype: "surge",
		Magnitude: &n, Importance: 1.2,
		Title:  fmt.Sprintf("레딧 언급 급증 · 24시간 %d회(전일 %d회) · 전체 %d위", r.Mentions, r.Prev, r.Rank),
		Detail: map[string]any{"mentions": r.Mentions, "mentionsPrev": r.Prev, "rank": r.Rank},
	}
	if hot {
		e.Subtype, e.Importance = "overheat", 1.6
		why := ""
		if r.Return5d != nil && *r.Return5d >= 15 {
			why = fmt.Sprintf("5일 %+.1f%%", *r.Return5d)
		} else if r.RSI != nil {
			why = fmt.Sprintf("RSI %.0f", *r.RSI)
		}
		e.Title = fmt.Sprintf("레딧 과열 · 언급 %d회(전일 %d회) · %s", r.Mentions, r.Prev, why)
	}
	return e, true
}
