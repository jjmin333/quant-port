package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// InsiderIngestService fills `insider_transactions` from DART (국내) and SEC
// EDGAR (미국).
//
// 조회 경로와 완전히 분리해 둔다 — 이 서비스는 워커 프로세스에서만 돌고, API
// 프로세스는 같은 표를 읽기만 한다. 이 분리가 이 기능을 다시 만든 이유 그
// 자체다: 예전에는 종목 탭을 누르는 사용자가 곧 EDGAR 호출자였고, 그래서
// 사용자가 늘면 데이터가 사라졌다.
type InsiderIngestService struct {
	repo  *repository.InsiderRepo
	sync  *repository.SyncStateRepo
	dart  *DartClient
	edgar *EdgarClient
	// dartGate — 다른 DART 배치(재무·공시)와 공유하는 직렬화 게이트.
	// 겹칠 때 서로를 실패시키는 대신 줄을 서게 한다(reference_ingest.go 참조).
	dartGate *providerGate
}

func NewInsiderIngestService(
	repo *repository.InsiderRepo,
	syncState *repository.SyncStateRepo,
	dart *DartClient,
	edgar *EdgarClient,
	dartGate *providerGate,
) *InsiderIngestService {
	return &InsiderIngestService{repo: repo, sync: syncState, dart: dart, edgar: edgar, dartGate: dartGate}
}

const (
	// insiderTrackedStaleAfter — 관심·보유 종목을 다시 받기까지의 간격.
	// 이보다 최근에 받았으면 그 종목은 우선순위에서 내려가고, 한 번도 안 받은
	// 종목이 그 자리를 가져간다.
	//
	// 하루인 이유: Form 4는 거래일로부터 2영업일 안에 제출되고 DART 소유보고도
	// 일 단위다. 하루보다 자주 받아도 새 데이터가 없다.
	insiderTrackedStaleAfter = 24 * time.Hour

	// 한 번 실행이 도는 종목 수의 기본값. 미국 쪽이 큰 것은 EDGAR 전용 버킷이
	// 분당 200회를 허용하기 때문이다 — 증분 실행에서 종목당 평균 1~2호출이면
	// 1,200종목은 약 20~30분이다. 국내는 DART 종목당 정확히 1호출이라 전 종목
	// (약 2,800)을 한 번에 돌 수 있지만, 다른 DART 배치와의 여유를 위해 절반쯤에
	// 둔다.
	insiderDefaultLimitOverseas = 1200
	insiderDefaultLimitDomestic = 1500
)

// IngestOverseas 는 미국 종목의 Form 4를 적재한다. limit<=0이면 기본값.
func (s *InsiderIngestService) IngestOverseas(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.edgar == nil || !s.edgar.cfg.Enabled() {
		return nil, "", fmt.Errorf("insider ingest: sec edgar user agent not configured")
	}
	if limit <= 0 {
		limit = insiderDefaultLimitOverseas
	}

	// ETF/ETN은 제외한다 — 내부자 신고 의무가 있는 발행 법인이 아니다.
	symbols, err := s.sync.SelectTargets(ctx, model.SyncKindInsiderEdgar, []string{string(model.AssetUS)}, insiderTrackedStaleAfter, limit)
	if err != nil {
		return nil, "", fmt.Errorf("insider ingest: select targets: %w", err)
	}

	stats := newIngestStats()
	for _, symbol := range symbols {
		if ctx.Err() != nil {
			break
		}
		s.ingestOneOverseas(ctx, symbol, stats)
	}
	return stats.finish("insider_edgar", len(symbols))
}

