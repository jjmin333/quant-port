package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
종목 타임라인 — 「주가가 왜 움직였는지」를 한 줄에 모은다 (V2 요구 3).

설계의 전제와 레인별 시작일 문제는 `model/stock_timeline.go` 머리글에 있다. 여기서는
**어떤 날을 보여 줄지 고르는 규칙**만 적는다.

# 왜 전부 보여 주지 않는가

90일이면 거래일이 60일 남짓이고, 그중 대부분은 아무 일도 없다. 전부 세우면 중요한
날이 묻힌다. 그래서 두 부류만 남긴다:

  ① **크게 움직인 날** — 문턱은 종목마다 다르다(그 창의 표준편차 × 1.5, 2~10%로 가둠).
     하루 8%가 예사인 종목과 1%도 드문 종목에 같은 잣대를 대면 한쪽은 전부 급등이고
     다른 쪽은 아무 일도 없다.
  ② **분명한 사건이 있던 날** — 실적 발표 · 공시 · 내부자 거래 · 큰 점수 변화.

②는 등락과 무관하게 남긴다. 「실적을 발표했는데 주가가 안 움직였다」도 답이다.

# 비용

읽기는 여섯 번이고 전부 인덱스를 탄다(종가 · 점수 · 뉴스 집계 · 내부자 집계 · 공시 ·
실적) + 레인 시작일 여섯. 뉴스는 SQL이 날짜로 접어 90행으로 돌아온다 — 15,234행을
Go로 가져오지 않는다.
*/

// StockTimelineService assembles the timeline from stored data only.
type StockTimelineService struct {
	repo   *repository.StockTimelineRepo
	scores *ScoreService
	stocks *repository.StockRepo
}

func NewStockTimelineService(
	repo *repository.StockTimelineRepo,
	scores *ScoreService,
	stocks *repository.StockRepo,
) *StockTimelineService {
	return &StockTimelineService{repo: repo, scores: scores, stocks: stocks}
}

// timelineDefaultDays / timelineMaxDays bound the window.
//
// 90일은 분기 하나다 — 실적 발표가 한 번은 들어오는 최소한이고, 종가 60여 행이면
// 차트가 읽힌다. 상한을 두는 이유는 뉴스 집계가 창에 비례하기 때문이다.
const (
	timelineDefaultDays = 90
	timelineMaxDays     = 365
)

// Get builds one symbol's timeline.
func (s *StockTimelineService) Get(
	ctx context.Context, symbol string, days int,
) (*model.StockTimeline, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("timeline service not configured")
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, fmt.Errorf("symbol required")
	}
	if days <= 0 {
		days = timelineDefaultDays
	}
	if days > timelineMaxDays {
		days = timelineMaxDays
	}

	now := time.Now().In(model.KST)
	to := now.Format("2006-01-02")
	from := now.AddDate(0, 0, -days).Format("2006-01-02")

	closes, err := s.repo.Closes(ctx, symbol, from, to)
	if err != nil {
		return nil, err
	}

	out := &model.StockTimeline{
		Symbol:   symbol,
		FromDate: from,
		ToDate:   to,
		Closes:   closes,
		Days:     []model.TimelineDay{},
		Lanes:    []model.TimelineLane{},
	}
	if s.stocks != nil {
		if info, err := s.stocks.GetBySymbol(ctx, symbol); err == nil && info != nil {
			out.Name = info.Name
			out.Currency = string(info.Currency)
		}
	}

	newsCounts, err := s.repo.NewsCounts(ctx, symbol, from, to)
	if err != nil {
		return nil, err
	}
	insiderDays, err := s.repo.InsiderDays(ctx, symbol, from, to)
	if err != nil {
		return nil, err
	}
	disclosures, err := s.repo.Disclosures(ctx, symbol, from, to)
	if err != nil {
		return nil, err
	}
	earnings, err := s.repo.EarningsDays(ctx, symbol, from, to)
	if err != nil {
		return nil, err
	}
	laneStarts, err := s.repo.LaneStarts(ctx, symbol)
	if err != nil {
		return nil, err
	}

	scoreDays := s.scoreDays(ctx, symbol, from, to)

	build := timelineBuild{
		from:        from,
		to:          to,
		closes:      closes,
		scores:      scoreDays,
		news:        newsCounts,
		insider:     insiderDays,
		disclosures: disclosures,
		earnings:    earnings,
	}
	out.Volatility, out.MoveThreshold = timelineThreshold(closes)
	out.Days = buildTimelineDays(build, out.MoveThreshold)
	out.Lanes = buildTimelineLanes(laneStarts, from, build)
	return out, nil
}

