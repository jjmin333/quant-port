package service

import (
	"context"
	"fmt"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

const (
	// insiderTabMaxRows — 화면이 보여주는 최근 건수.
	insiderTabMaxRows = 30

	insiderNoteKR = "국내 종목은 DART 임원ㆍ주요주주 소유보고를 기반으로 최근 %d건을 표시합니다."
	insiderNoteUS = "미국 종목은 SEC Form 4 최근 %d건의 보통주 거래 내역을 표시합니다(옵션ㆍRSU 등 파생 거래는 제외)."

	// 아직 배치가 이 종목까지 오지 않았을 때의 안내. "내역 없음"과 구분해야
	// 한다 — 사용자에게는 똑같이 빈 표지만, 하나는 사실이고 하나는 대기 상태다.
	insiderNotePending = "아직 수집되지 않은 종목입니다. 배치가 순차로 수집하며, 관심 종목에 추가하면 다음 실행에서 우선 수집됩니다."
)

// InsiderTabService assembles the 내부자 거래 탭 response.
//
// 외부 API를 부르지 않는다. 예전에는 이 서비스가 요청마다 EDGAR(종목당 4호출)
// 또는 DART를 직접 불렀고, 8/min짜리 공유 버킷 때문에 동시 접속 두 명이면
// 두 번째 사용자가 빈 화면을 받았다. 이제 적재는 worker-filings가 하고
// (insider_ingest.go), 여기서는 표만 읽는다 — 사용자가 늘어도 외부 호출은
// 늘지 않는다.
type InsiderTabService struct {
	repo   *repository.InsiderRepo
	sync   *repository.SyncStateRepo
	stocks *repository.StockRepo
}

func NewInsiderTabService(
	repo *repository.InsiderRepo,
	syncState *repository.SyncStateRepo,
	stocks *repository.StockRepo,
) *InsiderTabService {
	return &InsiderTabService{repo: repo, sync: syncState, stocks: stocks}
}

// insiderSyncKind maps the tab to the batch that fills it — 국내와 해외는 서로
// 다른 배치가 채우므로, "아직 수집 전"인지도 각자에게 물어야 한다.
func insiderSyncKind(assetType model.QuoteAssetType) string {
	if assetType == model.AssetDomestic {
		return model.SyncKindInsiderDART
	}
	return model.SyncKindInsiderEdgar
}

func (s *InsiderTabService) Get(ctx context.Context, symbol string) (*model.InsiderTabView, error) {
	stock, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return nil, fmt.Errorf("insider: unknown symbol %s", symbol)
	}

	assetType := ResolveFundamentalsAssetType(stock)
	view := &model.InsiderTabView{
		Symbol:       stock.Symbol,
		AssetType:    assetType,
		Transactions: []model.InsiderTransaction{},
	}

	txns, err := s.repo.ListForSymbol(ctx, stock.Symbol, insiderTabMaxRows)
	if err != nil {
		return nil, err
	}
	if len(txns) > 0 {
		view.Transactions = txns
		if assetType == model.AssetDomestic {
			view.Note = fmt.Sprintf(insiderNoteKR, len(txns))
		} else {
			view.Note = fmt.Sprintf(insiderNoteUS, len(txns))
		}
		return view, nil
	}

	// 빈 결과의 이유를 적재 상태에서 가져온다. 조회가 외부를 부르지 않게 된
	// 대신, "아직 안 받았다"를 사용자에게 말해 줄 수 있어야 한다.
	synced, err := s.sync.WasSynced(ctx, insiderSyncKind(assetType), stock.Symbol)
	if err != nil {
		return nil, err
	}
	if !synced {
		view.Note = insiderNotePending
	}
	return view, nil
}