func (s *InsiderIngestService) ingestOneOverseas(ctx context.Context, symbol string, stats *ingestStats) {
	known, err := s.repo.KnownFilingKeys(ctx, symbol, "edgar")
	if err != nil {
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindInsiderEdgar, Symbol: symbol,
			Status: model.SyncError, Message: "known keys: " + err.Error(),
		})
		return
	}

	res, err := s.edgar.FetchInsiderFilings(ctx, symbol, known)
	if err != nil {
		// CIK가 없다는 것은 "지금 실패했다"가 아니라 "이 소스에 이 종목이
		// 없다"이다. 우선주·유닛 티커가 여기 걸리는데, 매 순환마다 다시
		// 물어보면 그 자리를 실제로 받아야 할 종목이 잃는다.
		status := model.SyncError
		if strings.Contains(err.Error(), "cik not found") {
			status = model.SyncNotFound
		}
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindInsiderEdgar, Symbol: symbol,
			Status: status, Message: err.Error(),
		})
		return
	}

	rows := make([]model.InsiderTransactionRow, 0, 16)
	for _, filing := range res.Filings {
		for i, txn := range filing.Transactions {
			rows = append(rows, model.InsiderTransactionRow{
				Symbol:    symbol,
				FilingKey: filing.Key,
				LineNo:    i,
				Txn:       txn,
			})
		}
	}

	written := 0
	if len(rows) > 0 {
		if written, err = s.repo.Upsert(ctx, rows); err != nil {
			stats.record(ctx, s.sync, repository.SyncOutcome{
				Kind: model.SyncKindInsiderEdgar, Symbol: symbol,
				Status: model.SyncError, Message: err.Error(),
			})
			return
		}
	}

	// 'empty'는 "받았는데 없다"이고, 이미 다 가지고 있어서 새로 받을 게 없던
	// 경우(Skipped>0)도 성공이다 — 둘 다 다음 순환에서 서두를 이유가 없다.
	status := model.SyncEmpty
	if written > 0 || res.Skipped > 0 {
		status = model.SyncOK
	}
	stats.record(ctx, s.sync, repository.SyncOutcome{
		Kind: model.SyncKindInsiderEdgar, Symbol: symbol,
		Status: status, RowsStored: written, ItemsSeen: res.FormsSeen,
	})
}

// IngestDomestic 는 국내 종목의 DART 임원ㆍ주요주주 소유보고를 적재한다.
func (s *InsiderIngestService) IngestDomestic(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.dart == nil || !s.dart.cfg.Enabled() {
		return nil, "", fmt.Errorf("insider ingest: dart api key not configured")
	}
	if limit <= 0 {
		limit = insiderDefaultLimitDomestic
	}

	symbols, err := s.sync.SelectTargets(ctx, model.SyncKindInsiderDART, []string{string(model.AssetDomestic)}, insiderTrackedStaleAfter, limit)
	if err != nil {
		return nil, "", fmt.Errorf("insider ingest: select targets: %w", err)
	}

	stats := newIngestStats()
	for _, symbol := range symbols {
		if ctx.Err() != nil {
			break
		}
		s.ingestOneDomestic(ctx, symbol, stats)
	}
	return stats.finish("insider_dart", len(symbols))
}

func (s *InsiderIngestService) ingestOneDomestic(ctx context.Context, symbol string, stats *ingestStats) {
	// DART 게이트가 간격(1초)과 다른 DART 배치와의 순서를 함께 맡는다.
	release := s.dartGate.enter(ctx)
	rows, err := s.dart.FetchInsiderRows(ctx, symbol)
	release()
	if err != nil {
		status := model.SyncError
		if strings.Contains(err.Error(), "corp_code not found") {
			status = model.SyncNotFound
		}
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindInsiderDART, Symbol: symbol,
			Status: status, Message: err.Error(),
		})
		return
	}

	// elestock은 한 종목의 이력을 통째로 준다. 접수번호가 없어 해시를 키로 쓰는데,
	// 같은 해시가 한 응답 안에 두 번 나오면(내용이 완전히 같은 두 행) 한 INSERT
	// 문 안에서 유니크 키가 충돌한다 — line_no로 갈라 준다.
	seen := map[string]int{}
	for i := range rows {
		n := seen[rows[i].FilingKey]
		rows[i].LineNo = n
		rows[i].Symbol = symbol
		seen[rows[i].FilingKey] = n + 1
	}

	written := 0
	if len(rows) > 0 {
		if written, err = s.repo.Upsert(ctx, rows); err != nil {
			stats.record(ctx, s.sync, repository.SyncOutcome{
				Kind: model.SyncKindInsiderDART, Symbol: symbol,
				Status: model.SyncError, Message: err.Error(),
			})
			return
		}
	}

	status := model.SyncEmpty
	if written > 0 {
		status = model.SyncOK
	}
	stats.record(ctx, s.sync, repository.SyncOutcome{
		Kind: model.SyncKindInsiderDART, Symbol: symbol,
		Status: status, RowsStored: written, ItemsSeen: len(rows),
	})
}
