package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/textutil"
)

// HoldingTradesService owns manual buy/sell history and everything derived from
// it: the position a symbol adds up to, and the daily valuation series.
//
// Why the derivation lives here and not in the browser: reconstructing "how many
// shares did I hold on 2026-05-14" needs every trade plus every daily close for
// every symbol. Doing that client-side is N+1 fetches and a different answer per
// device clock. One endpoint, one answer.
type HoldingTradesService struct {
	repo       *repository.HoldingTradesRepo
	holdings   *UserHoldingsService
	dailyClose *repository.StockDailyCloseRepo
	fx         *ExchangeRateService
	stocks     *repository.StockRepo
}

func NewHoldingTradesService(
	repo *repository.HoldingTradesRepo,
	holdings *UserHoldingsService,
	dailyClose *repository.StockDailyCloseRepo,
	fx *ExchangeRateService,
) *HoldingTradesService {
	return &HoldingTradesService{repo: repo, holdings: holdings, dailyClose: dailyClose, fx: fx}
}

// SetCatalog 을 주면 없는 심볼로 매매를 기록하지 않는다.
func (s *HoldingTradesService) SetCatalog(stocks *repository.StockRepo) {
	s.stocks = stocks
}

func (s *HoldingTradesService) ListSymbol(ctx context.Context, symbol string) ([]model.HoldingTrade, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.ListSymbol(ctx, userID, symbol)
}

func (s *HoldingTradesService) List(ctx context.Context) ([]model.HoldingTrade, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, userID)
}

var errTradeInvalid = fmt.Errorf("invalid trade")

// Create records a trade and re-derives the holding it belongs to. Both happen
// together on purpose — a trade that does not move the holding it describes
// would leave the two views of the same position disagreeing.
func (s *HoldingTradesService) Create(ctx context.Context, t model.HoldingTrade) (model.HoldingPosition, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.HoldingPosition{}, err
	}
	sym := strings.ToUpper(strings.TrimSpace(t.Symbol))
	if sym == "" {
		return model.HoldingPosition{}, fmt.Errorf("%w: symbol required", errTradeInvalid)
	}
	if t.Quantity <= 0 {
		return model.HoldingPosition{}, fmt.Errorf("%w: quantity must be positive", errTradeInvalid)
	}
	if t.Price < 0 {
		return model.HoldingPosition{}, fmt.Errorf("%w: price must not be negative", errTradeInvalid)
	}
	side := strings.ToLower(strings.TrimSpace(t.Side))
	if side != "sell" {
		side = "buy"
	}
	t.Side = side
	t.Symbol = sym
	if t.Currency = strings.ToUpper(strings.TrimSpace(t.Currency)); t.Currency == "" {
		t.Currency = "KRW"
	}
	if t.TradeDate = strings.TrimSpace(t.TradeDate); t.TradeDate == "" {
		t.TradeDate = time.Now().In(seoulLocation()).Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", t.TradeDate); err != nil {
		return model.HoldingPosition{}, fmt.Errorf("%w: trade date must be YYYY-MM-DD", errTradeInvalid)
	}
	// 메모는 사람이 한국어로 적는 칸이다. 바이트로 자르면 200번째 바이트에 걸린
	// 글자가 반으로 쪼개져 저장된다(`internal/textutil` 참조). 컬럼도 VARCHAR(200)로
	// 문자를 세므로 글자 단위가 양쪽 모두에 맞다.
	t.Note = textutil.TruncateRunes(t.Note, 200)

	if err := ensureSymbolInCatalog(ctx, s.stocks, sym); err != nil {
		return model.HoldingPosition{}, err
	}

	dup, err := s.repo.FindDuplicateTrade(ctx, userID, t)
	if err != nil {
		return model.HoldingPosition{}, err
	}
	if dup != nil {
		return s.syncHolding(ctx, userID, sym)
	}

	/*
		보유분을 넘는 매도는 여기서 막는다 (2026-09-06 사용자 결정).

		`DerivePositions`는 같은 입력을 0으로 깎는다 — 조회가 음수 포지션을 만들지
		않기 위한 안전망이고 그대로 둔다. 다만 **쓰기에서까지 눈감으면 없던 매도가
		이력에 남는다**: 10주를 갖고 50주를 팔면 내역은 「50주 매도」인데 실현손익은
		10주분이라, 같은 화면의 두 숫자가 서로를 부정한다.

		**중복 검사 뒤에 둔다.** 같은 매도를 두 번 보내면 첫 번째가 이미 저장돼
		`existing`에 들어 있으므로, 위에서 걸러 내지 않으면 같은 매도를 두 번 세어
		멀쩡한 재전송이 「초과」로 거부된다.

		날짜로 되짚는 이유: 매도는 소급 입력될 수 있다. 「지금 보유 수량」이 아니라
		**그 날짜까지의 보유 수량**과 견줘야 4월에 산 것을 3월에 판 것으로 적는
		순서 뒤집힘도 함께 걸린다.
	*/
	if side == "sell" {
		existing, err := s.repo.ListSymbol(ctx, userID, sym)
		if err != nil {
			return model.HoldingPosition{}, err
		}
		candidate := t
		// 아직 행이 없다 — 같은 날짜 안에서는 가장 나중에 일어난 일로 본다.
		candidate.ID = math.MaxInt64
		if bad, held, ok := repository.FindOversell(append(existing, candidate)); ok {
			return model.HoldingPosition{}, fmt.Errorf(
				"%w: cannot sell %s of %s on %s — only %s held then",
				errTradeInvalid,
				strconv.FormatFloat(bad.Quantity, 'f', -1, 64),
				sym,
				bad.TradeDate,
				strconv.FormatFloat(held, 'f', -1, 64),
			)
		}
	}

	if _, err := s.repo.Insert(ctx, userID, t); err != nil {
		return model.HoldingPosition{}, err
	}
	return s.syncHolding(ctx, userID, sym)
}

