package service

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

// Facts payload for the AI 요약.
//
// The division of labour is deliberate: the server classifies, aggregates,
// deduplicates and normalizes; the LLM interprets, prioritizes, explains cause
// and effect, and writes. Anything the server can decide is decided here, so
// the prompt carries judgements rather than raw material.
//
// The previous payload shipped ~10.7KB per call — full TechnicalIndicators,
// four whole quarterly statements, ten insider transaction records and twelve
// article summaries — and the model spent its budget re-deriving facts we had
// already computed. Worse, none of it included the server-computed 종합 점수,
// so the summary and the number shown next to it were reasoning from different
// inputs.

// stockSummaryFacts is the compact payload sent to the model.
type stockSummaryFacts struct {
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	AsOf   string `json:"asOf,omitempty"`

	Market      *factsMarket      `json:"market,omitempty"`
	Score       *factsScore       `json:"score,omitempty"`
	Fundamental *factsFundamental `json:"fundamental,omitempty"`
	Earnings    *factsEarnings    `json:"earnings,omitempty"`
	Technical   *factsTechnical   `json:"technical,omitempty"`
	Insider     *factsInsider     `json:"insider,omitempty"`
	News        []factsArticle    `json:"news,omitempty"`

	// DataGaps names what we could not measure, so the model states less
	// rather than inventing coverage it does not have.
	DataGaps []string `json:"dataGaps,omitempty"`
}

type factsMarket struct {
	Price       float64  `json:"price"`
	Currency    string   `json:"currency,omitempty"`
	Drawdown52W *float64 `json:"drawdown52w,omitempty"`
	Return5D    *float64 `json:"return5d,omitempty"`
	Return20D   *float64 `json:"return20d,omitempty"`
}

// factsScore is the differentiator: our own cross-sectional score, already
// decomposed. Handing the model the axis tension ("성장 89 vs 가치 3") is what
// lets it explain *why* rather than restate numbers.
type factsScore struct {
	Total      int            `json:"total"`
	Grade      string         `json:"grade,omitempty"`
	TopPercent *int           `json:"peerTopPercent,omitempty"`
	PeerGroup  string         `json:"peerGroup,omitempty"`
	Axes       map[string]int `json:"axes,omitempty"`
	Strengths  []string       `json:"strengths,omitempty"`
	Weaknesses []string       `json:"weaknesses,omitempty"`
	Coverage   float64        `json:"coverage,omitempty"`
}

// factsFundamental pairs each ratio with its peer percentile. "PER 24.9" alone
// does not tell the model whether that is cheap; "상위 3%" does, and the server
// already computed it during the score batch.
type factsFundamental struct {
	PER             *float64 `json:"per,omitempty"`
	PERPeerPct      *int     `json:"perPeerPct,omitempty"`
	PBR             *float64 `json:"pbr,omitempty"`
	PBRPeerPct      *int     `json:"pbrPeerPct,omitempty"`
	ROE             *float64 `json:"roe,omitempty"`
	ROEPeerPct      *int     `json:"roePeerPct,omitempty"`
	OperatingMargin *float64 `json:"operatingMargin,omitempty"`
	DebtRatio       *float64 `json:"debtRatio,omitempty"`
	RevenueYoY      *float64 `json:"revenueYoY,omitempty"`
	EPSYoY          *float64 `json:"epsYoY,omitempty"`
}

type factsEarnings struct {
	RevenueQoQ         *float64 `json:"revenueQoQ,omitempty"`
	OperatingProfitQoQ *float64 `json:"operatingProfitQoQ,omitempty"`
	NetProfitQoQ       *float64 `json:"netProfitQoQ,omitempty"`
	Trend              string   `json:"trend,omitempty"`
}

type factsTechnical struct {
	Trend        string   `json:"trend,omitempty"`
	BelowMajorMA *bool    `json:"belowMajorMA,omitempty"`
	RSI          *float64 `json:"rsi,omitempty"`
	RSIState     string   `json:"rsiState,omitempty"`
	MACD         string   `json:"macd,omitempty"`
}

type factsInsider struct {
	Buy       int    `json:"buy"`
	Sell      int    `json:"sell"`
	Direction string `json:"direction,omitempty"`
}

type factsArticle struct {
	ID       string `json:"id"`
	Headline string `json:"headline"`
}

// ---- classification (server's job) ----------------------------------------

func classifyRSI(rsi float64) string {
	switch {
	case rsi >= 70:
		return "overbought"
	case rsi >= 55:
		return "strong"
	case rsi >= 45:
		return "neutral"
	case rsi > 30:
		return "weak"
	default:
		return "oversold"
	}
}

// classifyMACD distinguishes a cross that has already happened from one that is
// merely closing, which is the difference between "recovering" and "bearish".
func classifyMACD(macd, signal float64) string {
	diff := macd - signal
	scale := math.Abs(signal)
	if scale < 1e-9 {
		scale = math.Abs(macd)
	}
	if scale < 1e-9 {
		return "flat"
	}
	ratio := diff / scale
	switch {
	case ratio > 0.15:
		return "bullish"
	case ratio > 0:
		return "recovering"
	case ratio > -0.15:
		return "weakening"
	default:
		return "bearish"
	}
}