/*
scoreDays — 창 안의 점수와 **전 회차 대비 최대 원인**.

Phase 1의 귀인 엔진을 그대로 쓴다(`attributeChange`) — 타임라인이 자기만의 「왜」를
따로 계산하면 같은 날에 대해 두 화면이 다른 이유를 말하게 된다.

점수를 못 읽어도 타임라인은 그린다. 점수는 여섯 레인 중 하나일 뿐이고, 한 레인이
비었다고 나머지를 버리면 사용자는 아무것도 못 본다.
*/
func (s *StockTimelineService) scoreDays(
	ctx context.Context, symbol, from, to string,
) map[string]model.TimelineScore {
	out := map[string]model.TimelineScore{}
	if s.scores == nil {
		return out
	}
	// 보관이 30거래일이므로 그 이상 물어도 더 오지 않는다.
	rows, err := s.scores.History(ctx, symbol, 30)
	if err != nil || len(rows) == 0 {
		return out
	}
	settings := model.DefaultScoreSettings()
	weights := settings.Weights

	// History는 최신순이다. 직전 회차는 바로 다음 원소다.
	for i := range rows {
		row := rows[i]
		if row.TradeDate < from || row.TradeDate > to {
			continue
		}
		entry := model.TimelineScore{Value: row.Score}
		if i+1 < len(rows) {
			prev := rows[i+1]
			ch := attributeChange(model.ScoreChangePrevDay, &prev, row, weights, settings)
			if ch.Available {
				entry.Delta = ch.Delta
				if len(ch.Drivers) > 0 {
					entry.Driver = ch.Drivers[0].Detail
					entry.DriverKind = string(ch.Drivers[0].Kind)
				}
			}
		}
		out[row.TradeDate] = entry
	}
	return out
}

type timelineBuild struct {
	from, to    string
	closes      []model.TimelinePoint
	scores      map[string]model.TimelineScore
	news        []repository.TimelineDateCount
	insider     []repository.TimelineInsiderDay
	disclosures []repository.TimelineDisclosureRow
	earnings    []repository.TimelineEarningsRow
}

/*
timelineThreshold — 「큰 등락」의 문턱을 이 종목의 변동성에서 뽑는다.

표준편차만 따르면 극단이 생긴다: 아주 조용한 종목은 0.4%짜리 날이 전부 급등이 되고,
아주 사나운 종목은 15% 미만이 전부 조용한 날이 된다. 사람이 「큰 움직임」이라 부르는
범위(2~10%)에 가둔다.
*/
func timelineThreshold(closes []model.TimelinePoint) (volatility, threshold float64) {
	rates := make([]float64, 0, len(closes))
	for _, p := range closes {
		if p.ChangeRate != nil {
			rates = append(rates, *p.ChangeRate)
		}
	}
	if len(rates) < 2 {
		return 0, model.TimelineMoveFloor
	}
	sd := stdDev(rates)
	t := sd * model.TimelineMoveSigma
	if t < model.TimelineMoveFloor {
		t = model.TimelineMoveFloor
	}
	if t > model.TimelineMoveCeil {
		t = model.TimelineMoveCeil
	}
	return btRound2(sd), btRound2(t)
}

