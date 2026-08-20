package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
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
}

func NewHoldingTradesService(
	repo *repository.HoldingTradesRepo,
	holdings *UserHoldingsService,
	dailyClose *repository.StockDailyCloseRepo,
	fx *ExchangeRateService,
) *HoldingTradesService {
	return &HoldingTradesService{repo: repo, holdings: holdings, dailyClose: dailyClose, fx: fx}
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
	if len(t.Note) > 200 {
		t.Note = t.Note[:200]
	}

	dup, err := s.repo.FindDuplicateTrade(ctx, userID, t)
	if err != nil {
		return model.HoldingPosition{}, err
	}
	if dup != nil {
		return s.syncHolding(ctx, userID, sym)
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
	affected := map[string]bool{}
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

		trade := model.HoldingTrade{
			Symbol:    sym,
			Side:      "buy",
			Quantity:  item.Quantity,
			Price:     item.Price,
			Currency:  currency,
			TradeDate: tradeDate,
			Note:      bulkRegisterNote,
		}

		dup, err := s.repo.FindDuplicateTrade(ctx, userID, trade)
		if err != nil {
			return err
		}
		if dup != nil {
			affected[sym] = true
			continue
		}

		existing, err := s.repo.FindBulkRegisterTrade(ctx, userID, sym)
		if err != nil {
			return err
		}
		if existing != nil {
			if err := s.repo.UpdateTrade(ctx, userID, existing.ID, trade); err != nil {
				return err
			}
			affected[sym] = true
			continue
		}

		if _, err := s.repo.Insert(ctx, userID, trade); err != nil {
			return err
		}
		affected[sym] = true
	}

	for sym := range affected {
		if _, err := s.syncHolding(ctx, userID, sym); err != nil {
			return err
		}
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
