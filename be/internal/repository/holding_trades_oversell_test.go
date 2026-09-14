package repository

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

/*
보유분을 넘는 매도를 **쓰기 전에** 잡는다.

이 규칙이 없을 때 실제로 나온 결과(2026-09-06 실측): 10주를 사고 50주를 팔면
API가 200을 주고, 매매 이력에는 「50주 매도」가 남는데 실현손익은 10주분인
10,000원으로 계산됐다. 두 숫자가 같은 화면에서 서로를 부정한다.

`DerivePositions`는 같은 입력에서 매도를 깎아 0으로 만든다 — 그건 조회의 안전망이라
그대로 두고, 여기서는 「그 행이 애초에 들어오면 안 된다」만 본다.
*/

func TestFindOversell(t *testing.T) {
	tests := []struct {
		name     string
		trades   []model.HoldingTrade
		wantOver bool
		wantHeld float64
	}{
		{
			name:     "보유가 없는데 파는 것은 초과다",
			trades:   []model.HoldingTrade{trade(1, "2026-01-05", "sell", 1, 1000)},
			wantOver: true,
			wantHeld: 0,
		},
		{
			name: "산 만큼만 파는 것은 통과한다",
			trades: []model.HoldingTrade{
				trade(1, "2026-01-05", "buy", 10, 1000),
				trade(2, "2026-01-06", "sell", 10, 1000),
			},
		},
		{
			name: "산 것보다 많이 팔면 초과다",
			trades: []model.HoldingTrade{
				trade(1, "2026-01-05", "buy", 10, 1000),
				trade(2, "2026-01-06", "sell", 50, 1000),
			},
			wantOver: true,
			wantHeld: 10,
		},
		{
			name: "나눠 판 합이 보유분을 넘으면 초과다",
			trades: []model.HoldingTrade{
				trade(1, "2026-01-05", "buy", 10, 1000),
				trade(2, "2026-01-06", "sell", 6, 1000),
				trade(3, "2026-01-07", "sell", 6, 1000),
			},
			wantOver: true,
			wantHeld: 4,
		},
		{
			name: "되산 뒤에는 다시 팔 수 있다",
			trades: []model.HoldingTrade{
				trade(1, "2026-01-05", "buy", 10, 1000),
				trade(2, "2026-01-06", "sell", 10, 1000),
				trade(3, "2026-01-07", "buy", 5, 1000),
				trade(4, "2026-01-08", "sell", 5, 1000),
			},
		},
		{
			/*
				소급 입력. 4월에 산 것을 3월에 판 것으로 적으면, 「지금 보유 수량」만
				보는 검사는 통과시켜 버린다 — 날짜 순으로 되짚어야 걸린다.
			*/
			name: "매수보다 앞선 날짜의 매도는 초과다",
			trades: []model.HoldingTrade{
				trade(1, "2026-04-01", "buy", 10, 1000),
				trade(2, "2026-03-01", "sell", 10, 1000),
			},
			wantOver: true,
			wantHeld: 0,
		},
		{
			// 같은 날짜는 ID 순 — 먼저 기록된 매수가 그날의 매도를 받쳐 준다.
			name: "같은 날 매수 뒤 매도는 통과한다",
			trades: []model.HoldingTrade{
				trade(1, "2026-01-05", "buy", 10, 1000),
				trade(2, "2026-01-05", "sell", 10, 1000),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bad, held, over := FindOversell(tc.trades)
			if over != tc.wantOver {
				t.Fatalf("초과 판정 = %v, 원한 값 %v (걸린 행 %+v)", over, tc.wantOver, bad)
			}
			if over && held != tc.wantHeld {
				t.Fatalf("그 시점 보유 수량 = %v, 원한 값 %v", held, tc.wantHeld)
			}
		})
	}
}

/*
초과 매도가 이미 저장돼 있는 계정도 있다 — 이 검사가 생기기 전에 들어온 행이다.
그 계정의 조회가 500이 되거나 음수 포지션을 그리면 안 되므로, 깎는 안전망이
그대로 살아 있는지 함께 못박는다.
*/
func TestDerivePositionsStillFloorsLegacyOversell(t *testing.T) {
	pos := DerivePositions([]model.HoldingTrade{
		trade(1, "2026-01-05", "buy", 10, 1000),
		trade(2, "2026-01-06", "sell", 50, 1000),
	})["005930"]

	if pos.Quantity != 0 {
		t.Fatalf("보유 수량 = %v, 음수가 아니라 0이어야 한다", pos.Quantity)
	}
}