func buildTimelineDays(b timelineBuild, threshold float64) []model.TimelineDay {
	byDate := map[string]*model.TimelineDay{}
	touch := func(date string) *model.TimelineDay {
		if d, ok := byDate[date]; ok {
			return d
		}
		d := &model.TimelineDay{Date: date}
		byDate[date] = d
		return d
	}

	// 종가는 **모든 날**에 붙는다 — 사건이 있는 날의 등락률을 채워야 하기 때문이다.
	closeByDate := make(map[string]model.TimelinePoint, len(b.closes))
	for _, p := range b.closes {
		closeByDate[p.Date] = p
	}

	for date, sc := range b.scores {
		d := touch(date)
		entry := sc
		d.Score = &entry
	}
	for _, n := range b.news {
		if n.Count <= 0 {
			continue
		}
		d := touch(n.Date)
		d.News = &model.TimelineNews{Count: n.Count}
	}
	for _, in := range b.insider {
		d := touch(in.Date)
		d.Insider = &model.TimelineInsider{
			Count:      in.Count,
			BuyShares:  in.BuyShares,
			SellShares: in.SellShares,
			NetShares:  in.BuyShares - in.SellShares,
		}
	}
	for _, dis := range b.disclosures {
		d := touch(dis.Date)
		d.Disclosures = append(d.Disclosures, model.TimelineDisclosure{
			Title: dis.Title, ReceiptNo: dis.ReceiptNo, Filer: dis.Filer,
		})
	}
	for _, e := range b.earnings {
		d := touch(e.Date)
		d.Earnings = timelineEarnings(e)
	}
	// 큰 등락만으로도 날이 선다.
	for _, p := range b.closes {
		if p.ChangeRate != nil && math.Abs(*p.ChangeRate) >= threshold {
			touch(p.Date)
		}
	}

	out := make([]model.TimelineDay, 0, len(byDate))
	for _, d := range byDate {
		if p, ok := closeByDate[d.Date]; ok {
			close := p.Close
			d.Close = &close
			d.ChangeRate = p.ChangeRate
		}
		d.Reasons = timelineReasons(*d, threshold)
		d.Significant = len(d.Reasons) > 0
		if !d.Significant {
			// 뉴스만 있는 평범한 날은 목록에 세우지 않는다 — 거의 매일이 그렇다.
			continue
		}
		out = append(out, *d)
	}
	// 최신순 — 사람이 타임라인을 여는 이유는 대개 「요즘 무슨 일이 있었나」다.
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

/*
timelineReasons — 이 날이 왜 목록에 있는지.

비어 있으면 목록에 세우지 않는다. 뉴스 건수만으로는 이유가 되지 않는다 — 삼성전자는
하루 평균 180건이라 그것으로 고르면 모든 날이 남는다.
*/
func timelineReasons(d model.TimelineDay, threshold float64) []string {
	var reasons []string
	/*
		등락률은 **문장에 넣지 않는다.** 같은 줄에 `ChangeRate`가 이미 있고, 그 숫자는
		화면이 자기 규칙으로 적는다(음수 기호도 화면 쪽이 −를 쓴다). 여기서 한 번 더
		적으면 같은 줄에 같은 값이 두 번, 그것도 다른 글자로 선다.
	*/
	if d.ChangeRate != nil && math.Abs(*d.ChangeRate) >= threshold {
		if *d.ChangeRate > 0 {
			reasons = append(reasons, "급등")
		} else {
			reasons = append(reasons, "급락")
		}
	}
	if d.Earnings != nil {
		reasons = append(reasons, "실적 발표")
	}
	if len(d.Disclosures) > 0 {
		reasons = append(reasons, fmt.Sprintf("공시 %d건", len(d.Disclosures)))
	}
	if d.Insider != nil && d.Insider.Count > 0 {
		reasons = append(reasons, fmt.Sprintf("내부자 거래 %d건", d.Insider.Count))
	}
	if d.Score != nil && d.Score.Delta != nil && abs(*d.Score.Delta) >= model.TimelineScoreJump {
		reasons = append(reasons, fmt.Sprintf("AI 점수 %+d", *d.Score.Delta))
	}
	return reasons
}

func timelineEarnings(e repository.TimelineEarningsRow) *model.TimelineEarnings {
	out := &model.TimelineEarnings{
		Session: e.Session.String,
		Status:  "estimate",
	}
	if e.FiscalYear.Valid && e.Period.Valid {
		out.Period = fmt.Sprintf("%d %s", e.FiscalYear.Int64, e.Period.String)
	} else if e.Period.Valid {
		out.Period = e.Period.String
	}
	if e.Currency.Valid {
		out.Currency = e.Currency.String
	}
	if e.RevenueAct.Valid {
		v := e.RevenueAct.Float64
		out.RevenueAct = &v
		out.Status = "actual"
	}
	if e.RevenueEst.Valid {
		v := e.RevenueEst.Float64
		out.RevenueEst = &v
	}
	if e.OpIncomeAct.Valid {
		v := e.OpIncomeAct.Float64
		out.OpIncomeAct = &v
		out.Status = "actual"
	}
	if e.EPSAct.Valid {
		v := e.EPSAct.Float64
		out.EPSAct = &v
		out.Status = "actual"
	}
	if e.EPSEst.Valid {
		v := e.EPSEst.Float64
		out.EPSEst = &v
	}
	/*
		서프라이즈는 **추정치가 있을 때만**. 컨센서스가 없는데 0%로 적으면
		「예상대로였다」가 되는데, 사실은 「비교할 예상이 없었다」이다.
	*/
	if out.EPSAct != nil && out.EPSEst != nil && math.Abs(*out.EPSEst) > 1e-9 {
		s := btRound2((*out.EPSAct - *out.EPSEst) / math.Abs(*out.EPSEst) * 100)
		out.Surprise = &s
	}
	return out
}

/*
buildTimelineLanes — 레인마다 **언제부터 기록이 있는지**와 창 안의 건수.

이 기능에서 가장 중요한 한 줄이다. 실측(005930): 종가는 2024-12-16부터인데 공시는
2026-08-20부터다. 1년을 그리면 공시 레인이 열 달 비는데, 그것은 「공시가 없었다」가
아니라 「수집 전」이다 (요구 8).
*/
func buildTimelineLanes(
	starts map[model.TimelineLaneID]string, from string, b timelineBuild,
) []model.TimelineLane {
	counts := map[model.TimelineLaneID]int{
		model.TimelineLaneClose:      len(b.closes),
		model.TimelineLaneScore:      len(b.scores),
		model.TimelineLaneEarnings:   len(b.earnings),
		model.TimelineLaneDisclosure: len(b.disclosures),
		model.TimelineLaneInsider:    len(b.insider),
	}
	for _, n := range b.news {
		counts[model.TimelineLaneNews] += n.Count
	}

	out := make([]model.TimelineLane, 0, len(model.TimelineLaneOrder))
	for _, id := range model.TimelineLaneOrder {
		start := starts[id]
		out = append(out, model.TimelineLane{
			ID:            id,
			Label:         model.TimelineLaneLabels[id],
			AvailableFrom: start,
			Count:         counts[id],
			// 기록이 창보다 먼저 시작했는가. 시작일이 없으면(자료 0건) false다.
			CoversWindow: start != "" && start <= from,
		})
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

/*
DayNews — 타임라인에서 하루를 펼쳤을 때의 기사 목록.

본문 응답이 건수만 싣는 이유는 `model` 머리글에 있다. 이 입구는 **사람이 그 날을 열었을
때만** 불린다.

한국어 제목이 있으면 그것을 쓴다 — 해외 기사도 번역이 있으면 같은 목록에서 읽힌다
(`headline_ko`가 번역의 증거는 아니다: 국내 기사도 그 칸을 채운다. 여기서는 「화면에
보여 줄 제목」만 고르므로 출처를 따지지 않는다).
*/
func (s *StockTimelineService) DayNews(
	ctx context.Context, symbol, date string, limit int,
) (*model.TimelineDayNews, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("timeline service not configured")
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	date = strings.TrimSpace(date)
	if symbol == "" || date == "" {
		return nil, fmt.Errorf("symbol and date required")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, fmt.Errorf("date must be YYYY-MM-DD")
	}

	rows, err := s.repo.NewsForDate(ctx, symbol, date, limit)
	if err != nil {
		return nil, err
	}
	out := &model.TimelineDayNews{
		Symbol: symbol,
		Date:   date,
		Items:  make([]model.TimelineNewsItem, 0, len(rows)),
	}
	for _, r := range rows {
		headline := r.Headline
		if r.HeadlineKo.Valid && strings.TrimSpace(r.HeadlineKo.String) != "" {
			headline = r.HeadlineKo.String
		}
		publisher := r.Publisher.String
		if publisher == "" {
			publisher = r.PublisherID.String
		}
		out.Items = append(out.Items, model.TimelineNewsItem{
			ID:          r.ID,
			Headline:    headline,
			URL:         r.URL,
			Publisher:   publisher,
			PublishedAt: r.PublishedAt,
			ViewCount:   r.ViewCount,
		})
	}
	return out, nil
}