const bulkRegisterNote = "보유 종목 일괄 등록"

// BulkRegister upserts bulk-register trades for untracked holdings.
// Existing bulk-register rows are updated instead of duplicated.
func (s *HoldingTradesService) BulkRegister(ctx context.Context, items []model.BulkRegisterTradeItem) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	/*
		검증 → 원자적 쓰기 → 파생 갱신, 셋을 분명히 나눈다.

		예전에는 항목마다 조회 두 번과 쓰기 한 번을 따로 냈고 트랜잭션이 없었다.
		중간에서 실패하면 **앞쪽 종목만 등록된 상태로 남았다** — 스크린샷 임포트가
		한 번에 수십 종목을 넣는 경로라 사용자가 "일부만 들어온" 포트폴리오를 보게
		됐다(docs/PERFORMANCE_AUDIT.md P1-8).

		검증을 먼저 전부 끝내는 이유: 잘못된 날짜 하나 때문에 트랜잭션을 열었다
		되돌리는 것보다, 열기 전에 걸러 내는 편이 싸고 오류 메시지도 그대로다.
	*/
	trades := make([]model.HoldingTrade, 0, len(items))
	for _, item := range items {
		sym := strings.ToUpper(strings.TrimSpace(item.Symbol))
		if sym == "" || item.Quantity <= 0 || item.Price <= 0 {
			continue
		}
		currency := strings.ToUpper(strings.TrimSpace(item.Currency))
		if currency == "" {
			currency = "KRW"
		}
		tradeDate := strings.TrimSpace(item.TradeDate)
		if tradeDate == "" {
			tradeDate = time.Now().In(seoulLocation()).Format("2006-01-02")
		}
		if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
			return fmt.Errorf("%w: trade date must be YYYY-MM-DD for %s", errTradeInvalid, sym)
		}

		trades = append(trades, model.HoldingTrade{
			Symbol:    sym,
			Side:      "buy",
			Quantity:  item.Quantity,
			Price:     item.Price,
			Currency:  currency,
			TradeDate: tradeDate,
			Note:      bulkRegisterNote,
		})
	}
	if len(trades) == 0 {
		return nil
	}

	affected, err := s.repo.BulkRegisterAll(ctx, userID, trades)
	if err != nil {
		return err
	}

	/*
		파생 보유 갱신은 **커밋 뒤에** 한다.

		syncHolding은 user_holdings를 다시 쓰고 Redis 캐시를 버린다. 트랜잭션
		안에서 캐시를 버리면 롤백됐을 때 캐시만 날아가 — 데이터는 그대로인데
		캐시는 비는, 고치기 어려운 어긋남이 생긴다.
	*/
	return s.syncHoldings(ctx, userID, affected)
}

