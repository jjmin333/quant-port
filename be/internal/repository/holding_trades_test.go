package repository

import (
	"math"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func trade(id int64, date, side string, qty, price float64) model.HoldingTrade {
	return model.HoldingTrade{
		ID: id, Symbol: "005930", Side: side, Quantity: qty,
		Price: price, Currency: "KRW", TradeDate: date,
	}
}

func approx(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestDerivePositionsWeightedAverage(t *testing.T) {
	pos := DerivePositions([]model.HoldingTrade{
		trade(1, "2026-01-05", "buy", 10, 100),
		trade(2, "2026-02-10", "buy", 30, 200),
	})["005930"]

	approx(t, "quantity", pos.Quantity, 40)
	// (10*100 + 30*200) / 40
	approx(t, "avgPrice", pos.AvgPrice, 175)
	if pos.TradeCount != 2 {
		t.Errorf("tradeCount = %d, want 2", pos.TradeCount)
	}
	if pos.FirstDate != "2026-01-05" {
		t.Errorf("firstDate = %q, want 2026-01-05", pos.FirstDate)
	}
}

// A sell takes shares out at the running average; it must not move the average
// itself. If it did, selling at a profit would silently lower the cost basis and
// make the remaining position look better than it was bought for.
func TestDerivePositionsSellKeepsAverage(t *testing.T) {
	pos := DerivePositions([]model.HoldingTrade{
		trade(1, "2026-01-05", "buy", 10, 100),
		trade(2, "2026-02-10", "buy", 30, 200),
		trade(3, "2026-03-01", "sell", 20, 300),
	})["005930"]

	approx(t, "quantity", pos.Quantity, 20)
	approx(t, "avgPrice", pos.AvgPrice, 175)
	// (300 - 175) * 20
	approx(t, "realized", pos.RealizedPnL, 2500)
}

// Selling out and buying back starts a fresh average — carrying the old one
// forward would price the new position with a cost the user no longer paid.
func TestDerivePositionsRoundTripResetsAverage(t *testing.T) {
	pos := DerivePositions([]model.HoldingTrade{
		trade(1, "2026-01-05", "buy", 10, 100),
		trade(2, "2026-02-10", "sell", 10, 150),
		trade(3, "2026-03-01", "buy", 5, 400),
	})["005930"]

	approx(t, "quantity", pos.Quantity, 5)
	approx(t, "avgPrice", pos.AvgPrice, 400)
	approx(t, "realized", pos.RealizedPnL, 500)
}

// Over-selling is a typo, not a short position. Floor at zero rather than
// carrying a negative quantity into the valuation series.
func TestDerivePositionsOverSellFloorsAtZero(t *testing.T) {
	pos := DerivePositions([]model.HoldingTrade{
		trade(1, "2026-01-05", "buy", 10, 100),
		trade(2, "2026-02-10", "sell", 25, 150),
	})["005930"]

	approx(t, "quantity", pos.Quantity, 0)
	approx(t, "avgPrice", pos.AvgPrice, 0)
}

// Rows arrive from the API in whatever order the caller built them; the fold
// must sort by date first or a later buy could be applied before an earlier one.
func TestDerivePositionsSortsByDate(t *testing.T) {
	unordered := DerivePositions([]model.HoldingTrade{
		trade(3, "2026-03-01", "sell", 20, 300),
		trade(1, "2026-01-05", "buy", 10, 100),
		trade(2, "2026-02-10", "buy", 30, 200),
	})["005930"]

	approx(t, "quantity", unordered.Quantity, 20)
	approx(t, "avgPrice", unordered.AvgPrice, 175)
}

func TestDerivePositionsFeeRaisesCostBasis(t *testing.T) {
	buy := trade(1, "2026-01-05", "buy", 10, 100)
	buy.Fee = 50
	pos := DerivePositions([]model.HoldingTrade{buy})["005930"]

	// (10*100 + 50) / 10
	approx(t, "avgPrice", pos.AvgPrice, 105)
}