func classifyTrend(t *TechnicalIndicators) (trend string, belowMajor *bool) {
	if t == nil || t.LastClose == nil {
		return "", nil
	}
	last := *t.LastClose
	var above, total int
	for _, ma := range []*float64{t.SMA20, t.SMA50, t.SMA200} {
		if ma == nil || *ma <= 0 {
			continue
		}
		total++
		if last >= *ma {
			above++
		}
	}
	if total == 0 {
		return "", nil
	}
	below := above == 0
	switch {
	case above == total:
		trend = "bullish"
	case above == 0:
		trend = "bearish"
	default:
		trend = "mixed"
	}
	return trend, &below
}

// classifyEarningsTrend reads the direction and force of the latest quarter.
// Counting how many lines improved is a server job; judging what it means for
// the company is not.
func classifyEarningsTrend(revenue, operating, net *float64) string {
	vals := []*float64{revenue, operating, net}
	var up, down, seen int
	strong := false
	for _, v := range vals {
		if v == nil {
			continue
		}
		seen++
		switch {
		case *v > 0:
			up++
			if *v >= 30 {
				strong = true
			}
		case *v < 0:
			down++
		}
	}
	if seen == 0 {
		return ""
	}
	switch {
	case up == seen && strong:
		return "strong_improving"
	case up == seen:
		return "improving"
	case down == seen:
		return "deteriorating"
	default:
		return "mixed"
	}
}

func insiderDirection(buy, sell int) string {
	switch {
	case buy == 0 && sell == 0:
		return ""
	case buy > sell*2:
		return "net_buying"
	case sell > buy*2:
		return "net_selling"
	case buy > sell:
		return "slight_buying"
	case sell > buy:
		return "slight_selling"
	default:
		return "balanced"
	}
}

// dedupeHeadlines drops near-identical wire copy and keeps only the headline.
//
// Reuses the news pipeline's own dedupe (news_dedupe.go) rather than a second
// notion of "same story": it already handles publisher suffixes and translated
// duplicates, and a divergent rule here would make the sentiment aggregate
// disagree with what the 뉴스 tab shows. Sending the same story five times
// costs tokens twice over and skews sentiment toward whatever was syndicated
// most.
//
// Summaries are dropped on purpose — headlines carry enough for both sentiment
// labelling and event extraction, and the summaries were the bulk of the old
// payload.
func dedupeHeadlines(articles []model.NewsArticle, limit int) []factsArticle {
	deduped := dedupeNewsArticles(articles)
	out := make([]factsArticle, 0, limit)
	for _, a := range deduped {
		headline := strings.TrimSpace(a.HeadlineKo)
		if headline == "" {
			headline = strings.TrimSpace(a.Headline)
		}
		if headline == "" {
			continue
		}
		out = append(out, factsArticle{ID: a.ID, Headline: headline})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// ---- assembly --------------------------------------------------------------

func pctPtr(v float64) *int {
	i := int(v + 0.5)
	return &i
}

// round1 trims a ratio to one decimal. Sending per=24.934903853743542 spends
// tokens on digits that are not meaningful — the underlying inputs are
// quarterly filings — and invites the model to echo false precision.
func round1(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := math.Round(*v*10) / 10
	return &r
}

// round2 keeps cents for prices, where the second decimal is real for USD.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// axisRank names the strongest and weakest axes so the model does not have to
// sort, and so its narrative always agrees with the number on screen.
func axisStrengthsWeaknesses(axes map[string]int) (strengths, weaknesses []string) {
	type kv struct {
		k string
		v int
	}
	items := make([]kv, 0, len(axes))
	for k, v := range axes {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].v != items[j].v {
			return items[i].v > items[j].v
		}
		return items[i].k < items[j].k
	})
	for i, it := range items {
		if i < 2 && it.v >= 60 {
			strengths = append(strengths, it.k)
		}
		if i >= len(items)-2 && it.v <= 40 {
			weaknesses = append(weaknesses, it.k)
		}
	}
	return strengths, weaknesses
}

