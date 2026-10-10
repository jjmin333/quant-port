package service

// TableAccess controls what the admin DB console may do with a table.
type TableAccess string

const (
	// AccessFull allows insert / update / delete.
	AccessFull TableAccess = "full"
	// AccessReadOnly allows browsing only. Used for machine-managed tables
	// where hand-editing would desynchronise a cache or a running poller.
	AccessReadOnly TableAccess = "read-only"
	// AccessRedacted is read-only and additionally masks secret columns.
	AccessRedacted TableAccess = "redacted"
)

// TableGroup buckets tables in the console sidebar.
type TableGroup string

const (
	GroupUser      TableGroup = "user"
	GroupPortfolio TableGroup = "portfolio"
	GroupMarket    TableGroup = "market"
	GroupNews      TableGroup = "news"
	GroupReference TableGroup = "reference"
	GroupOps       TableGroup = "ops"
)

// TableSpec describes one table for the admin console. Only tables listed here
// are reachable — the table name in every generated statement comes from this
// catalog, never from the request, so a request can not name an arbitrary table.
type TableSpec struct {
	Name        string      `json:"name"`
	Group       TableGroup  `json:"group"`
	Label       string      `json:"label"`
	Purpose     string      `json:"purpose"`
	Access      TableAccess `json:"access"`
	PrimaryKey  []string    `json:"primaryKey"`
	DefaultSort string      `json:"defaultSort"`
	// RedactColumns are never returned in row payloads.
	RedactColumns []string `json:"redactColumns,omitempty"`
	// Note surfaces an operational warning in the UI.
	Note string `json:"note,omitempty"`
}

