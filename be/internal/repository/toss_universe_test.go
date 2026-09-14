package repository

import (
	"context"
	"strings"
	"testing"
)

/*
유니버스 반영의 **안전선**을 잠근다 (2026-09-12 사용자 결정 1).

이 잡이 틀리면 증상이 「종목이 화면에서 사라진다」다. 되돌릴 수 없는 종류의 실수라
반영 조건을 코드가 아니라 시험이 지킨다.
*/

func TestApplyUniverse_refusesIncompleteFetch(t *testing.T) {
	// 마켓 하나라도 실패한 합집합으로 「나머지를 숨김」을 돌리면 멀쩡한 종목이 숨는다.
	repo := NewTossUniverseRepo(nil)
	many := make([]string, 5000)
	for i := range many {
		many[i] = "SYM"
		many[i] += string(rune('A' + i%26))
	}
	_, _, err := repo.ApplyUniverse(context.Background(), many, false)
	if err == nil {
		t.Fatal("불완전한 수집을 반영했다")
	}
	if !strings.Contains(err.Error(), "일곱 마켓") {
		t.Fatalf("이유가 드러나지 않는다: %v", err)
	}
}

func TestApplyUniverse_refusesSuspiciouslySmallUniverse(t *testing.T) {
	// 상류 장애·권한 변화로 몇 건만 오는 경우. 그걸 「이것뿐」으로 받아들이면
	// 화면이 통째로 빈다.
	repo := NewTossUniverseRepo(nil)
	for _, n := range []int{0, 1, 999} {
		symbols := make([]string, n)
		for i := range symbols {
			symbols[i] = "S"
		}
		_, _, err := repo.ApplyUniverse(context.Background(), symbols, true)
		if err == nil {
			t.Fatalf("%d건짜리 유니버스를 반영했다", n)
		}
	}
}

func TestTossTradableFilter_treatsUnknownAsVisible(t *testing.T) {
	/*
		**모르는 것(NULL)은 숨기지 않는다.**

		기본값을 0으로 두거나 조건을 `= 1`로만 쓰면, 배치가 처음 돌기 전까지 모든
		종목이 숨는다 — 화면을 통째로 비우는 실패다.
	*/
	f := TossTradableFilter("")
	if !strings.Contains(f, "IS NULL") {
		t.Fatalf("NULL 을 보이게 두는 조건이 없다: %s", f)
	}
	if !strings.Contains(f, "= 1") {
		t.Fatalf("거래 가능 조건이 없다: %s", f)
	}
	if strings.Contains(f, "= 0") {
		t.Fatalf("숨김을 직접 비교하고 있다 — NULL 처리가 어긋난다: %s", f)
	}
}

func TestTossTradableFilter_appliesAlias(t *testing.T) {
	// 조인이 있는 쿼리에서 별칭 없이 쓰면 ambiguous column 으로 깨진다.
	if got := TossTradableFilter("st"); !strings.Contains(got, "st.toss_tradable") {
		t.Fatalf("별칭이 붙지 않았다: %s", got)
	}
}
