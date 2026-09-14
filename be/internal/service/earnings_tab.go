package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// noConsensusMessage explains an *empty* consensus, not an impossible one.
//
// 예전 문구는 "무료 데이터 소스(DART/Finnhub/EDGAR)에서는 컨센서스·서프라이즈
// 수치를 제공하지 않습니다"였고, 그건 사실이 아니었다 — Finnhub 응답에
// epsEstimate/epsActual이 처음부터 실려 왔는데 파서가 버리고 있었고, 국내는
// FnGuide 잠정실적발표가 추정치와 발표치를 함께 준다. 지금 이 문장이 뜨는 경우는
// 소스에 이 종목의 컨센서스가 아직 없을 때뿐이다(주로 소형주·신규 상장).
const noConsensusMessage = "이 종목의 컨센서스 추정치가 아직 집계되지 않았습니다. 발표 실적은 아래 정기보고서 표에서 볼 수 있습니다."

// EarningsTabService assembles the 실적 탭 response, branching on domestic
// (DART) vs US (Finnhub + EDGAR) the same way FundamentalsService does.
type EarningsTabService struct {
	stocks *repository.StockRepo
	// periods — 분기 실적. 재무제표 상세 모달과 **같은 표**를 읽는다.
	periods *repository.FinancialPeriodRepo
	// disclosures — 국내 「최근 공시」. 시장 전체 일별 스윕이 채운다.
	disclosures *repository.DisclosureRepo
	// events — 실적 발표 이벤트 표(컨센서스·서프라이즈). 캘린더 화면과 **같은
	// 표**를 읽는다. 탭이 따로 계산하면 같은 발표에 두 숫자가 생긴다.
	events *repository.EarningsEventRepo
	// reactions — 발표 후 주가 반응 계산. 캘린더 서비스의 것을 그대로 빌려 쓴다.
	// 같은 계산을 두 벌 두면 같은 발표에 두 반응이 생긴다.
	reactions *EarningsCalendarService
}

// SetEvents wires the earnings_events reader. 생성자를 늘리지 않고 따로 둔 이유는
// 기존 호출 지점(server.go)과 시험 코드를 한꺼번에 흔들지 않기 위한 것이다 —
// BatchAdminService.SetResearch와 같은 방식.
func (s *EarningsTabService) SetEvents(events *repository.EarningsEventRepo, reactions *EarningsCalendarService) {
	if s != nil {
		s.events = events
		s.reactions = reactions
	}
}

func NewEarningsTabService(
	stocks *repository.StockRepo,
	periods *repository.FinancialPeriodRepo,
	disclosures *repository.DisclosureRepo,
) *EarningsTabService {
	return &EarningsTabService{
		stocks:      stocks,
		periods:     periods,
		disclosures: disclosures,
	}
}

