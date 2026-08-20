package service

import (
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// The auto-generated draft (docs/RESEARCH_CALL_DESIGN.md §5.4).
//
// Two things live here, and they are used at different moments:
//
//   - buildResearchDraft — a two-line prompt shown in the empty editor, which
//     the compose screen clears on the author's first click and never
//     publishes.
//   - buildResearchDataBlock — the 발행 시점 데이터 section, inserted only when
//     the author presses the button for it.
//
// Both write only what they were given. No level here is invented: they come
// from stock_briefing.go, which derives them from moving averages, swing
// extremes and the 52-week band, so this file costs zero LLM calls and zero
// provider budget. A draft that made up a target would poison exactly the thing
// this feature exists to protect.

// buildResearchDraft is the one-or-two line prompt the compose screen shows in
// an otherwise empty editor.
//
// It used to be a full skeleton with headings and the whole data block. That
// made every author's first act deleting someone else's text, which is worse
// than a blank page: a blank page at least does not have to be cleaned up. The
// data block still exists (buildResearchDataBlock) but is now inserted only
// when the author asks for it.
func buildResearchDraft(_ model.ResearchCall, direction model.ResearchDirection) string {
	return draftPrompt(direction) + "\n(클릭하면 이 안내는 사라집니다.)"
}

// buildResearchDataBlock renders the auto-generated 발행 시점 데이터 section.
//
// Nothing here is invented: the numbers come from the frozen evidence snapshot
// and stock_briefing.go, so the block costs zero LLM calls and zero provider
// budget. A draft that made up a target would poison exactly the thing this
// feature exists to protect.
func buildResearchDataBlock(c model.ResearchCall) string {
	var b strings.Builder

	b.WriteString("## 발행 시점 데이터 (자동 생성)\n\n")
	b.WriteString(draftScoreLine(c))

	if br := c.Evidence.Briefing; br != nil {
		if len(br.KeyPoints) > 0 {
			b.WriteString("\n### 투자 포인트\n\n")
			for _, p := range br.KeyPoints {
				b.WriteString(fmt.Sprintf("- **%s** — %s\n", p.Label, p.Text))
			}
		}
		if s := draftScenario("상승 시나리오", br.Upside, c.Currency); s != "" {
			b.WriteString("\n")
			b.WriteString(s)
		}
		if s := draftScenario("하락 시나리오", br.Downside, c.Currency); s != "" {
			b.WriteString("\n")
			b.WriteString(s)
		}
	}

	if len(c.Context.News) > 0 {
		b.WriteString("\n### 발행 시점 주요 뉴스\n\n")
		for _, n := range c.Context.News {
			headline := strings.TrimSpace(n.Headline)
			if headline == "" {
				continue
			}
			if n.URL != "" {
				b.WriteString(fmt.Sprintf("- [%s](%s)\n", headline, n.URL))
			} else {
				b.WriteString(fmt.Sprintf("- %s\n", headline))
			}
		}
	}

	if gaps := draftGapNote(c.Context.DataGaps); gaps != "" {
		b.WriteString("\n")
		b.WriteString(gaps)
	}
	return b.String()
}

// draftPrompt asks the one question the server cannot answer. The whole point
// of auto-filling everything else is to leave the author with only this.
//
// Plain text, not an HTML comment: it is meant to be read in the editor, and it
// is safe to show in plain because the compose screen drops it if the author
// never touched it (see research-compose.tsx).
func draftPrompt(direction model.ResearchDirection) string {
	switch direction {
	case model.DirectionSell:
		return "왜 하락 쪽에 거는지, 무엇이 확인되면 이 판단이 틀린 것인지 적어주세요."
	case model.DirectionHold:
		return "왜 지금은 움직이지 않는 것이 맞다고 보는지 적어주세요."
	default:
		return "왜 상승 쪽에 거는지, 무엇이 확인되면 이 판단이 틀린 것인지 적어주세요."
	}
}

func draftScoreLine(c model.ResearchCall) string {
	if c.ScoreAtPub == nil {
		return "종합 점수 없음 (산출에 필요한 데이터가 부족한 종목입니다)\n"
	}
	parts := []string{fmt.Sprintf("**종합 %d점(%s)**", *c.ScoreAtPub, c.GradeAtPub)}
	if c.Evidence.PeerPct != nil {
		parts = append(parts, fmt.Sprintf("동종 상위 %d%%", *c.Evidence.PeerPct))
	}
	if c.Evidence.Coverage > 0 {
		parts = append(parts, fmt.Sprintf("커버리지 %.0f%%", c.Evidence.Coverage*100))
	}
	line := strings.Join(parts, " · ") + "\n"

	axes := draftAxes(c.Evidence.Breakdown)
	if axes != "" {
		line += "\n" + axes + "\n"
	}
	return line
}

// draftAxes renders the five sub-scores. A missing axis is omitted rather than
// printed as 0 — the scoring design treats absent and zero as different facts.
func draftAxes(b model.ScoreBreakdown) string {
	labels := []struct {
		name string
		v    *float64
	}{
		{"가치", b.Valuation},
		{"수익성", b.Quality},
		{"성장", b.Growth},
		{"추세", b.Technical},
		{"심리", b.Sentiment},
	}
	parts := make([]string, 0, len(labels))
	for _, l := range labels {
		if l.v == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %.0f", l.name, *l.v))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " / ")
}

func draftScenario(heading string, sc *model.BriefingScenario, currency model.QuoteCurrency) string {
	if sc == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("### " + heading + "\n\n")
	if summary := strings.TrimSpace(sc.Summary); summary != "" {
		b.WriteString(summary + "\n")
	}
	for _, l := range sc.Levels {
		b.WriteString(fmt.Sprintf("- %s %s (%+.1f%%)\n",
			l.Label, formatDraftPrice(l.Price, currency), l.DistancePct))
	}
	return b.String()
}

func formatDraftPrice(price float64, currency model.QuoteCurrency) string {
	// Go's fmt has no thousands-separator verb, so group the digits by hand.
	if currency == model.CurrencyUSD {
		return "$" + groupThousands(fmt.Sprintf("%.2f", price))
	}
	return groupThousands(fmt.Sprintf("%.0f", price)) + "원"
}

// groupThousands inserts commas into the integer part of an already-formatted
// decimal string, preserving any sign and fractional part.
func groupThousands(s string) string {
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	intPart, frac := s, ""
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		intPart, frac = s[:dot], s[dot:]
	}
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return sign + b.String() + frac
}

// draftGapNote names what was unavailable, so the draft says "we did not have
// this" instead of implying the moment had no news or no score.
func draftGapNote(gaps []string) string {
	if len(gaps) == 0 {
		return ""
	}
	labels := map[string]string{
		"quote":       "실시간 시세",
		"score":       "종합 점수",
		"briefing":    "룰 기반 분석",
		"aiSummary":   "AI 요약",
		"news":        "뉴스",
		"disclosures": "공시",
		"chart":       "차트",
	}
	named := make([]string, 0, len(gaps))
	for _, g := range gaps {
		if l, ok := labels[g]; ok {
			named = append(named, l)
		}
	}
	if len(named) == 0 {
		return ""
	}
	return fmt.Sprintf("> 발행 시점에 확보하지 못한 데이터: %s\n", strings.Join(named, ", "))
}
