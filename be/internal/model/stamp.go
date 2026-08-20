package model

import "time"

// StampStance is the user's hypothesis at stamping time. It is what makes a
// stamp scoreable later — without it a stamp is only a bookmark.
//
// Since v2 the stance is *derived* from the target price rather than picked in
// the form: a target above the captured price is bullish, below is bearish, and
// within StampNeutralBandPct either way is 관망. Deriving it makes the
// contradictory combination ("약세인데 목표가는 위") structurally impossible.
// It is still stored, because the derivation uses the captured price and that
// price must not be re-read later.
type StampStance string

const (
	StanceBullish StampStance = "bullish"
	StanceBearish StampStance = "bearish"
	StanceNeutral StampStance = "neutral"
)

// StampNeutralBandPct is how far price and target may sit apart and still count
// as 관망 — and, once the horizon elapses, how far a 관망 stamp may drift and
// still count as right. One number so the two rules cannot disagree.
const StampNeutralBandPct = 3.0

// StanceForTarget derives the stance from the captured price and the target.
func StanceForTarget(price, target float64) StampStance {
	if price <= 0 || target <= 0 {
		return StanceNeutral
	}
	diff := (target - price) / price * 100
	switch {
	case diff > StampNeutralBandPct:
		return StanceBullish
	case diff < -StampNeutralBandPct:
		return StanceBearish
	default:
		return StanceNeutral
	}
}

// StampHorizonKind labels how the user chose the verification deadline. The
// deadline itself is frozen in Stamp.DueAt at creation time — recomputing it
// later would let a holiday-calendar update move a past stamp's verdict.
type StampHorizonKind string

const (
	// HorizonSessionClose is today's regular close (or the next one if today's
	// has already passed when the stamp is taken).
	HorizonSessionClose StampHorizonKind = "session_close"
	// HorizonNextSessionClose is the close after HorizonSessionClose.
	HorizonNextSessionClose StampHorizonKind = "next_session_close"
	Horizon1W               StampHorizonKind = "1w"
	Horizon1M               StampHorizonKind = "1m"
	Horizon3M               StampHorizonKind = "3m"
	// HorizonCustom is a user-picked calendar date, settled at that day's close.
	HorizonCustom StampHorizonKind = "custom"
)

// StampVerdict is the automatic judgement once the horizon has elapsed.
type StampVerdict string

const (
	VerdictPending StampVerdict = "pending"
	VerdictHit     StampVerdict = "hit"
	VerdictMiss    StampVerdict = "miss"
	// VerdictUnknown means the horizon passed but we have no close to judge with.
	VerdictUnknown StampVerdict = "unknown"
)