func (s *EarningsTabService) Get(ctx context.Context, symbol string) (*model.EarningsTabView, error) {
	stock, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return nil, fmt.Errorf("earnings: unknown symbol %s", symbol)
	}

	assetType := ResolveFundamentalsAssetType(stock)
	view := &model.EarningsTabView{
		Symbol:    stock.Symbol,
		AssetType: assetType,
	}

	if assetType == model.AssetDomestic {
		view.Calendar = s.domesticCalendar(ctx, stock.Symbol)
		view.Quarterly = s.domesticQuarterly(ctx, stock.Symbol)
	} else {
		view.Calendar = s.usCalendar(ctx, stock.Symbol)
		view.Quarterly = s.usQuarterly(ctx, stock.Symbol)
	}

	// 발표 이벤트(컨센서스 포함)는 시장에 상관없이 같은 표에서 온다. 국내·해외
	// 분기를 여기서 한 번 더 하지 않는 이유는, 적재 쪽이 이미 시장별 소스를
	// 갈라 놓았기 때문이다 — 읽는 쪽에서 또 가르면 두 곳이 어긋날 수 있다.
	view.Announcements = s.announcements(ctx, stock.Symbol)
	view.Consensus = latestConsensus(view.Announcements)
	// 한 번의 서프라이즈는 우연일 수 있다. 「5분기 중 4번 상회」는 습관이고,
	// 그게 다음 분기를 볼 때 쓰는 정보다.
	view.HitHistory = model.ComputeHitHistory(view.Announcements, earningsHitHistoryQuarters)
	// 발표 캘린더에 「다음 발표 추정」을 얹는다. 국내에는 예정일 소스가 없어서
	// 이게 없으면 이 탭은 앞일을 한 번도 말하지 못한다.
	if assetType == model.AssetDomestic {
		if next, ok := nextEstimatedEvent(view.Announcements); ok {
			view.Calendar = append([]model.EarningsCalendarEvent{next}, view.Calendar...)
		}
	}

	if view.Calendar == nil {
		view.Calendar = []model.EarningsCalendarEvent{}
	}
	if view.Quarterly == nil {
		view.Quarterly = []model.EarningsQuarter{}
	}
	if view.Announcements == nil {
		view.Announcements = []model.EarningsEvent{}
	}
	return view, nil
}

// earningsHitHistoryQuarters — 적중 이력을 몇 분기까지 볼지.
//
// 8분기(2년)는 FnGuide 이력 깊이와 맞고, 경기 순환을 한 바퀴 담는다. 전부 보면
// 종목마다 표본 수가 달라져 「4/5」와 「7/10」을 나란히 놓고 비교하게 된다.
const earningsHitHistoryQuarters = 8

// announcements reads this symbol's stored announcements, newest first.
//
// 캘린더 목록과 **같은 가공**을 거친다 — 연결/개별·해외 회계연도 접기, 어제보다
// 오래된 해외 「예정」 숨김, 발표 후 주가 반응. 여기서 빠뜨리면 같은 발표가
// 캘린더에서는 한 줄인데 종목 탭에서는 두 줄이 되고, 지난 예정이 탭에만 남는다.
func (s *EarningsTabService) announcements(ctx context.Context, symbol string) []model.EarningsEvent {
	if s.events == nil {
		return nil
	}
	bySymbol, err := s.events.ListForSymbols(ctx, []string{symbol})
	if err != nil {
		return nil
	}
	events := model.CollapseCalendarEvents(bySymbol[symbol])
	if s.reactions != nil {
		s.reactions.AttachReactions(ctx, events)
	}
	model.ApplyOverseasDisplayDates(events)
	return events
}

// latestConsensus surfaces the newest announcement that actually has a
// comparison. 가장 최근 발표에 컨센서스가 없더라도 그 이전 분기에 있으면 그것을
// 보여준다 — "없다"고 말하는 것보다 "지난 분기는 이랬다"가 낫다.
func latestConsensus(events []model.EarningsEvent) model.EarningsConsensus {
	for _, ev := range events {
		if ev.Surprise == nil {
			continue
		}
		return model.EarningsConsensus{
			Available: true,
			Period:    fmt.Sprintf("%d %s", ev.FiscalYear, ev.FiscalPeriod),
			Surprise:  ev.Surprise,
		}
	}
	return model.EarningsConsensus{Available: false, Message: noConsensusMessage}
}