// buildStockSummaryFacts assembles the compact payload. Every number the model
// may cite must appear here, because the prompt forbids inventing values.
func buildStockSummaryFacts(
	stock *model.Stock,
	fundamentals *model.FundamentalsView,
	history []model.FinancialHistoryPeriod,
	insiderTxns []model.InsiderTransaction,
	technical *TechnicalIndicators,
	articles []model.NewsArticle,
	score *model.StockScoreView,
) stockSummaryFacts {
	facts := stockSummaryFacts{
		Name:   stock.Name,
		Symbol: stock.Symbol,
	}
	if score != nil {
		facts.AsOf = score.TradeDate
	}

	if technical != nil && technical.LastClose != nil {
		facts.Market = &factsMarket{
			Price:       round2(*technical.LastClose),
			Currency:    string(stock.Currency),
			Drawdown52W: round1(technical.PctFromHigh52Week),
			Return5D:    round1(technical.Return5D),
			Return20D:   round1(technical.Return20D),
		}
	}

	var peerPct map[string]float64
	if score != nil && score.Score != nil {
		axes := map[string]int{}
		for _, axis := range model.ScoreAxes {
			if v := score.Breakdown.Get(axis); v != nil {
				axes[string(axis)] = int(*v + 0.5)
			}
		}
		strengths, weaknesses := axisStrengthsWeaknesses(axes)
		facts.Score = &factsScore{
			Total:      *score.Score,
			Grade:      score.Grade,
			TopPercent: score.PeerPct,
			PeerGroup:  score.PeerGroup,
			Axes:       axes,
			Strengths:  strengths,
			Weaknesses: weaknesses,
			Coverage:   math.Round(score.Coverage*100) / 100,
		}
		peerPct = scorePercentiles(score)
	}

	if fundamentals != nil {
		f := &factsFundamental{
			PER:             round1(fundamentals.PER),
			PBR:             round1(fundamentals.PBR),
			ROE:             round1(fundamentals.ROE),
			OperatingMargin: round1(fundamentals.OperatingMargin),
			DebtRatio:       round1(fundamentals.DebtRatio),
			RevenueYoY:      round1(fundamentals.RevenueGrowthYoY),
			EPSYoY:          round1(fundamentals.EPSGrowthYoY),
		}
		if v, ok := peerPct["per"]; ok {
			f.PERPeerPct = pctPtr(v)
		}
		if v, ok := peerPct["pbr"]; ok {
			f.PBRPeerPct = pctPtr(v)
		}
		if v, ok := peerPct["roe"]; ok {
			f.ROEPeerPct = pctPtr(v)
		}
		facts.Fundamental = f
	}

	if len(history) > 0 {
		latest := history[len(history)-1]
		e := &factsEarnings{
			RevenueQoQ:         round1(latest.RevenueGrowthQoQ),
			OperatingProfitQoQ: round1(latest.OperatingIncomeGrowthQoQ),
			NetProfitQoQ:       round1(latest.NetIncomeGrowthQoQ),
		}
		e.Trend = classifyEarningsTrend(e.RevenueQoQ, e.OperatingProfitQoQ, e.NetProfitQoQ)
		if e.RevenueQoQ != nil || e.OperatingProfitQoQ != nil || e.NetProfitQoQ != nil {
			facts.Earnings = e
		}
	}

	if technical != nil {
		t := &factsTechnical{}
		t.Trend, t.BelowMajorMA = classifyTrend(technical)
		if technical.RSI14 != nil {
			t.RSI = round1(technical.RSI14)
			t.RSIState = classifyRSI(*technical.RSI14)
		}
		if technical.MACD != nil && technical.MACDSignal != nil {
			t.MACD = classifyMACD(*technical.MACD, *technical.MACDSignal)
		}
		if t.Trend != "" || t.RSI != nil || t.MACD != "" {
			facts.Technical = t
		}
	}

	if len(insiderTxns) > 0 {
		buy, sell := 0, 0
		for _, tx := range insiderTxns {
			if tx.SharesChange == nil {
				continue
			}
			if *tx.SharesChange > 0 {
				buy++
			} else if *tx.SharesChange < 0 {
				sell++
			}
		}
		if buy+sell > 0 {
			facts.Insider = &factsInsider{Buy: buy, Sell: sell, Direction: insiderDirection(buy, sell)}
		}
	}

	facts.News = dedupeHeadlines(articles, stockSummaryNewsLimit)

	// Naming the gaps is cheaper than letting the model discover them by
	// omission and hedge across the whole summary.
	if facts.Fundamental == nil {
		facts.DataGaps = append(facts.DataGaps, "fundamentals")
	}
	if facts.Technical == nil {
		facts.DataGaps = append(facts.DataGaps, "technical")
	}
	if facts.Earnings == nil {
		facts.DataGaps = append(facts.DataGaps, "earnings")
	}
	if facts.Insider == nil {
		facts.DataGaps = append(facts.DataGaps, "insider")
	}
	if len(facts.News) == 0 {
		facts.DataGaps = append(facts.DataGaps, "news")
	}
	return facts
}

// scorePercentiles pulls the peer percentiles the score batch already computed
// out of its stored inputs blob.
func scorePercentiles(score *model.StockScoreView) map[string]float64 {
	out := map[string]float64{}
	if score == nil || score.Inputs == nil {
		return out
	}
	raw, ok := score.Inputs["percentile"].(map[string]any)
	if !ok {
		return out
	}
	for k, v := range raw {
		if f, ok := toFloat(v); ok {
			out[k] = f
		}
	}
	return out
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	}
	return 0, false
}

// describeScoreForPrompt renders a one-line orientation the model reads before
// the JSON, so the narrative is anchored to the number the user sees.
func describeScoreForPrompt(facts stockSummaryFacts) string {
	if facts.Score == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	parts = append(parts, fmt.Sprintf("종합 %d점(%s)", facts.Score.Total, facts.Score.Grade))
	if facts.Score.TopPercent != nil {
		parts = append(parts, fmt.Sprintf("동종 상위 %d%%", *facts.Score.TopPercent))
	}
	return strings.Join(parts, ", ")
}
