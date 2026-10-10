package model

import "time"

// Research call — a published, checkable stock thesis (docs/RESEARCH_CALL_DESIGN.md).
//
// The rule that shapes every type here: what the author *claims* and what the
// server *measured* are stored separately, and only the claim's prose can ever
// be edited. A call whose numbers could be revised after the fact would make
// the leaderboard an advertisement instead of a scorecard.

// ResearchDirection is the author's call.
type ResearchDirection string

const (
	DirectionBuy  ResearchDirection = "buy"
	DirectionSell ResearchDirection = "sell"
	DirectionHold ResearchDirection = "hold"
)

// ResearchVisibility controls who can read a call. Turning a call private does
// not remove it from the author's own statistics — see ResearchStatus.
type ResearchVisibility string

const (
	VisibilityPublic   ResearchVisibility = "public"
	VisibilityUnlisted ResearchVisibility = "unlisted"
	VisibilityPrivate  ResearchVisibility = "private"
)

// ResearchStatus is the verdict state. Judgement uses closes only — an intraday
// wick through the target on a thin symbol is not a call coming true.
type ResearchStatus string

const (
	// StatusOpen means the horizon is still running and neither bound was hit.
	StatusOpen ResearchStatus = "open"
	// StatusTargetHit means a close reached the target price.
	StatusTargetHit ResearchStatus = "target_hit"
	// StatusStopHit means a close reached the stop price.
	StatusStopHit ResearchStatus = "stop_hit"
	// StatusExpiredHit means the horizon elapsed with the direction right.
	StatusExpiredHit ResearchStatus = "expired_hit"
	// StatusExpiredMiss means the horizon elapsed with the direction wrong.
	StatusExpiredMiss ResearchStatus = "expired_miss"
	// StatusExpiredUnknown means the horizon elapsed but no close exists to
	// judge with. It is never counted as a hit or a miss.
	StatusExpiredUnknown ResearchStatus = "expired_unknown"
	// StatusRetracted means the author withdrew it. The row stays and the
	// retraction is reported — that is the whole point.
	StatusRetracted ResearchStatus = "retracted"
	// StatusNote — 부동산·금융상품 「분석 리포트」(2026-10-09 요청 9). 종목·목표가·판정이 없고 성적에 들지 않는다.
	StatusNote ResearchStatus = "note"
)

// Resolved reports whether a status is a finished judgement.
func (s ResearchStatus) Resolved() bool {
	switch s {
	case StatusTargetHit, StatusStopHit, StatusExpiredHit, StatusExpiredMiss:
		return true
	}
	return false
}

// Hit reports whether a resolved status counts toward the hit rate.
func (s ResearchStatus) Hit() bool {
	return s == StatusTargetHit || s == StatusExpiredHit
}

// ResearchHorizons are the only accepted horizons. Free-form day counts would
// make the "same horizon" comparisons on the leaderboard meaningless.
var ResearchHorizons = []int{30, 90, 180, 365}

