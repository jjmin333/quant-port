package service

import (
	"context"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// reactionWindowDays — 발표일 앞뒤로 종가를 얼마나 넉넉히 받을지.
//
// 필요한 것은 「직전 거래일」과 「직후 거래일」 둘뿐이지만, 연휴가 끼면 그 둘이
// 달력상 며칠씩 떨어진다(설·추석은 최장 닷새). 열흘이면 어떤 연휴도 넘는다.
const reactionWindowDays = 10

// maxReactionGapDays — 기준·반응 종가가 발표일에서 며칠까지 떨어져도 되는지.
//
// **이 가드가 없으면 조용히 거짓말을 한다.** `stock_daily_closes`는 최근 세션의
// 커버리지가 얇다(실측: 2026-08-07까지는 16,190종목/일인데 08-10부터는 200~700).
// 그래서 가드 없이 「발표일 이후 첫 종가」를 찾으면 08-13 발표에 08-14가 아니라
// 그 다음 주 종가가 잡히고, 7일치 등락이 「다음 거래일」이라는 이름으로 나간다
// (클리오 08-13 발표의 +28.2%가 실제로는 08-07→08-14 한 주 등락이었다).
//
// 5일은 금요일 발표 → 월요일 반응(3일)과 연휴 하루를 덮는다. 그보다 멀면 그것은
// 「다음 세션」이 아니므로 값을 내지 않고, 화면이 「집계 중」으로 말한다.
const maxReactionGapDays = 5

// AttachReactions fills in each confirmed announcement's post-earnings move.
//
// 종가 조회는 **한 번**이다 — 목록의 심볼을 모아 한 쿼리로 받고 짝은 메모리에서
// 맞춘다(항목당 I/O 금지, AGENT_STATE 응답 속도 원칙 #1).
func (s *EarningsCalendarService) AttachReactions(ctx context.Context, events []model.EarningsEvent) {
	if s == nil || s.events == nil || len(events) == 0 {
		return
	}

	// 예정·추정 행에는 반응이 있을 수 없다. 심볼 목록에서도 빼야 헛된 종가를
	// 받아 오지 않는다.
	symbolSet := make(map[string]struct{}, len(events))
	var minDate, maxDate time.Time
	for _, ev := range events {
		if ev.DateStatus != model.EarningsDateConfirmed {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", ev.Date, time.UTC)
		if err != nil {
			continue
		}
		symbolSet[ev.Symbol] = struct{}{}
		if minDate.IsZero() || day.Before(minDate) {
			minDate = day
		}
		if maxDate.IsZero() || day.After(maxDate) {
			maxDate = day
		}
	}
	if len(symbolSet) == 0 {
		return
	}
	symbols := make([]string, 0, len(symbolSet))
	for sym := range symbolSet {
		symbols = append(symbols, sym)
	}

	closes, err := s.events.ClosesForSymbols(ctx, symbols,
		minDate.AddDate(0, 0, -reactionWindowDays),
		maxDate.AddDate(0, 0, reactionWindowDays))
	if err != nil {
		// 반응은 부가 정보다. 못 구하면 그 열만 비고 목록은 그대로 나간다.
		return
	}

	for i := range events {
		if events[i].DateStatus != model.EarningsDateConfirmed {
			continue
		}
		events[i].Reaction = reactionFor(events[i], closes[events[i].Symbol])
		// 접어 둔 개별 기준 행도 같은 종목·같은 날이라 반응이 같다. 대표만
		// 채우고 대안은 비워 두면 펼쳤을 때 칸이 비어 고장으로 보인다.
		for j := range events[i].Alternates {
			events[i].Alternates[j].Reaction = events[i].Reaction
		}
	}
}

// reactionFor picks the right pair of closes for one announcement.
//
// **세션을 보지 않으면 하루를 잘못 읽는다.** 장전(bmo) 발표는 그날 장에 반영되고,
// 장후(amc) 발표는 다음 거래일에 반영된다. 국내 잠정실적은 통상 마감 후 공시되고
// 시각 데이터 자체가 없어 장후와 같게 본다.
func reactionFor(event model.EarningsEvent, points []repository.DailyClose) *model.EarningsReaction {
	if len(points) < 2 {
		return nil
	}
	day, err := time.ParseInLocation("2006-01-02", event.Date, time.UTC)
	if err != nil {
		return nil
	}

	if event.Session == model.EarningsSessionBeforeOpen {
		// 기준 = 발표일 **이전** 마지막 종가, 반응 = 발표일 종가.
		base, okBase := lastCloseBefore(points, day)
		reaction, okReaction := closeOn(points, day)
		return buildReaction(base, reaction, okBase && okReaction, "sameDay", day)
	}

	// 기준 = 발표일까지의 마지막 종가(발표일이 휴장이면 그 직전), 반응 = 그 이후 첫 종가.
	base, okBase := lastCloseOnOrBefore(points, day)
	reaction, okReaction := firstCloseAfter(points, day)
	return buildReaction(base, reaction, okBase && okReaction, "nextDay", day)
}

func buildReaction(
	base, reaction repository.DailyClose, ok bool, basis string, event time.Time,
) *model.EarningsReaction {
	// 기준가가 0이면 나눌 수 없다. 상장 첫날 등 드문 경우에 실제로 0이 온다.
	if !ok || base.Close == 0 {
		return nil
	}
	// 발표일에서 너무 먼 종가는 「다음 세션」이 아니다 (maxReactionGapDays 참고).
	if gapDays(event, base.Date) > maxReactionGapDays || gapDays(event, reaction.Date) > maxReactionGapDays {
		return nil
	}
	return &model.EarningsReaction{
		BaseClose: base.Close,
		BaseDate:  base.Date.Format("2006-01-02"),
		Close:     reaction.Close,
		Date:      reaction.Date.Format("2006-01-02"),
		Pct:       (reaction.Close/base.Close - 1) * 100,
		Basis:     basis,
	}
}

// gapDays is the absolute calendar-day distance between two dates.
func gapDays(a, b time.Time) int {
	d := int(a.Sub(b).Hours() / 24)
	if d < 0 {
		return -d
	}
	return d
}

// points는 오래된 것부터 정렬돼 있다(repo의 ORDER BY).

func lastCloseOnOrBefore(points []repository.DailyClose, day time.Time) (repository.DailyClose, bool) {
	for i := len(points) - 1; i >= 0; i-- {
		if !points[i].Date.After(day) {
			return points[i], true
		}
	}
	return repository.DailyClose{}, false
}

func lastCloseBefore(points []repository.DailyClose, day time.Time) (repository.DailyClose, bool) {
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Date.Before(day) {
			return points[i], true
		}
	}
	return repository.DailyClose{}, false
}

func closeOn(points []repository.DailyClose, day time.Time) (repository.DailyClose, bool) {
	for _, p := range points {
		if p.Date.Equal(day) {
			return p, true
		}
	}
	return repository.DailyClose{}, false
}

func firstCloseAfter(points []repository.DailyClose, day time.Time) (repository.DailyClose, bool) {
	for _, p := range points {
		if p.Date.After(day) {
			return p, true
		}
	}
	return repository.DailyClose{}, false
}
