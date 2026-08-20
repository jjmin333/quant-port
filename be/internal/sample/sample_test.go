package sample

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func now(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 8, 16, 3, 0, 0, 0, time.UTC)
}

// A sample exists to show what the feature does, so the thing worth asserting
// is coverage of the screen's states — not row counts. If a future edit trims
// the set down to five happy-path hits, the seals, the gauge and half the
// scorecard stop having anything to draw and the demo quietly gets worse.
func TestStampsCoverEveryRenderedState(t *testing.T) {
	views := Stamps(now(t))

	var hit, miss, pending, early, opened, dueUnopened, neutral, bearish int
	for _, v := range views {
		switch v.Verdict {
		case model.VerdictHit:
			hit++
		case model.VerdictMiss:
			miss++
		case model.VerdictPending:
			pending++
		}
		if v.TargetHit != nil {
			early++
		}
		if v.Opened {
			opened++
		}
		if v.Due && !v.Opened {
			dueUnopened++
		}
		switch v.Stance {
		case model.StanceNeutral:
			neutral++
		case model.StanceBearish:
			bearish++
		}
	}

	for _, c := range []struct {
		name string
		got  int
	}{
		{"적중", hit}, {"빗나감", miss}, {"검증 중", pending},
		{"장중 조기 적중", early}, {"이미 개봉함", opened},
		{"개봉 대기", dueUnopened}, {"관망", neutral}, {"약세", bearish},
	} {
		if c.got == 0 {
			t.Errorf("샘플에 %q 상태가 하나도 없다 — 그 화면 요소는 빈 채로 보인다", c.name)
		}
	}
}

// Stance is derived from the target on the real create path. A fixture that
// hand-set a contradictory pair would show a combination the app cannot produce.
func TestStampsStanceMatchesTarget(t *testing.T) {
	for _, v := range Stamps(now(t)) {
		want := model.StanceForTarget(v.Price, v.TargetPrice)
		if v.Stance != want {
			t.Errorf("%s: stance=%q but target %v vs price %v implies %q",
				v.Symbol, v.Stance, v.TargetPrice, v.Price, want)
		}
	}
}

// The tiles are computed from the rows, so a summary that disagrees with them
// would have the header contradicting the list directly underneath it.
func TestStampSummaryAgreesWithRows(t *testing.T) {
	at := now(t)
	views := Stamps(at)
	got := StampSummary(at)

	if got.Total != len(views) {
		t.Fatalf("Total = %d, want %d", got.Total, len(views))
	}
	var judged, hits int
	for _, v := range views {
		if v.Verdict == model.VerdictHit || v.Verdict == model.VerdictMiss {
			judged++
			if v.Verdict == model.VerdictHit {
				hits++
			}
		}
	}
	if got.Judged != judged || got.Hits != hits {
		t.Fatalf("Judged/Hits = %d/%d, want %d/%d", got.Judged, got.Hits, judged, hits)
	}
	if got.HitRate == nil {
		t.Fatal("HitRate should be set — the fixture has judged stamps")
	}
	if got.AvgTargetError == nil {
		t.Fatal("AvgTargetError should be set — the scorecard's second axis would read '—'")
	}
}

// Dates are relative so the demo never looks abandoned. A fixture with a
// deadline already behind "검증 중" rows would render a countdown of D-negative.
func TestStampDatesStayRelativeToNow(t *testing.T) {
	at := now(t)
	for _, v := range Stamps(at) {
		if v.StampedAt.After(at) {
			t.Errorf("%s: stamped in the future", v.Symbol)
		}
		if v.Verdict == model.VerdictPending && !v.DueAt.After(at) {
			t.Errorf("%s: pending but the deadline has passed", v.Symbol)
		}
		if v.Due && v.TargetHit == nil && v.DueAt.After(at) {
			t.Errorf("%s: marked due but the deadline is still ahead", v.Symbol)
		}
	}
}

// Every menu has to return something: an empty fixture would land the visitor
// back on the empty screen the sample exists to replace.
func TestEveryMenuFixtureIsPopulated(t *testing.T) {
	at := now(t)
	if len(Stamps(at)) == 0 {
		t.Error("stamps fixture is empty")
	}
	if len(Watchlist(at)) == 0 {
		t.Error("watchlist fixture is empty")
	}
	if len(Holdings(at)) == 0 {
		t.Error("holdings fixture is empty")
	}
	if len(Journal(at)) == 0 {
		t.Error("journal fixture is empty")
	}
	if len(Targets(at)) == 0 {
		t.Error("targets fixture is empty")
	}
}

// 손익이 한쪽으로만 쏠린 표는 화면이 무엇을 구분해 보여주는지 알려주지 못한다.
func TestHoldingsAndWatchlistShowBothDirections(t *testing.T) {
	at := now(t)

	var gain, loss int
	for _, h := range Holdings(at) {
		if h.AvgPrice == nil {
			t.Fatalf("%s: holdings fixture needs an average price", h.Symbol)
		}
		if h.Price > *h.AvgPrice {
			gain++
		} else if h.Price < *h.AvgPrice {
			loss++
		}
	}
	if gain == 0 || loss == 0 {
		t.Errorf("보유 샘플이 이익 %d건 손실 %d건 — 손익 색이 한쪽만 나온다", gain, loss)
	}

	var up, down int
	for _, w := range Watchlist(at) {
		if w.AddedPrice == 0 {
			t.Errorf("%s: 담은 가격이 없으면 「담은 뒤」 열이 —로 남는다", w.Symbol)
		}
		if w.Price > w.AddedPrice {
			up++
		} else if w.Price < w.AddedPrice {
			down++
		}
	}
	if up == 0 || down == 0 {
		t.Errorf("관심 샘플이 상승 %d건 하락 %d건 — 「담은 뒤」 열의 한쪽이 안 보인다", up, down)
	}
}

// 고정 메모가 없으면 「고정한 메모」 카드가 샘플에서도 비어 있다.
func TestJournalHasPinnedAndLinkedNotes(t *testing.T) {
	var pinned, linked int
	for _, n := range Journal(now(t)) {
		if n.Pinned {
			pinned++
		}
		if n.Symbol != "" {
			linked++
		}
	}
	if pinned == 0 {
		t.Error("고정한 메모가 없다 — 레일의 고정 카드가 빈 채로 보인다")
	}
	if linked == 0 {
		t.Error("종목에 연결된 메모가 없다 — 종목별 메모 카드가 빈 채로 보인다")
	}
}

// 목표를 이미 넘긴 행이 하나는 있어야 「도달」 상태가 화면에 나온다.
func TestTargetsIncludeAReachedOne(t *testing.T) {
	var reached, pending int
	for _, tg := range Targets(now(t)) {
		if tg.Price >= tg.TargetPrice {
			reached++
		} else {
			pending++
		}
	}
	if reached == 0 || pending == 0 {
		t.Errorf("목표가 샘플이 도달 %d건 미도달 %d건 — 두 상태가 다 나와야 한다", reached, pending)
	}
}
