package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

func TestFlowRowsKeepConfirmedDaysOnly(t *testing.T) {
	ind := &toss.TradingVolume{NetBuyVolume: "291850"}
	recs := []toss.StockInvestorTradingRecord{
		// 장중 잠정 — 개인이 비어 있다.
		{Date: "2026-09-24", Foreigner: toss.TradingVolume{NetBuyVolume: "100"}, Institution: toss.TradingVolume{NetBuyVolume: "-5"}},
		{Date: "2026-09-23", Individual: ind, Foreigner: toss.TradingVolume{NetBuyVolume: "-319700"},
			Institution: toss.TradingVolume{NetBuyVolume: "37900"},
			ForeignerHolding: &toss.ForeignerHolding{HoldingRate: "0.5089"}},
		// 숫자가 아닌 값 — 0으로 읽지 말고 버린다.
		{Date: "2026-09-22", Individual: ind, Foreigner: toss.TradingVolume{NetBuyVolume: ""},
			Institution: toss.TradingVolume{NetBuyVolume: "1"}},
	}
	rows := flowRowsFromRecords(recs)
	if len(rows) != 1 || rows[0].TradeDate != "2026-09-23" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].ForeignNet != -319700 || rows[0].InstitutionNet != 37900 || *rows[0].IndividualNet != 291850 {
		t.Fatalf("parsed = %+v", rows[0])
	}
	if rows[0].ForeignHoldRate == nil || *rows[0].ForeignHoldRate != 0.5089 {
		t.Fatalf("hold rate = %v", rows[0].ForeignHoldRate)
	}
}

func TestFlowFetchCount(t *testing.T) {
	now := time.Date(2026, 9, 24, 20, 30, 0, 0, time.UTC)
	if got := flowFetchCount("", now); got != investorFlowBackfill {
		t.Errorf("new symbol = %d", got)
	}
	if got := flowFetchCount("2026-09-23", now); got != investorFlowDaily {
		t.Errorf("fresh = %d", got)
	}
	if got := flowFetchCount("2026-09-01", now); got != investorFlowCatchUp {
		t.Errorf("stale = %d", got)
	}
}