// Stamp is one captured moment. Everything except the hypothesis fields
// (stance/conviction/horizon/note/tags) is immutable after creation.
type Stamp struct {
	ID            int64          `json:"id"`
	Symbol        string         `json:"symbol"`
	SymbolName    string         `json:"symbolName,omitempty"`
	AssetType     QuoteAssetType `json:"assetType,omitempty"`
	Currency      QuoteCurrency  `json:"currency,omitempty"`
	StampedAt     time.Time      `json:"stampedAt"`
	TradeDate     string         `json:"tradeDate"`
	MarketSession string         `json:"marketSession,omitempty"`
	BenchmarkID   string         `json:"benchmarkId,omitempty"`

	Price      float64  `json:"price"`
	PrevClose  *float64 `json:"prevClose,omitempty"`
	ChangeRate *float64 `json:"changeRate,omitempty"`
	Volume     int64    `json:"volume"`
	Score      *int     `json:"score,omitempty"`
	Grade      string   `json:"grade,omitempty"`

	// Stance is derived from TargetPrice vs Price (see StanceForTarget) and
	// frozen here so later reads never re-derive it from a moved price.
	Stance      StampStance      `json:"stance"`
	TargetPrice float64          `json:"targetPrice"`
	Conviction  int              `json:"conviction"`
	HorizonKind StampHorizonKind `json:"horizonKind"`
	// DueAt is the verification deadline, settled against the market calendar
	// when the stamp is created.
	DueAt time.Time `json:"dueAt"`
	Note  string    `json:"note"`
	Tags  []string  `json:"tags,omitempty"`

	Snapshot StampSnapshot `json:"snapshot"`
	Context  StampContext  `json:"context"`
	Chart    StampChart    `json:"chart"`

	OpenedAt  *time.Time `json:"openedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// StampSnapshot is the quote/valuation/score state at stamping time.
type StampSnapshot struct {
	Open         float64  `json:"open,omitempty"`
	High         float64  `json:"high,omitempty"`
	Low          float64  `json:"low,omitempty"`
	Week52High   float64  `json:"week52High,omitempty"`
	Week52Low    float64  `json:"week52Low,omitempty"`
	MarketCap    float64  `json:"marketCap,omitempty"`
	TradingValue float64  `json:"tradingValue,omitempty"`
	PER          *float64 `json:"per,omitempty"`
	PBR          *float64 `json:"pbr,omitempty"`

	ScoreKind string         `json:"scoreKind,omitempty"`
	Breakdown ScoreBreakdown `json:"breakdown"`
	Coverage  float64        `json:"coverage,omitempty"`
	PeerGroup string         `json:"peerGroup,omitempty"`
	PeerPct   *int           `json:"peerPct,omitempty"`

	Watchlisted     bool     `json:"watchlisted"`
	Held            bool     `json:"held"`
	HoldingQuantity float64  `json:"holdingQuantity,omitempty"`
	HoldingAvgPrice *float64 `json:"holdingAvgPrice,omitempty"`
}

// StampContext is the "what was going on" material copied at stamping time.
// Disclosures in particular have no historical table, so they cannot be
// reconstructed later — see docs/STAMP_DESIGN.md §5.2.
type StampContext struct {
	AISummary   string          `json:"aiSummary,omitempty"`
	AIKeywords  []string        `json:"aiKeywords,omitempty"`
	News        []StampNews     `json:"news"`
	Disclosures []BriefingEvent `json:"disclosures"`
	Indices     []StampIndex    `json:"indices"`
	// DataGaps names what was unavailable at capture time so the UI can say so
	// instead of implying the moment had no news/score/chart.
	DataGaps []string `json:"dataGaps,omitempty"`
}

type StampNews struct {
	ID            string    `json:"id,omitempty"`
	Headline      string    `json:"headline"`
	URL           string    `json:"url,omitempty"`
	PublisherName string    `json:"publisherName,omitempty"`
	PublishedAt   time.Time `json:"publishedAt"`
}

type StampIndex struct {
	ID         string  `json:"id"`
	Name       string  `json:"name,omitempty"`
	Value      float64 `json:"value"`
	ChangeRate float64 `json:"changeRate"`
}

// StampChart freezes the bars that were on screen. Daily bars could be replayed
// from candle_bars, but intraday coverage is not guaranteed, so the visible
// window is stored with the stamp.
type StampChart struct {
	Interval string   `json:"interval"`
	Bars     []Candle `json:"bars"`
}

// StampView is a stamp plus everything computed at read time (never stored).
// DueAt is not here — it is stored on the Stamp and reaches the client through
// the embedded struct.
type StampView struct {
	Stamp
	Current *StampCurrent `json:"current,omitempty"`
	Returns []StampReturn `json:"returns"`
	// Horizon is the return over the user's own horizon (nil until it elapses).
	Horizon *StampReturn `json:"horizon,omitempty"`
	Verdict StampVerdict `json:"verdict"`

	// TargetHit is set when the price reached the target intraday inside the
	// verification window. That closes the stamp early as a hit — waiting for
	// the deadline would score "목표에 닿았다가 되돌아왔다" as a miss.
	TargetHit *StampTargetHit `json:"targetHit,omitempty"`
	// TargetAccuracy is |settled − target| ÷ target × 100, filled once the stamp
	// settles. It never decides the verdict — direction does — so a stamp can be
	// 적중 with a poor accuracy ("방향은 맞췄고 폭은 틀렸다").
	TargetAccuracy *float64 `json:"targetAccuracy,omitempty"`
	// TargetProgress is how far the current price has travelled from the stamp
	// price toward the target: 100 means reached, negative means it moved the
	// other way. Nil for 관망 stamps, where "progress" has no direction.
	TargetProgress *float64 `json:"targetProgress,omitempty"`
	// ExpectedReturn is the move the target implies, measured from the captured
	// price — what the user was predicting, in percent.
	ExpectedReturn *float64 `json:"expectedReturn,omitempty"`
	// PeakReturn and TroughReturn are the best and worst the price reached since
	// the stamp, in percent. A hit that spent a week 8% underwater and one that
	// went straight up are not the same call.
	PeakReturn   *float64 `json:"peakReturn,omitempty"`
	TroughReturn *float64 `json:"troughReturn,omitempty"`

	Due    bool `json:"due"`
	Opened bool `json:"opened"`
}

// StampTargetHit records the bar that first reached the target.
type StampTargetHit struct {
	Date string `json:"date"`
	// Price is the high (bullish) or low (bearish) that reached the target.
	Price float64 `json:"price"`
}

// StampCurrent is the "지금" side of the 당시 vs 현재 comparison.
type StampCurrent struct {
	Price      float64        `json:"price"`
	ChangeRate *float64       `json:"changeRate,omitempty"` // vs stamp price
	Score      *int           `json:"score,omitempty"`
	Grade      string         `json:"grade,omitempty"`
	Breakdown  ScoreBreakdown `json:"breakdown"`
	TradeDate  string         `json:"tradeDate,omitempty"`
	Source     string         `json:"source,omitempty"` // quote | close
}

// StampReturn is one horizon bucket. Nil rates mean "not there yet" and render
// as "—" rather than 0.
type StampReturn struct {
	Horizon      string   `json:"horizon"` // 1D | 1W | 1M | 3M | horizon
	AsOfDate     string   `json:"asOfDate,omitempty"`
	Price        *float64 `json:"price,omitempty"`
	Return       *float64 `json:"return,omitempty"`
	BenchmarkID  string   `json:"benchmarkId,omitempty"`
	BenchmarkRet *float64 `json:"benchmarkReturn,omitempty"`
	Alpha        *float64 `json:"alpha,omitempty"`
}

// StampSummary powers the header of the 스탬프 탭.
type StampSummary struct {
	Total       int      `json:"total"`
	Symbols     int      `json:"symbols"`
	DueCount    int      `json:"dueCount"`
	Judged      int      `json:"judged"`
	Hits        int      `json:"hits"`
	HitRate     *float64 `json:"hitRate,omitempty"`
	AvgReturn   *float64 `json:"avgReturn,omitempty"`
	AvgAlpha    *float64 `json:"avgAlpha,omitempty"`
	LastStamped *string  `json:"lastStampedAt,omitempty"`

	// AvgTargetError is the mean |settled − target| ÷ target across settled
	// stamps — the second axis the 성적표 reports next to the hit rate.
	AvgTargetError *float64 `json:"avgTargetError,omitempty"`
	// WithinBand counts settled stamps whose price landed within
	// StampNeutralBandPct of the target ("폭까지 맞춘" stamps).
	WithinBand int `json:"withinBand"`
	// EarlyHits counts stamps closed by an intraday touch of the target.
	EarlyHits int `json:"earlyHits"`
}

// StampState drives the stamp button on the stock page.
type StampState struct {
	Symbol            string     `json:"symbol"`
	Count             int        `json:"count"`
	LastStampedAt     *time.Time `json:"lastStampedAt,omitempty"`
	CooldownRemaining int        `json:"cooldownRemainingSeconds"`
}