// nextEstimatedEvent guesses when the next domestic report lands, from the same
// fiscal period one year earlier.
//
// 국내에는 실적발표 예정일을 주는 소스가 없다(FnGuide 캘린더도 미래 달은 빈
// 배열). 그래서 이 추정이 이 탭의 유일한 앞일이다. 근거를 같이 실어 화면이
// 「추정」임을 밝힐 수 있게 한다.
func nextEstimatedEvent(events []model.EarningsEvent) (model.EarningsCalendarEvent, bool) {
	if len(events) == 0 {
		return model.EarningsCalendarEvent{}, false
	}
	// events는 발표일 내림차순. 가장 최근 발표의 **1년 전 같은 분기**가 아니라,
	// 「아직 발표되지 않은 다음 분기」의 작년 짝을 찾아야 한다.
	reported := make(map[string]struct{}, len(events))
	for _, ev := range events {
		reported[fmt.Sprintf("%d|%s", ev.FiscalYear, ev.FiscalPeriod)] = struct{}{}
	}

	latest := events[0]
	nextYear, nextPeriod := followingPeriod(latest.FiscalYear, latest.FiscalPeriod)
	if _, done := reported[fmt.Sprintf("%d|%s", nextYear, nextPeriod)]; done {
		return model.EarningsCalendarEvent{}, false
	}

	// 그 분기의 작년 짝 = (nextYear-1, nextPeriod).
	for _, ev := range events {
		if ev.FiscalYear != nextYear-1 || ev.FiscalPeriod != nextPeriod {
			continue
		}
		prior, err := time.Parse("2006-01-02", ev.Date)
		if err != nil {
			continue
		}
		guess, basis := EstimateNextAnnouncement(prior, nextPeriod)
		return model.EarningsCalendarEvent{
			Date:          guess.Format("2006-01-02"),
			Label:         fmt.Sprintf("%d %s 실적발표", nextYear, nextPeriod),
			Source:        "fnguide",
			Kind:          "upcoming",
			DateStatus:    model.EarningsDateEstimated,
			EstimateBasis: basis,
		}, true
	}
	return model.EarningsCalendarEvent{}, false
}

// followingPeriod steps a fiscal period forward: 1Q→2Q→3Q→4Q→FY→(다음해)1Q.
//
// 4Q 다음에 FY가 오는 것은 국내 관행이다 — 4분기 잠정실적과 연간 결산이 따로
// 공시되고, 실측에서도 2월 발표분에 `4Q` 44건과 `D`(연간) 480건이 함께 있었다.
func followingPeriod(year int, period string) (int, string) {
	switch period {
	case "1Q":
		return year, "2Q"
	case "2Q":
		return year, "3Q"
	case "3Q":
		return year, "4Q"
	case "4Q":
		return year, "FY"
	default: // "FY"
		return year + 1, "1Q"
	}
}

