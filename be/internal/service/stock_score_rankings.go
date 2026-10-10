package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
AI 추천 랭킹 — **축 여섯 개 × 기간 네 개**를 배치가 미리 계산해 둔다.

# 왜 미리 계산하나

「추세 점수 · 최근 1주 최고」 같은 질문의 답은 `stock_scores`를 기간만큼 훑어 종목별
MAX를 구한 뒤 정렬해야 나온다. 실측(개발 PC)으로 1주 창이 3.9초, 1개월 창이 0.6~1.1초이고
운영 DB는 1 ECPU라 몇 배 느리다. 요청 경로에 둘 수 없다.

점수 자체가 **하루 한 번** 만들어지는 값이므로 랭킹도 하루 한 번이면 충분하다.
`market_rankings`(마켓 레이더 스냅샷)와 같은 모양이다.

# 계산 한 판의 모양

시장마다 기간별로 **한 번씩만** DB를 훑는다:

	기간 4개 × 「종목별 여섯 측정치 MAX」 한 방  (국내 2.08초 / 해외 1.30초, 1개월 창)
	+ 상위 목록의 합집합에 대한 표시용 조회 한 방

축을 따로 물으면 같은 행을 여섯 번 훑게 되므로, 여섯 측정치를 한 번에 받아 **정렬만
Go가** 한다. 18,000행을 여섯 번 정렬하는 비용은 밀리초다.
*/

// scoreRankingTopN — 저장하는 상위 종목 수. 화면은 20~100 사이를 요청한다.
const scoreRankingTopN = 100

// ScoreRankingService computes and serves the precomputed leaderboards.
type ScoreRankingService struct {
	rankings *repository.StockScoreRankingRepo
	scores   *ScoreService
}

func NewScoreRankingService(
	rankings *repository.StockScoreRankingRepo,
	scores *ScoreService,
) *ScoreRankingService {
	return &ScoreRankingService{rankings: rankings, scores: scores}
}

// scoreRankingMarkets pairs each market tab with the asset types it covers.
// Same split as ScoreService.TopScores — ETFs group with overseas here too.
var scoreRankingMarkets = map[string][]string{
	"domestic": domesticAssetTypes,
	"overseas": overseasAssetTypes,
}

/*
Get은 저장된 목록 하나를 돌려준다.

**등락률은 읽을 때 덮어쓴다.** 순위와 점수는 배치의 결과지만 그 옆의 등락률은
「지금 이렇다」여야 하고, 그 값은 Redis에 있는 시세라 DB를 건드리지 않는다
(`overlayLiveChanges` — TopScores가 쓰는 것과 같은 함수).

배치가 아직 돌지 않았으면 nil이다. 그때 화면이 무엇을 할지는 핸들러가 정한다.
*/
func (s *ScoreRankingService) Get(
	ctx context.Context,
	market string,
	axis model.ScoreRankingAxis,
	duration model.ScoreRankingDuration,
	limit int,
) (*model.ScoreRankingView, error) {
	if s == nil || s.rankings == nil {
		return nil, nil
	}
	view, err := s.rankings.Get(ctx, normalizeRankingMarket(market), axis, duration)
	if err != nil || view == nil {
		return nil, err
	}
	if limit > 0 && limit < len(view.Items) {
		view.Items = view.Items[:limit]
	}
	if s.scores != nil {
		s.scores.overlayLiveChanges(ctx, view.Items)
	}
	return view, nil
}

func normalizeRankingMarket(market string) string {
	if strings.EqualFold(strings.TrimSpace(market), "overseas") {
		return "overseas"
	}
	return "domestic"
}

