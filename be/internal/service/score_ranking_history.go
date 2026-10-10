package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
AI 랭킹의 날짜별 구성을 남긴다 — 백테스트가 읽을 유일한 과거 기록.

# 왜 점수 배치의 꼬리에 붙는가

랭킹은 그날 점수에서 나오고 점수는 하루 한 번 계산된다. 별도 배치를 두면 그 둘 사이가
벌어질 수 있고(점수는 돌았는데 스냅샷은 실패한 날), 그런 날은 **백테스트에서 조용히
빠진다** — 빠진 날은 「그날 시장이 없었다」와 구분되지 않는다.

# 왜 여기서 다시 계산하는가

`ScoreRankingService`가 이미 48개 목록을 만들지만 그것은 **표시용**이다(상위 20~50,
가격·이름·등급이 붙은 JSON). 백테스트가 필요로 하는 것은 상위 200의 심볼과 순위뿐이고,
표시 행을 200개까지 만들면 조회가 몇 배로 무거워진다. 같은 원재료(`WindowBests`)를
쓰되 표시 조립을 건너뛴다.
*/

// ScoreRankingHistoryService writes the dated ranking snapshots.
type ScoreRankingHistoryService struct {
	history  *repository.ScoreRankingHistoryRepo
	rankings *repository.StockScoreRankingRepo
	scores   *ScoreService
}

func NewScoreRankingHistoryService(
	history *repository.ScoreRankingHistoryRepo,
	rankings *repository.StockScoreRankingRepo,
	scores *ScoreService,
) *ScoreRankingHistoryService {
	return &ScoreRankingHistoryService{history: history, rankings: rankings, scores: scores}
}

/*
SnapshotLatest — 가장 최근 점수일의 구성을 남긴다. 점수 배치의 꼬리에서 부른다.

이미 그 날짜가 채워져 있으면 **다시 쓴다**. 재계산(관리자 「지금 재계산」)이 순위를
바꿨는데 스냅샷만 옛 것으로 남으면, 백테스트와 화면이 같은 날짜에 서로 다른 상위
목록을 말하게 된다.
*/
func (s *ScoreRankingHistoryService) SnapshotLatest(ctx context.Context) error {
	if s == nil || s.scores == nil {
		return fmt.Errorf("ranking history service not configured")
	}
	tradeDate, err := s.scores.LatestScoredDate(ctx)
	if err != nil {
		return err
	}
	if tradeDate == "" {
		log.Println("ranking history: no scored date yet — nothing to snapshot")
		return nil
	}
	return s.SnapshotDate(ctx, tradeDate)
}

// SnapshotDate writes every (market, axis) list for one trade date.
func (s *ScoreRankingHistoryService) SnapshotDate(ctx context.Context, tradeDate string) error {
	if s == nil || s.history == nil || s.rankings == nil {
		return fmt.Errorf("ranking history service not configured")
	}
	var failures []string
	for market, assetTypes := range scoreRankingMarkets {
		if err := s.snapshotMarket(ctx, tradeDate, market, assetTypes); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", market, err))
		}
	}
	/*
		축 점수 이력도 같은 날짜로 남긴다 — 랭킹 구성만 있고 축 값이 없는 날이 생기면
		백테스트와 보유 점검이 같은 날짜에 서로 다른 과거를 본다.
	*/
	if _, err := s.history.ArchiveScoreAxes(ctx, tradeDate); err != nil {
		failures = append(failures, err.Error())
	}
	if len(failures) > 0 {
		return fmt.Errorf("ranking history %s: %s", tradeDate, strings.Join(failures, "; "))
	}
	return nil
}

func (s *ScoreRankingHistoryService) snapshotMarket(
	ctx context.Context, tradeDate, market string, assetTypes []string,
) error {
	/*
		랭킹 자격 집합을 **자르기 전에** 적용한다 — 화면 랭킹과 같은 규칙이다
		(`computeMarket` 주석). 여기서 어긋나면 백테스트가 화면에 없던 종목을
		산 것으로 계산한다.
	*/
	eligible, err := s.rankings.RankableSymbols(ctx, tradeDate, assetTypes)
	if err != nil {
		return err
	}
	// 하루짜리 창 — from == to. `WindowBests`가 그날의 종목별 여섯 측정치를 한 번에 준다.
	rows, err := s.rankings.WindowBests(ctx, tradeDate, tradeDate, assetTypes)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	for _, rankAxis := range model.ScoreRankingAxes {
		ranked := rankSnapshotAxis(rows, eligible, rankAxis)
		out := make([]repository.ScoreRankingHistoryRow, 0, len(ranked))
		for i, r := range ranked {
			out = append(out, repository.ScoreRankingHistoryRow{
				TradeDate: tradeDate,
				Market:    market,
				Axis:      string(rankAxis),
				Rank:      i + 1,
				Symbol:    r.symbol,
				Score:     int(r.value + 0.5),
			})
		}
		if err := s.history.ReplaceDate(ctx, tradeDate, market, string(rankAxis), out); err != nil {
			return err
		}
	}
	return nil
}

