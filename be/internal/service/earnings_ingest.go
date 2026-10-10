package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/fnguide"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// EarningsIngestService fills `earnings_events` from the two upstream sources.
//
// 조회 경로와 완전히 분리해 둔다 — 이 서비스는 워커 프로세스에서만 돌고, API
// 프로세스는 같은 표를 읽기만 한다. FnGuide가 느려지거나 Finnhub이 429를 뱉는
// 동안 사용자 요청을 처리하는 프로세스가 함께 멈추면 안 된다.
type EarningsIngestService struct {
	events  *repository.EarningsEventRepo
	stocks  *repository.StockRepo
	fnguide *fnguide.Client
	finnhub *EarningsClient
}

func NewEarningsIngestService(
	events *repository.EarningsEventRepo,
	stocks *repository.StockRepo,
	fn *fnguide.Client,
	finnhub *EarningsClient,
) *EarningsIngestService {
	return &EarningsIngestService{events: events, stocks: stocks, fnguide: fn, finnhub: finnhub}
}

// IngestDomestic pulls `months` calendar months ending with the current one.
//
// 기본이 2개월인 이유는 **정정 공시**다. FnGuide는 정정을 원 발표와 별 행으로
// 주는데(삼성 2Q: 07/07 원본 + 07/30 정정), 정정이 다음 달에 오면 당월만 받는
// 배치는 그것을 영원히 못 본다. 두 달을 겹쳐 받으면 그 창이 닫힌다.
//
// months를 크게 주면 백필이다. FnGuide 이력은 약 2년이라(202408 있음, 202308
// 없음) 24를 넘겨도 더 얻을 게 없다.
func (s *EarningsIngestService) IngestDomestic(ctx context.Context, months int) (int, error) {
	if s == nil || s.fnguide == nil || !s.fnguide.Enabled() {
		return 0, fmt.Errorf("earnings ingest: fnguide disabled")
	}
	if months <= 0 {
		months = 2
	}

	known, err := s.stocks.ExistingSymbolSet(ctx)
	if err != nil {
		return 0, fmt.Errorf("earnings ingest: symbol set: %w", err)
	}

	// KST 기준 "이번 달". 컨테이너 시간대(UTC)로 잡으면 매월 1일 09시 이전에
	// 지난달을 이번 달로 착각한다.
	nowKST := time.Now().In(model.KST)
	total := 0
	var firstErr error
	skipped := 0

	// 달 사이의 간격. FnGuide는 공개 API가 아니라 스스로 낮게 잡은 30초 간격을
	// 쓰는데, 그건 "기다리면 풀리는" 제약이므로 실패로 다루지 않고 **기다린다**.
	// 처음 구현에서는 안 기다렸고, 15개월 백필이 첫 달만 넣고 14개월을 잃었다.
	pace := s.fnguide.MinInterval()

	for i := months - 1; i >= 0; i-- {
		if i < months-1 && pace > 0 {
			select {
			case <-ctx.Done():
				return total, ctx.Err()
			case <-time.After(pace):
			}
		}
		month := nowKST.AddDate(0, -i, 0)
		rows, err := s.fnguide.FetchPreliminary(ctx, month.Year(), month.Month())
		if err != nil {
			// 한 달이 실패해도 나머지 달은 넣는다. 전체를 되돌리면 정정 하나 때문에
			// 이번 시즌 발표가 통째로 안 들어온다.
			log.Printf("earnings ingest (domestic %04d-%02d): %v", month.Year(), int(month.Month()), err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		upserts := make([]model.EarningsEventUpsert, 0, len(rows))
		for _, row := range rows {
			if _, ok := known[row.Symbol]; !ok {
				// 카탈로그에 없는 종목은 이름·로고·시세·모달을 그릴 수 없다.
				// 캘린더에 클릭도 안 되는 줄을 남기는 것보다 빼는 쪽이 낫다.
				skipped++
				continue
			}
			upserts = append(upserts, domesticUpsert(row))
		}
		n, err := s.events.Upsert(ctx, upserts)
		if err != nil {
			log.Printf("earnings ingest (domestic %04d-%02d upsert): %v", month.Year(), int(month.Month()), err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		total += n
	}

	if skipped > 0 {
		log.Printf("earnings ingest (domestic): skipped %d rows with no catalog entry", skipped)
	}
	// 한 달이라도 들어갔으면 성공으로 본다 — 부분 성공을 실패로 기록하면
	// 관리자 화면이 "안 됨"이라고 말하는데 데이터는 늘어 있는 상태가 된다.
	if total == 0 && firstErr != nil {
		return 0, firstErr
	}
	return total, nil
}

func domesticUpsert(row fnguide.Preliminary) model.EarningsEventUpsert {
	var periodEnd *time.Time
	if !row.PeriodEnd.IsZero() {
		pe := row.PeriodEnd
		periodEnd = &pe
	}
	return model.EarningsEventUpsert{
		Symbol: row.Symbol,
		Market: string(model.AssetDomestic),
		// 국내 소스는 이미 발표된 것만 준다 — 미래 달은 빈 배열이다.
		EventDate:    row.AnnouncedAt,
		DateStatus:   model.EarningsDateConfirmed,
		FiscalYear:   row.FiscalYear,
		FiscalPeriod: row.FiscalPeriod,
		PeriodEnd:    periodEnd,
		Basis:        row.Basis,

		RevenueEst:   row.RevenueEst,
		RevenueAct:   row.RevenueAct,
		OpIncomeEst:  row.OpIncomeEst,
		OpIncomeAct:  row.OpIncomeAct,
		NetIncomeEst: row.NetIncomeEst,
		NetIncomeAct: row.NetIncomeAct,

		RevenueQoQ:   row.RevenueQoQ,
		OpIncomeQoQ:  row.OpIncomeQoQ,
		NetIncomeQoQ: row.NetIncomeQoQ,

		Currency:   "KRW",
		Source:     "fnguide",
		SourceDate: row.AnnouncedAt,
		Revised:    row.Revised,
	}
}

// IngestOverseas pulls the US calendar for a window around today.
//
// 06:00 전체 창은 일정 정정과 먼 미래를 위한 것이다. 기본 −49/+76(7일 조각
// 18회). 발표 직후 실적치는 IngestOverseasLive가 5분마다 어제~오늘만 받는다.
func (s *EarningsIngestService) IngestOverseas(ctx context.Context, backDays, forwardDays int) (int, error) {
	if backDays <= 0 {
		backDays = OverseasFullBackDays
	}
	if forwardDays <= 0 {
		forwardDays = OverseasFullForwardDays
	}
	nowKST := time.Now().In(model.KST)
	return s.ingestOverseasRange(ctx, nowKST.AddDate(0, 0, -backDays), nowKST.AddDate(0, 0, forwardDays))
}

// IngestOverseasLive pulls only KST yesterday..today (1 HTTP).
//
// 창이 어제인 이유: Finnhub 날짜는 미국 세션일이다. KST 05시는 아직 미국
// 어제라, 오늘만 받으면 막 나온 장후가 빠진다. 이 잡이 「어제 이전 예정」을
// 지우는 일은 없다 — 그건 조회 DropStaleOverseasScheduled의 몫이다. 라이브
// 창 밖의 행은 06:00 전체 창이 계속 적재하고, 실적치가 나중에 오면 확정으로
// 갱신돼야 하므로 DB에서 지우면 안 된다.
func (s *EarningsIngestService) IngestOverseasLive(ctx context.Context) (int, error) {
	from, to := OverseasLiveWindow(time.Now())
	return s.ingestOverseasRange(ctx, from, to)
}

func (s *EarningsIngestService) ingestOverseasRange(ctx context.Context, from, to time.Time) (int, error) {
	if s == nil || s.finnhub == nil || !s.finnhub.cfg.Enabled() {
		return 0, fmt.Errorf("earnings ingest: finnhub disabled")
	}

	known, err := s.stocks.ExistingSymbolSet(ctx)
	if err != nil {
		return 0, fmt.Errorf("earnings ingest: symbol set: %w", err)
	}

	nowKST := time.Now().In(model.KST)
	// 반드시 조각내어 받는다 — 한 번에 넓게 부르면 1,500행 상한에 걸려 구간의
	// 앞부분이 조용히 사라진다(첫 실행이 4개월 창에서 10월 3주치만 담았다).
	rows, err := s.finnhub.FetchCalendarChunked(ctx, from, to)
	if err != nil {
		return 0, err
	}

	today := nowKST.Format("2006-01-02")
	upserts := make([]model.EarningsEventUpsert, 0, len(rows))
	skipped := 0
	for _, row := range rows {
		if _, ok := known[row.Symbol]; !ok {
			// Finnhub은 미국 전 상장을 준다(2주에 1,446행). 우리 카탈로그에 없는
			// 것을 다 넣으면 표가 부풀고, 화면은 그 행을 그릴 수 없다.
			skipped++
			continue
		}
		u, ok := overseasUpsert(row, today)
		if !ok {
			continue
		}
		upserts = append(upserts, u)
	}

	n, err := s.events.Upsert(ctx, upserts)
	if err != nil {
		return 0, err
	}
	if skipped > 0 {
		log.Printf("earnings ingest (overseas): skipped %d rows with no catalog entry", skipped)
	}
	return n, nil
}

func overseasUpsert(row FinnhubEarningsRow, today string) (model.EarningsEventUpsert, bool) {
	date, err := time.ParseInLocation("2006-01-02", row.Date, time.UTC)
	if err != nil {
		return model.EarningsEventUpsert{}, false
	}
	period := "FY"
	if row.Quarter >= 1 && row.Quarter <= 4 {
		period = fmt.Sprintf("%dQ", row.Quarter)
	}
	year := row.Year
	if year <= 0 {
		year = date.Year()
	}

	// **발표됨인지 예정인지는 날짜가 아니라 실적 유무로 가른다.** 날짜로 판정하면
	// 어제 날짜인데 아직 실적이 안 들어온 행이 "발표됨"이 되어, 서프라이즈 칸이
	// 빈 채로 확정 배지를 달게 된다. 실적이 있으면 발표된 것이다.
	//
	// 실적 없는 지난 날짜를 여기서 버리지 않는다. Finnhub는 소형·중형도 숫자를
	// 안 주는 경우가 많고(실측 8/12 예정 140 vs 발표 131, 8/19 37 vs 26),
	// 나중에 채우면 같은 unique key로 확정이 된다. 화면에서 빼는 것은 조회
	// 경로 DropStaleOverseasScheduled — KST 어제보다 오래된 「예정」만.
	status := model.EarningsDateScheduled
	if row.EPSActual != nil || row.RevenueActual != nil {
		status = model.EarningsDateConfirmed
	}

	// source_date — Finnhub은 회차 개념이 없다. 오늘을 쓰면 매일 최신이 되어
	// 정정 병합 규칙(더 큰 source_date만 덮기)이 항상 통과하는데, 그게 맞다:
	// 이 소스는 같은 행을 갱신해 주는 방식이라 최신 응답이 언제나 옳다.
	sourceDate, err := time.ParseInLocation("2006-01-02", today, time.UTC)
	if err != nil {
		sourceDate = date
	}

	return model.EarningsEventUpsert{
		Symbol:       row.Symbol,
		Market:       string(model.AssetUS),
		EventDate:    date,
		DateStatus:   status,
		Session:      row.Hour,
		FiscalYear:   year,
		FiscalPeriod: period,
		Basis:        "",

		RevenueEst: row.RevenueEstimate,
		RevenueAct: row.RevenueActual,
		EPSEst:     row.EPSEstimate,
		EPSAct:     row.EPSActual,

		Currency:   "USD",
		Source:     "finnhub",
		SourceDate: sourceDate,
	}, true
}

// NormalizeEarningsSymbols cleans a comma-separated symbol filter.
func NormalizeEarningsSymbols(raw string, max int) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		s := strings.ToUpper(strings.TrimSpace(p))
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

// SnapshotEstimates records today's US consensus EPS where it changed (EPS 리비전 재료).
func (s *EarningsIngestService) SnapshotEstimates(ctx context.Context) (int64, error) {
	if s == nil || s.events == nil {
		return 0, nil
	}
	return s.events.SnapshotEstimates(ctx, time.Now().In(model.KST).Format("2006-01-02"))
}