// adminTableCatalog is the single source of truth for the DB console.
var adminTableCatalog = []TableSpec{
	// ---- user / auth ----
	{
		Name: "users", Group: GroupUser, Label: "사용자",
		Purpose:    "가입 계정. 이메일·비밀번호 해시·표시명·권한(role)을 보관합니다. 관리자 판별에도 사용됩니다.",
		Access:     AccessRedacted,
		PrimaryKey: []string{"id"}, DefaultSort: "created_at DESC",
		RedactColumns: []string{"password_hash"},
		Note:          "비밀번호 해시는 표시되지 않습니다. 계정 삭제는 연결된 토큰·보유·워치리스트에 영향을 줍니다.",
	},
	{
		Name: "user_refresh_tokens", Group: GroupUser, Label: "리프레시 토큰",
		Purpose:    "로그인 세션 유지용 refresh 토큰(7일 창). 행을 지우면 해당 세션이 즉시 만료됩니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "created_at DESC",
		RedactColumns: []string{"token_hash"},
	},

	// ---- portfolio ----
	{
		Name: "user_holdings", Group: GroupPortfolio, Label: "수동 보유종목",
		Purpose:    "스크린샷 임포트/수동 입력으로 등록한 보유종목. 증권사 연동 없이 포트폴리오를 구성할 때 사용됩니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "updated_at DESC",
	},
	{
		Name: "watchlist_items", Group: GroupPortfolio, Label: "관심종목",
		Purpose:    "사용자별 워치리스트. 정렬 순서와 담은 시점 가격을 함께 보관합니다. 시세 폴러의 대상 종목을 결정합니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "sort_order ASC",
	},
	{
		Name: "journal_notes", Group: GroupPortfolio, Label: "메모장",
		Purpose:    "사용자별 메모. 종목 연동·고정(pinned) 여부를 함께 보관합니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "updated_at DESC",
	},
	{
		Name: "stamps", Group: GroupPortfolio, Label: "스탬프",
		Purpose:    "사용자가 찍은 순간의 불변 스냅샷(시세·점수·뉴스·공시·차트)과 가설(관점/확신도/검증기간). 스냅샷은 사후 수정 대상이 아니므로 읽기 전용입니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"id"}, DefaultSort: "stamped_at DESC",
	},
	{
		Name: "research_calls", Group: GroupPortfolio, Label: "투자 리포트",
		Purpose:    "공개 종목 논증 + 검증 가능한 예측. 의견 값(방향/기준가/목표가/손절가/기간/확신도)과 근거 스냅샷은 발행 후 불변이므로, 여기서는 관리자 숨김 처리만 가능하도록 읽기 전용입니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"id"}, DefaultSort: "published_at DESC",
	},
	{
		Name: "research_revisions", Group: GroupPortfolio, Label: "리포트 본문 수정 이력",
		Purpose:    "리포트 본문의 수정 이력(append-only). 이력을 고칠 수 있으면 '수정 이력 공개'가 의미를 잃으므로 읽기 전용입니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"id"}, DefaultSort: "created_at DESC",
	},
	{
		Name: "research_reports", Group: GroupPortfolio, Label: "리포트 신고",
		Purpose:    "리포트 신고 접수 기록. 관리자 숨김 처리의 근거가 됩니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"id"}, DefaultSort: "created_at DESC",
	},
	{
		Name: "research_reactions", Group: GroupPortfolio, Label: "리포트 반응",
		Purpose:    "리포트 좋아요·북마크. 좋아요 수를 이 테이블에서 집계하므로 행을 고치면 표시 수치가 함께 바뀝니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"id"}, DefaultSort: "created_at DESC",
	},
	{
		Name: "research_comments", Group: GroupPortfolio, Label: "리포트 댓글",
		Purpose:    "리포트 댓글(대댓글 1단계). 삭제는 soft delete라 대댓글이 고아가 되지 않습니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"id"}, DefaultSort: "created_at DESC",
	},
	{
		Name: "research_ratings", Group: GroupPortfolio, Label: "리포트 평가",
		Purpose:    "논거 3축 평가(명확성·데이터 근거·리스크 언급). 판정 확정 전에만 남길 수 있습니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"id"}, DefaultSort: "updated_at DESC",
	},

	// ---- market data ----
	{
		Name: "candle_bars", Group: GroupMarket, Label: "캔들 봉",
		Purpose:    "완성된 OHLCV. 차트의 과거 데이터 원본이며 가장 큰 테이블입니다. bar_interval별로 파티션되어 있고, 인트라데이는 오래될수록 1m→5m→10m→30m→60m으로 접힙니다(src_bars = 그 봉을 만든 1분봉 수).",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"symbol", "bar_interval", "bar_time"}, DefaultSort: "bar_time DESC",
		Note: "폴러/백필이 계속 upsert하고 롤업 배치가 계층을 옮깁니다. 수동 편집은 차트 정합성을 깨뜨릴 수 있어 읽기 전용입니다.",
	},
	{
		Name: "quote_snapshots", Group: GroupMarket, Label: "시세 스냅샷",
		Purpose:    "종목별 마지막 QuoteDetail JSON. 장 마감·Redis 미스 시 복구에 쓰입니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"symbol"}, DefaultSort: "updated_at DESC",
	},
	{
		Name: "orderbook_snapshots", Group: GroupMarket, Label: "호가 스냅샷",
		Purpose:    "종목별 마지막 호가창 JSON.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"symbol"}, DefaultSort: "updated_at DESC",
	},
	{
		Name: "trade_ticks", Group: GroupMarket, Label: "체결 틱",
		Purpose:    "종목당 최근 50건의 체결 내역. 터미널 체결창의 복구용 사본입니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"symbol", "trade_id"}, DefaultSort: "trade_time DESC",
	},
	{
		Name: "symbol_last_prices", Group: GroupMarket, Label: "최종가",
		Purpose:    "목록 화면용 종목별 최종 체결가. 배치 시세 갱신의 영속 사본입니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"symbol"}, DefaultSort: "updated_at DESC",
	},
	{
		Name: "market_indices", Group: GroupMarket, Label: "시장 지수",
		Purpose:    "KOSPI·S&P500·VIX 등 지수 스냅샷. 지수 폴러가 갱신합니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"index_id"}, DefaultSort: "updated_at DESC",
	},
	{
		Name: "market_fx_rates", Group: GroupMarket, Label: "환율",
		Purpose:    "USD/KRW 환율(매매기준·매수). 원화 환산 전반에 사용됩니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"pair"}, DefaultSort: "updated_at DESC",
	},
	{
		Name: "market_rankings", Group: GroupMarket, Label: "랭킹 스냅샷",
		Purpose:    "마켓 레이더의 국가/유형/기간별 랭킹 payload. 최신 1건씩 유지됩니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"market_country", "ranking_type", "duration"}, DefaultSort: "updated_at DESC",
	},
	{
		Name: "ranking_snapshot_entries", Group: GroupMarket, Label: "랭킹 이력",
		Purpose:    "랭킹 순위의 시계열 이력. 종목별 랭킹 추이 스파크라인에 사용됩니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "ranked_at DESC",
		Note: "무제한 증가하는 테이블입니다. 오래된 구간은 주기적으로 삭제해도 됩니다.",
	},

	// ---- news ----
	{
		Name: "news_articles", Group: GroupNews, Label: "뉴스 기사",
		Purpose:    "수집된 국내/미국 뉴스 본문 메타. 목록·검색·조회수의 원본입니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "published_at DESC",
	},
	{
		Name: "news_poll_keywords", Group: GroupNews, Label: "뉴스 키워드",
		Purpose:    "네이버/카카오 뉴스 수집에 사용할 검색 키워드와 활성 여부.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "sort_order ASC",
	},
	{
		Name: "news_poll_settings", Group: GroupNews, Label: "뉴스 수집 설정",
		Purpose:    "틱당 키워드 수 등 뉴스 폴러 파라미터. 단일 행입니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "id ASC",
	},
	{
		Name: "news_reader_translations", Group: GroupNews, Label: "본문 번역 캐시",
		Purpose:    "리더 모드에서 추출·번역한 기사 본문 캐시. 재번역 비용을 줄입니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"url_hash"}, DefaultSort: "updated_at DESC",
	},

	// ---- reference ----
	{
		Name: "stocks", Group: GroupReference, Label: "종목 마스터",
		Purpose:    "국내/미국 종목 카탈로그(심볼·이름·시장·자산유형). 검색과 종목명 표시의 기준입니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"symbol"}, DefaultSort: "symbol ASC",
	},
	{
		Name: "stock_fundamentals", Group: GroupReference, Label: "재무 지표",
		Purpose:    "DART/EDGAR에서 수집한 종목별 재무·밸류에이션 지표.",
		Access:     AccessFull,
		PrimaryKey: []string{"symbol"}, DefaultSort: "updated_at DESC",
	},

	{
		Name: "stock_daily_closes", Group: GroupMarket, Label: "일별 종가",
		Purpose: "candle_bars 1d에서 정규화한 종목 일별 종가 투영. 등락률과 점수·AI 요약이 함께 붙습니다.",
		// One-way projection from candle_bars: hand-editing a close would be
		// overwritten by the next backfill and would silently disagree with
		// the chart.
		Access:     AccessReadOnly,
		PrimaryKey: []string{"symbol", "trade_date"}, DefaultSort: "trade_date DESC",
		Note: "적재 전용(단방향 투영). 값 수정은 backfill-daily-closes 재실행으로 처리하세요.",
	},
	{
		Name: "index_daily_closes", Group: GroupMarket, Label: "지수 일별 종가",
		Purpose:    "지수별 일별 종가 이력. market_indices는 최신 1행만 보관하므로 이력은 이 표에만 있습니다.",
		Access:     AccessReadOnly,
		PrimaryKey: []string{"index_id", "trade_date"}, DefaultSort: "trade_date DESC",
	},
	{
		Name: "stock_scores", Group: GroupMarket, Label: "종목 종합 점수",
		Purpose: "일자별 종합 점수와 축별 분해(breakdown)·근거 수치(inputs). 서버 배치가 계산합니다.",
		// Editing a score by hand would break the audit trail the inputs blob
		// exists to provide.
		Access:     AccessReadOnly,
		PrimaryKey: []string{"symbol", "trade_date"}, DefaultSort: "trade_date DESC",
		Note: "배치 산출물. 재계산은 관리자 > 점수 관리에서 실행하세요.",
	},

	// ---- ops ----
	{
		Name: "market_data_settings", Group: GroupOps, Label: "마켓 폴링 설정",
		Purpose:    "랭킹·참조데이터 폴링 주기 설정. 단일 행이며 관리자 화면에서 조정합니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "id ASC",
	},
	{
		Name: "llm_settings", Group: GroupOps, Label: "LLM 설정",
		Purpose:    "AI 요약 파일럿 종목 목록. 단일 행이며 관리자 > LLM 모델 관리에서 조정합니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "id ASC",
	},
	{
		Name: "score_settings", Group: GroupOps, Label: "점수 가중치 (전역)",
		Purpose:    "종합 점수의 축별 기본 가중치와 커버리지 하한·수축 지점. 단일 행입니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"id"}, DefaultSort: "id ASC",
		Note: "변경 후 점수에 반영하려면 관리자 > 점수 관리에서 재계산이 필요합니다.",
	},
	{
		Name: "score_symbol_weights", Group: GroupOps, Label: "점수 가중치 (종목별)",
		Purpose:    "전역 가중치를 대체하는 종목별 오버라이드. 행이 없으면 전역값을 따릅니다.",
		Access:     AccessFull,
		PrimaryKey: []string{"symbol"}, DefaultSort: "updated_at DESC",
	},

	{
		Name: "batch_job_runs", Group: GroupOps, Label: "배치 실행 이력",
		Purpose: "적재·점수 배치의 실행 기록(상태·소요·결과 JSON). 관리자 > 배치 관리에서 조회·실행합니다.",
		// Machine-written audit trail; editing it would only falsify history.
		Access:     AccessReadOnly,
		PrimaryKey: []string{"id"}, DefaultSort: "started_at DESC",
	},
}

func catalogByName(name string) (TableSpec, bool) {
	for _, spec := range adminTableCatalog {
		if spec.Name == name {
			return spec, true
		}
	}
	return TableSpec{}, false
}

// GroupLabels maps group ids to display labels for the console sidebar.
var GroupLabels = map[TableGroup]string{
	GroupUser:      "사용자 · 인증",
	GroupPortfolio: "포트폴리오",
	GroupMarket:    "시장 데이터",
	GroupNews:      "뉴스",
	GroupReference: "참조 데이터",
	GroupOps:       "운영 설정",
}