// rankSnapshotAxis is rankWindow's shape at snapshot depth (200 rather than the
// display top-N), with the same tie-break so a screen and a backtest never
// disagree about who was 47th.
func rankSnapshotAxis(
	rows []repository.ScoreWindowRow,
	eligible map[string]struct{},
	rankAxis model.ScoreRankingAxis,
) []scoreRankedSymbol {
	axis, isAxis := rankAxis.Axis()
	list := make([]scoreRankedSymbol, 0, len(rows))
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
	if len(list) > repository.ScoreRankingHistoryDepth {
		list = list[:repository.ScoreRankingHistoryDepth]
	}
	return list
}

/*
Backfill — 아직 `stock_scores`에 남아 있는 날짜들을 한 번에 채운다.

이 기능이 켜지는 날 표가 비어 있으면 백테스트는 **0거래일**로 시작한다. 그런데 점수
표에는 보관 기간만큼(최대 30거래일) 과거가 이미 들어 있으므로, 그만큼은 오늘 바로
만들 수 있다. 하루를 기다려 하루를 얻는 대신 첫날에 한 달을 얻는다.

이미 채워진 날짜는 건너뛴다 — 다시 돌려도 안전하고, 배치가 중간에 끊겨도 이어서 돈다.
*/
func (s *ScoreRankingHistoryService) Backfill(ctx context.Context) (int, error) {
	if s == nil || s.history == nil || s.scores == nil {
		return 0, fmt.Errorf("ranking history service not configured")
	}
	dates, err := s.scores.ScoredDates(ctx)
	if err != nil {
		return 0, err
	}
	// 종합·국내 하나만 물어도 「그 날짜를 찍었는가」는 알 수 있다 — 한 날짜의 여섯
	// 축은 같은 트랜잭션 묶음에서 함께 쓰인다.
	covered, err := s.history.CoveredDates(ctx, "domestic", string(model.RankAxisComposite))
	if err != nil {
		return 0, err
	}

	/*
		축 점수 이력은 랭킹 이력보다 늦게 생겼다(2026-09-23). 랭킹은 이미 채워졌는데 축
		값이 빈 날짜가 있으므로 따로 센다 — 랭킹 기준으로만 건너뛰면 그 날짜들의 축 값은
		30거래일 뒤 `stock_scores`와 함께 영영 사라진다.
	*/
	archived := map[string]bool{}
	if len(dates) > 0 {
		if archived, err = s.history.ArchivedScoreDates(ctx, dates[0]); err != nil {
			return 0, err
		}
	}

	filled := 0
	for _, date := range dates {
		if covered[date] > 0 {
			if !archived[date] {
				if _, err := s.history.ArchiveScoreAxes(ctx, date); err != nil {
					log.Printf("score history backfill %s: %v", date, err)
				}
			}
			continue
		}
		if err := s.SnapshotDate(ctx, date); err != nil {
			// 한 날짜가 실패해도 나머지를 포기하지 않는다 — 빠진 날은 백테스트에서
			// 「그날 시장이 없었다」로 읽히므로 최대한 채운다.
			log.Printf("ranking history backfill %s: %v", date, err)
			continue
		}
		filled++
	}
	return filled, nil
}

// ScoreRankingHistoryStatus is what the table actually holds, per market.
//
// 화면이 「표본 N거래일」을 말하려면 그 N이 필요하다. **요청한 창이 아니라 가진 창**을
// 말하는 것이 이 기능의 규칙이고, 그 규칙은 여기서 시작한다.
type ScoreRankingHistoryStatus struct {
	Market   string `json:"market"`
	FromDate string `json:"fromDate,omitempty"`
	ToDate   string `json:"toDate,omitempty"`
	Dates    int    `json:"dates"`
}

// Status reports coverage for each market on the composite axis. 한 날짜의 여섯
// 축은 같은 묶음에서 함께 쓰이므로 종합 하나로 날짜 수가 드러난다.
func (s *ScoreRankingHistoryService) Status(ctx context.Context) ([]ScoreRankingHistoryStatus, error) {
	if s == nil || s.history == nil {
		return nil, fmt.Errorf("ranking history service not configured")
	}
	markets := make([]string, 0, len(scoreRankingMarkets))
	for market := range scoreRankingMarkets {
		markets = append(markets, market)
	}
	sort.Strings(markets)

	out := make([]ScoreRankingHistoryStatus, 0, len(markets))
	for _, market := range markets {
		from, to, dates, err := s.history.Span(ctx, market, string(model.RankAxisComposite))
		if err != nil {
			return nil, err
		}
		out = append(out, ScoreRankingHistoryStatus{
			Market: market, FromDate: from, ToDate: to, Dates: dates,
		})
	}
	return out, nil
}
