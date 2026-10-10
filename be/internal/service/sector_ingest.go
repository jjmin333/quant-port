package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
SectorIngestService — 종목별 산업 코드를 받아 공통 섹터로 옮겨 `stock_sectors`에 둔다.

# 왜 이 배치가 필요한가

점수 피어그룹이 섹터 없이 (자산유형 × 시장 × 시총)으로만 갈려, 가치 축이 업종을
판별하는 장치가 되어 있다(`docs/STOCK_SCORE_DESIGN.md` §2.5 · `docs/COMPETITIVE_ANALYSIS.md` B1).
이 배치는 **재료만** 쌓는다 — 피어그룹을 실제로 바꾸는 것은 커버리지를 잰 뒤의 별도 결정이다.

# 주기

섹터는 거의 바뀌지 않는다. 로테이션(`SelectTargets`)으로 대상을 고르되 `sectorMaxAge`
안에 받은 종목은 건너뛴다 — 첫 바퀴가 끝나면 하루 호출은 신규 상장과 90일 지난 몇 십
종목뿐이다.

	국내  DART company.json  종목당 1호출 · 1초 간격 · 기본 1,000종목(≈17분)
	미국  SEC submissions     종목당 1호출 · 0.3초 간격 · 기본 3,000종목(≈15분)

국내 첫 바퀴는 사흘, 미국은 이틀쯤 걸린다. DART 한도(일 18,000)는 재무 배치가 가장 많이
쓰므로 국내 쪽을 작게 잡았다.
*/
type SectorIngestService struct {
	repo     *repository.StockSectorRepo
	sync     *repository.SyncStateRepo
	dart     *DartClient
	edgar    *EdgarClient
	dartGate *providerGate
}

func NewSectorIngestService(
	repo *repository.StockSectorRepo,
	syncState *repository.SyncStateRepo,
	dart *DartClient,
	edgar *EdgarClient,
	dartGate *providerGate,
) *SectorIngestService {
	return &SectorIngestService{repo: repo, sync: syncState, dart: dart, edgar: edgar, dartGate: dartGate}
}

const (
	sectorMaxAge               = 90 * 24 * time.Hour
	sectorDefaultLimitDomestic = 1000
	sectorDefaultLimitOverseas = 3000
)

// Run classifies domestic then US symbols. limit<=0 uses each side's default.
func (s *SectorIngestService) Run(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil {
		return nil, "", fmt.Errorf("sector ingest not configured")
	}
	result := map[string]any{}
	var messages, failures []string

	if s.dart != nil && s.dart.cfg.Enabled() {
		n := limit
		if n <= 0 {
			n = sectorDefaultLimitDomestic
		}
		res, msg, err := s.runSide(ctx, model.SyncKindSectorDART, model.AssetDomestic, n, s.classifyDomestic)
		result["domestic"] = res
		messages = append(messages, msg)
		if err != nil {
			failures = append(failures, err.Error())
		}
	} else {
		messages = append(messages, "국내: DART 키 없음 — 건너뜀")
	}

	if s.edgar != nil && s.edgar.cfg.Enabled() && ctx.Err() == nil {
		n := limit
		if n <= 0 {
			n = sectorDefaultLimitOverseas
		}
		res, msg, err := s.runSide(ctx, model.SyncKindSectorSEC, model.AssetUS, n, s.classifyOverseas)
		result["us"] = res
		messages = append(messages, msg)
		if err != nil {
			failures = append(failures, err.Error())
		}
	} else if ctx.Err() == nil {
		messages = append(messages, "미국: SEC User-Agent 없음 — 건너뜀")
	}

	message := strings.Join(messages, " · ")
	// 한쪽만 통째로 실패하면 잡은 성공이다 — 다른 쪽 결과가 원장에 남아야 한다.
	// 둘 다 실패했을 때만 잡의 문제로 올린다.
	if len(failures) == 2 {
		return result, message, fmt.Errorf("sector ingest: %s", strings.Join(failures, "; "))
	}
	return result, message, nil
}

type sectorClassifier func(ctx context.Context, symbol string) (repository.StockSectorRow, error)

func (s *SectorIngestService) runSide(
	ctx context.Context, kind string, asset model.QuoteAssetType, limit int, classify sectorClassifier,
) (map[string]any, string, error) {
	symbols, err := s.sync.SelectTargets(ctx, kind, []string{string(asset)}, sectorMaxAge, limit)
	if err != nil {
		return nil, "", fmt.Errorf("%s: select targets: %w", kind, err)
	}
	fresh, err := s.repo.FreshSymbols(ctx, symbols, sectorMaxAge)
	if err != nil {
		return nil, "", fmt.Errorf("%s: fresh: %w", kind, err)
	}
	todo := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		if !fresh[sym] {
			todo = append(todo, sym)
		}
	}

	stats := newIngestStats()
	for _, symbol := range todo {
		if ctx.Err() != nil {
			break
		}
		row, err := classify(ctx, symbol)
		if err != nil {
			status := model.SyncError
			msg := err.Error()
			if strings.Contains(msg, "corp_code not found") || strings.Contains(msg, "cik not found") {
				status = model.SyncNotFound
			}
			stats.record(ctx, s.sync, repository.SyncOutcome{Kind: kind, Symbol: symbol, Status: status, Message: msg})
			continue
		}
		if err := s.repo.Upsert(ctx, row); err != nil {
			stats.record(ctx, s.sync, repository.SyncOutcome{Kind: kind, Symbol: symbol, Status: model.SyncError, Message: err.Error()})
			continue
		}
		// 코드가 비었거나 옮길 수 없으면 empty — 받았지만 섹터를 못 정했다.
		status := model.SyncOK
		stored := 1
		if row.Sector == "" {
			status, stored = model.SyncEmpty, 0
		}
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: kind, Symbol: symbol, Status: status, RowsStored: stored, ItemsSeen: 1,
		})
	}
	res, msg, err := stats.finish(kind, len(todo))
	if res != nil {
		res["skippedFresh"] = len(symbols) - len(todo)
	}
	return res, msg, err
}

func (s *SectorIngestService) classifyDomestic(ctx context.Context, symbol string) (repository.StockSectorRow, error) {
	release := s.dartGate.enter(ctx)
	code, name, err := s.dart.FetchIndustryCode(ctx, symbol)
	release()
	if err != nil {
		return repository.StockSectorRow{}, err
	}
	// DART는 산업 이름을 주지 않으므로 industry_name에 종목명을 남긴다 — 지주회사를
	// 이름으로 가른 판정(SectorFromKSICNamed)을 나중에 표만 보고도 되짚을 수 있다.
	return repository.StockSectorRow{
		Symbol: symbol, Source: "dart", IndustryCode: code, IndustryName: name,
		Sector: string(model.SectorFromKSICNamed(code, name)),
	}, nil
}

func (s *SectorIngestService) classifyOverseas(ctx context.Context, symbol string) (repository.StockSectorRow, error) {
	code, name, err := s.edgar.FetchSIC(ctx, symbol)
	if err != nil {
		return repository.StockSectorRow{}, err
	}
	return repository.StockSectorRow{
		Symbol: symbol, Source: "sec", IndustryCode: code, IndustryName: name,
		Sector: string(model.SectorFromSIC(code)),
	}, nil
}