/*
syncHoldings — 영향받은 심볼 전체의 파생 보유를 **한 번에** 다시 계산한다.

`syncHolding`을 심볼마다 부르면 왕복이 심볼 수만큼 난다 — 체결 조회 1 + upsert 1.
80종목 임포트면 약 160회 순차 왕복이고, `[HW]` DB가 1 ECPU라 그동안 한 요청이
커넥션을 붙들고 있었다(감사가 「항목당 약 1.65 질의가 남는다」로 남긴 부분).

여기서는 세 번으로 끝낸다:

	① ListSymbolsTrades — 전체 체결을 IN 한 방으로
	② UpsertForUser     — 남은 포지션을 다중행 upsert 한 방으로
	③ 전량 매도분 삭제  — 있을 때만

호출자는 커밋 뒤에 이걸 부른다. 캐시 무효화가 여기서 일어나므로 트랜잭션 안에서
부르면 롤백 시 캐시만 날아간다(BulkRegister 주석 참조).
*/
func (s *HoldingTradesService) syncHoldings(ctx context.Context, userID int64, symbols []string) error {
	if len(symbols) == 0 {
		return nil
	}

	tradesBySymbol, err := s.repo.ListSymbolsTrades(ctx, userID, symbols)
	if err != nil {
		return err
	}
	if s.holdings == nil {
		return nil
	}

	rows := make([]model.UserHolding, 0, len(tradesBySymbol))
	var soldOut []string
	for sym, trades := range tradesBySymbol {
		pos := repository.DerivePositions(trades)[sym]
		pos.Symbol = sym
		if pos.Quantity <= 0 {
			// 전량 매도. 보유 행은 지우되 체결 로그는 남긴다 — 실현 손익이 거기 있다.
			soldOut = append(soldOut, sym)
			continue
		}
		row := model.UserHolding{
			Symbol:       sym,
			Quantity:     pos.Quantity,
			CostCurrency: pos.Currency,
			Source:       "trades",
		}
		if avg := pos.AvgPrice; avg > 0 {
			row.AvgPrice = &avg
		}
		rows = append(rows, row)
	}

	if len(rows) > 0 {
		if err := s.holdings.UpsertForUser(ctx, userID, rows); err != nil {
			return err
		}
	}
	for _, sym := range soldOut {
		// "없음"은 무시한다 — 애초에 보유가 없었을 수 있고, 호출자가 요청한 것은
		// 체결 로그 변경이지 보유 행의 존재 여부가 아니다.
		_ = s.holdings.RemoveHoldingRowForUser(ctx, userID, sym)
	}
	return nil
}

func (s *HoldingTradesService) Delete(ctx context.Context, id int64, symbol string) (model.HoldingPosition, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.HoldingPosition{}, err
	}
	if err := s.repo.Delete(ctx, userID, id); err != nil {
		return model.HoldingPosition{}, err
	}
	return s.syncHolding(ctx, userID, strings.ToUpper(strings.TrimSpace(symbol)))
}

// syncHolding pushes the derived position into user_holdings so every existing
// reader (quotes "held" filter, dashboard, stamps) keeps working unchanged.
//
// A position that nets to zero removes the holding rather than storing a 0-share
// row — a list of things you own should not contain things you sold.
func (s *HoldingTradesService) syncHolding(ctx context.Context, userID int64, symbol string) (model.HoldingPosition, error) {
	trades, err := s.repo.ListSymbol(ctx, userID, symbol)
	if err != nil {
		return model.HoldingPosition{}, err
	}
	pos := repository.DerivePositions(trades)[symbol]
	pos.Symbol = symbol

	if s.holdings == nil {
		return pos, nil
	}
	if pos.Quantity <= 0 {
		// Sold out. Drop the holdings row but keep the trade log — realized P&L
		// lives there. Ignore "not found": the holding may never have existed,
		// and the trade log is still the thing the caller asked us to change.
		_ = s.holdings.RemoveHoldingRowForUser(ctx, userID, symbol)
		return pos, nil
	}
	avg := pos.AvgPrice
	row := model.UserHolding{
		Symbol:       symbol,
		Quantity:     pos.Quantity,
		CostCurrency: pos.Currency,
		Source:       "trades",
	}
	if avg > 0 {
		row.AvgPrice = &avg
	}
	if err := s.holdings.UpsertForUser(ctx, userID, []model.UserHolding{row}); err != nil {
		return pos, err
	}
	return pos, nil
}

// Positions returns every symbol's derived position, for callers that need to
// know which holdings are backed by real history.
func (s *HoldingTradesService) Positions(ctx context.Context) (map[string]model.HoldingPosition, error) {
	trades, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	return repository.DerivePositions(trades), nil
}

