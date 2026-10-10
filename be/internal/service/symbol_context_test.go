package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

/*
「내 기록」 시험 — 재는 것은 **없는 것을 있다고 말하지 않는가**다.

이 줄이 틀리면 사용자가 손해를 보는 자리는 둘이다: ① 판 종목에 「보유 중」이 붙는가
② 아무 기록도 없는데 점선 칩 여섯 개를 늘어놓는가.
*/

// 전량 매도한 종목은 **보유가 아니다** — 행이 남아 있어도.
func TestSymbolContextZeroQuantityIsNotHeld(t *testing.T) {
	sold := model.SymbolContext{Symbol: "005930", Quantity: 0}
	if sold.Held {
		t.Error("수량 0은 보유가 아니다")
	}
	if sold.HasAny() {
		t.Error("아무 기록도 없으면 줄을 그리지 않는다")
	}
}

/*
# 아무것도 없으면 줄을 안 그린다

점선 칩 여섯 개를 늘어놓는 것은 안내가 아니라 잡음이다. 하나라도 있을 때만 줄이
선다 — 그때는 없는 칸도 「다음에 할 일」이라는 뜻을 갖는다.
*/
func TestSymbolContextDrawsOnlyWhenSomethingExists(t *testing.T) {
	empty := model.SymbolContext{Symbol: "005930"}
	if empty.HasAny() {
		t.Error("빈 기록에 줄을 그리면 안 된다")
	}

	price := 45000.0
	for name, c := range map[string]model.SymbolContext{
		"보유":  {Held: true, Quantity: 10},
		"관심":  {Watchlisted: true},
		"스탬프": {StampCount: 1},
		"리포트": {ReportCount: 1},
		"목표가": {TargetPrice: &price},
	} {
		if !c.HasAny() {
			t.Errorf("%s 하나만 있어도 줄은 선다", name)
		}
	}
}