// domesticCalendar lists this symbol's earnings-related DART filings from the
// stored sweep.
//
// 예전에는 요청마다 DART list.json을 종목별로 불렀다. 이제 공시는 시장 전체
// 일별 스윕이 표에 담아 두므로, 여기서는 그 표를 실적 키워드로 거른다 —
// 거르는 규칙(matchesEarningsKeyword)은 예전과 같은 것을 그대로 쓴다.
func (s *EarningsTabService) domesticCalendar(ctx context.Context, symbol string) []model.EarningsCalendarEvent {
	if s.disclosures == nil {
		return nil
	}
	rows, err := s.disclosures.ListRecentForSymbol(ctx, symbol, 365, 100)
	if err != nil {
		return nil
	}
	out := make([]model.EarningsCalendarEvent, 0, 8)
	for _, row := range rows {
		if !matchesEarningsKeyword(row.Title) {
			continue
		}
		out = append(out, model.EarningsCalendarEvent{
			Date:   row.ReceiptDate,
			Label:  row.Title,
			Source: "dart",
			Kind:   "filed",
		})
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func (s *EarningsTabService) domesticQuarterly(ctx context.Context, symbol string) []model.EarningsQuarter {
	// 국내 실적 탭은 **누적**을 본다 — DART가 「반기」·「연간」으로 보고하는 값
	// 그대로다. 상세 모달이 쓰는 분기값과 다른 축이라 basis로 갈라 저장한다.
	return s.storedQuarters(ctx, symbol, FinancialBasisCumulative)
}

// usCalendar reads the stored schedule rather than calling Finnhub.
//
// 예전에는 `marketStore.EnsureEarningsForSymbols`를 불러 **조회 중에 Finnhub을**
// 때렸다(캐시 미스 시). 이제 배치가 같은 데이터를 `earnings_events`에 넣어 두므로
// 표에서 읽는다 — 조회 경로에서 외부 HTTP를 하지 않는다는 규칙(AGENT_STATE 응답
// 속도 원칙 #2)에 맞고, `hour`(장전·장후)와 컨센서스도 함께 온다.
//
// 배치가 아직 한 번도 안 돌았을 때만 예전 경로로 되돌아간다.
func (s *EarningsTabService) usCalendar(ctx context.Context, symbol string) []model.EarningsCalendarEvent {
	sym := strings.ToUpper(symbol)
	if s.events != nil {
		bySymbol, err := s.events.ListForSymbols(ctx, []string{sym})
		if err == nil {
			rows := model.CollapseCalendarEvents(bySymbol[sym])
			model.ApplyOverseasDisplayDates(rows)
			out := make([]model.EarningsCalendarEvent, 0, len(rows))
			for _, ev := range rows {
				if ev.DateStatus != model.EarningsDateScheduled {
					continue
				}
				out = append(out, model.EarningsCalendarEvent{
					Date:       ev.Date,
					Label:      fmt.Sprintf("%d %s 실적발표", ev.FiscalYear, ev.FiscalPeriod),
					Source:     ev.Source,
					Kind:       "upcoming",
					DateStatus: ev.DateStatus,
					Session:    ev.Session,
				})
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	// 폴백을 두지 않는다. 예전에는 여기서 marketStore.EnsureEarningsForSymbols가
	// Finnhub을 직접 불렀는데, 그게 "조회 경로에서 외부 HTTP 금지" 규칙을 어기는
	// 마지막 한 줄이었다 — 배치가 아직 안 돌았으면 캘린더가 비는 편이 맞다.
	return nil
}

func (s *EarningsTabService) usQuarterly(ctx context.Context, symbol string) []model.EarningsQuarter {
	return s.storedQuarters(ctx, symbol, FinancialBasisQuarter)
}

// storedQuarters reads 분기 실적 from `financial_periods`, newest first.
//
// 예전에는 이 두 함수가 각각 DART/EDGAR를 직접 불렀다. 이제 같은 표를 읽으므로
// 재무제표 상세 모달과 숫자가 어긋날 수 없다.
func (s *EarningsTabService) storedQuarters(ctx context.Context, symbol, basis string) []model.EarningsQuarter {
	if s.periods == nil {
		return nil
	}
	rows, err := s.periods.ListForSymbol(ctx, symbol, basis, 12)
	if err != nil || len(rows) == 0 {
		return nil
	}
	out := make([]model.EarningsQuarter, 0, len(rows))
	// 표는 오래된 순으로 오고 화면은 최신 순을 쓴다.
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		out = append(out, model.EarningsQuarter{
			FiscalYear:      row.Period.FiscalYear,
			Period:          row.PeriodLabel,
			ReportCode:      row.ReportCode,
			PeriodEnd:       row.Period.PeriodEnd,
			Revenue:         row.Period.Revenue,
			OperatingIncome: row.Period.OperatingIncome,
			NetIncome:       row.Period.NetIncome,
			EPSBasic:        row.Period.EPSBasic,
		})
	}
	return out
}

func snapshotsToQuarters(snaps []model.FinancialSnapshot) []model.EarningsQuarter {
	out := make([]model.EarningsQuarter, 0, len(snaps))
	for _, snap := range snaps {
		out = append(out, model.EarningsQuarter{
			FiscalYear:      snap.FiscalYear,
			Period:          snap.FiscalPeriod,
			ReportCode:      snap.ReportCode,
			PeriodEnd:       snap.PeriodEnd,
			Revenue:         snap.Revenue,
			OperatingIncome: snap.OperatingIncome,
			NetIncome:       snap.NetIncome,
			EPSBasic:        snap.EPSBasic,
		})
	}
	return out
}