/*
ComputeAll은 48개 목록을 전부 다시 만든다. 점수 배치가 끝난 직후에 부른다.

한 시장이 실패해도 나머지는 계속한다 — 국내가 안 됐다고 해외 랭킹까지 어제 값으로
남을 이유는 없다. 실패는 로그로 남기고 마지막에 하나로 묶어 돌려준다.
*/
func (s *ScoreRankingService) ComputeAll(ctx context.Context) error {
	if s == nil || s.rankings == nil || s.scores == nil {
		return fmt.Errorf("score ranking service not configured")
	}
	tradeDate, err := s.scores.LatestScoredDate(ctx)
	if err != nil {
		return err
	}
	if tradeDate == "" {
		log.Println("score rankings: no scored date yet — nothing to compute")
		return nil
	}
	/*
		추천 목록의 거래대금 하한(`scoreRecommendableClause`)이 읽는 값을 먼저 채운다.
		점수 배치가 이미 채웠으면 0행이다. 이 규칙 전에 매긴 최신일이면 여기서 채워져,
		배포 직후 랭킹을 다시 만들 때 다음 점수 배치를 기다리지 않는다.
	*/
	if n, err := s.scores.EnsureTradeValue(ctx, tradeDate); err != nil {
		log.Printf("score rankings: avg trade value %s: %v", tradeDate, err)
	} else if n > 0 {
		log.Printf("score rankings: filled avg trade value on %d rows (%s)", n, tradeDate)
	}

	var failures []string
	for market, assetTypes := range scoreRankingMarkets {
		if err := s.computeMarket(ctx, market, assetTypes, tradeDate); err != nil {
			log.Printf("score rankings %s: %v", market, err)
			failures = append(failures, fmt.Sprintf("%s: %v", market, err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("score rankings: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (s *ScoreRankingService) computeMarket(
	ctx context.Context,
	market string,
	assetTypes []string,
	tradeDate string,
) error {
	latest, err := time.Parse("2006-01-02", tradeDate)
	if err != nil {
		return fmt.Errorf("bad trade date %q: %w", tradeDate, err)
	}

	// 기간별 상위 목록을 먼저 다 만들고(순위·랭킹값만), 표시용 정보는 그 합집합에
	// 대해 **한 번만** 조회한다.
	type pending struct {
		duration model.ScoreRankingDuration
		fromDate string
		byAxis   map[model.ScoreRankingAxis][]scoreRankedSymbol
	}

	/*
		랭킹 자격 집합을 **먼저** 받는다 (2026-09-04 사용자 요청 2).

		「최신 점수일에 축이 3개 이상이고 지수 추종 상품이 아닌 종목」이다. 상위 N을
		자르기 **전에** 걸러야 목록이 짧아지지 않는다 — 100줄을 뽑아 놓고 거르면 그중
		상당수가 사라진 채로 저장된다. 조건의 근거와 실측은 `repository/
		stock_score_universe.go` 머리글에 있다.
	*/
	eligible, err := s.rankings.RankableSymbols(ctx, tradeDate, assetTypes)
	if err != nil {
		return err
	}

	plans := make([]pending, 0, len(model.ScoreRankingDurations))
	needed := map[string]struct{}{}

	for _, duration := range model.ScoreRankingDurations {
		fromDate := latest.AddDate(0, 0, -duration.LookbackDays()).Format("2006-01-02")
		rows, err := s.rankings.WindowBests(ctx, fromDate, tradeDate, assetTypes)
		if err != nil {
			return err
		}
		byAxis := rankWindow(rows, eligible)
		for _, list := range byAxis {
			for _, r := range list {
				needed[r.symbol] = struct{}{}
			}
		}
		plans = append(plans, pending{duration: duration, fromDate: fromDate, byAxis: byAxis})
	}

	symbols := make([]string, 0, len(needed))
	for sym := range needed {
		symbols = append(symbols, sym)
	}
	display, err := s.rankings.DisplayRows(ctx, tradeDate, symbols)
	if err != nil {
		return err
	}

	for _, plan := range plans {
		for _, axis := range model.ScoreRankingAxes {
			items := buildRankingItems(plan.byAxis[axis], display, tradeDate)
			view := model.ScoreRankingView{
				Market:    market,
				Axis:      axis,
				Duration:  plan.duration,
				TradeDate: tradeDate,
				FromDate:  plan.fromDate,
				Items:     items,
			}
			if err := s.rankings.Save(ctx, view); err != nil {
				return err
			}
		}
	}
	log.Printf("score rankings %s: %d lists from %d symbols (%d rankable, as of %s)",
		market, len(plans)*len(model.ScoreRankingAxes), len(symbols), len(eligible), tradeDate)
	return nil
}

type scoreRankedSymbol struct {
	symbol string
	value  float64
}

/*
rankWindow — 한 창의 종목별 최고점에서 축마다 상위 N을 뽑는다.

**값이 없는 종목은 목록에서 뺀다.** 예를 들어 심리·수급 축은 요약 배치가 다녀간
종목에만 붙어서 최신일 기준 17,916종목 중 250종목만 값이 있다(실측 2026-09-02).
값 없는 종목을 0점으로 채워 넣으면 그 축의 목록이 「데이터 없음」 종목으로 채워진다.

동점은 심볼 순으로 갈라 **매 배치가 같은 순서**를 내게 한다. 그러지 않으면 값이
그대로인 날에도 순위가 흔들려 「어제와 오늘 랭킹이 다르다」로 보인다.

`eligible`은 랭킹 자격 집합이다(`RankableSymbols`). **상위 N을 자르기 전에** 여기서
거른다 — 자른 뒤에 거르면 목록이 짧아진다. nil이면 거르지 않는다.
*/
func rankWindow(
	rows []repository.ScoreWindowRow,
	eligible map[string]struct{},
) map[model.ScoreRankingAxis][]scoreRankedSymbol {
	out := make(map[model.ScoreRankingAxis][]scoreRankedSymbol, len(model.ScoreRankingAxes))

	for _, rankAxis := range model.ScoreRankingAxes {
		list := make([]scoreRankedSymbol, 0, len(rows))
		axis, isAxis := rankAxis.Axis()
		for _, row := range rows {
			if eligible != nil {
				if _, ok := eligible[row.Symbol]; !ok {
					continue
				}
			}
			value := row.Composite
			if isAxis {
				value = row.Axes[axis]
			}
			if !value.Valid {
				continue
			}
			list = append(list, scoreRankedSymbol{symbol: row.Symbol, value: value.Float64})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].value != list[j].value {
				return list[i].value > list[j].value
			}
			return list[i].symbol < list[j].symbol
		})
		if len(list) > scoreRankingTopN {
			list = list[:scoreRankingTopN]
		}
		out[rankAxis] = list
	}
	return out
}

/*
buildRankingItems — 순위 목록에 표시용 정보를 붙인다.

표시 행이 없는 종목은 **버린다**(순위를 비워 두지 않고 다음 종목이 당겨진다).
그런 종목은 최신 점수일에 행이 없는 경우인데, 창 안의 옛 날짜에서만 점수가 있었다는
뜻이라 지금 추천 목록에 올릴 종목이 아니다.
*/
func buildRankingItems(
	ranked []scoreRankedSymbol,
	display map[string]model.StockScoreListItem,
	tradeDate string,
) []model.StockScoreListItem {
	items := make([]model.StockScoreListItem, 0, len(ranked))
	for _, r := range ranked {
		item, ok := display[r.symbol]
		if !ok {
			continue
		}
		item.TradeDate = tradeDate
		value := int(r.value + 0.5)
		item.RankValue = &value
		item.Rank = len(items) + 1
		items = append(items, item)
	}
	return items
}