// ResearchCall is one published thesis. Every field above `body_md` is frozen
// at publish time.
type ResearchCall struct {
	ID         int64          `json:"id"`
	UserID     int64          `json:"userId"`
	AuthorName string         `json:"authorName,omitempty"`
	// Topic — stock | real_estate | finance (model.Topic*). stock이 아니면 Symbol은 빈 값이다.
	Topic      string         `json:"topic"`
	Symbol     string         `json:"symbol"`
	SymbolName string         `json:"symbolName,omitempty"`
	AssetType  QuoteAssetType `json:"assetType,omitempty"`
	Currency   QuoteCurrency  `json:"currency,omitempty"`
	Title      string         `json:"title"`

	PublishedAt time.Time `json:"publishedAt"`
	TradeDate   string    `json:"tradeDate"`
	BenchmarkID string    `json:"benchmarkId,omitempty"`

	Direction   ResearchDirection `json:"direction"`
	BasePrice   float64           `json:"basePrice"`
	TargetPrice *float64          `json:"targetPrice,omitempty"`
	StopPrice   *float64          `json:"stopPrice,omitempty"`
	HorizonDays int               `json:"horizonDays"`
	Conviction  int               `json:"conviction"` // 0~100 (%)

	ScoreAtPub *int   `json:"scoreAtPublish,omitempty"`
	GradeAtPub string `json:"gradeAtPublish,omitempty"`
	// Contrarian marks a call that runs against our own score. It is computed
	// once, at publish time, from the frozen score.
	Contrarian bool `json:"contrarian"`

	Evidence ResearchEvidence `json:"evidence"`
	Context  ResearchContext  `json:"context"`
	Chart    ResearchChart    `json:"chart"`

	BodyMD  string `json:"bodyMd"`
	BodyRev int    `json:"bodyRev"`
	/**
	 * Cover image for the list card. Empty means the screen falls back to the
	 * stock's logo — the point is that every card has something, not that every
	 * author must find a picture.
	 */
	ThumbnailURL string             `json:"thumbnailUrl,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	Visibility   ResearchVisibility `json:"visibility"`

	Status        ResearchStatus `json:"status"`
	VerdictAt     *time.Time     `json:"verdictAt,omitempty"`
	VerdictDate   string         `json:"verdictDate,omitempty"`
	VerdictReturn *float64       `json:"verdictReturn,omitempty"`
	VerdictAlpha  *float64       `json:"verdictAlpha,omitempty"`

	ViewCount   int        `json:"viewCount"`
	RetractedAt *time.Time `json:"retractedAt,omitempty"`
	HiddenAt    *time.Time `json:"hiddenAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// ResearchEvidence is what the server measured at publish time. The author
// never supplies any of it.
type ResearchEvidence struct {
	PrevClose  *float64 `json:"prevClose,omitempty"`
	ChangeRate *float64 `json:"changeRate,omitempty"`
	Volume     int64    `json:"volume,omitempty"`

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

	MarketSession string `json:"marketSession,omitempty"`
	// Briefing is the rule-based read (투자포인트·시나리오·주요 가격대) as it
	// stood at publish time. It costs no LLM call and anchors the draft.
	Briefing *MarketBriefing `json:"briefing,omitempty"`
}

// ResearchContext is the "what was going on" material copied at publish time.
//
// Research owns this shape rather than borrowing the stamp one: the two
// features are independent, and a frozen snapshot format that can change
// because a different feature evolved is not frozen.
type ResearchContext struct {
	AISummary   string          `json:"aiSummary,omitempty"`
	AIKeywords  []string        `json:"aiKeywords,omitempty"`
	News        []ResearchNews  `json:"news"`
	Disclosures []BriefingEvent `json:"disclosures"`
	Indices     []ResearchIndex `json:"indices"`
	// DataGaps names what was unavailable at publish time so the UI can say so
	// instead of implying the moment had no news/score/chart.
	DataGaps []string `json:"dataGaps,omitempty"`
}

type ResearchNews struct {
	ID            string    `json:"id,omitempty"`
	Headline      string    `json:"headline"`
	URL           string    `json:"url,omitempty"`
	PublisherName string    `json:"publisherName,omitempty"`
	PublishedAt   time.Time `json:"publishedAt"`
}

type ResearchIndex struct {
	ID         string  `json:"id"`
	Name       string  `json:"name,omitempty"`
	Value      float64 `json:"value"`
	ChangeRate float64 `json:"changeRate"`
}

// ResearchChart freezes the bars that were on screen at publish time.
//
// 상세 응답은 `bars`(OHLCV 전량)를, 목록 응답은 `closes`(종가만, 솎아낸 것)를
// 채운다. 둘 다 채우지는 않는다 — 목록 카드가 그리는 스파크라인은 종가만 읽는데,
// OHLCV 200봉은 리포트 한 건에 19KB라 목록 응답의 대부분을 차지했다.
type ResearchChart struct {
	Interval string               `json:"interval"`
	Bars     []Candle             `json:"bars,omitempty"`
	Closes   []ResearchClosePoint `json:"closes,omitempty"`
}

// ResearchClosePoint is one sparkline point — everything the list cards read.
type ResearchClosePoint struct {
	Time  string  `json:"time"`
	Close float64 `json:"close"`
}

// ResearchCurrent is the "지금" side of the 발행 시점 vs 현재 comparison.
type ResearchCurrent struct {
	Price      float64  `json:"price"`
	ChangeRate *float64 `json:"changeRate,omitempty"` // vs base price
	TradeDate  string   `json:"tradeDate,omitempty"`
	Source     string   `json:"source,omitempty"` // quote | close
}

// ResearchReturn is one horizon bucket. Nil rates mean "not there yet" and
// render as "—" rather than 0.
type ResearchReturn struct {
	Horizon      string   `json:"horizon"` // 1W | 1M | 3M | horizon
	AsOfDate     string   `json:"asOfDate,omitempty"`
	Price        *float64 `json:"price,omitempty"`
	Return       *float64 `json:"return,omitempty"`
	BenchmarkID  string   `json:"benchmarkId,omitempty"`
	BenchmarkRet *float64 `json:"benchmarkReturn,omitempty"`
	Alpha        *float64 `json:"alpha,omitempty"`
}

// ResearchView is a call plus everything computed at read time.
type ResearchView struct {
	ResearchCall
	// Mentions — 본문의 데이터 태그(상세에서만, 2026-09-28).
	Mentions *MentionSet `json:"mentions,omitempty"`
	Current   *ResearchCurrent       `json:"current,omitempty"`
	Returns   []ResearchReturn       `json:"returns"`
	Horizon   *ResearchReturn        `json:"horizon,omitempty"`
	DueAt     time.Time              `json:"dueAt"`
	Due       bool                   `json:"due"`
	Progress  float64                `json:"progress"` // 0~1 elapsed fraction of the horizon
	ToTarget  *float64               `json:"toTargetPct,omitempty"`
	ToStop    *float64               `json:"toStopPct,omitempty"`
	Expected  *ResearchExpectedValue `json:"expectedValue,omitempty"`
	CanEdit   bool                   `json:"canEdit"`
	Revisions int                    `json:"revisions"`
	// Reactions/Comments are the social layer (Phase 2~3).
	LikeCount     int  `json:"likeCount"`
	CommentCount  int  `json:"commentCount"`
	BookmarkCount int  `json:"bookmarkCount"`
	Liked         bool `json:"liked"`
	Bookmarked    bool `json:"bookmarked"`
	// Rating is the 3-axis reader score. Nil until someone rates it.
	Rating *ResearchRatingSummary `json:"rating,omitempty"`
	// MyRating is the viewer's own rating, if they left one.
	MyRating *ResearchRating `json:"myRating,omitempty"`
	// CanRate is false once the verdict is in — rating a settled call would
	// just be re-scoring the outcome (§10).
	CanRate bool `json:"canRate"`
}

// ResearchExpectedValue turns the target/stop pair into a number, twice: once
// with the author's stated conviction and once with their measured hit rate.
// The gap between the two is the point.
type ResearchExpectedValue struct {
	UpsidePct   float64  `json:"upsidePct"`
	DownsidePct float64  `json:"downsidePct"`
	RiskReward  *float64 `json:"riskReward,omitempty"`
	// Stated uses the author's own conviction as P(target).
	Stated *float64 `json:"statedEV,omitempty"`
	// Historical uses the author's realised hit rate instead. Nil until they
	// have enough resolved calls to have one.
	Historical  *float64 `json:"historicalEV,omitempty"`
	HitRateUsed *float64 `json:"hitRateUsed,omitempty"`
}

// ResearchTargetOdds is the measured answer to "has this symbol ever actually
// moved that far in this much time?", computed from stored daily bars.
type ResearchTargetOdds struct {
	HorizonDays int      `json:"horizonDays"`
	RequiredPct float64  `json:"requiredPct"`
	Windows     int      `json:"windows"`
	Reached     int      `json:"reached"`
	Probability *float64 `json:"probability,omitempty"` // 0~100
	BestPct     *float64 `json:"bestPct,omitempty"`
	MedianPct   *float64 `json:"medianPct,omitempty"`
	FromDate    string   `json:"fromDate,omitempty"`
	ToDate      string   `json:"toDate,omitempty"`
	// Insufficient marks "we do not have enough history to say" — which the UI
	// must render as such rather than as a low probability.
	Insufficient bool `json:"insufficient"`
}

// ResearchPreview is what the compose screen shows before anything is stored.
type ResearchPreview struct {
	Symbol      string           `json:"symbol"`
	SymbolName  string           `json:"symbolName,omitempty"`
	AssetType   QuoteAssetType   `json:"assetType,omitempty"`
	Currency    QuoteCurrency    `json:"currency,omitempty"`
	BasePrice   float64          `json:"basePrice"`
	BenchmarkID string           `json:"benchmarkId,omitempty"`
	Score       *int             `json:"score,omitempty"`
	Grade       string           `json:"grade,omitempty"`
	Contrarian  bool             `json:"contrarian"`
	Evidence    ResearchEvidence `json:"evidence"`
	Context     ResearchContext  `json:"context"`
	// DraftMD is the short prompt shown in the empty editor. The compose screen
	// clears it on the author's first click and never publishes it.
	DraftMD string `json:"draftMd"`
	// DataMD is the 발행 시점 데이터 section (점수·투자 포인트·가격대·뉴스),
	// inserted only when the author asks for it. Rule-based, no LLM.
	DataMD string `json:"dataMd"`
	// Levels are the real derived prices a target may be snapped to.
	Levels []PriceLevel `json:"levels"`
	// Odds is filled only when the request carried a target price.
	Odds     *ResearchTargetOdds    `json:"odds,omitempty"`
	Expected *ResearchExpectedValue `json:"expectedValue,omitempty"`
	// Eligible reports whether this symbol may be published publicly (§14 게이트).
	Eligible         bool   `json:"eligible"`
	IneligibleReason string `json:"ineligibleReason,omitempty"`
	// DailyCap / DailyRemaining 은 발행 확인 화면이 「오늘 몇 건 남았는지」를
	// 말할 수 있게 한다. 발행은 되돌릴 수 없고 한도를 소모하므로, 누르기 전에
	// 남은 수를 보여주지 않으면 사용자는 그 대가를 모르고 결정하게 된다.
	DailyCap       int `json:"dailyCap"`
	DailyRemaining int `json:"dailyRemaining"`
}

// ResearchSummary is the author's scorecard.
type ResearchSummary struct {
	Total     int      `json:"total"`
	Open      int      `json:"open"`
	Resolved  int      `json:"resolved"`
	Hits      int      `json:"hits"`
	Retracted int      `json:"retracted"`
	Symbols   int      `json:"symbols"`
	HitRate   *float64 `json:"hitRate,omitempty"`
	AvgReturn *float64 `json:"avgReturn,omitempty"`
	AvgAlpha  *float64 `json:"avgAlpha,omitempty"`
	// RetractRate is shown next to the hit rate on purpose: a hit rate without
	// it can be manufactured by withdrawing the losers.
	RetractRate *float64 `json:"retractRate,omitempty"`
	// Calibration is the mean squared error between stated conviction and
	// outcome (Brier score, 0 = perfect). Nil until there are resolved calls.
	Calibration *float64 `json:"calibration,omitempty"`
	// Contrarian slice — calls that ran against our score.
	ContrarianResolved int      `json:"contrarianResolved"`
	ContrarianHits     int      `json:"contrarianHits"`
	ContrarianHitRate  *float64 `json:"contrarianHitRate,omitempty"`
	// MinSample is the threshold below which the rates above are not meaningful.
	MinSample int `json:"minSample"`
}

// ResearchRevision is one recorded body edit. Append-only.
type ResearchRevision struct {
	ID        int64     `json:"id"`
	CallID    int64     `json:"callId"`
	Rev       int       `json:"rev"`
	Title     string    `json:"title"`
	BodyMD    string    `json:"bodyMd"`
	CreatedAt time.Time `json:"createdAt"`
}

// ---------------------------------------------------------------- social

// ResearchReactionKind is a one-tap signal. Kept as an enum rather than a
// boolean pair so a third kind never needs a schema change.
type ResearchReactionKind string

const (
	ReactionLike     ResearchReactionKind = "like"
	ResearchBookmark ResearchReactionKind = "bookmark"
)

// ResearchComment is one reader comment. Replies go exactly one level deep —
// deeper threads turn a research page into a forum.
type ResearchComment struct {
	ID         int64             `json:"id"`
	CallID     int64             `json:"callId"`
	ParentID   *int64            `json:"parentId,omitempty"`
	UserID     int64             `json:"userId"`
	AuthorName string            `json:"authorName,omitempty"`
	Body       string            `json:"body"`
	Deleted    bool              `json:"deleted"`
	CanEdit    bool              `json:"canEdit"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
	Replies    []ResearchComment `json:"replies,omitempty"`

	/*
		추천 (2026-09-14 사용자 요청 3).

		한 글 아래 스무 줄이 쌓이면 **읽을 만한 반론이 어느 것인지**를 읽는 사람이
		알 수 없다. 글 자체에는 추천이 있는데(`research_reactions`) 댓글에는 없어서
		좋은 반박이 스무 번째 줄에 그대로 묻혔다. 표는 `research_comment_likes`
		(migration 072)이고, 카운터는 비정규화하지 않는다 — 그 이유는 거기 머리글에.
	*/
	LikeCount int  `json:"likeCount"`
	Liked     bool `json:"liked"`

	/*
		글쓴이의 **누적 활동 수** — 발행한 리포트 + 남긴 댓글 (2026-09-14 요청 3).

		처음 보는 이름이 단 반론과 백 번 써 온 사람의 반론을 같은 무게로 읽게 두면,
		읽는 사람은 매번 글쓴이를 눌러 프로필을 확인해야 한다. 숫자를 **그대로**
		보내고 화면이 구간으로 접는다(100+·300+…) — 정확한 수는 읽는 사람에게
		아무 결정도 바꾸지 않으면서 그 사람의 활동량을 필요 이상으로 드러낸다.

		재료는 전부 공개 자료다(공개 리포트와 공개 댓글). 삭제한 댓글은 세지 않는다.
	*/
	AuthorActivity int `json:"authorActivity,omitempty"`
}

// ResearchRating is a reader's 3-axis score of the *argument*, not the outcome.
// Valley AI's single 82점 is a popularity number; splitting it forces the rater
// to say what was actually good, and locking it before the verdict stops it
// from decaying into outcome-scoring.
type ResearchRating struct {
	CallID    int64     `json:"callId"`
	UserID    int64     `json:"userId"`
	Clarity   int       `json:"clarity"`  // 논거 명확성 1~5
	Evidence  int       `json:"evidence"` // 데이터 근거 1~5
	Risk      int       `json:"risk"`     // 리스크 언급 1~5
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ResearchRatingSummary struct {
	Count    int     `json:"count"`
	Clarity  float64 `json:"clarity"`
	Evidence float64 `json:"evidence"`
	Risk     float64 `json:"risk"`
	// Average of the three axes, 1~5.
	Overall float64 `json:"overall"`
}

// ------------------------------------------------------------------- authors

// ResearchAuthorProfile is one author's public scorecard.
//
// There is no ranking type next to it on purpose: ranking authors against each
// other turns a report into a contest entry, and a contest needs prizes to keep
// running (docs/RESEARCH_CALL_DESIGN.md §15-1).
type ResearchAuthorProfile struct {
	UserID     int64           `json:"userId"`
	AuthorName string          `json:"authorName"`
	Summary    ResearchSummary `json:"summary"`
	Calls      []ResearchView  `json:"calls"`
}

// ResearchSymbolSummary powers the "이 종목 콜 N건" block on the stock page.
type ResearchSymbolSummary struct {
	Symbol   string `json:"symbol"`
	Total    int    `json:"total"`
	Buy      int    `json:"buy"`
	Sell     int    `json:"sell"`
	Hold     int    `json:"hold"`
	Open     int    `json:"open"`
	Resolved int    `json:"resolved"`
	Hits     int    `json:"hits"`
}
