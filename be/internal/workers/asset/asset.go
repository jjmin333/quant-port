// Package asset is the worker-asset role (2026-10-01 사용자 결정 — 성격별 워커): 부동산·금리·공매·금융상품의
// 공공데이터 적재와 그 점수·AI 해설. 외부 호출은 공공데이터 provider들과 자산 전용 LLM 키(llm_sub)뿐이다.
package asset

import (
	"context"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/workers/slots"
)

/*
시각(KST). 각자 다른 provider라 겹쳐도 되지만, 점수는 적재 뒤, AI는 점수 뒤에 둔다.

	03:30 실거래(국토부)   — 끝에 전 사용자 자산 재평가
	05:10 금융상품(금감원)
	06:00 부동산 지수(R-ONE)
	07:40 공매(온비드)
	09:20 금리(ECOS)
	09:50 자산 점수(DB만)  — 단지·시군구·금융상품·공매
	10:20 자산 AI(llm_sub) — 금리 한 줄·지역·단지·우대조건
*/
/*
2026-10-10 예약 중지(사업화 검토 N2): 부동산·공매·금융상품 화면이 숨겨져(fe `lib/hidden-features.ts`)
아무도 보지 않는 결과에 국토부 호출 1~1.5만/밤·DB·LLM 250콜을 쓰고 있었다. 멈춘 넷은 나중에 다시 받을 수
있다 — 실거래는 국토부가 과거 월을 그대로 주고, 점수·AI는 적재분으로 다시 계산한다. 이력이 끊기면 되찾을 수
없는 스냅숏(금융상품 공시·R-ONE 지수)과 홈 지표가 쓰는 금리는 계속 돈다. 관리자 수동 실행은 그대로 된다.
되살릴 때: 아래 주석 넷을 풀고, 실거래가 밀린 달을 따라잡는지 첫 밤 로그를 본다.
*/
var list = []slots.Slot{
	// {Job: model.BatchReTrades, Hour: 3, Minute: 30},
	{Job: model.BatchFinProducts, Hour: 5, Minute: 10},
	{Job: model.BatchReIndex, Hour: 6, Minute: 0},
	// {Job: model.BatchOnbid, Hour: 7, Minute: 40},
	{Job: model.BatchEcosRates, Hour: 9, Minute: 20},
	// {Job: model.BatchAssetScores, Hour: 9, Minute: 50},
	// {Job: model.BatchAssetAI, Hour: 10, Minute: 20},
}

// Run starts the asset role and blocks until ctx is done.
func Run(ctx context.Context, deps *service.WorkerDeps) {
	slots.Run(ctx, deps.Runs, "worker-asset", list,
		func(ctx context.Context, job model.BatchJobID, limit int) (map[string]any, string, error) {
			switch job {
			case model.BatchReTrades:
				return deps.PublicData.RunTrades(ctx, limit)
			case model.BatchFinProducts:
				// limit 은 쓰지 않는다 — 전체 공시를 받아야 사라진 상품을 숨길 수 있다.
				return deps.FinProducts.Run(ctx)
			case model.BatchReIndex:
				return deps.PublicData.RunIndex(ctx)
			case model.BatchOnbid:
				return deps.PublicData.RunOnbid(ctx, limit)
			case model.BatchEcosRates:
				return deps.PublicData.RunRates(ctx)
			case model.BatchAssetScores:
				return deps.AssetScores.Run(ctx)
			case model.BatchAssetAI:
				return deps.AssetAI.Run(ctx)
			}
			return nil, "", nil
		})
}
