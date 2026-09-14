package model

import (
	"math"
	"sort"
	"time"
)

/*
백필한 분기 ↔ 8-K 발표일 짝짓기.

이 파일이 이 기능에서 **틀렸을 때 가장 알아채기 어려운 부분**이다. 한 칸만 밀려도
모든 분기가 그럴듯한 날짜를 갖게 되고, 화면에서는 그냥 「과거 캘린더가 채워졌다」로
보인다. 그래서 규칙을 좁게 잡고 어기면 버린다.
*/

const (
	// AnnounceMatchToleranceDays — 기간말과 발표일의 허용 간격.
	//
	// Finnhub의 기간말은 **달력 분기로 반올림**한 값이라 회사의 실제 회계 분기와
	// 어긋난다. 실측 간격은 −42일(NVDA·WMT·TGT — 회계연도가 1월에 끝나 반올림이
	// 뒤로 밀린 경우)부터 +48일(AMAT)까지였다. 45일로 잡았다가 AMAT을 놓쳐 60으로
	// 넓혔다.
	//
	// 60이 상한인 이유는 분기 간격이 **90일**이기 때문이다. 그보다 넓히면 한
	// 분기가 옆 분기의 발표를 집어갈 수 있다.
	AnnounceMatchToleranceDays = 60
)

// QuarterToMatch is one stored quarter waiting for its announcement date.
type QuarterToMatch struct {
	FiscalYear   int
	FiscalPeriod string
	PeriodEnd    time.Time
}

// AnnounceCandidate is one 8-K Item 2.02 filing.
type AnnounceCandidate struct {
	Filed    time.Time
	Session  string
	Accession string
}

// AnnounceMatch pairs a quarter with the filing that announced it.
type AnnounceMatch struct {
	Quarter   QuarterToMatch
	Filed     time.Time
	Session   string
	Accession string
	// GapDays — 기간말 대비 며칠 뒤인지. 음수면 발표가 (반올림된) 기간말보다 앞선다.
	GapDays int
}

// MatchAnnouncements assigns each quarter its nearest unused filing.
//
// # 규칙
//
//  1. 기간말이 최근인 분기부터, 아직 안 쓴 8-K 중 **가장 가까운 것**을 집는다.
//  2. 간격이 AnnounceMatchToleranceDays를 넘으면 그 분기는 포기한다(부분 매칭 허용).
//  3. 짝지은 결과가 **단조**여야 한다 — 기간말이 더 최근인 분기의 발표일이 더
//     오래된 분기의 발표일보다 앞서면 어딘가 한 칸 밀린 것이므로 **전부 버린다**.
//
// 3번이 이 함수의 핵심 방어다. 1·2번만으로는 8-K가 하나 빠지거나 더 있을 때
// 조용히 어긋날 수 있는데, 그 경우 거의 항상 순서가 뒤집힌다.
func MatchAnnouncements(
	quarters []QuarterToMatch, candidates []AnnounceCandidate,
) []AnnounceMatch {
	if len(quarters) == 0 || len(candidates) == 0 {
		return nil
	}

	qs := make([]QuarterToMatch, len(quarters))
	copy(qs, quarters)
	sort.SliceStable(qs, func(i, j int) bool { return qs[i].PeriodEnd.After(qs[j].PeriodEnd) })

	used := make([]bool, len(candidates))
	out := make([]AnnounceMatch, 0, len(qs))

	for _, q := range qs {
		best, bestGap := -1, math.MaxFloat64
		for i, c := range candidates {
			if used[i] {
				continue
			}
			gap := math.Abs(c.Filed.Sub(q.PeriodEnd).Hours() / 24)
			if gap < bestGap {
				best, bestGap = i, gap
			}
		}
		if best < 0 || bestGap > AnnounceMatchToleranceDays {
			continue
		}
		used[best] = true
		c := candidates[best]
		out = append(out, AnnounceMatch{
			Quarter:   q,
			Filed:     c.Filed,
			Session:   c.Session,
			Accession: c.Accession,
			GapDays:   int(math.Round(c.Filed.Sub(q.PeriodEnd).Hours() / 24)),
		})
	}

	if !announceMatchesAreMonotonic(out) {
		return nil
	}
	return out
}

// announceMatchesAreMonotonic checks that newer quarters got newer filings.
//
// out은 기간말 내림차순으로 쌓였으므로 발표일도 내림차순이어야 한다. 같은 날짜가
// 두 번 나오는 것도 이상하다 — 8-K 하나가 두 분기를 발표하지는 않는다(1:1로
// 집었으니 코드상 불가능하지만, 규칙을 여기 적어 둔다).
func announceMatchesAreMonotonic(matches []AnnounceMatch) bool {
	for i := 1; i < len(matches); i++ {
		if !matches[i-1].Filed.After(matches[i].Filed) {
			return false
		}
	}
	return true
}