// Valuation reconstructs the day-by-day market value of what the user actually
// held, from `days` calendar days ago until today.
//
// The shape of the answer matters more than the numbers: a portfolio curve that
// silently omits a symbol is worse than no curve, so symbols we could not price
// come back by name in MissingSymbols and symbols with no history at all come
// back in UntrackedSymbols. The UI is expected to say so.
func (s *HoldingTradesService) Valuation(ctx context.Context, days int) (model.ValuationSeries, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.ValuationSeries{}, err
	}
	if days <= 0 || days > 3650 {
		days = 90
	}
	out := model.ValuationSeries{Points: []model.ValuationPoint{}}

	trades, err := s.repo.List(ctx, userID)
	if err != nil {
		return out, err
	}

	rate := 0.0
	if s.fx != nil {
		if snap, ok := s.fx.Ensure(ctx); ok {
			rate = snap.MidRate
		}
	}
	out.ExchangeRate = rate

	// Symbols the user holds but never recorded a trade for. They cannot appear
	// on any past day, so name them instead of quietly dropping them.
	if s.holdings != nil {
		held, err := s.holdings.ListForUser(ctx, userID)
		if err == nil {
			traded := map[string]bool{}
			for _, t := range trades {
				traded[strings.ToUpper(t.Symbol)] = true
			}
			for _, h := range held {
				if !traded[strings.ToUpper(h.Symbol)] {
					out.UntrackedSymbols = append(out.UntrackedSymbols, strings.ToUpper(h.Symbol))
				}
			}
			sort.Strings(out.UntrackedSymbols)
		}
	}

	if len(trades) == 0 {
		return out, nil
	}

	loc := seoulLocation()
	today := time.Now().In(loc)
	start := today.AddDate(0, 0, -days)
	// Never start before the first trade — an empty run-up of zeros reads as a
	// crash to zero on the chart.
	firstTrade, err := time.ParseInLocation("2006-01-02", trades[0].TradeDate, loc)
	if err == nil && firstTrade.After(start) {
		start = firstTrade
	}
	startStr := start.Format("2006-01-02")

	// Per symbol: the ordered trade log, and the closes we can price it with.
	symbols := map[string][]model.HoldingTrade{}
	for _, t := range trades {
		sym := strings.ToUpper(t.Symbol)
		symbols[sym] = append(symbols[sym], t)
	}

	type priced struct {
		closes   map[string]float64
		dates    []string
		currency string
	}
	series := map[string]*priced{}
	var missing []string
	for sym, list := range symbols {
		rows, err := s.dailyClose.ClosesSince(ctx, sym, startStr)
		if err != nil {
			return out, err
		}
		if len(rows) == 0 {
			missing = append(missing, sym)
			continue
		}
		p := &priced{closes: make(map[string]float64, len(rows)), currency: strings.ToUpper(list[0].Currency)}
		for _, r := range rows {
			p.closes[r.Date] = r.Close
			p.dates = append(p.dates, r.Date)
		}
		series[sym] = p
	}
	sort.Strings(missing)
	out.MissingSymbols = missing

	if len(series) == 0 {
		return out, nil
	}

	// The x-axis is the union of every symbol's trading days. Using one symbol's
	// calendar would drop days a US name traded and a Korean one did not.
	dateSet := map[string]bool{}
	for _, p := range series {
		for _, d := range p.dates {
			dateSet[d] = true
		}
	}
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	// Running state per symbol as we walk forward: quantity held, cost of that
	// quantity, and the last close seen (carried into non-trading days).
	type running struct {
		qty       float64
		cost      float64
		lastClose float64
	}
	state := map[string]*running{}
	for sym := range series {
		state[sym] = &running{}
	}

	// Trades are applied in date order, including any that happened before the
	// window — otherwise day one starts from zero shares.
	pending := map[string][]model.HoldingTrade{}
	for sym, list := range symbols {
		if _, ok := series[sym]; !ok {
			continue
		}
		sorted := make([]model.HoldingTrade, len(list))
		copy(sorted, list)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].TradeDate != sorted[j].TradeDate {
				return sorted[i].TradeDate < sorted[j].TradeDate
			}
			return sorted[i].ID < sorted[j].ID
		})
		pending[sym] = sorted
	}

	applyUpTo := func(sym, date string) {
		st := state[sym]
		list := pending[sym]
		i := 0
		for ; i < len(list) && list[i].TradeDate <= date; i++ {
			t := list[i]
			if strings.ToLower(t.Side) == "sell" {
				sellQty := t.Quantity
				if sellQty > st.qty {
					sellQty = st.qty
				}
				if st.qty > 0 {
					unit := st.cost / st.qty
					st.cost -= unit * sellQty
				}
				st.qty -= sellQty
				if st.qty <= 0.00005 {
					st.qty = 0
					st.cost = 0
				}
				continue
			}
			st.qty += t.Quantity
			st.cost += t.Price*t.Quantity + t.Fee
		}
		pending[sym] = list[i:]
	}

	for _, date := range dates {
		var valueKRW, costKRW float64
		priceable := false
		for sym, p := range series {
			applyUpTo(sym, date)
			st := state[sym]
			if close, ok := p.closes[date]; ok && close > 0 {
				st.lastClose = close
			}
			if st.qty <= 0 || st.lastClose <= 0 {
				continue
			}
			fx := 1.0
			if p.currency == "USD" {
				if rate <= 0 {
					// No rate means we cannot state a KRW total. Skip the USD
					// leg rather than under-report the portfolio as if it were
					// smaller than it is.
					continue
				}
				fx = rate
			}
			valueKRW += st.qty * st.lastClose * fx
			costKRW += st.cost * fx
			priceable = true
		}
		if !priceable {
			continue
		}
		out.Points = append(out.Points, model.ValuationPoint{
			Date:     date,
			ValueKRW: valueKRW,
			CostKRW:  costKRW,
		})
	}

	return out, nil
}

func seoulLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}
